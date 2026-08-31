#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

normalize_api_base() {
  local value="${1:-}"
  value="${value%/}"
  case "$value" in
    http://*|https://*) printf '%s\n' "$value" ;;
    *)
      printf 'API endpoint must start with http:// or https://.\n' >&2
      return 1
      ;;
  esac
}

select_api_base() {
  if [[ -n "${API_BASE_URL:-}" ]]; then
    normalize_api_base "$API_BASE_URL"
    return
  fi

  local tailscale_ip="${TAILSCALE_IP:-}"
  if [[ -z "$tailscale_ip" ]] && command -v tailscale >/dev/null 2>&1; then
    tailscale_ip="$(tailscale ip -4 2>/dev/null | head -n 1 || true)"
  fi
  if [[ -n "$tailscale_ip" ]]; then
    normalize_api_base "http://${tailscale_ip}:${ANDROID_BACKEND_PORT:-8080}"
    return
  fi

  local lan_ip="${LAN_IP:-$(ipconfig getifaddr en0 2>/dev/null || true)}"
  if [[ -z "$lan_ip" ]]; then
    printf 'Could not detect Tailscale or Wi-Fi IPv4. Set TAILSCALE_IP, LAN_IP, or API_BASE_URL and re-run.\n' >&2
    return 1
  fi
  normalize_api_base "http://${lan_ip}:${ANDROID_BACKEND_PORT:-8080}"
}

main() {
  cd "$repo_root"

  local api_base
  api_base="$(select_api_base)"
  local health="$api_base/healthz"
  local code
  code="$(curl -sS -o /tmp/devenglish-android-healthz.out -w '%{http_code}' --connect-timeout 3 "$health" || true)"
  if [[ "$code" != "200" ]]; then
    printf 'Backend is not reachable at %s (HTTP %s).\n' "$health" "$code" >&2
    printf 'Start it with: docker compose --env-file .env.local -f infra/docker-compose.yml up -d\n' >&2
    exit 1
  fi

  printf 'Building Android APK against %s\n' "$api_base"
  flutter build apk --release \
    --dart-define=DEVENGLISH_ENV=development \
    --dart-define=API_BASE_URL="$api_base"

  local apk="$repo_root/build/app/outputs/flutter-apk/app-release.apk"
  local desktop="${DEVENGLISH_APK_OUTPUT:-$HOME/Desktop/DevEnglish-local.apk}"
  cp "$apk" "$desktop"
  printf 'APK: %s\n' "$apk"
  printf 'Copy: %s\n' "$desktop"
  if [[ "$api_base" == http://100.*:* ]]; then
    printf 'Install Tailscale on the phone, sign in to the same tailnet, and keep this Mac plus Docker running.\n'
  else
    printf 'Install on a phone that can reach this endpoint. Keep this Mac and Docker backend running.\n'
  fi
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
