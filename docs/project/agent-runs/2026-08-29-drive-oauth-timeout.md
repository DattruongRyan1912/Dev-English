# Google Drive OAuth refresh timeout hardening — 2026-08-29

## Scope

Bound the OAuth refresh path used by the read-only Google Drive connector. The
previous source created its token source with `context.Background()` and did
not attach the Drive HTTP timeout to the OAuth token exchange. A slow or stuck
refresh could therefore outlive the 30-second request boundary.

## Change

- `NewGoogleDriveFromEnv` now creates one 30-second HTTP client for the Drive
  API and OAuth token exchange.
- The OAuth token source receives that client through the official
  `oauth2.HTTPClient` context key.
- Static access-token compatibility, read-only scope and in-memory token
  behavior are unchanged.
- The test helper keeps the OAuth client injectable so the token exchange is
  exercised without contacting Google.

## Evidence

- `rtk go test ./backend/internal/integrations -count=1` — exit `0`; `15`
  tests passed.
- `rtk go test ./backend/internal/integrations ./backend/internal/connectors
  -count=1` — exit `0`; `100` tests passed across 2 packages.
- `rtk go test ./... -count=1` — exit `0`; `980` tests passed across 23
  packages.
- `rtk go test -race ./... -count=1` — exit `0`; `980` tests passed across 23
  packages.
- PostgreSQL-enabled normal and race runs — exit `0`; `1048` tests passed in
  each run across 23 packages.
- PostgreSQL coverage profile
  `/tmp/devenglish-go-db-20260829-drive-oauth-timeout.cov` — `9292/12014`
  statements (`77.34%`); the raw seven-package `>=90%` gate and R0 total
  floor both passed.
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`.
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh` — exit `0`; 19 migrations and the
  auth, Work, Knowledge, MCP, CORS and provider-fail-closed checks passed.
- `rtk bash .cyberos/cuo/gates/run-gates.sh` — exit `0`; machine gates GREEN,
  Flutter `52` tests passed with `2` environment skips, and doctor was skipped
  only because the local CyberOS memory CLI is unavailable.

## Boundary

This is local reliability evidence only. It does not validate live Google OAuth
consent, refresh-token validity, quota/rate limits, production secret
provisioning, operator acceptance, independent review or Git/release actions.
