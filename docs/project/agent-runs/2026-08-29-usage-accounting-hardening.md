# Usage accounting hardening — 2026-08-29

## Scope

- preserve provider-reported input/output usage for DeepSeek assistant and
  learning flows when the provider returns it;
- stop writing reserved-token or guessed speech cost as final usage;
- mark STT, pronunciation and TTS records as unavailable when the provider
  does not expose billable usage metadata;
- expose the unavailable-record count in the usage summary and Settings so a
  zero cost total cannot be mistaken for a complete billing total.

## Implementation

- `backend/internal/ai/provider.go` exposes optional usage-aware provider
  interfaces and keeps the model returned by the provider response;
- `backend/internal/application/grounded_generator.go` records actual usage
  only and releases the preflight reservation when strict usage is unavailable;
- `backend/internal/learning/service.go` and `features.go` preserve provider
  usage for DeepSeek and mark speech usage unavailable without inventing
  tokens, seconds or cost;
- `backend/internal/domain/models.go` and the Flutter Settings model expose
  `unavailableRecords`;
- reservations remain preflight budget estimates only; they are not usage
  records and are not included in the final cost total.

## Verification

- `rtk go test ./backend/internal/learning ./backend/internal/httpapi ./backend/internal/application -count=1` — exit `0`; `76` tests passed;
- `rtk go test ./... -count=1` — exit `0`; `550` tests passed across `23` packages;
- `rtk go test -race ./... -count=1` — exit `0`; `550` tests passed across `23` packages;
- `rtk go vet ./...` — exit `0`; no issues found;
- `rtk go build ./...` — exit `0`;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk git diff --check` — exit `0`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash scripts/production_runtime_smoke.sh` — exit `0`; `19` migrations, embedding/login/session `200`, Work create/update/conflict/history `201/200/409/200`, MCP issue/initialize/revoke/revoked `201/200/204/401`, Knowledge import/search `201/200`, CORS `204/403`, provider fail-closed `500` and logout `204`.

## Boundary

- cost remains an application estimate derived from provider-reported token
  counts and the configured model rate; it is not a provider invoice;
- speech providers currently do not return a billable usage payload through
  the project interfaces, so those records remain explicitly unavailable;
- coverage is still `68.6%` against the plan threshold of `90%`;
- Flutter full-suite compiler startup, real-device/operator acceptance,
  lifecycle acceptance, independent review and human release authorization
  remain open;
- no commit, push, merge or deployment was performed.
