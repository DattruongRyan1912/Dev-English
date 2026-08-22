#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${COMPOSE_FILE:-$repo_root/infra/docker-compose.yml}"
env_file="${ENV_FILE:-$repo_root/.env.local}"
migrations_dir="${MIGRATIONS_DIR:-$repo_root/infra/migrations}"
postgres_service="${POSTGRES_SERVICE:-postgres}"
postgres_user="${POSTGRES_USER:-devenglish}"
postgres_db="${POSTGRES_DB:-devenglish}"

compose=(docker compose -f "$compose_file")
if [[ -f "$env_file" ]]; then
  compose+=(--env-file "$env_file")
fi

psql_exec() {
  "${compose[@]}" exec -T "$postgres_service" psql \
    -v ON_ERROR_STOP=1 \
    -U "$postgres_user" \
    -d "$postgres_db" \
    "$@"
}

psql_exec -c '
  CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
  )
'

shopt -s nullglob
migrations=("$migrations_dir"/*.sql)
if (( ${#migrations[@]} == 0 )); then
  printf 'No SQL migrations found in %s\n' "$migrations_dir" >&2
  exit 1
fi

for migration in "${migrations[@]}"; do
  version="$(basename "$migration")"
  if [[ ! "$version" =~ ^[A-Za-z0-9._-]+$ ]]; then
    printf 'Unsafe migration filename: %s\n' "$version" >&2
    exit 1
  fi

  applied="$(psql_exec -Atqc \
    "SELECT 1 FROM schema_migrations WHERE version = '$version' LIMIT 1")"

  if [[ "$applied" == "1" ]]; then
    printf 'already applied %s\n' "$version"
    continue
  fi

  # Every migration must record its own version in the same transaction as its
  # schema changes. The tracked SQL files do this explicitly; -1 makes a
  # partial migration impossible if either statement fails.
  psql_exec --single-transaction < "$migration"
  recorded="$(psql_exec -Atqc \
    "SELECT 1 FROM schema_migrations WHERE version = '$version' LIMIT 1")"
  if [[ "$recorded" != "1" ]]; then
    printf 'Migration %s completed without recording its version.\n' "$version" >&2
    exit 1
  fi
  printf 'applied %s\n' "$version"
done
