# Drive OAuth refresh — 2026-08-29

## Scope

Keep the V1 Google Drive connector read-only while making long-running local
and development sessions independent from a short-lived static access token.

## Implemented

- `backend/internal/integrations/google_drive.go` accepts either
  `GOOGLE_DRIVE_ACCESS_TOKEN`/`DRIVE_ACCESS_TOKEN` or an OAuth refresh source
  built from `GOOGLE_DRIVE_CLIENT_ID`, `GOOGLE_DRIVE_CLIENT_SECRET` and
  `GOOGLE_DRIVE_REFRESH_TOKEN`.
- The refresh source requests only the Google Drive read-only scope
  `https://www.googleapis.com/auth/drive.readonly`.
- Static access tokens retain precedence for bounded local smoke and existing
  fixtures.
- Refreshed tokens remain in memory; refresh failures and provider bodies are
  reduced to generic redacted errors.
- `.env.example`, `infra/production.env.example` and `README.md` document the
  two configuration modes without including credential values.

## Verification

- `rtk go test ./backend/internal/integrations ./backend/internal/connectors
  -count=1` — exit `0`; `99` tests passed across two packages.
- The new OAuth tests use `httptest` and an in-memory token source. They do not
  call Google or validate a live account.
- `rtk gofmt -w backend/internal/integrations/google_drive.go
  backend/internal/integrations/connectors_read_test.go` — exit `0`.
- After the change, the default Go normal/race runs passed `979` tests each;
  the PostgreSQL-enabled normal/race runs passed `1047` tests each across `23`
  packages. The disposable PostgreSQL profile measured `9287/12007`
  statements (`77.35%`) and the strict seven-package gate passed.
- The production-shaped Docker smoke rebuilt the backend and passed its full
  auth/Work/Knowledge/MCP/CORS/provider-failure sequence.

## Boundary

This closes the token-source implementation and local redaction tests only.
Live OAuth consent, refresh-token validity, Drive quota/rate-limit behavior and
production secret provisioning remain open acceptance items. No Drive write
scope was added and no external data was changed.
