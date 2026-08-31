# DevEnglish Wave 2 — Repair R3 Evidence

## Scope

This repair addresses the two HIGH findings from `WAVE-2-REVIEW-R8` on the
dedicated integration worktree:

- `TASK-004-R2`: enforce the workspace-owner pair for Projects, Tasks,
  Decisions and Work idempotency rows.
- `TASK-005-R3`: carry `workspaceID` through safe-write metadata, challenges,
  receipts and receipt-store lookup/reservation identity.

No canonical WIP outside the approved Wave 2 cones was copied or modified.
Commit, push, merge and deploy remain unauthorized.

## Implementation

- Added a composite `(id, owner_user_id)` key to `workspaces`.
- Added composite owner-scope foreign keys to the Work tables.
- Added composite challenge scope and challenge-to-receipt scope foreign keys.
- Made challenge/receipt in-memory stores and deterministic receipt IDs
  workspace-aware.
- Made guarded safe-write metadata require a normalized workspace ID.
- Added unit coverage for workspace-scoped reservations and challenge mismatch.
- Added opt-in PostgreSQL rejection tests for cross-owner Work rows and
  cross-scope safe-write rows.

## Files changed in the integration worktree

- `infra/migrations/003_platform_foundation.sql`
- `infra/migrations/006_work.sql`
- `backend/internal/connectors/contracts.go`
- `backend/internal/connectors/fakes.go`
- `backend/internal/connectors/safe_write.go`
- `backend/internal/connectors/contracts_test.go`
- `backend/internal/connectors/coverage_edges_test.go`
- `backend/internal/connectors/safe_write_service_test.go`
- `backend/internal/connectors/postgres_integrity_test.go`
- `backend/internal/work/postgres_integrity_test.go`

The integration branch contains the previously approved Wave 2 files plus
these bounded repairs. There are 36 staged files in total and no unstaged or
untracked files in the integration worktree after staging.

## Verification

- Fresh isolated PostgreSQL migration chain, migrations `000` through `006`:
  exit `0`.
- Second migration run was idempotent: all seven migrations were already
  applied.
- Intentional failing migration rolled back and was not recorded.
- PostgreSQL-enabled focused race test across Knowledge, Store, Work,
  Connectors and MCP: exit `0`, `310 passed`, `0 failed`.
- Normal repository test: exit `0`, `352 passed`.
- `go vet ./...`: exit `0`.
- `gofmt -d` on changed Go cones: exit `0` with no output.
- `git diff --cached --check`: exit `0`.
- Protocol verifier before handoff: `PASS protocol`, `PASS entrypoints`,
  `PASS manifests`.

The PostgreSQL database used for these checks was an isolated disposable
Compose project; the shared development database was not touched.

## Review handoff

The exact same Sol Ultra reviewer session is retained for `WAVE-2-REVIEW-R9`.
R9 must independently inspect the source and test output, with special focus
on composite foreign-key behavior, API/store scope propagation, migration
safety and regression coverage. No release decision has been made yet.
