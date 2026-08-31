#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_file="$(mktemp)"
trap 'rm -f "$output_file"' EXIT

set -a
# shellcheck disable=SC1091
source "$repo_root/infra/production.env.example"
set +a

set +e
(
  cd "$repo_root"
  go run ./backend/cmd/server >"$output_file" 2>&1
)
status=$?
set -e

if (( status == 0 )); then
  printf 'The unchanged production example unexpectedly passed startup validation.\n' >&2
  exit 1
fi

if ! grep -Fq 'DEVENGLISH_AUTH_SECRET must be at least 32 characters in production' "$output_file"; then
  printf 'The production example was rejected outside the expected auth-secret boundary.\n' >&2
  exit 1
fi

printf 'Unchanged production example is rejected by production startup validation.\n'
