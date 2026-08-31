# Dev-English Wave 2 — Repair R4

Status: implementation and gates complete; awaiting the retained Sol Ultra review R10.

## Finding addressed

WAVE-2-REVIEW-R9 left one TASK-005 HIGH finding: the durable PostgreSQL receipt uniqueness index used only `(workspace_id, idempotency_key)`, while the Go `SafeWriteReceiptStore` contract scopes lookup, reservation, and deterministic receipt identity by `(workspace_id, user_id, operation, idempotency_key)`.

## Bounded changes

- Updated `infra/migrations/003_platform_foundation.sql` so `action_receipts_active_idempotency_uidx` uses `(workspace_id, user_id, action, idempotency_key)`. The SQL `action` column is the persisted form of the Go operation.
- Extended `backend/internal/connectors/postgres_integrity_test.go` to prove that:
  - a duplicate receipt with the same workspace, user, operation, and idempotency key is rejected;
  - the same workspace, user, and idempotency key with a different operation is accepted.
- No source outside the TASK-005 cone was changed. No provider call, credential, shared database, commit, push, merge, or deploy was performed.

## Verification evidence

All commands below ran against the dedicated integration worktree `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-2-integration` unless noted.

| Check | Result |
| --- | --- |
| Fresh isolated migration chain `000..006` | exit 0; all seven migrations applied |
| Second migration run | exit 0; all seven migrations reported already applied |
| Intentional failing migration rollback | exit 0; rollback and non-recording checks passed |
| PostgreSQL focused race suite | exit 0; 310 tests in 5 packages |
| `TestPostgresSafeWriteChallengeAndReceiptScope` | exit 0; 1 test |
| Repository unit suite | exit 0; 352 tests in 15 packages |
| `go vet ./...` | exit 0; no issues |
| `gofmt -d backend/internal/connectors backend/internal/work` | exit 0; clean |
| `git diff --cached --check` | exit 0 |
| Live PostgreSQL index inspection | confirmed `(workspace_id, user_id, action, idempotency_key)` |

The isolated PostgreSQL container is the temporary `devenglish-wave2-r8` Compose project and is separate from the shared development database. The focused PostgreSQL tests are opt-in through `DEVENGLISH_TEST_DATABASE_URL`; the ordinary repository suite remains provider-free.

## Handoff

The next gate is `WAVE-2-REVIEW-R10` in the existing Sol Ultra reviewer session. The reviewer must inspect the source and fresh test output independently and return `approved`, `changes_requested`, or `blocked`. Commit, push, merge, and deploy remain unauthorized.
