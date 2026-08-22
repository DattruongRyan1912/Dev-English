#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${COMPOSE_FILE:-$repo_root/infra/docker-compose.yml}"
env_file="${ENV_FILE:-$repo_root/.env.local}"
backup_file="${BACKUP_FILE:-}"
postgres_service="${POSTGRES_SERVICE:-postgres}"
postgres_user="${POSTGRES_USER:-devenglish}"
postgres_db="${POSTGRES_DB:-devenglish}"
runtime_environment="${DEVENGLISH_ENV:-development}"

compose=(docker compose -f "$compose_file")
if [[ -f "$env_file" ]]; then
  compose+=(--env-file "$env_file")
fi

if [[ -z "$backup_file" || ! -f "$backup_file" ]]; then
  printf 'Set BACKUP_FILE to an existing pg_dump custom-format file.\n' >&2
  exit 1
fi

if [[ "$runtime_environment" == "production" && "${ALLOW_PRODUCTION_RESTORE:-NO}" != "YES" ]]; then
  printf 'Production restore requires ALLOW_PRODUCTION_RESTORE=YES in addition to confirmation.\n' >&2
  exit 1
fi

if [[ "${CONFIRM_RESTORE:-NO}" != "YES" ]]; then
  printf 'Restore is destructive. Re-run with CONFIRM_RESTORE=YES for database %s.\n' "$postgres_db" >&2
  exit 1
fi

"${compose[@]}" exec -T "$postgres_service" pg_restore \
  --clean \
  --if-exists \
  --exit-on-error \
  --no-owner \
  --no-privileges \
  --username="$postgres_user" \
  --dbname="$postgres_db" < "$backup_file"

printf 'Restore completed into database %s from %s\n' "$postgres_db" "$backup_file"
