#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${COMPOSE_FILE:?COMPOSE_FILE is required}"
env_file="${ENV_FILE:-}"
postgres_service="${POSTGRES_SERVICE:-postgres}"
postgres_user="${POSTGRES_USER:-devenglish}"
postgres_db="${POSTGRES_DB:-devenglish}"
migrations_dir="${MIGRATIONS_DIR:-$repo_root/infra/migrations}"

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

run_migrate() {
  COMPOSE_FILE="$compose_file" \
    ENV_FILE="$env_file" \
    POSTGRES_SERVICE="$postgres_service" \
    POSTGRES_USER="$postgres_user" \
    POSTGRES_DB="$postgres_db" \
    MIGRATIONS_DIR="$1" \
    "$repo_root/scripts/db_migrate.sh"
}

first_output="$(run_migrate "$migrations_dir")"
printf '%s\n' "$first_output"

shopt -s nullglob
normal_migrations=("$migrations_dir"/*.sql)
if (( ${#normal_migrations[@]} == 0 )); then
  printf 'No normal migrations found in %s\n' "$migrations_dir" >&2
  exit 1
fi
for migration in "${normal_migrations[@]}"; do
  version="$(basename "$migration")"
  if ! grep -Fq "applied $version" <<<"$first_output"; then
    printf 'First migration run did not apply %s.\n' "$version" >&2
    exit 1
  fi
done

second_output="$(run_migrate "$migrations_dir")"
printf '%s\n' "$second_output"
for migration in "${normal_migrations[@]}"; do
  version="$(basename "$migration")"
  if ! grep -Fq "already applied $version" <<<"$second_output"; then
    printf 'Second migration run was not idempotent for %s.\n' "$version" >&2
    exit 1
  fi
done

recorded_count="$(psql_exec -Atqc 'SELECT count(*) FROM schema_migrations')"
if [[ "$recorded_count" != "${#normal_migrations[@]}" ]]; then
  printf 'Expected %d recorded migrations, got %s.\n' "${#normal_migrations[@]}" "$recorded_count" >&2
  exit 1
fi

failure_dir="$repo_root/testdata/migrations/failing"
failure_output="$(mktemp)"
trap 'rm -f "$failure_output"' EXIT
set +e
run_migrate "$failure_dir" >"$failure_output" 2>&1
failure_status=$?
set -e
sed -n '1,120p' "$failure_output"
if (( failure_status == 0 )); then
  printf 'The failing migration unexpectedly succeeded.\n' >&2
  exit 1
fi

probe_exists="$(psql_exec -Atqc "SELECT to_regclass('public.migration_failure_probe') IS NULL")"
if [[ "$probe_exists" != "t" ]]; then
  printf 'Failed migration left partial schema state.\n' >&2
  exit 1
fi
recorded_failure="$(psql_exec -Atqc "SELECT count(*) FROM schema_migrations WHERE version = '003_failure.sql'")"
if [[ "$recorded_failure" != "0" ]]; then
  printf 'Failed migration was falsely recorded.\n' >&2
  exit 1
fi

printf 'Migration runner idempotency and rollback checks passed.\n'
