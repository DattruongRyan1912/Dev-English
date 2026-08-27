#!/usr/bin/env bash
set -euo pipefail

# This harness is intentionally destructive only to the explicitly named,
# disposable Compose project it owns. It is not a production recovery tool.

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
expected_compose_file="$repo_root/infra/docker-compose.ci.yml"
compose_file="${COMPOSE_FILE:-}"
project_name="${COMPOSE_PROJECT_NAME:-}"
postgres_service="${POSTGRES_SERVICE:-}"
postgres_user="${POSTGRES_USER:-}"
postgres_db="${POSTGRES_DB:-}"
ci_port="${DEVENGLISH_CI_PORT:-}"
migrations_dir="$repo_root/infra/migrations"
restore_db="devenglish_restore"
temp_root=""
dump_file=""
no_env_file=""
cleanup_compose=0
database_service_ready=0
restore_database_created=0

fail() {
  printf '%s\n' "$1" >&2
  exit "$2"
}

if [[ "${DEVENGLISH_BACKUP_RESTORE_DISPOSABLE:-}" != "1" ]]; then
  fail 'Set DEVENGLISH_BACKUP_RESTORE_DISPOSABLE=1 to opt into the disposable backup/restore harness.' 2
fi

if [[ -z "$compose_file" || -z "$project_name" || -z "$postgres_service" ||
  -z "$postgres_user" || -z "$postgres_db" || -z "$ci_port" ]]; then
  fail 'COMPOSE_FILE, COMPOSE_PROJECT_NAME, POSTGRES_SERVICE, POSTGRES_USER, POSTGRES_DB and DEVENGLISH_CI_PORT are required.' 2
fi

if [[ "$compose_file" != /* ]]; then
  compose_file="$repo_root/$compose_file"
fi
if [[ "$compose_file" != "$expected_compose_file" ]]; then
  fail "Only the repository disposable Compose file is allowed: $expected_compose_file" 2
fi
if [[ ! -f "$compose_file" ]]; then
  fail "Compose file does not exist: $compose_file" 2
fi
if [[ ! "$project_name" =~ ^devenglish-backup-restore-[a-z0-9][a-z0-9-]*$ ||
  ${#project_name} -gt 63 ]]; then
  fail 'COMPOSE_PROJECT_NAME must be a unique devenglish-backup-restore-* project name.' 2
fi
if [[ "$postgres_service" != "postgres" || "$postgres_user" != "devenglish" ||
  "$postgres_db" != "devenglish" ]]; then
  fail 'The harness only accepts the disposable CI postgres service/database configuration.' 2
fi
if [[ ! "$ci_port" =~ ^[0-9]+$ ]] || (( ci_port < 1024 || ci_port > 65535 || ci_port == 5432 )); then
  fail 'DEVENGLISH_CI_PORT must be an explicit TCP port from 1024 to 65535 other than 5432.' 2
fi

expected_migrations="
000_schema_migrations.sql
001_initial.sql
002_provider_secrets.sql
003_platform_foundation.sql
004_platform_safety.sql
005_knowledge.sql
006_work.sql
"
while IFS= read -r migration_name; do
  [[ -z "$migration_name" ]] && continue
  if [[ ! -f "$migrations_dir/$migration_name" ]]; then
    fail "Missing migration: $migrations_dir/$migration_name" 1
  fi
done <<< "$expected_migrations"

shopt -s nullglob
for migration_path in "$migrations_dir"/*.sql; do
  migration_name="$(basename "$migration_path")"
  case "$migration_name" in
    000_schema_migrations.sql|001_initial.sql|002_provider_secrets.sql|\
    003_platform_foundation.sql|004_platform_safety.sql|005_knowledge.sql|\
    006_work.sql)
      ;;
    *)
      fail "Unexpected migration in the recovery chain: $migration_name" 1
      ;;
  esac
done

compose_run() {
  DEVENGLISH_CI_PORT="$ci_port" COMPOSE_PROJECT_NAME="$project_name" \
    docker compose --project-name "$project_name" --file "$compose_file" "$@"
}

psql_exec_db() {
  local database="$1"
  shift
  compose_run exec -T "$postgres_service" psql \
    -X \
    -v ON_ERROR_STOP=1 \
    -U "$postgres_user" \
    -d "$database" \
    "$@"
}

psql_query_db() {
  local database="$1"
  local query="$2"
  shift 2
  psql_exec_db "$database" "$@" -Atq <<< "$query"
}

user_relation_count() {
  local database="$1"
  psql_query_db "$database" "
    SELECT count(*)
    FROM pg_class AS relation
    JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
    WHERE namespace.nspname NOT LIKE 'pg_%'
      AND namespace.nspname <> 'information_schema'
      AND relation.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')"
}

require_blank_database() {
  local database="$1"
  local relation_count
  relation_count="$(user_relation_count "$database")"
  if [[ "$relation_count" != "0" ]]; then
    fail "Database $database must be blank before the recovery operation; found $relation_count user relations." 1
  fi
}

require_exact_line() {
  local output="$1"
  local expected_line="$2"
  local line
  while IFS= read -r line; do
    if [[ "$line" == "$expected_line" ]]; then
      return 0
    fi
  done <<< "$output"
  fail "Expected exact migration output line was not found: $expected_line" 1
}

run_migrate() {
  local database="$1"
  COMPOSE_FILE="$compose_file" \
    ENV_FILE="$no_env_file" \
    COMPOSE_PROJECT_NAME="$project_name" \
    DEVENGLISH_CI_PORT="$ci_port" \
    POSTGRES_SERVICE="$postgres_service" \
    POSTGRES_USER="$postgres_user" \
    POSTGRES_DB="$database" \
    MIGRATIONS_DIR="$migrations_dir" \
    "$repo_root/scripts/db_migrate.sh"
}

cleanup() {
  local exit_code="$?"
  local cleanup_failed=0

  if (( restore_database_created == 1 && database_service_ready == 1 )); then
    if ! psql_exec_db "$postgres_db" -c "DROP DATABASE IF EXISTS \"$restore_db\"" >/dev/null 2>&1; then
      printf 'Failed to remove restore database %s.\n' "$restore_db" >&2
      cleanup_failed=1
    fi
  fi

  if (( cleanup_compose == 1 )); then
    if ! compose_run down --volumes --remove-orphans >/dev/null 2>&1; then
      printf 'Failed to clean disposable Compose project %s.\n' "$project_name" >&2
      cleanup_failed=1
    fi
  fi

  if [[ -n "$temp_root" && -d "$temp_root" ]]; then
    if ! rm -rf -- "$temp_root"; then
      printf 'Failed to remove temporary recovery directory.\n' >&2
      cleanup_failed=1
    fi
  fi

  if (( cleanup_failed == 1 && exit_code == 0 )); then
    exit_code=1
  fi
  exit "$exit_code"
}

service_list="$(compose_run config --services)"
if [[ "$service_list" != "$postgres_service" ]]; then
  fail 'Unexpected Compose services for the disposable recovery harness.' 1
fi
existing_containers="$(compose_run ps -aq)"
if [[ -n "$existing_containers" ]]; then
  fail "Disposable Compose project already has resources: $project_name" 1
fi

temp_root="$(mktemp -d)"
dump_file="$temp_root/devenglish-source.dump"
no_env_file="$temp_root/no-env"
trap cleanup EXIT

cleanup_compose=1
compose_run up -d >/dev/null 2>&1

ready=0
for attempt in $(seq 1 90); do
  if compose_run ps --status running --services | grep -Fxq "$postgres_service" &&
    compose_run exec -T "$postgres_service" pg_isready -U "$postgres_user" -d "$postgres_db" >/dev/null 2>&1 &&
    psql_exec_db "$postgres_db" -Atqc 'SELECT 1' >/dev/null 2>&1; then
    ready=1
    database_service_ready=1
    break
  fi
  sleep 1
done
if (( ready == 0 )); then
  fail 'Disposable postgres service did not become healthy within 90 seconds.' 1
fi

require_blank_database "$postgres_db"

first_migration_output="$(run_migrate "$postgres_db" 2>/dev/null)"
while IFS= read -r migration_name; do
  [[ -z "$migration_name" ]] && continue
  require_exact_line "$first_migration_output" "applied $migration_name"
done <<< "$expected_migrations"

user_a='backup-restore-user-a'
user_b='backup-restore-user-b'
user_c='backup-restore-user-c'
legacy_context_a='backup-restore-context-a'
legacy_context_b='backup-restore-context-b'
legacy_source_a='backup-restore-source-a'
legacy_source_b='backup-restore-source-b'
workspace_a1='backup-restore-workspace-a1'
workspace_a2='backup-restore-workspace-a2'
workspace_c='backup-restore-workspace-c'
knowledge_source_c='backup-restore-knowledge-source-c'
source_item_c='backup-restore-source-item-c'
revision_c='backup-restore-revision-c'
chunk_c='backup-restore-chunk-c'
topic_c='backup-restore-topic-c'
claim_c='backup-restore-claim-c'
evidence_c='backup-restore-evidence-c'
project_c='backup-restore-project-c'
task_c='backup-restore-task-c'
decision_c='backup-restore-decision-c'
history_c='backup-restore-history-c'
idempotency_c='backup-restore-idempotency-c'
challenge_c='backup-restore-challenge-c'
receipt_c='backup-restore-receipt-c'
action_hash_c='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
request_hash_c='bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'

if ! psql_exec_db "$postgres_db" \
  --set=user_a="$user_a" \
  --set=user_b="$user_b" \
  --set=user_c="$user_c" \
  --set=legacy_context_a="$legacy_context_a" \
  --set=legacy_context_b="$legacy_context_b" \
  --set=legacy_source_a="$legacy_source_a" \
  --set=legacy_source_b="$legacy_source_b" \
  --set=workspace_a1="$workspace_a1" \
  --set=workspace_a2="$workspace_a2" \
  --set=workspace_c="$workspace_c" \
  --set=knowledge_source_c="$knowledge_source_c" \
  --set=source_item_c="$source_item_c" \
  --set=revision_c="$revision_c" \
  --set=chunk_c="$chunk_c" \
  --set=topic_c="$topic_c" \
  --set=claim_c="$claim_c" \
  --set=evidence_c="$evidence_c" \
  --set=project_c="$project_c" \
  --set=task_c="$task_c" \
  --set=decision_c="$decision_c" \
  --set=history_c="$history_c" \
  --set=idempotency_c="$idempotency_c" \
  --set=challenge_c="$challenge_c" \
  --set=receipt_c="$receipt_c" \
  --set=action_hash_c="$action_hash_c" \
  --set=request_hash_c="$request_hash_c" \
  --single-transaction > /dev/null 2> "$temp_root/seed.err" <<'SQL'
INSERT INTO users (id, display_name, cefr, created_at)
VALUES
  (:'user_a', 'Backup restore user A', 'B1', TIMESTAMPTZ '2026-01-01 00:00:00+00'),
  (:'user_b', 'Backup restore user B', 'A2', TIMESTAMPTZ '2026-01-01 00:01:00+00'),
  (:'user_c', 'Backup restore user C', 'B2', TIMESTAMPTZ '2026-01-01 00:02:00+00');

INSERT INTO work_context (id, user_id, source_type, source_url, title, content, domain, created_at)
VALUES
  (:'legacy_context_a', :'user_a', 'manual', 'https://example.invalid/recovery/a',
    'Legacy recovery context A', 'synthetic legacy payload A', 'recovery-a',
    TIMESTAMPTZ '2026-01-02 00:00:00+00'),
  (:'legacy_context_b', :'user_b', 'github', 'https://example.invalid/recovery/b',
    'Legacy recovery context B', 'synthetic legacy payload B', 'recovery-b',
    TIMESTAMPTZ '2026-01-02 00:01:00+00');

INSERT INTO imported_sources (id, user_id, source_type, title, content, created_at)
VALUES
  (:'legacy_source_a', :'user_a', 'drive', 'Legacy recovery source A',
    'synthetic imported payload A', TIMESTAMPTZ '2026-01-03 00:00:00+00'),
  (:'legacy_source_b', :'user_b', 'github', 'Legacy recovery source B',
    'synthetic imported payload B', TIMESTAMPTZ '2026-01-03 00:01:00+00');

INSERT INTO workspaces (id, owner_user_id, name, slug, description, metadata, version, created_at, updated_at)
VALUES
  (:'workspace_a1', :'user_a', 'Backup restore workspace A1', 'backup-restore-a1',
    'synthetic ambiguous topology', '{"legacy":true}'::jsonb, 1,
    TIMESTAMPTZ '2026-01-04 00:00:00+00', TIMESTAMPTZ '2026-01-04 00:00:00+00'),
  (:'workspace_a2', :'user_a', 'Backup restore workspace A2', 'backup-restore-a2',
    'synthetic ambiguous topology', '{"legacy":true}'::jsonb, 1,
    TIMESTAMPTZ '2026-01-04 00:01:00+00', TIMESTAMPTZ '2026-01-04 00:01:00+00'),
  (:'workspace_c', :'user_c', 'Backup restore workspace C', 'backup-restore-c',
    'synthetic canonical fixture', '{"canonical":true}'::jsonb, 2,
    TIMESTAMPTZ '2026-01-04 00:02:00+00', TIMESTAMPTZ '2026-01-04 00:03:00+00');

INSERT INTO knowledge_sources
  (id, workspace_id, kind, name, uri, metadata, version, created_at, updated_at)
VALUES
  (:'knowledge_source_c', :'workspace_c', 'manual', 'Recovery knowledge source',
    'https://example.invalid/recovery/knowledge', '{"fixture":true}'::jsonb, 1,
    TIMESTAMPTZ '2026-01-05 00:00:00+00', TIMESTAMPTZ '2026-01-05 00:00:00+00');

INSERT INTO source_items
  (id, workspace_id, source_id, external_id, title, uri, mime_type,
   current_revision_id, version, created_at, updated_at)
VALUES
  (:'source_item_c', :'workspace_c', :'knowledge_source_c', 'recovery-item-1',
    'Recovery source item', 'https://example.invalid/recovery/knowledge/item-1',
    'text/plain', NULL, 2, TIMESTAMPTZ '2026-01-05 00:01:00+00',
    TIMESTAMPTZ '2026-01-05 00:03:00+00');

INSERT INTO source_revisions
  (id, workspace_id, source_item_id, revision_key, content_hash, content_type,
   source_uri, content, modified_at, ingested_at)
VALUES
  (:'revision_c', :'workspace_c', :'source_item_c', 'revision-1',
    'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
    'text/plain', 'https://example.invalid/recovery/knowledge/item-1',
    'The recovery fixture preserves source evidence.',
    TIMESTAMPTZ '2026-01-05 00:02:00+00', TIMESTAMPTZ '2026-01-05 00:02:00+00');

UPDATE source_items
SET current_revision_id = :'revision_c',
    updated_at = TIMESTAMPTZ '2026-01-05 00:03:00+00'
WHERE workspace_id = :'workspace_c' AND id = :'source_item_c';

INSERT INTO knowledge_chunks
  (id, workspace_id, revision_id, ordinal, chunk_text, token_count, created_at)
VALUES
  (:'chunk_c', :'workspace_c', :'revision_c', 0,
    'The recovery fixture preserves source evidence.', 7,
    TIMESTAMPTZ '2026-01-05 00:04:00+00');

INSERT INTO topics
  (id, workspace_id, name, description, version, created_at, updated_at)
VALUES
  (:'topic_c', :'workspace_c', 'Data recovery', 'Synthetic recovery topic', 1,
    TIMESTAMPTZ '2026-01-05 00:05:00+00', TIMESTAMPTZ '2026-01-05 00:05:00+00');

INSERT INTO knowledge_claims
  (id, workspace_id, topic_id, statement, certainty, freshness, version, created_at, updated_at)
VALUES
  (:'claim_c', :'workspace_c', :'topic_c',
    'The recovery fixture has current evidence.',
    'canonical', 'current', 1, TIMESTAMPTZ '2026-01-05 00:06:00+00',
    TIMESTAMPTZ '2026-01-05 00:06:00+00');

INSERT INTO claim_evidence
  (id, workspace_id, claim_id, source_revision_id, chunk_id, locator, quote, freshness, created_at)
VALUES
  (:'evidence_c', :'workspace_c', :'claim_c', :'revision_c', :'chunk_c',
    'item-1#chunk-0', 'The recovery fixture preserves source evidence.',
    'current', TIMESTAMPTZ '2026-01-05 00:07:00+00');

INSERT INTO projects
  (id, workspace_id, owner_user_id, name, description, status, origin, version, created_at, updated_at)
VALUES
  (:'project_c', :'workspace_c', :'user_c', 'Recovery project',
    'Synthetic project for backup and restore.', 'active', 'canonical', 2,
    TIMESTAMPTZ '2026-01-06 00:00:00+00', TIMESTAMPTZ '2026-01-06 00:01:00+00');

INSERT INTO tasks
  (id, workspace_id, owner_user_id, project_id, title, description, status, priority,
   origin, version, created_at, updated_at)
VALUES
  (:'task_c', :'workspace_c', :'user_c', :'project_c', 'Verify recovery evidence',
    'Synthetic task with scoped ownership.', 'in_progress', 'high', 'canonical', 2,
    TIMESTAMPTZ '2026-01-06 00:02:00+00', TIMESTAMPTZ '2026-01-06 00:03:00+00');

INSERT INTO decisions
  (id, workspace_id, owner_user_id, project_id, title, context, outcome, rationale,
   status, origin, version, created_at, updated_at)
VALUES
  (:'decision_c', :'workspace_c', :'user_c', :'project_c', 'Keep recovery evidence',
    'The fixture represents a reviewed source.', 'Retain the source revision and evidence link.',
    'Canonical facts require current evidence.', 'accepted', 'canonical', 1,
    TIMESTAMPTZ '2026-01-06 00:04:00+00', TIMESTAMPTZ '2026-01-06 00:04:00+00');

INSERT INTO work_history
  (id, workspace_id, actor_user_id, entity_type, entity_id, action,
   from_version, to_version, idempotency_key, before_snapshot, after_snapshot, created_at)
VALUES
  (:'history_c', :'workspace_c', :'user_c', 'task', :'task_c', 'updated', 1, 2,
    'backup-restore-task-update-1',
    '{"status":"todo","version":1}'::jsonb,
    '{"status":"in_progress","version":2}'::jsonb,
    TIMESTAMPTZ '2026-01-06 00:05:00+00');

INSERT INTO work_idempotency
  (workspace_id, user_id, idempotency_key, operation, request_hash, entity_type, entity_id, result, created_at)
VALUES
  (:'workspace_c', :'user_c', :'idempotency_c', 'task.update', :'request_hash_c',
    'task', :'task_c', '{"version":2,"status":"in_progress"}'::jsonb,
    TIMESTAMPTZ '2026-01-06 00:06:00+00');

INSERT INTO action_challenges
  (id, workspace_id, user_id, action, target_type, target_id, action_hash, prompt,
   parameters, status, expires_at, consumed_at, version, created_at, updated_at)
VALUES
  (:'challenge_c', :'workspace_c', :'user_c', 'github.issue.create', 'issue',
    'synthetic-issue-1', :'action_hash_c', 'Confirm synthetic issue creation',
    '{"repository":"synthetic/recovery","title":"Recovery fixture"}'::jsonb,
    'consumed', TIMESTAMPTZ '2026-01-06 00:10:00+00',
    TIMESTAMPTZ '2026-01-06 00:09:00+00', 2,
    TIMESTAMPTZ '2026-01-06 00:07:00+00', TIMESTAMPTZ '2026-01-06 00:09:00+00');

INSERT INTO action_receipts
  (id, challenge_id, workspace_id, user_id, action, target_type, target_id,
   idempotency_key, status, message, output, completed_at, created_at, updated_at)
VALUES
  (:'receipt_c', :'challenge_c', :'workspace_c', :'user_c', 'github.issue.create',
    'issue', 'synthetic-issue-1', 'backup-restore-issue-create-1', 'accepted',
    'Synthetic recovery receipt.',
    '{"provider":"synthetic","remote_id":"issue-1"}'::jsonb,
    TIMESTAMPTZ '2026-01-06 00:09:00+00', TIMESTAMPTZ '2026-01-06 00:08:00+00',
    TIMESTAMPTZ '2026-01-06 00:09:00+00');
SQL
then
  fail 'Synthetic recovery seed failed.' 1
fi

snapshot() {
  local database="$1"
  psql_exec_db "$database" -Atq <<'SQL'
WITH snapshots(label, row_count, payload) AS (
  SELECT 'action_challenges', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, user_id, action, target_type, target_id, action_hash,
      prompt, parameters::text, status,
      to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY id), '')
  FROM action_challenges
  UNION ALL
  SELECT 'action_receipts', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, challenge_id, workspace_id, user_id, action, target_type, target_id,
      idempotency_key, status, output::text), E'\n' ORDER BY id), '')
  FROM action_receipts
  UNION ALL
  SELECT 'claim_evidence', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, claim_id, source_revision_id, coalesce(chunk_id, ''),
      locator, quote, freshness), E'\n' ORDER BY id), '')
  FROM claim_evidence
  UNION ALL
  SELECT 'decisions', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, owner_user_id, coalesce(project_id, ''), title, context,
      outcome, rationale, status, origin, version), E'\n'
      ORDER BY workspace_id, owner_user_id, id), '')
  FROM decisions
  UNION ALL
  SELECT 'imported_sources', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s',
      id, user_id, source_type, title, content,
      to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY id), '')
  FROM imported_sources
  UNION ALL
  SELECT 'knowledge_claims', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, coalesce(topic_id, ''), statement, certainty, freshness,
      version, to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY id), '')
  FROM knowledge_claims
  UNION ALL
  SELECT 'knowledge_chunks', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, revision_id, ordinal, chunk_text, token_count,
      coalesce(search_vector::text, '')), E'\n' ORDER BY id), '')
  FROM knowledge_chunks
  UNION ALL
  SELECT 'knowledge_sources', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, kind, name, uri, metadata::text, version),
      E'\n' ORDER BY id), '')
  FROM knowledge_sources
  UNION ALL
  SELECT 'projects', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, owner_user_id, name, description, status, origin, version,
      coalesce(deleted_at::text, ''), to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY workspace_id, owner_user_id, id), '')
  FROM projects
  UNION ALL
  SELECT 'schema_migrations', count(*),
    coalesce(string_agg(format('%s|%s', version,
      to_char(applied_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY version), '')
  FROM schema_migrations
  UNION ALL
  SELECT 'source_items', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, source_id, external_id, title, uri, mime_type,
      coalesce(current_revision_id, '')), E'\n' ORDER BY id), '')
  FROM source_items
  UNION ALL
  SELECT 'source_revisions', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, source_item_id, revision_key, content_hash, content_type,
      source_uri, content, to_char(ingested_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY id), '')
  FROM source_revisions
  UNION ALL
  SELECT 'tasks', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, owner_user_id, coalesce(project_id, ''), title, description,
      status, priority, origin, version, coalesce(deleted_at::text, '')),
      E'\n' ORDER BY workspace_id, owner_user_id, id), '')
  FROM tasks
  UNION ALL
  SELECT 'topics', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s',
      id, workspace_id, name, description, coalesce(parent_id, ''), version),
      E'\n' ORDER BY id), '')
  FROM topics
  UNION ALL
  SELECT 'users', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s',
      id, display_name, cefr,
      to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY id), '')
  FROM users
  UNION ALL
  SELECT 'work_context', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s',
      id, user_id, source_type, source_url, title, content, domain,
      to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
      E'\n' ORDER BY id), '')
  FROM work_context
  UNION ALL
  SELECT 'work_history', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
      id, workspace_id, coalesce(actor_user_id, ''), entity_type, entity_id, action,
      from_version, to_version, idempotency_key, after_snapshot::text),
      E'\n' ORDER BY id), '')
  FROM work_history
  UNION ALL
  SELECT 'work_idempotency', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s',
      workspace_id, user_id, idempotency_key, operation, request_hash, entity_type,
      entity_id, result::text), E'\n' ORDER BY workspace_id, user_id, idempotency_key), '')
  FROM work_idempotency
  UNION ALL
  SELECT 'workspaces', count(*),
    coalesce(string_agg(format('%s|%s|%s|%s|%s|%s|%s|%s',
      id, owner_user_id, name, slug, description, metadata::text, version,
      coalesce(deleted_at::text, '')), E'\n' ORDER BY id), '')
  FROM workspaces
)
SELECT label || '|' || row_count::text || '|' || md5(payload)
FROM snapshots
ORDER BY label;
SQL
}

schema_versions() {
  local database="$1"
  psql_query_db "$database" \
    "SELECT string_agg(version, ',' ORDER BY version) FROM schema_migrations"
}

assert_true() {
  local assertion_name="$1"
  local result="$2"
  if [[ "$result" != "t" ]]; then
    fail "Assertion failed: $assertion_name" 1
  fi
}

assert_data_invariants() {
  local database="$1"
  assert_true "$database user A owns exactly two active workspaces" \
    "$(psql_query_db "$database" \
      "SELECT count(*) = 2 FROM workspaces WHERE owner_user_id = :'user_a' AND deleted_at IS NULL" \
      --set=user_a="$user_a")"
  assert_true "$database user B owns zero active workspaces" \
    "$(psql_query_db "$database" \
      "SELECT count(*) = 0 FROM workspaces WHERE owner_user_id = :'user_b' AND deleted_at IS NULL" \
      --set=user_b="$user_b")"
  assert_true "$database user C owns the canonical workspace" \
    "$(psql_query_db "$database" \
      "SELECT count(*) = 1 FROM workspaces WHERE id = :'workspace_c' AND owner_user_id = :'user_c' \
        AND deleted_at IS NULL" \
      --set=workspace_c="$workspace_c" --set=user_c="$user_c")"
  assert_true "$database legacy rows remain user-scoped and unmapped" \
    "$(psql_query_db "$database" \
      "SELECT (SELECT count(*) FROM work_context WHERE user_id IN (:'user_a', :'user_b')) = 2
         AND (SELECT count(*) FROM imported_sources WHERE user_id IN (:'user_a', :'user_b')) = 2
         AND (SELECT count(*) FROM work_context WHERE user_id = :'user_c') = 0
         AND (SELECT count(*) FROM imported_sources WHERE user_id = :'user_c') = 0" \
      --set=user_a="$user_a" --set=user_b="$user_b" --set=user_c="$user_c")"
  assert_true "$database Knowledge evidence link is scoped and current" \
    "$(psql_query_db "$database" \
      "SELECT count(*) = 1
         FROM claim_evidence AS evidence
         JOIN knowledge_claims AS claim ON claim.workspace_id = evidence.workspace_id
           AND claim.id = evidence.claim_id
         JOIN source_revisions AS revision ON revision.workspace_id = evidence.workspace_id
           AND revision.id = evidence.source_revision_id
         JOIN knowledge_chunks AS chunk ON chunk.workspace_id = evidence.workspace_id
           AND chunk.id = evidence.chunk_id AND chunk.revision_id = evidence.source_revision_id
        WHERE evidence.id = :'evidence_c' AND evidence.workspace_id = :'workspace_c'
          AND claim.certainty = 'canonical'
          AND claim.freshness = 'current'
          AND evidence.freshness = 'current'" \
      --set=evidence_c="$evidence_c" --set=workspace_c="$workspace_c")"
  assert_true "$database Work history and idempotency are scoped" \
    "$(psql_query_db "$database" \
      "SELECT (SELECT count(*) FROM work_history
                WHERE id = :'history_c' AND workspace_id = :'workspace_c'
                  AND actor_user_id = :'user_c' AND entity_id = :'task_c') = 1
         AND (SELECT count(*) FROM work_idempotency
                WHERE workspace_id = :'workspace_c' AND user_id = :'user_c'
                  AND idempotency_key = :'idempotency_c'
                  AND entity_id = :'task_c') = 1" \
      --set=history_c="$history_c" --set=workspace_c="$workspace_c" \
      --set=user_c="$user_c" --set=task_c="$task_c" --set=idempotency_c="$idempotency_c")"
  assert_true "$database action challenge and receipt share the workspace owner" \
    "$(psql_query_db "$database" \
      "SELECT count(*) = 1
         FROM action_challenges AS challenge
         JOIN action_receipts AS receipt ON receipt.challenge_id = challenge.id
           AND receipt.workspace_id = challenge.workspace_id
           AND receipt.user_id = challenge.user_id
         JOIN workspaces AS workspace ON workspace.id = challenge.workspace_id
           AND workspace.owner_user_id = challenge.user_id
        WHERE challenge.id = :'challenge_c' AND receipt.id = :'receipt_c'
          AND challenge.workspace_id = :'workspace_c' AND challenge.user_id = :'user_c'
          AND challenge.status = 'consumed' AND receipt.status = 'accepted'" \
      --set=challenge_c="$challenge_c" --set=receipt_c="$receipt_c" \
      --set=workspace_c="$workspace_c" --set=user_c="$user_c")"
}

required_objects() {
  local database="$1"
  psql_exec_db "$database" -Atq <<'SQL'
WITH required(kind, object_name) AS (
  VALUES
    ('relation', 'schema_migrations'),
    ('relation', 'users'),
    ('relation', 'skill_profiles'),
    ('relation', 'learning_state'),
    ('relation', 'missions'),
    ('relation', 'mission_attempts'),
    ('relation', 'mistakes'),
    ('relation', 'vocabulary'),
    ('relation', 'vocabulary_reviews'),
    ('relation', 'skill_history'),
    ('relation', 'diagnostic_results'),
    ('relation', 'user_settings'),
    ('relation', 'work_context'),
    ('relation', 'imported_sources'),
    ('relation', 'mistake_occurrences'),
    ('relation', 'evaluations'),
    ('relation', 'writing_attempts'),
    ('relation', 'conversations'),
    ('relation', 'messages'),
    ('relation', 'speaking_sessions'),
    ('relation', 'ai_usage'),
    ('relation', 'content_items'),
    ('relation', 'prompt_versions'),
    ('relation', 'provider_secrets'),
    ('relation', 'workspaces'),
    ('relation', 'module_states'),
    ('relation', 'audit_events'),
    ('relation', 'background_jobs'),
    ('relation', 'action_challenges'),
    ('relation', 'action_receipts'),
    ('relation', 'knowledge_sources'),
    ('relation', 'source_items'),
    ('relation', 'source_revisions'),
    ('relation', 'knowledge_chunks'),
    ('relation', 'topics'),
    ('relation', 'knowledge_claims'),
    ('relation', 'claim_evidence'),
    ('relation', 'projects'),
    ('relation', 'tasks'),
    ('relation', 'decisions'),
    ('relation', 'work_history'),
    ('relation', 'work_idempotency'),
    ('index', 'knowledge_sources_active_uri_idx'),
    ('index', 'source_items_active_external_idx'),
    ('index', 'source_revisions_item_ingested_idx'),
    ('index', 'knowledge_chunks_search_idx'),
    ('index', 'knowledge_chunks_embedding_idx'),
    ('index', 'topics_active_name_idx'),
    ('index', 'knowledge_claims_workspace_freshness_idx'),
    ('index', 'claim_evidence_claim_idx'),
    ('index', 'projects_scope_created_idx'),
    ('index', 'projects_scope_deleted_idx'),
    ('index', 'tasks_scope_created_idx'),
    ('index', 'tasks_scope_project_idx'),
    ('index', 'tasks_scope_deleted_idx'),
    ('index', 'decisions_scope_created_idx'),
    ('index', 'decisions_scope_project_idx'),
    ('index', 'decisions_scope_deleted_idx'),
    ('index', 'work_history_scope_entity_idx'),
    ('index', 'work_history_scope_created_idx'),
    ('index', 'work_idempotency_scope_created_idx'),
    ('index', 'action_challenges_user_status_idx'),
    ('index', 'action_challenges_workspace_expiry_idx'),
    ('index', 'action_challenges_pending_hash_uidx'),
    ('index', 'action_receipts_workspace_created_idx'),
    ('index', 'action_receipts_user_created_idx'),
    ('index', 'action_receipts_active_idempotency_uidx'),
    ('trigger', 'source_revisions_immutable'),
    ('trigger', 'knowledge_claims_evidence_required'),
    ('trigger', 'claim_evidence_preserves_canonical'),
    ('trigger', 'work_history_append_only_guard')
),
checked AS (
  SELECT kind, object_name,
    CASE
      WHEN kind IN ('relation', 'index')
        THEN to_regclass(format('public.%I', object_name)) IS NOT NULL
      WHEN kind = 'trigger'
        THEN EXISTS (
          SELECT 1
          FROM pg_trigger AS trigger_row
          JOIN pg_class AS relation ON relation.oid = trigger_row.tgrelid
          JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
          WHERE namespace.nspname = 'public'
            AND trigger_row.tgname = object_name
            AND NOT trigger_row.tgisinternal
        )
      ELSE FALSE
    END AS present
  FROM required
)
SELECT count(*)::text || '|' || count(*) FILTER (WHERE present)::text
FROM checked;
SQL
}

schema_expected='000_schema_migrations.sql,001_initial.sql,002_provider_secrets.sql,003_platform_foundation.sql,004_platform_safety.sql,005_knowledge.sql,006_work.sql'
if [[ "$(schema_versions "$postgres_db")" != "$schema_expected" ]]; then
  fail 'Source schema_migrations does not contain the exact 000..006 chain.' 1
fi
assert_data_invariants "$postgres_db"
source_objects="$(required_objects "$postgres_db")"
source_total="${source_objects%%|*}"
source_present="${source_objects##*|}"
if [[ "$source_total" != "$source_present" ]]; then
  fail "Source schema objects are incomplete: $source_present/$source_total." 1
fi

source_snapshot="$(snapshot "$postgres_db")"
printf 'Source snapshot:\n%s\n' "$source_snapshot"

backup_output="$(COMPOSE_FILE="$compose_file" ENV_FILE="$no_env_file" \
  COMPOSE_PROJECT_NAME="$project_name" DEVENGLISH_CI_PORT="$ci_port" \
  POSTGRES_SERVICE="$postgres_service" POSTGRES_USER="$postgres_user" \
  POSTGRES_DB="$postgres_db" BACKUP_FILE="$dump_file" \
  "$repo_root/scripts/db_backup.sh")"
if [[ -z "$backup_output" || ! -s "$dump_file" ]]; then
  fail 'Backup script did not produce a non-empty dump.' 1
fi
dump_mode="$(stat -c '%a' "$dump_file" 2>/dev/null || stat -f '%Lp' "$dump_file")"
if [[ "$dump_mode" != "600" ]]; then
  fail "Backup dump mode must be 600, got $dump_mode." 1
fi
dump_listing="$(compose_run exec -T "$postgres_service" pg_restore --list < "$dump_file")"
if [[ -z "$dump_listing" ]]; then
  fail 'Backup dump is not a readable PostgreSQL custom-format archive.' 1
fi

psql_exec_db "$postgres_db" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$restore_db\"" >/dev/null 2>&1
restore_database_created=1
target_before="$(user_relation_count "$restore_db")"
if [[ "$target_before" != "0" ]]; then
  fail 'Restore target was not blank before negative restore checks.' 1
fi

set +e
expected_restore_guard_error="Restore is destructive. Re-run with CONFIRM_RESTORE=YES and CONFIRM_RESTORE_TARGET=$restore_db."
missing_confirmation_output="$(COMPOSE_FILE="$compose_file" ENV_FILE="$no_env_file" \
  COMPOSE_PROJECT_NAME="$project_name" DEVENGLISH_CI_PORT="$ci_port" \
  POSTGRES_SERVICE="$postgres_service" POSTGRES_USER="$postgres_user" \
  POSTGRES_DB="$restore_db" BACKUP_FILE="$dump_file" \
  CONFIRM_RESTORE=NO CONFIRM_RESTORE_TARGET="$restore_db" \
  "$repo_root/scripts/db_restore.sh" 2>&1 >/dev/null)"
missing_confirmation_status="$?"
set -e
if [[ "$missing_confirmation_status" != "1" || "$missing_confirmation_output" != "$expected_restore_guard_error" ]]; then
  fail 'Restore without confirmation did not fail with the exact destructive-action guard.' 1
fi

set +e
wrong_target_output="$(COMPOSE_FILE="$compose_file" ENV_FILE="$no_env_file" \
  COMPOSE_PROJECT_NAME="$project_name" DEVENGLISH_CI_PORT="$ci_port" \
  POSTGRES_SERVICE="$postgres_service" POSTGRES_USER="$postgres_user" \
  POSTGRES_DB="$restore_db" BACKUP_FILE="$dump_file" \
  CONFIRM_RESTORE=YES CONFIRM_RESTORE_TARGET="$postgres_db" \
  "$repo_root/scripts/db_restore.sh" 2>&1 >/dev/null)"
wrong_target_status="$?"
set -e
if [[ "$wrong_target_status" != "1" || "$wrong_target_output" != "$expected_restore_guard_error" ]]; then
  fail 'Restore with a wrong target did not fail with the exact destructive-action guard.' 1
fi

target_after_missing="$(user_relation_count "$restore_db")"
if [[ "$target_after_missing" != "$target_before" ]]; then
  fail 'Missing confirmation changed the restore target.' 1
fi

target_after_wrong="$(user_relation_count "$restore_db")"
if [[ "$target_after_wrong" != "$target_before" ]]; then
  fail 'Wrong restore target changed the restore target.' 1
fi

restore_output="$(COMPOSE_FILE="$compose_file" ENV_FILE="$no_env_file" \
  COMPOSE_PROJECT_NAME="$project_name" DEVENGLISH_CI_PORT="$ci_port" \
  POSTGRES_SERVICE="$postgres_service" POSTGRES_USER="$postgres_user" \
  POSTGRES_DB="$restore_db" BACKUP_FILE="$dump_file" \
  CONFIRM_RESTORE=YES CONFIRM_RESTORE_TARGET="$restore_db" \
  "$repo_root/scripts/db_restore.sh" 2>/dev/null)"
if [[ -z "$restore_output" ]]; then
  fail 'Restore script returned no completion output.' 1
fi

restore_snapshot="$(snapshot "$restore_db")"
if [[ "$restore_snapshot" != "$source_snapshot" ]]; then
  fail 'Source and restored snapshots differ after the backup/restore round trip.' 1
fi
assert_data_invariants "$restore_db"
restore_objects="$(required_objects "$restore_db")"
restore_total="${restore_objects%%|*}"
restore_present="${restore_objects##*|}"
if [[ "$restore_total" != "$restore_present" ]]; then
  fail "Restored schema objects are incomplete: $restore_present/$restore_total." 1
fi
if [[ "$(schema_versions "$restore_db")" != "$schema_expected" ]]; then
  fail 'Restored schema_migrations does not contain the exact 000..006 chain.' 1
fi

second_migration_output="$(run_migrate "$restore_db" 2>/dev/null)"
while IFS= read -r migration_name; do
  [[ -z "$migration_name" ]] && continue
  require_exact_line "$second_migration_output" "already applied $migration_name"
done <<< "$expected_migrations"
restore_after_migrate_snapshot="$(snapshot "$restore_db")"
if [[ "$restore_after_migrate_snapshot" != "$restore_snapshot" ]]; then
  fail 'Rerunning migrations changed the restored snapshot.' 1
fi

printf 'Restore snapshot:\n%s\n' "$restore_snapshot"
printf 'Backup archive: non-empty custom format, mode 600.\n'
printf 'Negative restore preflights: missing confirmation and wrong target rejected without mutation.\n'
printf 'Recovery round-trip passed: %s/%s schema objects and exact source/restore snapshots.\n' \
  "$restore_present" "$restore_total"
