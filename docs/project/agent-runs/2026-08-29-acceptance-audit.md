# Acceptance audit — 2026-08-29

## Purpose

Record the current product-reset acceptance boundary from observed evidence.
This audit separates local implementation proof from operator, reviewer, CI
and production decisions. A passing local command is not treated as a release
approval.

The follow-up release web smoke on 2026-08-29 loaded `http://localhost:8093/`
as `DevEnglish`, rendered the canonical Today workspace and captured no
warning/error browser log entries. This refresh confirms the blank-page
regression is not reproduced in the current local web artifact; it does not
close the real-device or operator U0 gate.

The subsequent web-startup hardening added a static boot card to
`web/index.html`, including reduced-motion behavior and a 30-second reloadable
failure state. A fresh reload of the rebuilt artifact rendered the Flutter
host surface and removed the boot layer (`bootNodes=0`, `flutterPanes=1`) with
no captured browser warnings/errors. This is startup-resilience evidence, not
real-device acceptance or a latency SLO.

The final machine-gate refresh after that web change exited `0`: build, lint,
backend tests, content factory, Flutter tests, coverage and status generation
passed, with `52` Flutter tests passed and `2` environment skips. CyberOS
reported `GATES: GREEN (machine gates only)`; its doctor step remained an
environment skip because the local CyberOS memory CLI is unavailable. This
does not create a human lifecycle, reviewer or release verdict.

The subsequent action-boundary continuation verified that configured REST
legacy trash routes cannot bypass the confirmation service: Project, Task and
Decision all require a `work.entity.trash` challenge and return the canonical
Work receipt. The MCP full-application path was tested independently with the
same receipt boundary. Compatibility fixtures without `Actions` remain
deliberately unconfigured and are not production evidence.

The repository machine gate was rerun after that continuation. It exited `0`
for build, lint, backend tests, content-factory, Flutter tests, coverage and
status-page generation, while explicitly retaining the HITL requirement. The
CyberOS doctor step was skipped because the local CyberOS memory CLI is not
available; this is recorded as an environment limitation, not as a pass.

The task metadata audit then found four stale Flutter file-cone entries. They
pointed at removed `lib/src/screens/workspace_*.dart` paths while the canonical
implementations live under their owning `lib/src/features/` directories. The
four entries were corrected in the product task specs, and a filesystem audit
confirmed that every product `new_files` and `modified_files` path now exists.
This is metadata repair only; it does not create an audit verdict or advance a
task lifecycle.

The PostgreSQL-backed refresh then found the development volume one migration
behind the code: `018_assistant_conversation_context.sql` was pending. The
append-only migration was applied locally, the two conversation persistence
regressions passed, and the complete profile passed `1024` tests plus the race
suite with the same count across 23 packages. The Flutter suite remained at 52
passed with two environment skips, and the backend-aware Chrome smoke passed.

The CyberOS-only scope verifier was also rerun. Its entrypoint, surface and
static-runtime checks passed, but `backup_and_scope` still fails on the
protected pre-existing `.github/workflows/ci.yml` WIP hash. That file was not
reverted or rewritten; the failure remains an explicit scope/reconciliation
item for human review.

The task evidence ladder was then backfilled for `TASK-PRODUCT-001` and
`TASK-OPS-001`. Each task now has a co-located context map, edge-case matrix,
implementation evidence plan, observability evidence note and a hash-bound
manual spec audit. The audit is explicitly post-hoc and spec-only because no
executable task-audit runner is present in this checkout; it is not presented
as an independent Sol verdict or a lifecycle transition.

The following continuation backfilled the missing hash-bound manual
spec-correctness audits for `TASK-PRODUCT-002` through `TASK-PRODUCT-008`.
Their reconcile reports now pass R1/R2, while retaining the R3/R4/R5 and human
boundaries. The exact audit files and command results are recorded in
`docs/project/agent-runs/2026-08-29-product-task-audit-backfill.md`.

The read-only reconcile was rerun in normal and `--run-tests` modes. Both tasks
now pass R1 (spec integrity) and R2 (phase artefacts). R3 has no ship manifest,
and R4 remains red because the declared deliverables are not in `HEAD`; OPS
also has an R5 refusal because its cited verifier files are not tracked at
`HEAD`. The resulting recommendation remains `route_back` / HITL, which is the
correct result for this dirty, operator-owned worktree.

The continuation after this audit connected the compiled module registry to
runtime route registration. Its targeted suite passed `251` tests; a binary
smoke with `DEVENGLISH_MODULES=platform,work` kept Work available and returned
`404` for disabled Knowledge, Work trash and MCP surfaces. A fresh
production-shaped Docker smoke also passed. This extends local implementation
evidence only; it does not change the independent-review, operator-acceptance
or Git/release boundaries below.

The same slice was then tightened at the bootstrap boundary: selected module
projections are loaded only when enabled, disabled repositories are not
queried, and the JSON envelope keeps empty arrays for disabled projections.
The application/HTTP/platform regression set passed `308` tests, while the
runtime platform/HTTP/server subset passed `254`; full default-profile `976`
and PostgreSQL-enabled `1044` test runs also passed, including their race
variants. The fresh raw coverage profile passed at `77.27%` overall and the
seven-package `>=90%` gate. A real binary smoke also returned the expected `200/404`
surface matrix and stable disabled arrays. This remains local evidence only.

The latest continuation added an OAuth refresh source to the read-only Drive
connector while retaining the static access-token path for bounded local
smoke. The source requests only the Drive read-only scope, keeps refreshed
tokens in memory, redacts refresh failures, uses the same bounded 30-second
HTTP client for OAuth refresh and Drive API calls, and propagates request
cancellation to the token exchange. The integration tests use `httptest`, an
injected token endpoint and in-memory token sources; no live Google request
was made. Current full runs are `981` default-profile tests and `1049`
PostgreSQL-enabled tests, with the refreshed raw profile at
`9308/12034 = 77.35%` and the same seven-package gate passing. Live OAuth
consent, quota/rate-limit behavior and production secret provisioning remain
open.

The workspace-resolution continuation then added a fail-closed repository
boundary for legacy topology. A genuinely fresh owner may receive one
deterministic workspace under an owner-row lock; owner mismatch, deleted
targets, multiple active mappings, missing owners and ambiguous historical
mapping now return explicit errors, with the HTTP boundary mapping unresolved
operator cases to `503`. The default suite passed `985` tests, the
PostgreSQL-enabled suite passed `1060` tests in normal and race modes, and the
raw profile passed at `9354/12085 = 77.40%`. No production backfill or
automatic legacy mapping was performed.

## Evidence-backed status

| Area | Status | Evidence | Remaining owner/action |
| --- | --- | --- | --- |
| Backend/application | PASS locally | PostgreSQL-enabled Go tests/race (`1060` each across 23 packages), vet, build, Work/Knowledge/Assistant/action, workspace-resolution and isolation tests in `docs/project/STATUS.md` | None for the local implementation gate |
| MCP/REST parity | PASS locally | Streamable HTTP tests, official Go SDK conformance, auth/scope/replay and production-shaped smoke | Operator must configure and verify the Codex/Claude clients |
| Flutter/web/mobile-sized UI | PASS locally; device gate open | Analyze, 52 tests plus 2 environment skips, Chrome route smoke, web/APK builds and 390x844/430x932 goldens | Operator must perform the U0 walkthrough on a real device and record `approved` or `changes_requested` |
| Migration/recovery | PASS locally | `docs/project/agent-runs/2026-08-29-migration-dry-run.md`: dry-run, 19 migrations, idempotency and rollback | Production backup/restore rehearsal remains an operator task |
| Capacity | PASS locally | Disposable 100,000-chunk FTS and bounded-pagination harness | Repeat against the selected production topology if required |
| Coverage | PASS locally | Raw seven-package `>=90%` gate and R0 total-floor comparison; current `9354/12085` is above `4269/6393` | Run GitHub CI from the delivery commit and retain its artifact |
| CyberOS lifecycle | OPEN | `docs/tasks/BACKLOG.md` preserves the current `ready_to_review`/`testing`/`on_hold` states | Human must accept each lifecycle gate; do not set `done` automatically |
| Task reconciliation | ROUTE BACK / HITL | `task-reconcile` now reports R1/R2 pass for `TASK-PRODUCT-001` and `TASK-OPS-001`; R3 is absent, R4 is red for uncommitted deliverables, and OPS R5 refuses untracked cited verifier files at current `HEAD` | Preserve the working tree; human/reviewer must decide whether to authorize a delivery commit and lifecycle transition |
| Task file-cone metadata | VERIFIED LOCALLY | Product spec path audit reports every declared `new_files`/`modified_files` path exists; four stale Flutter paths were corrected | Re-run independent task review after the delivery state is intentionally committed |
| Independent current review | OPEN | The latest bounded Sol attempt ended at the usage limit without a verdict; prior repair review is historical evidence only | Call one bounded Sol read-only review when quota is available; never replace or parallelize it |
| Providers and billing | OPEN | Local fake/provider-failure and contract checks pass | Reconcile live credentials, provider usage and billing in the intended environment |
| Production release | OPEN | Production-shaped Docker/Caddy artifacts and runbook exist | Operator supplies target/domain/DNS/secrets, authorizes and executes deployment |
| Git release operations | NOT AUTHORIZED | No release authorization is recorded for this continuation | Human separately authorizes commit, push, merge and deploy |

## Next action order

1. Decide the authoritative mapping for zero, multiple, deleted or restored
   legacy workspaces before enabling production backfill or automatic resolver
   migration.
2. Record the operator U0/device verdict and human lifecycle decisions for the
   current task chain.
3. When reviewer quota is available, run exactly one bounded independent Sol
   review against the frozen delivery state and record its verdict.
4. If the human authorizes a delivery commit, commit the intended cone and
   rerun reconcile; only then can R4/R5 become meaningful at `HEAD`.
5. After explicit push authorization, wait for CI on that exact commit; do not
   treat local CI configuration as a CI result.
6. Complete provider/billing reconciliation and production backup, DNS,
   certificate, readiness and browser evidence before any go-live decision.

## Decision

Local implementation evidence is substantially complete through S7, but the
release is **not accepted**. No task status, commit, push, merge or deployment
was changed by this audit.
