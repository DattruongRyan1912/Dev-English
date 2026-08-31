# Wave 2 integration review R8

## Verdict

**CHANGES_REQUESTED** — reviewer Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`), turn `01a03d9f-622e-7a70-b451-dc0d95c62525`.

Per-task decisions:

- TASK-003 Knowledge: **APPROVED**.
- TASK-004 Work: **CHANGES_REQUESTED**.
- TASK-005 Connectors: **CHANGES_REQUESTED**.
- TASK-007A MCP: **APPROVED**.

## Findings

### HIGH — Work ownership is not anchored to workspace ownership

`workspaces` stores its owner independently at `infra/migrations/003_platform_foundation.sql:5`, while `projects` stores an independent `workspace_id` and `owner_user_id` pair at `infra/migrations/006_work.sql:10`. The service assumes the pair is valid (`backend/internal/work/service.go:57`), but the database does not enforce it. Sol's isolated PostgreSQL probe inserted a project in workspace A owned by user B and emitted `cross_owner_project_accepted`; the transaction was rolled back.

Required narrow repair: enforce the canonical workspace-owner pair with composite foreign keys for projects, tasks, decisions and work idempotency, and add PostgreSQL rejection tests.

### HIGH — Safe-write API and persistence schema do not share tenant scope

`SafeWriteMetadata`, challenge and receipt omit workspace identity at `backend/internal/connectors/contracts.go:140`, `backend/internal/connectors/safe_write.go:169` and `backend/internal/connectors/safe_write.go:252`. The in-memory/API lookup uses user/operation/idempotency at `backend/internal/connectors/safe_write.go:269`, while the SQL uniqueness boundary is workspace/key at `infra/migrations/003_platform_foundation.sql:131`. The receipt's challenge, workspace and user are independent foreign keys at `infra/migrations/003_platform_foundation.sql:113-115`. Sol's isolated PostgreSQL probe accepted a receipt in workspace/user B referencing a challenge from workspace/user A and emitted `cross_scope_receipt_accepted`; the transaction was rolled back.

Required narrow repair: propagate normalized workspace identity through challenge, metadata, receipt and store methods; align the database idempotency key with the API contract; add a composite challenge-scope foreign key and cross-workspace PostgreSQL/cross-service tests.

### MEDIUM — Manifest was stale during review

The canonical protocol verifier observed the accepted reviewer while the R8 entry was still running, so it returned `FAIL manifests`. This is a governance recording issue, not a source finding. The controller must record the terminal R8 result and rerun the verifier before the next review.

## Verified gates

- Exactly 34 files staged, all inside the authorized integration cones; no unstaged/untracked integration files.
- Migrations 003/004 match the explicitly authorized canonical source byte-for-byte.
- Fresh isolated migration run: exit `0`; all migrations 000–006 applied, second run was idempotent, failing migration rolled back and was not recorded.
- PostgreSQL-focused tests: exit `0`; 90 pass.
- PostgreSQL-enabled focused race: exit `0`; 307 pass, 0 skip across five packages.
- Wave package coverage remained above 90%: Knowledge 96.3%, Work 93.3%, Connectors 91.5%, MCP 95.3%.
- Repository test: exit `0`; 351 pass, 3 PostgreSQL-dependent skips in the unconfigured repository run.
- Focused/repository vet, gofmt, staged diff check and final status checks: exit `0`.
- Constraint probes: exit `0`; invalid rows were accepted by the current schema, then the probe transaction was rolled back and left zero rows.
- No live Drive, GitHub, MCP client or other provider was called. No shared/production database was used.

## Integration recommendation

Keep `integration/wave-2` frozen and uncommitted. Implement the two narrow HIGH repairs in the Work/Connectors cones and their migrations/tests, reconcile the manifest, rerun the protocol verifier, then reuse this same Sol reviewer for a bounded R9 review. Commit, push, merge and deploy remain unauthorized.
