#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${ENV_FILE:-$repo_root/.env.local}"
api_base="${API_BASE_URL:-http://localhost:8080}"
curl_bin="${CURL_BIN:-/usr/bin/curl}"
jq_bin="${JQ_BIN:-/usr/bin/jq}"
postgres_service="${POSTGRES_SERVICE:-postgres}"
postgres_user="${POSTGRES_USER:-devenglish}"
postgres_db="${POSTGRES_DB:-devenglish}"

if [[ "${DEVENGLISH_LIVE_SMOKE:-NO}" != "YES" ]]; then
  printf 'This smoke test calls configured providers and writes a disposable PostgreSQL user.\n' >&2
  printf 'Re-run with DEVENGLISH_LIVE_SMOKE=YES.\n' >&2
  exit 2
fi
if [[ ! -f "$env_file" ]]; then
  printf 'Environment file not found: %s\n' "$env_file" >&2
  exit 1
fi

bootstrap_key="$(awk -F= '$1 == "DEVENGLISH_BOOTSTRAP_KEY" {sub(/^[^=]*=/, "", $0); gsub(/^"|"$/, "", $0); print; exit}' "$env_file")"
if [[ -z "$bootstrap_key" ]]; then
  printf 'DEVENGLISH_BOOTSTRAP_KEY is required for the development smoke session.\n' >&2
  exit 1
fi

compose=(docker compose -f "$repo_root/infra/docker-compose.yml")
compose+=(--env-file "$env_file")

psql_exec() {
  "${compose[@]}" exec -T "$postgres_service" psql \
    -v ON_ERROR_STOP=1 \
    -U "$postgres_user" \
    -d "$postgres_db" \
    "$@"
}

smoke_user="rc1-smoke-$(date -u +%Y%m%d%H%M%S)-$$"
cleanup() {
  psql_exec -Atqc "DELETE FROM users WHERE id='$smoke_user'" >/dev/null 2>&1 || true
}
trap cleanup EXIT

session_json="$($curl_bin -fsS --max-time 20 \
  -X POST "$api_base/api/v1/auth/session" \
  -H 'Content-Type: application/json' \
  -H "X-Bootstrap-Key: $bootstrap_key" \
  --data "{\"userId\":\"$smoke_user\",\"displayName\":\"RC1 E2E\"}")"
token="$($jq_bin -er '.token' <<<"$session_json")"

api_get() {
  "$curl_bin" -fsS --max-time 120 "$api_base$1" \
    -H "Authorization: Bearer $token"
}

api_post() {
  "$curl_bin" -fsS --max-time 120 \
    -X POST "$api_base$1" \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    --data "$2"
}

questions="$(api_get /api/v1/diagnostic/questions)"
question_count="$($jq_bin -er '.questions | length' <<<"$questions")"
responses="$($jq_bin -c '{responses: [.questions[] | {questionId: .id, answer: .options[2]}]}' <<<"$questions")"
diagnostic="$(api_post /api/v1/diagnostic "$responses")"

mission="$(api_post /api/v1/missions/daily '{"workContext":"A retry-safe API client needs clear timeout handling and structured error messages."}')"
mission_id="$($jq_bin -er '.id' <<<"$mission")"
answer='Observed behavior: the client returns HTTP 500 after a transient upstream timeout. Expected behavior: retry bounded transient failures and return a useful error. Impact: users cannot complete the request. Next step: reproduce with logs, add a timeout test and deploy the fix.'
submission="$(api_post "/api/v1/missions/$mission_id/attempts" "$($jq_bin -cn --arg answer "$answer" '{answer: $answer}')")"

work="$(api_post /api/v1/work-context '{"sourceType":"error","title":"Retry-safe API client","content":"The API client returns HTTP 500 after a transient PostgreSQL timeout. The fix should add bounded retries, structured error handling, observability and a regression test for the failing request."}')"
library="$(api_get /api/v1/review)"
review_id="$($jq_bin -er '[.items[] | select(.kind == "vocabulary")][0].id' <<<"$library")"
review="$(api_post "/api/v1/review/vocabulary/$review_id" '{"success":true,"score":90}')"

scenarios="$(api_get /api/v1/roleplay/scenarios)"
scenario_id="$($jq_bin -er '.scenarios[0].id' <<<"$scenarios")"
conversation="$(api_post /api/v1/roleplay/conversations "$($jq_bin -cn --arg id "$scenario_id" '{scenarioId: $id}')")"
conversation_id="$($jq_bin -er '.id' <<<"$conversation")"
turn="$(api_post "/api/v1/roleplay/conversations/$conversation_id/turns" '{"answer":"I would first reproduce the timeout, inspect the request trace, then add a bounded retry with metrics and a regression test."}')"
copilot="$(api_post /api/v1/copilot '{"vietnamese":"API trả lỗi 500 khi timeout","context":"technical incident update"}')"

# This is a public repository smoke, not private-repository provisioning.
github="$(api_post /api/v1/integrations/github/import '{"url":"https://github.com/octocat/Spoon-Knife"}')"

counts_query="SELECT json_build_object(\
  'learning_state', (SELECT count(*) FROM learning_state WHERE user_id='$smoke_user'),\
  'diagnostic_results', (SELECT count(*) FROM diagnostic_results WHERE user_id='$smoke_user'),\
  'missions', (SELECT count(*) FROM missions WHERE user_id='$smoke_user'),\
  'attempts', (SELECT count(*) FROM mission_attempts WHERE user_id='$smoke_user'),\
  'evaluations', (SELECT count(*) FROM evaluations WHERE user_id='$smoke_user'),\
  'mistakes', (SELECT count(*) FROM mistakes WHERE user_id='$smoke_user'),\
  'work_context', (SELECT count(*) FROM work_context WHERE user_id='$smoke_user'),\
  'vocabulary', (SELECT count(*) FROM vocabulary WHERE user_id='$smoke_user'),\
  'vocabulary_reviews', (SELECT count(*) FROM vocabulary_reviews WHERE user_id='$smoke_user'),\
  'conversations', (SELECT count(*) FROM conversations WHERE user_id='$smoke_user'),\
  'roleplay_usage', (SELECT count(*) FROM ai_usage WHERE user_id='$smoke_user' AND feature='roleplay'),\
  'copilot_usage', (SELECT count(*) FROM ai_usage WHERE user_id='$smoke_user' AND feature='copilot')\
);"
counts="$(psql_exec -Atqc "$counts_query")"

printf 'user=%s\n' "$smoke_user"
printf 'diagnostic: questions=%s cefr=%s score=%s\n' \
  "$question_count" "$($jq_bin -r '.cefr' <<<"$diagnostic")" "$($jq_bin -r '.overallScore' <<<"$diagnostic")"
printf 'writing: mission=%s score=%s\n' "$mission_id" "$($jq_bin -r '.evaluation.score' <<<"$submission")"
printf 'work import: source=%s terms=%s\n' \
  "$($jq_bin -r '.context.sourceType' <<<"$work")" "$($jq_bin -r '.suggestedTerms | length' <<<"$work")"
printf 'review: item=%s persisted=%s\n' "$review_id" "$($jq_bin -r 'if type == "object" then "yes" else "no" end' <<<"$review")"
printf 'roleplay: conversation=%s reply_present=%s\n' \
  "$conversation_id" "$($jq_bin -r 'if ((.reply // .message // .evaluation.summary // "") | length) > 0 then "yes" else "no" end' <<<"$turn")"
printf 'copilot: simple/natural/professional=%s\n' \
  "$($jq_bin -r '[has("simple"), has("natural"), has("professional")] | map(tostring) | join(",")' <<<"$copilot")"
printf 'github: source=%s context=%s\n' \
  "$($jq_bin -r '.context.sourceType' <<<"$github")" "$($jq_bin -r '.context.id' <<<"$github")"
printf 'postgresql=%s\n' "$counts"
