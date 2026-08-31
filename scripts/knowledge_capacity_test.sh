#!/usr/bin/env bash
set -euo pipefail

# This gate owns only the explicitly named disposable Compose project. It
# proves that the canonical Knowledge indexes and bounded pagination remain
# usable with the S7 100,000-chunk fixture. It is not a production load test.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected_compose_file="$repo_root/infra/docker-compose.ci.yml"
compose_file="${COMPOSE_FILE:-}"
project_name="${COMPOSE_PROJECT_NAME:-}"
postgres_service="${POSTGRES_SERVICE:-}"
postgres_user="${POSTGRES_USER:-}"
postgres_db="${POSTGRES_DB:-}"

if [[ "${DEVENGLISH_CAPACITY_DISPOSABLE:-}" != "1" ]]; then
  printf 'Set DEVENGLISH_CAPACITY_DISPOSABLE=1 to opt into the disposable capacity gate.\n' >&2
  exit 2
fi
if [[ -z "$compose_file" || -z "$project_name" || -z "$postgres_service" || -z "$postgres_user" || -z "$postgres_db" ]]; then
  printf 'COMPOSE_FILE, COMPOSE_PROJECT_NAME, POSTGRES_SERVICE, POSTGRES_USER and POSTGRES_DB are required.\n' >&2
  exit 2
fi
if [[ "$compose_file" != /* ]]; then
  compose_file="$repo_root/$compose_file"
fi
if [[ "$compose_file" != "$expected_compose_file" || ! -f "$compose_file" ]]; then
  printf 'Only the repository disposable Compose file is allowed: %s\n' "$expected_compose_file" >&2
  exit 2
fi
if [[ ! "$project_name" =~ ^devenglish-capacity-[a-z0-9][a-z0-9-]*$ ]]; then
  printf 'COMPOSE_PROJECT_NAME must be a unique devenglish-capacity-* project.\n' >&2
  exit 2
fi
if [[ "$postgres_service" != "postgres" || "$postgres_user" != "devenglish" || "$postgres_db" != "devenglish" ]]; then
  printf 'The gate only accepts the disposable CI postgres service/database configuration.\n' >&2
  exit 2
fi

compose=(docker compose --project-name "$project_name" --file "$compose_file")
if [[ "$("${compose[@]}" config --services)" != "$postgres_service" ]]; then
  printf 'Unexpected Compose services for the disposable capacity gate.\n' >&2
  exit 2
fi
if ! "${compose[@]}" ps --status running --services | grep -Fxq "$postgres_service"; then
  printf 'The explicitly named disposable postgres service is not running.\n' >&2
  exit 2
fi

temp_root="$(mktemp -d)"
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
  rm -rf -- "$temp_root"
  exit "$exit_code"
}
trap cleanup EXIT

env_file="$temp_root/empty.env"
: > "$env_file"
run_migrate() {
  COMPOSE_FILE="$compose_file" \
    ENV_FILE="$env_file" \
    COMPOSE_PROJECT_NAME="$project_name" \
    POSTGRES_SERVICE="$postgres_service" \
    POSTGRES_USER="$postgres_user" \
    POSTGRES_DB="$postgres_db" \
    "$repo_root/scripts/db_migrate.sh"
}
psql_exec() {
  "${compose[@]}" exec -T "$postgres_service" psql \
    -X -v ON_ERROR_STOP=1 -U "$postgres_user" -d "$postgres_db" "$@"
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

printf 'Applying the complete migration chain.\n'
run_migrate >/dev/null

printf 'Seeding 100,000 canonical Knowledge chunks.\n'
psql_exec --single-transaction <<'SQL'
INSERT INTO users (id, display_name, cefr)
VALUES ('capacity-user', 'Capacity fixture', 'B1');

INSERT INTO workspaces (id, owner_user_id, name, slug, description)
VALUES ('capacity-workspace', 'capacity-user', 'Capacity fixture', 'capacity-fixture', 'Disposable S7 fixture');

INSERT INTO knowledge_sources (id, workspace_id, kind, name, uri)
VALUES ('capacity-source', 'capacity-workspace', 'fixture', 'Capacity fixture', 'memory://capacity-fixture');

INSERT INTO source_items (id, workspace_id, source_id, external_id, title, uri, mime_type)
VALUES ('capacity-item', 'capacity-workspace', 'capacity-source', 'capacity-item', 'Capacity item', 'memory://capacity-item', 'text/plain');

INSERT INTO source_revisions (id, workspace_id, source_item_id, revision_key, content_hash, content_type, source_uri, content)
VALUES ('capacity-revision', 'capacity-workspace', 'capacity-item', 'capacity-revision', 'capacity-content', 'text/plain', 'memory://capacity-item', 'capacity fixture canonical retrieval');

UPDATE source_items
SET current_revision_id = 'capacity-revision'
WHERE id = 'capacity-item' AND workspace_id = 'capacity-workspace';

INSERT INTO knowledge_chunks (id, workspace_id, revision_id, ordinal, chunk_text, token_count)
SELECT
  'capacity-chunk-' || lpad(generator::text, 6, '0'),
  'capacity-workspace',
  'capacity-revision',
  generator - 1,
  'capacity canonical retrieval chunk ' || generator::text,
  5
FROM generate_series(1, 100000) AS generator;

ANALYZE knowledge_chunks;
SQL

assert_true 'exactly 100000 chunks were inserted' "$(psql_exec -Atqc "SELECT count(*) = 100000 FROM knowledge_chunks WHERE workspace_id = 'capacity-workspace'")"
assert_true 'last page returns ten bounded rows' "$(psql_exec -Atqc "SELECT count(*) = 10 FROM (SELECT id FROM knowledge_chunks WHERE workspace_id = 'capacity-workspace' ORDER BY ordinal LIMIT 10 OFFSET 99990) AS page")"
assert_true 'full-text index returns matching chunks' "$(psql_exec -Atqc "SELECT count(*) > 0 FROM knowledge_chunks WHERE workspace_id = 'capacity-workspace' AND search_vector @@ plainto_tsquery('simple', 'canonical retrieval')")"
assert_true 'full-text index exists' "$(psql_exec -Atqc "SELECT to_regclass('public.knowledge_chunks_search_idx') IS NOT NULL")"

printf 'Knowledge capacity gate passed: 100000 chunks, FTS lookup and bounded last-page pagination are healthy.\n'
