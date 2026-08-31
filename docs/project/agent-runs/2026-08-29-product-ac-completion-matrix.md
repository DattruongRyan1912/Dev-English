# Product-reset acceptance matrix — 2026-08-29

## Purpose

This matrix maps the eight product-reset task specifications to observed
evidence in the current dirty checkout. It is an evidence index, not a task
status transition and not a human acceptance verdict. Frontmatter in
`docs/tasks/` remains the lifecycle source of truth.

The test references in the eight product specifications were reconciled with
the actual Go test functions and Flutter test titles on 2026-08-29. This keeps
the acceptance links executable/auditable without checking any lifecycle box
or inventing a reviewer verdict.

The declared product file-cones were also checked against the current checkout.
Four stale Flutter paths were corrected to the canonical feature-owned files;
the resulting filesystem audit reports no missing product cone path. This is a
specification repair, not a lifecycle transition.

The next reconciliation pass backfilled the phase evidence and hash-bound
manual spec audits for `TASK-PRODUCT-001` and `TASK-OPS-001`. This makes R1/R2
readable and reproducible, while leaving missing ship manifests,
uncommitted-object findings and human gates visible.

The following continuation added hash-bound manual spec audits for
`TASK-PRODUCT-002` through `TASK-PRODUCT-008`. This makes their R1 results
reproducible as well; it does not change any frontmatter lifecycle state or
turn the audits into independent review.

## Evidence key

- **Verified locally** means the named command/test or runtime artifact was
  observed with a successful exit/result in the current evidence set.
- **Pending human/external** means the implementation may have local evidence,
  but the required operator, reviewer, CI, live-provider or release decision
  is still open.
- File-cone existence is a local metadata check only; it does not prove that a
  task is committed, audited or accepted by CyberOS.
- A local pass never changes a task from `ready_to_review`, `testing` or
  `on_hold` to `done`.

## Acceptance mapping

| Task / AC | Current evidence | Decision boundary |
| --- | --- | --- |
| `TASK-PRODUCT-001` AC1 | **Verified locally** — `test/production_shell_test.dart` authenticates a production fixture, then navigates Today → Work → Knowledge → Learning → Today and asserts surface-specific content; the historical Home-only regression now has an explicit guard in `docs/project/agent-runs/2026-08-29-wave3-navigation-regression-guard.md`. | Operator U0 verdict remains pending. |
| `TASK-PRODUCT-001` AC2 | **Verified locally** — `test/workspace_state_golden_test.dart` covers loading, empty, stale/degraded, provider-error, offline, conflict and connector-sync states; `63` Flutter tests pass with `2` environment skips, including `32` focused UI/golden/state tests and keyboard push-to-talk coverage. | Real-device walkthrough remains pending. |
| `TASK-PRODUCT-001` AC3 | **Verified locally** — `docs/project/ux/HIGH_FIDELITY_SPEC.md`, `docs/project/ux/OPERATOR_VERDICT.md` and `docs/project/reports/PRODUCT-RESET-IMPLEMENTATION-REPORT.html` explicitly retain the human U0 decision. | Human must record `approved` or `changes_requested`. |
| `TASK-PRODUCT-002` AC1 | **Verified locally** — composed `/api/v2/bootstrap` and production-shaped login/session smoke return workspace/capability data; `backend/internal/httpapi` tests pass. | Task lifecycle acceptance remains open. |
| `TASK-PRODUCT-002` AC2 | **Verified locally** — PostgreSQL integration, legacy-upgrade and workspace-isolation harnesses pass; migrations `000..018` are replay-safe in disposable PostgreSQL. | Production backup/restore rehearsal remains pending. |
| `TASK-PRODUCT-002` AC3 | **Verified locally** — `test/production_shell_test.dart` proves production mode does not silently use the demo gateway. | Human review gate remains open. |
| `TASK-PRODUCT-003` AC1 | **Verified locally** — Work service and PostgreSQL tests cover workspace-scoped Project/Task/Decision CRUD, versions and conflicts. | Human sequential gate remains open. |
| `TASK-PRODUCT-003` AC2 | **Verified locally** — Work integrity tests cover history, trash, restore, purge dependency/retention boundaries and hidden normal-list records. | Purge remains a separate lifecycle operation; no autonomous purge is enabled. |
| `TASK-PRODUCT-003` AC3 | **Verified locally** — Flutter Work UI tests and state goldens cover loaded, empty and explicit conflict/reload states; configured REST trash uses the Work challenge/receipt path. | Real-device and operator acceptance remain pending. |
| `TASK-PRODUCT-004` AC1 | **Verified locally** — Knowledge import creates immutable revisions/chunks and evidence-linked claims; manual import/search tests pass. | Human task acceptance remains open. |
| `TASK-PRODUCT-004` AC2 | **Verified locally** — Drive reader tests cover paged bootstrap, incremental changes, removals and cursor behavior; connector sync is read-only. | Live Drive credentials/rate-limit behavior remains unverified. |
| `TASK-PRODUCT-004` AC3 | **Verified locally** — PostgreSQL Knowledge integrity and capacity harnesses pass; 100,000 chunks, FTS and bounded pagination pass; embedding degradation falls back to FTS. | Production topology/capacity rehearsal remains optional pending. |
| `TASK-PRODUCT-005` AC1 | **Verified locally** — assistant grounding tests enforce answer/evidence/unknown/stale contract and reject unsupported model facts. | Live provider behavior remains unverified. |
| `TASK-PRODUCT-005` AC2 | **Verified locally** — provider-failure tests and production-shaped smoke return fail-closed behavior without deterministic fallback in production mode. | Live credential/quota reconciliation remains pending. |
| `TASK-PRODUCT-005` AC3 | **Verified locally** — action HTTP/MCP tests cover challenge, expected version, one-time replay and receipt; configured REST Work trash and MCP `entity_trash` both use the canonical Work action service. | Human release/reviewer gates remain open. |
| `TASK-PRODUCT-006` AC1 | **Verified locally** — learning overlay and adaptive bilingual guidance tests cover Vietnamese/help handling, English starter output and bounded follow-up behavior. | Real-user language-level acceptance remains pending. |
| `TASK-PRODUCT-006` AC2 | **Verified locally** — Flutter roleplay/workspace tests cover editable transcript and on-demand speech controls; provider speech contracts are tested without live calls. | Real microphone/TTS device walkthrough remains pending. |
| `TASK-PRODUCT-006` AC3 | **Verified locally** — learning observation persistence is scope-isolated from Work and Knowledge in service tests and UI copy. | Human lifecycle acceptance remains open. |
| `TASK-PRODUCT-007` AC1 | **Verified locally** — MCP Streamable HTTP tests cover initialize, tool/resource discovery, JSON/SSE negotiation, scoped reads and protocol-version handling; official SDK conformance is `62/62`. Product-client setup is documented in `docs/project/MCP_CLIENT_SETUP.md`. | Operator-side Codex/Claude configuration remains unverified. |
| `TASK-PRODUCT-007` AC2 | **Verified locally** — PostgreSQL MCP token persistence covers restart, revocation, expiry and digest-only storage. | Production secret rotation/operator setup remains pending. |
| `TASK-PRODUCT-007` AC3 | **Verified locally** — production-shaped smoke covers MCP issue/initialize/revoke/revoked `201/200/204/401` and rejected replay paths. | Exact delivery-commit GitHub CI remains pending. |
| `TASK-PRODUCT-008` AC1 | **Verified locally** — production-shaped Docker smoke covers auth, embedding, Knowledge, MCP and provider-fail-closed checks; `19` migrations complete. | This is not production deployment evidence. |
| `TASK-PRODUCT-008` AC2 | **Verified locally** — legacy-upgrade preservation and 100,000-chunk capacity harnesses pass without integrity errors. | Production backup/restore rehearsal remains pending. |
| `TASK-PRODUCT-008` AC3 | **Verified locally** — STATUS, ROADMAP, TEST_LOG, changelog, runbook and implementation report distinguish local evidence from open gates. | Human reviewer/operator must accept the lifecycle and release boundary. |

## Current global gates

- Local machine gate: **verified**, `bash .cyberos/cuo/gates/run-gates.sh`
  exited `0`; the doctor step was skipped because the local CyberOS memory CLI
  is unavailable.
- Local backend: **verified**, PostgreSQL-enabled tests/race tests `1060` each
  across `23` packages; default-profile tests/race tests pass `985` each across
  `23` packages; seven required package raw coverage gates pass at the latest
  profile (`9354/12085`, `77.40%`).
- Local frontend: **verified**, analyze, `63` Flutter tests plus `2` environment
  skips, `32` focused UI/golden/state tests, Chrome smoke, web release,
  release APK and emulator smoke pass.
- Task reconcile: **R1/R2 pass, release still gated**, the eight Product tasks
  and OPS task now have hash-bound audit evidence where reconciled; R3 is
  absent and R4 is red because deliverables are not in `HEAD`. P007/P008 have
  R5 refusals for their untracked production smoke scripts, and OPS retains an
  R5 refusal for its untracked verifier citations.
- Current independent Sol review: **pending**, latest bounded attempt ended at
  the usage limit without a verdict; the attempt is recorded in
  `docs/project/agent-runs/2026-08-30-sol-review-usage-limit.md`. The historical
  approval is not reused as a current verdict.
- Human/external release gates: **pending**, including U0 device verdict,
  CyberOS task lifecycle acceptance, exact-commit GitHub CI, live provider and
  billing reconciliation, and commit/push/merge/deploy authorization.

No task frontmatter, commit, push, merge or deployment was changed by this
matrix.
