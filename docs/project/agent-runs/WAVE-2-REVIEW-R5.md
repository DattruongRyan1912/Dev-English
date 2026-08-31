# Wave 2 review R5 evidence

- Reviewer: Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)
- Thread turn: `01a03d2d-e520-7cc3-bd69-17f4d7d5a8dd`
- Observed terminal state: `idle` / `completed`
- Observed at: `2026-08-26T08:44:53Z`
- Verdict: `CHANGES_REQUESTED`

## Per-task decisions

- `TASK-003-R1` Knowledge: `APPROVED`; package coverage `96.3%`. PostgreSQL invariant tests were present but skipped because `DEVENGLISH_TEST_DATABASE_URL` was unset.
- `TASK-004-R1` Work: `APPROVED`; package coverage `93.3%`.
- `TASK-005-R1` Connectors: `CHANGES_REQUESTED`; package coverage `93.7%`.

## High-severity findings

1. Sync revision deduplication is not workspace-scoped. `backend/internal/connectors/sync.go:47,103,305` passes only a global revision key/item to the revision store. The observed consequence is that a revision imported into workspace A can be treated as already imported in workspace B. The repair must add workspace scope to the store API and persistence keys, propagate it through Drive/GitHub imports, and test the same revision independently in two workspaces.
2. Durable idempotency is recorded after the external mutation. `backend/internal/connectors/safe_write.go:377,390,393,408` looks up the receipt, consumes the challenge, calls the provider, then saves the receipt. If the provider succeeds and receipt persistence fails, a later challenge can invoke the same operation again. The repair must reserve the scoped key before mutation, represent pending/uncertain outcomes, prevent re-invocation, and add a provider-success/receipt-failure test asserting one provider call.

## Commands and observed exits

- Focused race/coverage tests: exit `0`; Knowledge `96.3%`, Work `93.3%`, Connectors `93.7%`.
- Focused `go vet`: exit `0` for all three packages.
- Repository tests: exit `0`; RTK reported Knowledge `134`, Work `211`, Connectors `85` passing tests across 12 packages respectively.
- Repository `go vet`: exit `0` for all three worktrees.
- `gofmt -d`: exit `0` for all three cones.

## Unverified items

- Migrations `003_platform_foundation.sql`/the canonical workspace owner were absent from the supplied base, so migrations 005/006 were not applied to PostgreSQL.
- Live Drive/GitHub calls were not made; fakes were used.
- Knowledge PostgreSQL tests were skipped because the database URL was unset.

## Integration decision

Do not integrate any worktree. Keep the three reviewed worktrees frozen. The durable next handoff is `TASK-005-R2`; after its bounded repair, reuse the same Sol reviewer for `WAVE-2-REVIEW-R6`. No replacement reviewer, duplicate assignment, or integration is authorized.
