#!/usr/bin/env bash
set -euo pipefail

# This harness is intentionally destructive only to the explicitly named,
# disposable Compose project it owns. It is not a production migration tool.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected_compose_file="$repo_root/infra/docker-compose.ci.yml"
compose_file="${COMPOSE_FILE:-}"
project_name="${COMPOSE_PROJECT_NAME:-}"
postgres_service="${POSTGRES_SERVICE:-}"
postgres_user="${POSTGRES_USER:-}"
postgres_db="${POSTGRES_DB:-}"

if [[ "${DEVENGLISH_LEGACY_UPGRADE_DISPOSABLE:-}" != "1" ]]; then
  printf 'Set DEVENGLISH_LEGACY_UPGRADE_DISPOSABLE=1 to opt into the disposable upgrade harness.\n' >&2
  exit 2
fi

if [[ -z "$compose_file" || -z "$project_name" || -z "$postgres_service" || -z "$postgres_user" || -z "$postgres_db" ]]; then
  printf 'COMPOSE_FILE, COMPOSE_PROJECT_NAME, POSTGRES_SERVICE, POSTGRES_USER and POSTGRES_DB are required.\n' >&2
  exit 2
fi

if [[ "$compose_file" != /* ]]; then
  compose_file="$repo_root/$compose_file"
fi
if [[ "$compose_file" != "$expected_compose_file" ]]; then
  printf 'Only the repository disposable Compose file is allowed: %s\n' "$expected_compose_file" >&2
  exit 2
fi
if [[ ! -f "$compose_file" ]]; then
  printf 'Compose file does not exist: %s\n' "$compose_file" >&2
  exit 2
fi
if [[ ! "$project_name" =~ ^devenglish-legacy-upgrade-[a-z0-9][a-z0-9-]*$ ]]; then
  printf 'COMPOSE_PROJECT_NAME must be a unique devenglish-legacy-upgrade-* project.\n' >&2
  exit 2
fi
if [[ "$postgres_service" != "postgres" || "$postgres_user" != "devenglish" || "$postgres_db" != "devenglish" ]]; then
  printf 'The harness only accepts the disposable CI postgres service/database configuration.\n' >&2
  exit 2
fi

compose=(docker compose --project-name "$project_name" --file "$compose_file")
service_list="$("${compose[@]}" config --services)"
if [[ "$service_list" != "$postgres_service" ]]; then
  printf 'Unexpected Compose services for the disposable harness.\n' >&2
  exit 2
fi
if ! "${compose[@]}" ps --status running --services | grep -Fxq "$postgres_service"; then
  printf 'The explicitly named disposable postgres service is not running.\n' >&2
  exit 2
fi

temp_root=""
cleanup_compose=0
cleanup() {
  exit_code=$?
  if (( cleanup_compose == 1 )); then
    printf 'Cleaning disposable Compose project %s.\n' "$project_name"
    if ! "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1; then
      printf 'Failed to clean disposable Compose project %s.\n' "$project_name" >&2
      if (( exit_code == 0 )); then
        exit_code=1
      fi
    fi
  fi
  if [[ -n "$temp_root" && -d "$temp_root" ]]; then
    rm -rf -- "$temp_root"
  fi
  exit "$exit_code"
}
trap cleanup EXIT

env_file=""
temp_root="$(mktemp -d)"
env_file="$temp_root/empty.env"
: > "$env_file"
migrations_dir="$temp_root/migrations"
phase1_dir="$temp_root/phase1"
legacy_catalog_dir="$temp_root/legacy-catalog"
phase2_dir="$temp_root/phase2"
mkdir -p "$migrations_dir" "$phase1_dir" "$legacy_catalog_dir" "$phase2_dir"

link_migration() {
  migration_name="$1"
  target_dir="$2"
  migration_path="$repo_root/infra/migrations/$migration_name"
  if [[ ! -f "$migration_path" ]]; then
    printf 'Missing migration: %s\n' "$migration_path" >&2
    exit 1
  fi
  ln -s "$migration_path" "$target_dir/$migration_name"
}

for migration_name in \
  000_schema_migrations.sql \
  001_initial.sql \
  002_provider_secrets.sql; do
  link_migration "$migration_name" "$phase1_dir"
done

for migration_name in \
  003_platform_foundation.sql \
  004_platform_safety.sql \
  005_knowledge.sql \
  006_work.sql; do
  link_migration "$migration_name" "$legacy_catalog_dir"
done

# Commit 104304f shipped the catalog through migration 006. Apply that exact
# legacy catalog first, then exercise the upgrade path for the newer modules.
for migration_name in \
  007_assistant_s1.sql \
  008_knowledge_root_fks.sql \
  009_knowledge_evidence_trigger_repair.sql \
  010_connector_sync.sql \
  011_usage_accounting.sql \
  012_action_receipt_hash.sql \
  013_learning_overlay.sql \
  014_workspace_backfill.sql \
  015_usage_reservations.sql \
  016_mcp_tokens.sql \
  017_workspace_scoped_challenge_identity.sql \
  018_assistant_conversation_context.sql \
  019_platform_constraint_repair.sql; do
  link_migration "$migration_name" "$phase2_dir"
done

psql_exec() {
  "${compose[@]}" exec -T "$postgres_service" psql \
    -X \
    -v ON_ERROR_STOP=1 \
    -U "$postgres_user" \
    -d "$postgres_db" \
    "$@"
}

psql_query() {
  query="$1"
  shift
  psql_exec "$@" -Atq <<< "$query"
}

run_migrate() {
  COMPOSE_FILE="$compose_file" \
    ENV_FILE="$env_file" \
    COMPOSE_PROJECT_NAME="$project_name" \
    POSTGRES_SERVICE="$postgres_service" \
    POSTGRES_USER="$postgres_user" \
    POSTGRES_DB="$postgres_db" \
    MIGRATIONS_DIR="$1" \
    "$repo_root/scripts/db_migrate.sh"
}

require_output() {
  output="$1"
  expected_line="$2"
  while IFS= read -r line; do
    if [[ "$line" == "$expected_line" ]]; then
      return 0
    fi
  done <<< "$output"
  printf 'Expected exact migration output line was not found: %s\n' "$expected_line" >&2
  exit 1
}

require_blank_database() {
  relation_count="$(psql_exec -Atqc "
    SELECT count(*)
    FROM pg_class AS relation
    JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
    WHERE namespace.nspname NOT LIKE 'pg_%'
      AND namespace.nspname <> 'information_schema'
      AND relation.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')")"
  if [[ "$relation_count" != "0" ]]; then
    printf 'Disposable database must be blank before migration or seed; found %s user relations.\n' "$relation_count" >&2
    return 1
  fi
}

legacy_snapshot() {
  psql_query "
    SELECT 'imported_sources|' || count(*)::text || '|' ||
           md5(coalesce(string_agg(
             format('%s|%s|%s|%s|%s|%s', id, user_id, source_type, title, content,
               to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
             '' ORDER BY id), ''))
    FROM imported_sources
    WHERE user_id IN (:'user_a', :'user_b')
    UNION ALL
    SELECT 'work_context|' || count(*)::text || '|' ||
           md5(coalesce(string_agg(
             format('%s|%s|%s|%s|%s|%s|%s|%s', id, user_id, source_type, source_url,
               title, content, domain,
               to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')),
             '' ORDER BY id), ''))
    FROM work_context
    WHERE user_id IN (:'user_a', :'user_b')
    ORDER BY 1" \
    --set=user_a="$user_a" \
    --set=user_b="$user_b"
}

assert_true() {
  assertion_name="$1"
  result="$2"
  if [[ "$result" != "t" ]]; then
    printf 'Assertion failed: %s\n' "$assertion_name" >&2
    exit 1
  fi
}

if ! psql_exec -Atqc 'SELECT 1' >/dev/null; then
  printf 'Unable to connect to the explicitly named disposable postgres service.\n' >&2
  exit 1
fi
cleanup_compose=1

if [[ "${DEVENGLISH_LEGACY_UPGRADE_NEGATIVE_CONTROL:-}" == "1" ]]; then
  if require_blank_database; then
    printf 'Negative gate failed: expected a pre-migrated disposable database.\n' >&2
    exit 1
  fi
  printf 'Negative gate passed: pre-migrated disposable database was rejected before seed.\n'
  exit 0
fi

if ! require_blank_database; then
  exit 1
fi

printf 'Running Phase 1 migrations 000..002.\n'
phase1_output="$(run_migrate "$phase1_dir")"
printf '%s\n' "$phase1_output"
for migration_name in \
  000_schema_migrations.sql \
  001_initial.sql \
  002_provider_secrets.sql; do
  require_output "$phase1_output" "applied $migration_name"
done

user_a='legacy-upgrade-user-a'
user_b='legacy-upgrade-user-b'
context_a='legacy-upgrade-context-a'
context_b='legacy-upgrade-context-b'
source_a='legacy-upgrade-source-a'
source_b='legacy-upgrade-source-b'

printf 'Seeding fixed synthetic legacy rows with parameterized SQL.\n'
psql_exec \
  --set=user_a="$user_a" \
  --set=user_b="$user_b" \
  --set=context_a="$context_a" \
  --set=context_b="$context_b" \
  --set=source_a="$source_a" \
  --set=source_b="$source_b" \
  --single-transaction <<'SQL'
INSERT INTO users (id, display_name, cefr, created_at)
VALUES
  (:'user_a', 'Legacy upgrade user A', 'B1', TIMESTAMPTZ '2026-01-01 00:00:00+00'),
  (:'user_b', 'Legacy upgrade user B', 'A2', TIMESTAMPTZ '2026-01-01 00:01:00+00');

INSERT INTO work_context (id, user_id, source_type, source_url, title, content, domain, created_at)
VALUES
  (:'context_a', :'user_a', 'manual', 'https://example.invalid/upgrade/a',
    'Legacy upgrade context A', 'synthetic legacy payload A', 'upgrade-a',
    TIMESTAMPTZ '2026-01-02 00:00:00+00'),
  (:'context_b', :'user_b', 'github', 'https://example.invalid/upgrade/b',
    'Legacy upgrade context B', 'synthetic legacy payload B', 'upgrade-b',
    TIMESTAMPTZ '2026-01-02 00:01:00+00');

INSERT INTO imported_sources (id, user_id, source_type, title, content, created_at)
VALUES
  (:'source_a', :'user_a', 'drive', 'Legacy upgrade source A',
    'synthetic imported payload A', TIMESTAMPTZ '2026-01-03 00:00:00+00'),
  (:'source_b', :'user_b', 'github', 'Legacy upgrade source B',
    'synthetic imported payload B', TIMESTAMPTZ '2026-01-03 00:01:00+00');
SQL

before_snapshot="$(legacy_snapshot)"
printf 'Legacy snapshot before upgrade: %s\n' "$before_snapshot"

printf 'Running legacy catalog migrations 003..006 from 104304f.\n'
legacy_catalog_output="$(run_migrate "$legacy_catalog_dir")"
printf '%s\n' "$legacy_catalog_output"
for migration_name in \
  003_platform_foundation.sql \
  004_platform_safety.sql \
  005_knowledge.sql \
  006_work.sql; do
  require_output "$legacy_catalog_output" "applied $migration_name"
done

printf 'Running upgrade migrations 007..019.\n'
phase2_output="$(run_migrate "$phase2_dir")"
printf '%s\n' "$phase2_output"
for migration_name in \
  007_assistant_s1.sql \
  008_knowledge_root_fks.sql \
  009_knowledge_evidence_trigger_repair.sql \
  010_connector_sync.sql \
  011_usage_accounting.sql \
  012_action_receipt_hash.sql \
  013_learning_overlay.sql \
  014_workspace_backfill.sql \
  015_usage_reservations.sql \
  016_mcp_tokens.sql \
  017_workspace_scoped_challenge_identity.sql \
  018_assistant_conversation_context.sql \
  019_platform_constraint_repair.sql; do
  require_output "$phase2_output" "applied $migration_name"
done

printf 'Checking full migration idempotency.\n'
phase2_second_output="$(run_migrate "$phase2_dir")"
printf '%s\n' "$phase2_second_output"
for migration_name in \
  007_assistant_s1.sql \
  008_knowledge_root_fks.sql \
  009_knowledge_evidence_trigger_repair.sql \
  010_connector_sync.sql \
  011_usage_accounting.sql \
  012_action_receipt_hash.sql \
  013_learning_overlay.sql \
  014_workspace_backfill.sql \
  015_usage_reservations.sql \
  016_mcp_tokens.sql \
  017_workspace_scoped_challenge_identity.sql \
  018_assistant_conversation_context.sql \
  019_platform_constraint_repair.sql; do
  require_output "$phase2_second_output" "already applied $migration_name"
done

after_snapshot="$(legacy_snapshot)"
printf 'Legacy snapshot after upgrade: %s\n' "$after_snapshot"
if [[ "$after_snapshot" != "$before_snapshot" ]]; then
  printf 'Legacy snapshot changed across migrations 003..019.\n' >&2
  exit 1
fi

schema_versions="$(psql_exec -Atqc "SELECT string_agg(version, ',' ORDER BY version) FROM schema_migrations")"
expected_schema_versions='000_schema_migrations.sql,001_initial.sql,002_provider_secrets.sql,003_platform_foundation.sql,004_platform_safety.sql,005_knowledge.sql,006_work.sql,007_assistant_s1.sql,008_knowledge_root_fks.sql,009_knowledge_evidence_trigger_repair.sql,010_connector_sync.sql,011_usage_accounting.sql,012_action_receipt_hash.sql,013_learning_overlay.sql,014_workspace_backfill.sql,015_usage_reservations.sql,016_mcp_tokens.sql,017_workspace_scoped_challenge_identity.sql,018_assistant_conversation_context.sql,019_platform_constraint_repair.sql'
if [[ "$schema_versions" != "$expected_schema_versions" ]]; then
  printf 'Unexpected schema_migrations contents: %s\n' "$schema_versions" >&2
  exit 1
fi

assert_true 'user A receives one deterministic default workspace' "$(psql_query "SELECT count(*) = 1 FROM workspaces WHERE owner_user_id = :'user_a' AND deleted_at IS NULL" --set=user_a="$user_a")"
assert_true 'user B receives one deterministic default workspace' "$(psql_query "SELECT count(*) = 1 FROM workspaces WHERE owner_user_id = :'user_b' AND deleted_at IS NULL" --set=user_b="$user_b")"
assert_true 'workspace backfill preserves migration metadata' "$(psql_exec -Atqc "SELECT count(*) = 2 FROM workspaces WHERE (metadata ->> 'migration') = '014_workspace_backfill' AND (metadata ->> 'legacyPreserved') = 'true'")"
assert_true 'legacy work_context remains one row per user' "$(psql_query "SELECT (SELECT count(*) FROM work_context WHERE user_id = :'user_a') = 1 AND (SELECT count(*) FROM work_context WHERE user_id = :'user_b') = 1" --set=user_a="$user_a" --set=user_b="$user_b")"
assert_true 'legacy imported_sources remains one row per user' "$(psql_query "SELECT (SELECT count(*) FROM imported_sources WHERE user_id = :'user_a') = 1 AND (SELECT count(*) FROM imported_sources WHERE user_id = :'user_b') = 1" --set=user_a="$user_a" --set=user_b="$user_b")"
assert_true 'no canonical Knowledge rows were implicitly created' "$(psql_exec -Atqc 'SELECT (SELECT count(*) FROM knowledge_sources) = 0 AND (SELECT count(*) FROM source_items) = 0 AND (SELECT count(*) FROM source_revisions) = 0')"
challenge_index_schema="$(psql_exec -Atq <<'SQL'
SELECT count(*) FILTER (WHERE indexname = 'action_challenges_pending_workspace_hash_uidx') = 1
  AND count(*) FILTER (WHERE indexname = 'action_challenges_pending_hash_uidx') = 0
FROM pg_indexes
WHERE schemaname = 'public'
  AND tablename = 'action_challenges'
  AND indexname LIKE 'action_challenges_pending%';
SQL
)"
assert_true 'one canonical pending challenge index survives the legacy upgrade' "$challenge_index_schema"
assistant_context_schema="$(psql_exec -Atq <<'SQL'
  SELECT
    EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'assistant_conversations' AND column_name = 'context_type')
    AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'assistant_conversations' AND column_name = 'context_id')
    AND EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'assistant_conversations_context_ref_ck' AND conrelid = 'assistant_conversations'::regclass)
    AND to_regclass('public.assistant_conversations_context_scope_idx') IS NOT NULL;
SQL
)"
assert_true 'assistant context columns and constraint are present' "$assistant_context_schema"

printf 'Legacy schema upgrade, workspace backfill, preservation and isolation checks passed.\n'
