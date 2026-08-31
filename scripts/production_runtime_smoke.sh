#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/infra/docker-compose.production.smoke.yml"
work_dir="$(mktemp -d)"
project="devenglish-production-smoke-$(date -u +%Y%m%d%H%M%S)-$$"
backend_port="${DEVENGLISH_SMOKE_BACKEND_PORT:-18080}"
embedding_port="${DEVENGLISH_SMOKE_EMBEDDING_PORT:-18090}"

if [[ "${DEVENGLISH_PRODUCTION_SMOKE:-NO}" != "YES" ]]; then
  printf 'This test starts a disposable production-shaped Docker stack and writes a temporary database.\n' >&2
  printf 'Re-run with DEVENGLISH_PRODUCTION_SMOKE=YES.\n' >&2
  exit 2
fi
if [[ ! -f "$compose_file" ]]; then
  printf 'Production smoke compose file not found: %s\n' "$compose_file" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
  printf 'Docker and curl are required for the production runtime smoke.\n' >&2
  exit 1
fi

compose=(docker compose --project-name "$project" --file "$compose_file")

cleanup() {
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$1"
    return
  fi
  date -u +%s%N
}

export DEVENGLISH_SMOKE_POSTGRES_DB=devenglish
export DEVENGLISH_SMOKE_POSTGRES_USER=devenglish
export DEVENGLISH_SMOKE_POSTGRES_PASSWORD="$(random_hex 24)"
export DEVENGLISH_ALLOWED_ORIGINS="https://app.example.com"
export DEVENGLISH_AUTH_SECRET="$(random_hex 32)"
export DEVENGLISH_LOGIN_SECRET="$(random_hex 24)"
export DEVENGLISH_SECRET_ENCRYPTION_KEY="$(random_hex 32)"
# This key is intentionally accepted by the app but the endpoint is loopback
# port 1, so the smoke proves production fail-closed behavior without a live
# provider request or provider credentials.
export DEEPSEEK_API_KEY="production-smoke-key"
export DEEPSEEK_BASE_URL="http://127.0.0.1:1"
export DEEPSEEK_FAST_MODEL=deepseek-v4-flash
export DEEPSEEK_SMART_MODEL=deepseek-v4-pro
export DEVENGLISH_SMOKE_BACKEND_PORT="$backend_port"
export DEVENGLISH_SMOKE_EMBEDDING_PORT="$embedding_port"

if curl --silent --show-error --connect-timeout 1 --max-time 2 "http://127.0.0.1:$backend_port/healthz" >/dev/null 2>&1; then
  printf 'Smoke backend port is already in use: %s\n' "$backend_port" >&2
  exit 1
fi
if curl --silent --show-error --connect-timeout 1 --max-time 2 "http://127.0.0.1:$embedding_port/healthz" >/dev/null 2>&1; then
  printf 'Smoke embedding port is already in use: %s\n' "$embedding_port" >&2
  exit 1
fi

"${compose[@]}" up --detach --build postgres embedding backend

backend_url="http://127.0.0.1:$backend_port"
embedding_url="http://127.0.0.1:$embedding_port"

wait_for_url() {
  local url="$1"
  local attempts="$2"
  local attempt
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl --silent --show-error --fail --max-time 5 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  printf 'Timed out waiting for %s\n' "$url" >&2
  "${compose[@]}" logs --no-color --tail=120 backend embedding postgres >&2 || true
  return 1
}

http_status() {
  local output="$1"
  shift
  curl --silent --show-error --max-time 20 -o "$output" -w '%{http_code}' "$@"
}

wait_for_url "$embedding_url/healthz" 180
wait_for_url "$backend_url/readyz" 120

embedding_body="$work_dir/embedding.json"
embedding_status="$(http_status "$embedding_body" "$embedding_url/embed" \
  -H 'Content-Type: application/json' \
  --data '{"text":"production smoke embedding probe","input_type":"query"}')"
if [[ "$embedding_status" != "200" ]] || ! grep -q '"dimensions":384' "$embedding_body"; then
  printf 'Embedding sidecar returned %s or did not expose the expected 384-dimensional vector.\n' "$embedding_status" >&2
  exit 1
fi

migration_count="$("${compose[@]}" exec --tty=false postgres psql -v ON_ERROR_STOP=1 -U "$DEVENGLISH_SMOKE_POSTGRES_USER" -d "$DEVENGLISH_SMOKE_POSTGRES_DB" -Atqc 'SELECT count(*) FROM schema_migrations')"
if [[ "$migration_count" != "19" ]]; then
	printf 'Production smoke applied %s migrations; want 19.\n' "$migration_count" >&2
  exit 1
fi

anonymous_body="$work_dir/anonymous.json"
anonymous_status="$(http_status "$anonymous_body" "$backend_url/api/v2/bootstrap")"
if [[ "$anonymous_status" != "401" ]]; then
  printf 'Anonymous bootstrap returned %s; want 401.\n' "$anonymous_status" >&2
  exit 1
fi

login_body="$work_dir/login.json"
login_headers="$work_dir/login.headers"
login_status="$(curl --silent --show-error --max-time 20 -D "$login_headers" -o "$login_body" -w '%{http_code}' \
  -X POST "$backend_url/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  --data "{\"secret\":\"$DEVENGLISH_LOGIN_SECRET\"}")"
if [[ "$login_status" != "200" ]]; then
  printf 'Production login returned %s.\n' "$login_status" >&2
  exit 1
fi
if grep -q '"token"' "$login_body"; then
  printf 'Production login leaked a bearer token in the response.\n' >&2
  exit 1
fi
if ! grep -qi 'HttpOnly' "$login_headers" || ! grep -qi 'Secure' "$login_headers" || ! grep -qi 'SameSite=Strict' "$login_headers"; then
  printf 'Production login did not issue the expected secure cookie attributes.\n' >&2
  exit 1
fi
cookie="$(grep -i '^Set-Cookie:' "$login_headers" | sed -n 's/^[^:]*:[[:space:]]*\([^;]*\).*/\1/p' | head -n 1)"
if [[ -z "$cookie" ]]; then
  printf 'Production login did not issue a session cookie.\n' >&2
  exit 1
fi

me_body="$work_dir/me.json"
me_status="$(http_status "$me_body" "$backend_url/api/v1/auth/me" -H "Cookie: $cookie")"
if [[ "$me_status" != "200" ]] || ! grep -q '"id":"user-1"' "$me_body"; then
  printf 'Authenticated session reload returned %s.\n' "$me_status" >&2
  exit 1
fi

bootstrap_body="$work_dir/bootstrap.json"
bootstrap_status="$(http_status "$bootstrap_body" "$backend_url/api/v2/bootstrap" -H "Cookie: $cookie")"
if [[ "$bootstrap_status" != "200" ]] || ! grep -q '"workspace"' "$bootstrap_body" || \
  ! grep -q '"capabilities"' "$bootstrap_body" || ! grep -q '"work":true' "$bootstrap_body"; then
  printf 'Authenticated workspace bootstrap returned %s.\n' "$bootstrap_status" >&2
  exit 1
fi

project_body="$work_dir/project.json"
project_status="$(http_status "$project_body" "$backend_url/api/v2/projects" \
  -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-project-create' \
  --data '{"name":"Production smoke project","description":"Verify canonical work lifecycle."}')"
if [[ "$project_status" != "201" ]] || ! grep -q '"version":1' "$project_body"; then
  printf 'Production project create returned %s.\n' "$project_status" >&2
  exit 1
fi
project_id="$(sed -n 's/.*"id":"\([^"]*\)".*/\1/p' "$project_body")"
if [[ -z "$project_id" ]]; then
  printf 'Production project create response could not be parsed safely.\n' >&2
  exit 1
fi

project_update_body="$work_dir/project-update.json"
project_update_status="$(http_status "$project_update_body" "$backend_url/api/v2/projects/$project_id" \
  -X PATCH -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-project-update' \
  --data '{"description":"Updated canonical work lifecycle.","expectedVersion":1}')"
if [[ "$project_update_status" != "200" ]] || ! grep -q '"version":2' "$project_update_body"; then
  printf 'Production project update returned %s.\n' "$project_update_status" >&2
  exit 1
fi
project_conflict_body="$work_dir/project-conflict.json"
project_conflict_status="$(http_status "$project_conflict_body" "$backend_url/api/v2/projects/$project_id" \
  -X PATCH -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-project-stale' \
  --data '{"description":"Stale update must not overwrite.","expectedVersion":1}')"
if [[ "$project_conflict_status" != "409" ]]; then
  printf 'Production project stale update returned %s; want 409.\n' "$project_conflict_status" >&2
  exit 1
fi
project_history_body="$work_dir/project-history.json"
project_history_status="$(http_status "$project_history_body" "$backend_url/api/v2/projects/$project_id/history" -H "Cookie: $cookie")"
if [[ "$project_history_status" != "200" ]] || ! grep -q '"action":"updated"' "$project_history_body"; then
  printf 'Production project history returned %s or missed the update event.\n' "$project_history_status" >&2
  exit 1
fi

task_body="$work_dir/task.json"
task_status="$(http_status "$task_body" "$backend_url/api/v2/projects/$project_id/tasks" \
  -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-task-create' \
  --data '{"title":"Verify production task","description":"Exercise task version checks.","priority":"high"}')"
if [[ "$task_status" != "201" ]] || ! grep -q '"version":1' "$task_body"; then
  printf 'Production task create returned %s.\n' "$task_status" >&2
  exit 1
fi
task_id="$(sed -n 's/.*"id":"\([^"]*\)".*/\1/p' "$task_body")"
if [[ -z "$task_id" ]]; then
  printf 'Production task create response could not be parsed safely.\n' >&2
  exit 1
fi
task_update_body="$work_dir/task-update.json"
task_update_status="$(http_status "$task_update_body" "$backend_url/api/v2/tasks/$task_id" \
  -X PATCH -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-task-update' \
  --data '{"status":"in_progress","expectedVersion":1}')"
if [[ "$task_update_status" != "200" ]] || ! grep -q '"version":2' "$task_update_body"; then
  printf 'Production task update returned %s.\n' "$task_update_status" >&2
  exit 1
fi
task_conflict_body="$work_dir/task-conflict.json"
task_conflict_status="$(http_status "$task_conflict_body" "$backend_url/api/v2/tasks/$task_id" \
  -X PATCH -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-task-stale' \
  --data '{"status":"done","expectedVersion":1}')"
if [[ "$task_conflict_status" != "409" ]]; then
  printf 'Production task stale update returned %s; want 409.\n' "$task_conflict_status" >&2
  exit 1
fi
task_history_body="$work_dir/task-history.json"
task_history_status="$(http_status "$task_history_body" "$backend_url/api/v2/tasks/$task_id/history" -H "Cookie: $cookie")"
if [[ "$task_history_status" != "200" ]] || ! grep -q '"action":"updated"' "$task_history_body"; then
  printf 'Production task history returned %s or missed the update event.\n' "$task_history_status" >&2
  exit 1
fi

decision_body="$work_dir/decision.json"
decision_status="$(http_status "$decision_body" "$backend_url/api/v2/decisions" \
  -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-decision-create' \
  --data "{\"projectId\":\"$project_id\",\"title\":\"Keep canonical facts\",\"context\":\"Provider output is untrusted.\",\"outcome\":\"Require evidence.\",\"rationale\":\"Prevents fabricated work data.\"}")"
if [[ "$decision_status" != "201" ]] || ! grep -q '"version":1' "$decision_body"; then
  printf 'Production decision create returned %s.\n' "$decision_status" >&2
  exit 1
fi
decision_id="$(sed -n 's/.*"id":"\([^"]*\)".*/\1/p' "$decision_body")"
if [[ -z "$decision_id" ]]; then
  printf 'Production decision create response could not be parsed safely.\n' >&2
  exit 1
fi
decision_update_body="$work_dir/decision-update.json"
decision_update_status="$(http_status "$decision_update_body" "$backend_url/api/v2/decisions/$decision_id" \
  -X PATCH -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-decision-update' \
  --data '{"status":"accepted","expectedVersion":1}')"
if [[ "$decision_update_status" != "200" ]] || ! grep -q '"version":2' "$decision_update_body"; then
  printf 'Production decision update returned %s.\n' "$decision_update_status" >&2
  exit 1
fi
decision_conflict_body="$work_dir/decision-conflict.json"
decision_conflict_status="$(http_status "$decision_conflict_body" "$backend_url/api/v2/decisions/$decision_id" \
  -X PATCH -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-decision-stale' \
  --data '{"status":"rejected","expectedVersion":1}')"
if [[ "$decision_conflict_status" != "409" ]]; then
  printf 'Production decision stale update returned %s; want 409.\n' "$decision_conflict_status" >&2
  exit 1
fi
decision_history_body="$work_dir/decision-history.json"
decision_history_status="$(http_status "$decision_history_body" "$backend_url/api/v2/decisions/$decision_id/history" -H "Cookie: $cookie")"
if [[ "$decision_history_status" != "200" ]] || ! grep -q '"action":"updated"' "$decision_history_body"; then
  printf 'Production decision history returned %s or missed the update event.\n' "$decision_history_status" >&2
  exit 1
fi

mcp_token_body="$work_dir/mcp-token.json"
mcp_token_headers="$work_dir/mcp-token.headers"
mcp_token_status="$(curl --silent --show-error --max-time 20 -D "$mcp_token_headers" -o "$mcp_token_body" -w '%{http_code}' \
  -X POST "$backend_url/api/v2/mcp/tokens" -H "Cookie: $cookie" -H 'Content-Type: application/json' \
  --data '{"scopes":["assistant:use","work:read"]}')"
if [[ "$mcp_token_status" != "201" ]] || ! grep -q '"id"' "$mcp_token_body" || ! grep -q '"token"' "$mcp_token_body"; then
  printf 'MCP token issue returned %s or omitted token metadata.\n' "$mcp_token_status" >&2
  exit 1
fi
if ! grep -qi '^Cache-Control:.*no-store' "$mcp_token_headers"; then
  printf 'MCP token issue did not advertise no-store.\n' >&2
  exit 1
fi
mcp_token_id="$(sed -n 's/.*"id":"\([^"]*\)".*/\1/p' "$mcp_token_body")"
mcp_token="$(sed -n 's/.*"token":"\([^"]*\)".*/\1/p' "$mcp_token_body")"
if [[ -z "$mcp_token_id" || -z "$mcp_token" ]]; then
  printf 'MCP token issue response could not be parsed safely.\n' >&2
  exit 1
fi

mcp_initialize_body="$work_dir/mcp-initialize.json"
mcp_initialize_status="$(http_status "$mcp_initialize_body" "$backend_url/mcp" \
  -H "Authorization: Bearer $mcp_token" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"production-smoke","version":"1"}}}')"
if [[ "$mcp_initialize_status" != "200" ]] || grep -Fq "$mcp_token" "$mcp_initialize_body" || ! grep -Fq '"instructions":"DevEnglish MCP exposes canonical Work and Knowledge data.' "$mcp_initialize_body"; then
  printf 'MCP initialize returned %s, leaked the bearer token or omitted server instructions.\n' "$mcp_initialize_status" >&2
  exit 1
fi

mcp_revoke_status="$(curl --silent --show-error --max-time 20 -o /dev/null -w '%{http_code}' \
  -X DELETE "$backend_url/api/v2/mcp/tokens/$mcp_token_id" -H "Cookie: $cookie")"
if [[ "$mcp_revoke_status" != "204" ]]; then
  printf 'MCP token revoke returned %s; want 204.\n' "$mcp_revoke_status" >&2
  exit 1
fi
mcp_revoked_body="$work_dir/mcp-revoked.json"
mcp_revoked_status="$(http_status "$mcp_revoked_body" "$backend_url/mcp" \
  -H "Authorization: Bearer $mcp_token" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":2,"method":"ping"}')"
if [[ "$mcp_revoked_status" != "401" ]]; then
  printf 'Revoked MCP token returned %s; want 401.\n' "$mcp_revoked_status" >&2
  exit 1
fi

source_body="$work_dir/source.json"
source_status="$(http_status "$source_body" "$backend_url/api/v2/knowledge/sources" \
  -H "Cookie: $cookie" -H 'Content-Type: application/json' -H 'Idempotency-Key: production-smoke-source' \
  --data '{"name":"Production smoke source","kind":"manual","uri":"manual://production-smoke","mimeType":"text/plain","content":"The production smoke verifies strict authentication, source-backed retrieval and provider fail-closed behavior."}')"
if [[ "$source_status" != "201" ]] || ! grep -q '"revisionId"' "$source_body"; then
  printf 'Authenticated source import returned %s.\n' "$source_status" >&2
  exit 1
fi

search_body="$work_dir/search.json"
search_status="$(http_status "$search_body" "$backend_url/api/v2/knowledge/search?q=source-backed" \
  -H "Cookie: $cookie")"
if [[ "$search_status" != "200" ]] || ! grep -q 'source-backed' "$search_body"; then
  printf 'Authenticated knowledge search returned %s or missed the smoke source.\n' "$search_status" >&2
  exit 1
fi

cors_headers="$work_dir/cors.headers"
cors_status="$(curl --silent --show-error --max-time 20 -D "$cors_headers" -o /dev/null -w '%{http_code}' \
  -X OPTIONS "$backend_url/api/v2/bootstrap" \
  -H 'Origin: https://app.example.com' \
  -H 'Access-Control-Request-Method: GET' \
  -H 'Access-Control-Request-Headers: content-type, idempotency-key')"
if [[ "$cors_status" != "204" ]] || ! grep -qi 'Access-Control-Allow-Origin: https://app.example.com' "$cors_headers"; then
  printf 'Production CORS preflight returned %s or the allowlist was not advertised.\n' "$cors_status" >&2
  exit 1
fi

blocked_cors_headers="$work_dir/blocked-cors.headers"
blocked_cors_status="$(curl --silent --show-error --max-time 20 -D "$blocked_cors_headers" -o /dev/null -w '%{http_code}' \
  -X OPTIONS "$backend_url/api/v2/bootstrap" \
  -H 'Origin: https://not-allowed.example.com' \
  -H 'Access-Control-Request-Method: GET')"
if [[ "$blocked_cors_status" != "403" ]] || grep -qi 'Access-Control-Allow-Origin: https://not-allowed.example.com' "$blocked_cors_headers"; then
  printf 'Production CORS accepted or advertised a forbidden origin (status %s).\n' "$blocked_cors_status" >&2
  exit 1
fi

assistant_body="$work_dir/assistant.json"
assistant_status="$(http_status "$assistant_body" "$backend_url/api/v2/assistant/conversations" \
  -H "Cookie: $cookie" -H 'Content-Type: application/json' \
  --data '{"message":"What is in my workspace?"}')"
if [[ "$assistant_status" != "500" ]]; then
  printf 'Provider failure returned %s; want a fail-closed 500.\n' "$assistant_status" >&2
  exit 1
fi
if grep -qi '"grounding"' "$assistant_body" || grep -qi 'deterministic-fallback' "$assistant_body"; then
  printf 'Production provider failure exposed a fallback assistant response.\n' >&2
  exit 1
fi

logout_status="$(curl --silent --show-error --max-time 20 -o /dev/null -w '%{http_code}' \
  -X POST "$backend_url/api/v1/auth/logout" -H "Cookie: $cookie")"
if [[ "$logout_status" != "204" ]]; then
  printf 'Production logout returned %s; want 204.\n' "$logout_status" >&2
  exit 1
fi
revoked_body="$work_dir/revoked.json"
revoked_status="$(http_status "$revoked_body" "$backend_url/api/v2/bootstrap" -H "Cookie: $cookie")"
if [[ "$revoked_status" != "401" ]]; then
  printf 'Revoked session returned %s; want 401.\n' "$revoked_status" >&2
  exit 1
fi

printf 'Production runtime smoke passed: migrations=%s, embedding=%s, login=%s, session=%s, project-create=%s, project-update=%s, project-conflict=%s, project-history=%s, task-create=%s, task-update=%s, task-conflict=%s, task-history=%s, decision-create=%s, decision-update=%s, decision-conflict=%s, decision-history=%s, mcp-issue=%s, mcp-initialize=%s, mcp-revoke=%s, mcp-revoked=%s, knowledge-import=%s, knowledge-search=%s, cors=%s, forbidden-cors=%s, provider-fail-closed=%s, logout=%s.\n' \
	"$migration_count" "$embedding_status" "$login_status" "$me_status" "$project_status" "$project_update_status" "$project_conflict_status" "$project_history_status" "$task_status" "$task_update_status" "$task_conflict_status" "$task_history_status" "$decision_status" "$decision_update_status" "$decision_conflict_status" "$decision_history_status" "$mcp_token_status" "$mcp_initialize_status" "$mcp_revoke_status" "$mcp_revoked_status" "$source_status" "$search_status" "$cors_status" "$blocked_cors_status" "$assistant_status" "$logout_status"
