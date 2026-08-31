# Google Drive OAuth request-context cancellation — 2026-08-29

## Scope

The preceding Drive OAuth refresh source used a fixed background context. Its
HTTP client had a 30-second timeout, but cancellation from a caller that
stopped a Drive sync could not reach the OAuth token exchange. This slice is
limited to the existing read-only Drive connector and its tests.

## Implementation

- `googleDriveTokenSource` retains refreshed tokens in memory behind a mutex;
- a concrete `TokenContext` path passes the caller context to
  `oauth2.Config.TokenSource` and preserves the shared bounded HTTP client via
  `oauth2.HTTPClient`;
- injected legacy `oauth2.TokenSource` implementations remain supported;
- static access tokens, the read-only scope and redacted public errors are
  unchanged.

## Evidence

- `rtk go test ./backend/internal/integrations ./backend/internal/connectors
  -count=1`: exit `0`, `101` tests passed;
- `rtk go test ./... -count=1`: exit `0`, `981` tests passed across `23`
  packages;
- `rtk go test -race ./... -count=1`: exit `0`, `981` tests passed across
  `23` packages;
- PostgreSQL-enabled normal and race suites: exit `0`, `1049` tests passed
  for each run across `23` packages;
- raw profile `/tmp/devenglish-go-db-20260829-drive-oauth-context.cov`:
  `9308/12034 = 77.35%`; the seven-package `>=90%` gate and R0 floor passed;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check`: exit `0`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh`: exit `0`; 19 migrations and the
  complete auth/Work/Knowledge/MCP/CORS/provider-failure sequence passed;
- `rtk bash .cyberos/cuo/gates/run-gates.sh`: exit `0`; machine gates are
  GREEN, Flutter `52` passed with `2` environment skips; CyberOS doctor was
  skipped because its memory CLI is unavailable.

## Boundary

The OAuth test uses an injected transport and never calls Google. Live consent,
refresh-token validity, quota/rate-limit behavior, production secret
provisioning, device acceptance, independent current review and release
authorization remain open. No commit, push, merge or deployment was performed
in this slice.
