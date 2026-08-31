# Runtime module-manifest enforcement — 2026-08-29

## Scope

Connected the compiled module registry to the Go server composition boundary.
This slice is intentionally limited to module selection and route-surface
gating; it does not introduce dynamic plugins or change the legacy V1 API.

## Implementation

- `backend/internal/platform/builtins.go` defines the compiled-in module graph,
  defaults and `DEVENGLISH_MODULES` parser.
- `backend/cmd/server/main.go` parses the manifest at startup and exits before
  serving when the selection is unknown or misses a declared dependency.
- `backend/internal/httpapi/server.go` gates the MCP endpoint.
- `backend/internal/httpapi/v2.go` gates V2 route groups and passes the
  selected module projections into bootstrap. Work trash/purge is additionally
  gated by the Actions module so a partial composition cannot bypass the
  challenge/receipt boundary.
- `backend/internal/application/application.go` keeps bootstrap envelopes
  stable while skipping repository queries for disabled Work, Knowledge and
  Assistant modules.
- Docker and example environment files expose `DEVENGLISH_MODULES`; the
  default is `all`.

## Evidence

- `rtk go test ./backend/internal/platform ./backend/internal/httpapi
  ./backend/cmd/server -count=1` — exit `0`; `254` tests passed across `3`
  packages.
- module-aware application/bootstrap suite — exit `0`; `308` tests passed
  across `3` packages, including disabled-repository and bootstrap error-path
  assertions.
- `rtk go test ./... -count=1` — exit `0`; `976` tests passed across `23`
  packages.
- `rtk go test -race ./... -count=1` — exit `0`; `976` tests passed across
  `23` packages.
- PostgreSQL-enabled `rtk go test ./... -count=1` — exit `0`; `1044` tests
  passed across `23` packages.
- PostgreSQL-enabled `rtk go test -race ./... -count=1` — exit `0`; `1044`
  tests passed across `23` packages.
- PostgreSQL-backed coverage profile `/tmp/devenglish-go-db-20260829-bootstrap3.cov`
  — exit `0`; `9261/11985` statements (`77.27%`) and the strict seven-package
  `>=90%` gate passed.
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`.
- production-shaped `rtk bash scripts/production_runtime_smoke.sh` — exit
  `0`; `19` migrations, auth/session, Work, Knowledge, MCP issue/revoke,
  CORS and provider fail-closed checks passed.
- binary smoke with `DEVENGLISH_MODULES=platform,work` — exit `0`; Work
  projects returned `200`, while Knowledge search, Work trash and MCP returned
  `404` as disabled surfaces; the bootstrap response returned `200` with
  stable empty `sources` and `conversations` projections.
- Local runtime health remained ready at the backend and embedding endpoints;
  no credential values were included in the evidence.

## Boundary

The worktree remains dirty and operator-owned. No task lifecycle transition,
commit, push, merge or deployment was performed. Independent Sol review,
operator/device acceptance, CI on the intended delivery commit and production
release authorization remain open.
