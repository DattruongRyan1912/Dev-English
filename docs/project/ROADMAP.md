# DevEnglish V1 rebuild roadmap

This roadmap supersedes the old learning-only phase labels. The source of truth
for scope and acceptance is `REBUILD_PLAN.md` revision 2.0; this file tracks the
implementation order and evidence boundary.

## R0 — Baseline and UX lock

Status: **reference package prepared; operator/device verdict required**.

- preserve the existing WIP checkout and record the measured baseline;
- define Today/Work/Knowledge/Learning flows;
- lock screenshot audit, low-fi wireframes, token sheet, high-fi specification
  and prototype;
- review mobile 390×844/430×932 and desktop 1440×900 before broad UI work.

Evidence: `docs/project/ux/`.

## S1 — Walking skeleton

Status: **implemented and locally verified**.

- API v2 bootstrap and canonical workspace shell;
- Flutter API/domain/controller contracts;
- basic Today/Work/Knowledge/Learning routing;
- report: `docs/project/reports/S1-WALKING-SKELETON-REPORT.md`.

## S2 — Work depth

Status: **implemented; final product acceptance open**.

- versioned Project/Task/Decision CRUD;
- history, optimistic conflicts, trash/restore/purge and audit boundaries;
- internal action preview/idempotency/receipt semantics;
- Today priority and next-action UI;
- conflict/trash/empty/error UX plus final unit/race evidence are present;
  the raw 90% touched/new package gate now passes locally; product acceptance
  remains open.

## S3 — Knowledge and connectors

Status: **implemented; provider and release evidence open**.

- source/item/revision/chunk/topic/claim/evidence model;
- exact search, multilingual-e5-small sidecar and RRF hybrid retrieval;
- Drive/GitHub read-only incremental sync, full bootstrap/removal tombstones,
  workspace-scoped identity and stale/degraded state;
- disposable migration dry-run/application/rollback evidence and 100,000-chunk
  capacity evidence are present; live provisioning and
  broader connector acceptance remain open.

## S4 — Grounded assistant and safe actions

Status: **implemented; production acceptance open**.

- capability-based DeepSeek routing and persistent conversations;
- strict evidence/unknown/stale response contract;
- usage accounting, per-feature reservations and transient retry;
- challenge/confirm/receipt boundary for limited GitHub writes;
- billing reconciliation and production credential acceptance remain open;
  provider-failure and fail-closed runtime evidence is present locally.

## S5 — Learning and voice overlay

Status: **implemented; UX replacement and device review open**.

- work-derived English missions and observations;
- Vietnamese help with immediately usable English starter;
- editable STT transcript and opt-in TTS/pronunciation;
- canonical assistant push-to-talk composer with editable transcript and
  on-demand read-aloud;
- keep learning observations separate from canonical Work/Knowledge;
- move remaining legacy learning flows behind the Learning surface.

## S6 — MCP parity

Status: **implemented; SDK conformance passed, operator configuration open**.

- Product-client setup and least-privilege acceptance steps are documented in
  `docs/project/MCP_CLIENT_SETUP.md`; actual Codex/Claude connection,
  scoped discovery and revoke/401 still require operator execution.
- Streamable HTTP `/mcp` with separate bearer token and scopes;
- read/write/confirmed tools and resources;
- shared application services with REST semantics;
- digest-only persistent token lifecycle, replay, conflict, auth, scope and
  secret-redaction tests;
- validate with Codex/Claude-compatible clients without granting extra scope;
  the official Go SDK conformance test passes, while operator-side Codex/Claude
  configuration remains open.

## S7 — Cutover and hardening

Status: **local implementation evidence complete; release acceptance open**.

- backfill the deterministic default workspace while preserving legacy data;
- rollback flag/runbook and compatibility adapters;
- Docker E2E with PostgreSQL, backend and embedding sidecar;
- 100k-chunk capacity, race, responsive/state-golden, offline, provider
  failure and secret-redaction gates;
- update status/changelog/report and obtain independent review plus human
  authorization before release operations.

Current evidence: `docs/project/reports/PRODUCT-RESET-IMPLEMENTATION-REPORT.html`,
`docs/project/agent-runs/2026-08-29-acceptance-final.md`,
`docs/project/agent-runs/2026-08-29-r0-baseline.md`,
`docs/project/agent-runs/2026-08-29-migration-dry-run.md`,
`docs/project/agent-runs/2026-08-29-http-boundary-and-coverage-refresh.md` and
`docs/project/agent-runs/2026-08-29-acceptance-refresh.md` and
`docs/project/agent-runs/2026-08-29-drive-oauth-context-cancellation.md`.
The workspace-resolution guard is recorded in
`docs/project/agent-runs/2026-08-29-workspace-resolution-guard.md`; it fails
closed on ambiguous legacy topology and maps unresolved operator cases to
`503` without performing production backfill.
The latest local runtime smoke passed with migration `19` and the MCP
issue/initialize/revoke/revoked sequence `201/200/204/401`. The remaining
release boundary is explicit: the current PostgreSQL-enabled all-backend profile
is `77.40%` (`9354/12085` statements), above the recorded R0 baseline
`4269/6393 = 66.78%` from `104304f12f411dd534aa4b385e964b3b8c11ae43`. The
raw-count checker
`scripts/backend_coverage_gate.mjs` passes the seven V1 core packages at
`>=90%`: Application `90.60%`, Assistant `92.88%`, Connectors `90.06%`, HTTP
API `90.12%`, Knowledge `90.25%`, MCP `90.84%` and Work `90.03%`. The checker
is wired into `.github/workflows/postgres-integrations.yml` with the
machine-readable baseline manifest
`docs/project/coverage/2026-08-29-r0-backend-coverage-baseline.json`; CI has
not yet run from this local checkout. The baseline record is
`docs/project/agent-runs/2026-08-29-r0-baseline.md`. U0 still needs an operator/device
verdict; lifecycle acceptance, an independent current review and human release
authorization remain open. No commit/push/merge/deploy is implied.

## Historical gate update — 2026-08-29

The MCP write boundary now binds Project, Task, Decision and manual Knowledge
import to the authenticated token principal even when the request context has
no caller identity. The latest assistant continuation also pins canonical
Project/Task/Decision/Knowledge Source context through retrieval and durable
conversation state, and exposes lightweight conversation history with lazy
transcript loading. Local verification is 837 Go tests, 837 race tests, clean
vet/build, official MCP Go SDK conformance `62/62`, 19-migration
production-shaped smoke including Work history routes, and Flutter analyze
plus a web release build using `--no-wasm-dry-run`; Android debug and release
artifacts also build locally. The latest full Flutter
suite passed `49` tests with `2` environment skips after the local Dart AOT
runtime path was repaired.
Fresh browser visual smoke rendered the rebuilt Today and Learning surfaces with
no console warnings/errors; the Flutter Chrome smoke covers all four
destinations and mobile goldens cover 390x844/430x932. Real-device/operator
acceptance remains open.
Usage accounting now preserves provider-reported DeepSeek usage and exposes
unavailable speech/provider records without counting preflight reservations as
final usage.

The previous checkpoint recorded `75.1%` overall coverage and is retained for
history. The current raw-count gate and profile are recorded in
`docs/project/agent-runs/2026-08-29-acceptance-final.md`. U0 operator/device
acceptance, lifecycle acceptance, an independent current review and human
release authorization remain open. The canonical Flutter shell now imports
public Today/Work/Knowledge/Learning feature boundaries. Screen implementations
are now located under their owning feature directories; the old workspace
screen remains only as a compatibility facade.

## Release gate

No slice is called release-complete from a file list alone. The final tree must
have command exit codes, runtime evidence, independent review and human
authorization recorded. Commit, push, merge and deploy remain separate actions.
