# Wave 2 review — R2

- Reviewer: Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)
- Verdict: `CHANGES_REQUESTED`
- Canonical checkout: `/Users/ryantruong/Project/Orther/Dev-English`
- Canonical HEAD at review: `d39019abb93b333654b14e068a1ae0d476b1cae2`
- Review boundary: read-only; no files were edited, staged, committed, pushed, merged or deployed.

## Per-task decisions

| Task | Decision | Main reason |
| --- | --- | --- |
| TASK-003 | `CHANGES_REQUESTED` | Coverage 51.1%; knowledge migration lacks canonical workspace FKs and complete evidence-integrity validation. |
| TASK-004 | `CHANGES_REQUESTED` | Declared `006_work.sql` is absent; package has no tests; history can disclose data after same-ID reuse across workspaces. |
| TASK-005 | `CHANGES_REQUESTED` | Frozen output conflicts with the canonical connector API; only `errors.go` was present in the reviewed worktree; formatting/tests are incomplete. |
| TASK-007A | `CHANGES_REQUESTED` | Package has no tests; mutating tool registration can fail open; issued-token reveal state is copyable; replay keeps the old JSON-RPC ID. |

## Blocking gates

- Configured CyberOS coverage minimum is 90%; measured package coverage was TASK-003 51.1%, TASK-004 0%, TASK-005 0%, TASK-007A 0%.
- No Wave 2 output is approved for integration until focused positive/negative, isolation, replay and concurrency tests are added and pass.
- The manifest must retain frozen paths, worker test evidence, timestamps and terminal reasons before integration.

## Required repair themes

1. Add workspace-root foreign keys and PostgreSQL isolation/evidence-integrity tests for Knowledge.
2. Add the missing Work migration; scope history by workspace/user and test purge plus same-ID reuse.
3. Reconcile Connectors with the canonical API; restore tests, formatting and safe structured error serialization.
4. Require explicit allow-listed write scopes for mutating MCP tools; make token reveal state shared/private and fix replay request IDs.
5. Run combined integration validation only after all four repair tasks pass the coverage and security gates.

## Commands observed

- `rtk bash scripts/verify_multi_agent_protocol.sh` — exit 0.
- Focused race tests — TASK-003 exit 0 with 8 tests; TASK-004, TASK-005 and TASK-007A reported no tests.
- Focused coverage — TASK-003 51.1%; TASK-004, TASK-005 and TASK-007A 0.0%.
- Canonical `rtk go test -mod=readonly -count=1 ./...` — exit 0, 82 tests passed.
- `rtk gofmt -d backend/internal/connectors/*.go` — exit 1 with formatting differences.
- `rtk ls -l infra/migrations/006_work.sql` — exit 1; file absent.

## Unverified

- SQL migrations were not applied because the review was read-only and no read-only database harness was available.
- A combined integrated tree was not assembled; combined compilation remains unverified.
