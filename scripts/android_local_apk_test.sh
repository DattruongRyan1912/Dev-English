#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/android_local_apk.sh
source "$repo_root/scripts/android_local_apk.sh"

assert_equals() {
  local expected="$1"
  local actual="$2"
  local label="$3"
  if [[ "$actual" != "$expected" ]]; then
    printf 'FAIL %s: expected %q, got %q\n' "$label" "$expected" "$actual" >&2
    exit 1
  fi
  printf 'PASS %s\n' "$label"
}

api_base_from_explicit="$(API_BASE_URL='https://devenglish.example.test/' select_api_base)"
assert_equals 'https://devenglish.example.test' "$api_base_from_explicit" 'explicit API_BASE_URL wins'

api_base_from_tailscale="$(API_BASE_URL='' TAILSCALE_IP=100.126.52.73 ANDROID_BACKEND_PORT=18080 select_api_base)"
assert_equals 'http://100.126.52.73:18080' "$api_base_from_tailscale" 'explicit TAILSCALE_IP is used'

api_base_from_lan="$(PATH=/usr/bin:/bin API_BASE_URL='' TAILSCALE_IP='' LAN_IP=192.168.1.44 ANDROID_BACKEND_PORT=8080 select_api_base)"
assert_equals 'http://192.168.1.44:8080' "$api_base_from_lan" 'LAN_IP remains the fallback'

if API_BASE_URL='ftp://invalid.example.test' select_api_base >/dev/null 2>&1; then
  printf 'FAIL invalid API_BASE_URL was accepted\n' >&2
  exit 1
fi
printf 'PASS invalid API_BASE_URL is rejected\n'
