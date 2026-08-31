# Dev-English Wave 2 — Review R10

Reviewer: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)

Submission: `01a03dce-7540-71e2-b7fd-1bbea65170df`

Verdict: **APPROVED**

## Per-task decisions

- TASK-003 Knowledge: approved.
- TASK-004 Work: approved.
- TASK-005 Connectors: approved after TASK-005-R4.
- TASK-007A MCP: approved.

No blocker, high, or medium finding remains in the reviewed integration scope.

## R9 finding verified as resolved

The durable receipt identity is now consistent across the API, in-memory store, deterministic ID, and PostgreSQL:

- Go lookup identity is workspace, user, operation, and idempotency key at `backend/internal/connectors/safe_write.go:277`.
- In-memory and deterministic identities use the same fields at `backend/internal/connectors/fakes.go:524` and `backend/internal/connectors/safe_write.go:525`.
- PostgreSQL enforces `(workspace_id, user_id, action, idempotency_key)` at `infra/migrations/003_platform_foundation.sql:143`.
- SQL `action` values match the Go operation constants at `backend/internal/connectors/safe_write.go:16`.
- The PostgreSQL integrity test rejects a duplicate tuple and accepts the same key for a different operation at `backend/internal/connectors/postgres_integrity_test.go:110`.

The existing workspace challenge/receipt binding remains enforced at `infra/migrations/003_platform_foundation.sql:103` and `infra/migrations/003_platform_foundation.sql:132`; durable reservation still occurs before provider invocation at `backend/internal/connectors/safe_write.go:416`.

## Independently observed commands

| Command/gate | Exit | Result |
| --- | ---: | --- |
| `rtk bash scripts/verify_multi_agent_protocol.sh` | 0 | Protocol, entrypoints, and manifests passed |
| Raw staged-cone check against base | 0 | 36 staged files; 0 outside approved cones |
| HEAD/branch/merge-base checks | 0 | Correct base and `integration/wave-2` |
| Fresh isolated `scripts/db_migrate_test.sh` | 0 | `000..006` applied; second run idempotent; intentional failure rolled back |
| PostgreSQL-enabled focused race/coverage gate | 0 | 310 passed; 0 skipped |
| `TestPostgresSafeWriteChallengeAndReceiptScope` | 0 | 1 passed |
| Live `pg_indexes` query | 0 | Four-column unique index confirmed |
| `rtk proxy go test -mod=readonly -count=1 -json ./...` | 0 | 352 passed; no package failures |
| `rtk proxy go vet ./...` | 0 | Clean |
| `rtk gofmt -d` over changed packages | 0 | Empty diff |
| `rtk proxy git diff --cached --check` | 0 | Clean |
| `rtk proxy go list ./...` | 0 | All 15 packages loaded |
| Temporary database/role cleanup | 0 | Both confirmed absent |

Measured focused coverage: Knowledge 96.3%, Work 93.3%, Connectors 91.4%, and MCP 95.3%. Store measured 13.8% and is outside the Wave 2 package coverage threshold.

## Remaining unverified items

- No live Drive or GitHub provider was called.
- No shared or production database was used.
- An upgrade from a database that had already recorded an earlier pre-R4 draft of migration 003 was not tested; the approved path is the fresh chain from the supplied base, and such a pre-release database would need a rebuild or forward migration.

The integration branch is ready for the next human-controlled gate. Commit, push, merge, and deploy remain unauthorized.
