#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${COMPOSE_FILE:-$repo_root/infra/docker-compose.yml}"
env_file="${ENV_FILE:-$repo_root/.env.local}"
backup_dir="${BACKUP_DIR:-$repo_root/backups}"
postgres_service="${POSTGRES_SERVICE:-postgres}"
postgres_user="${POSTGRES_USER:-devenglish}"
postgres_db="${POSTGRES_DB:-devenglish}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_file="${BACKUP_FILE:-$backup_dir/devenglish-${timestamp}.dump}"
temp_file="${backup_file}.tmp.$$"

compose=(docker compose -f "$compose_file")
if [[ -f "$env_file" ]]; then
  compose+=(--env-file "$env_file")
fi

if [[ -e "$backup_file" ]]; then
  printf 'Backup file already exists: %s\n' "$backup_file" >&2
  exit 1
fi

mkdir -p "$backup_dir"
trap 'rm -f "$temp_file"' EXIT

"${compose[@]}" exec -T "$postgres_service" pg_dump \
  --format=custom \
  --no-owner \
  --no-privileges \
  --username="$postgres_user" \
  --dbname="$postgres_db" > "$temp_file"

if [[ ! -s "$temp_file" ]]; then
  printf 'The database backup is empty: %s\n' "$temp_file" >&2
  exit 1
fi

chmod 600 "$temp_file"
mv "$temp_file" "$backup_file"
trap - EXIT
printf 'Backup written: %s\n' "$backup_file"
