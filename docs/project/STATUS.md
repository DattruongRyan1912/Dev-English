# DevEnglish product-reset status

Last updated: 2026-08-30

Overall state: **V1 implementation evidence is substantially complete through
the local S7 gates. The strict 90% gate for the seven V1 core packages passes
on a PostgreSQL-enabled profile, and the current overall coverage is above the
recorded R0 baseline; operator/device acceptance, lifecycle acceptance,
current independent review and release authorization remain open.**

The canonical Flutter shell now keeps compatibility learning and Settings
behind strict lazy data gates: it does not render legacy sample data while a
backend request is pending or unavailable, deduplicates concurrent learning
loads, and exposes retry states. The explicit legacy rollback shell retains
its development fallback. Focused production-mode and widget checks for this
boundary passed on 2026-08-30. Logout and session expiry now invalidate
in-flight controller generations and reset the workspace controller, so late
responses cannot repopulate the previous user's state. The result does not
close the remaining operator, CI, provider or release gates.

The application search boundary was also hardened on 2026-08-30: it now keeps
the relevance order supplied by Knowledge hybrid/lexical searchers through the
combined result limit instead of re-sorting hits by ID. The focused application
regression and the full backend test/vet gates passed; this does not change the
open acceptance or release boundaries.

The canonical bootstrap contract now also returns an explicit manifest-derived
`capabilities` map for every compiled V1 module. The application copies this
map defensively, and Today displays the Work/Knowledge/Assistant/Learning
readiness state when it is supplied by the backend. Missing capability data
remains backward-compatible as an empty map; this does not turn any module on
or close the open operator, provider or release gates.

REST manual Knowledge import now has the same mutation boundary as the MCP
`knowledge_import_manual` tool: callers must send `Idempotency-Key`, and the
Flutter adapter generates one for each user import. The current application
keeps deterministic source/revision identity for safe retries; a separate
durable Knowledge replay table remains intentionally outside this slice.

The 2026-08-30 continuation reconciliation reran the CyberOS machine gates
(`GATES: GREEN`) and the multi-agent protocol verifier (`PASS`). No
`ready_to_implement` task is currently eligible: two tasks await human review
and one awaits final acceptance. The operator confirmed all post-baseline
changes listed in the protected-WIP manifest as valid user-owned WIP outside
`TASK-CYB-001`; their hashes were re-baselined without changing file contents.
The CyberOS-only verifier now passes all checks. See
`docs/project/agent-runs/2026-08-30-cyberos-wip-rebaseline.md`.

This file is the current status for the rebuild described in
`docs/project/REBUILD_PLAN.md`. The older Home/Practice/Review/Progress shell is
kept only as a compatibility surface and is not the target product direction.

## Self-contained gate refresh — 2026-08-30

The repository-owned S7 checks were rerun without depending on the deferred
human/device and Tailscale gates. Disposable PostgreSQL runs passed migration
dry-run/application/idempotency/rollback, legacy workspace backfill and data
preservation, the `100000`-chunk Knowledge capacity check, production
fail-closed configuration, and all four raw coverage-gate tests. This closes
no external acceptance boundary: operator U0/device testing, a current
independent review, provider/billing reconciliation, CI delivery-commit
evidence and release authorization remain open.

The current source also passed the disposable production-shaped runtime smoke:
authentication/session revocation, workspace-scoped Project/Task/Decision
version/history behavior, MCP issue/initialize/revoke/replay, Knowledge
import/search, CORS allow/deny and provider fail-closed behavior. The smoke
stack used a loopback fake provider endpoint and was removed after the run; it
is not live-provider, CI or production evidence. The smoke assertion now also
requires the authenticated bootstrap to expose `capabilities` with `work=true`.

The current source gate refresh also passed default Go normal/race runs
(`989` tests each), PostgreSQL-enabled normal/race runs (`1060` tests each),
`go vet`, `go build`, Flutter analyze, the Flutter suite (`64` passed, `2`
environment skips), the focused UI/golden/state suite (`32` passed), web
release build, Android debug/release builds, emulator smoke and the raw
coverage gate. The PostgreSQL profile remains `77.40%` overall
(`9354/12085`), above the R0 floor, with all seven V1 core packages at or above
the raw `90%` threshold. Details are in
`docs/project/agent-runs/2026-08-30-current-source-gates.md`.

After the gate refresh, the compact Today header was corrected so Settings
stays in the top-right action slot on a 390×844 viewport. Workspace UI tests
passed (`19`), the Workspace surface and state golden suites passed (`32` in
the focused run), and push-to-talk now supports keyboard focus plus Space/Enter
press/release activation. The rebuilt Tailscale-configured release APK passed the
repeatable emulator smoke on `emulator-5554`. The retained evidence is
`/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.Saifwv`.
This is still emulator evidence; U0 operator acceptance on a real device is
open.

The product MCP handshake now includes bounded server instructions in both
`initialize` and `server/discover`. The instructions tell connected clients to
treat retrieved content as data, preserve evidence/unknown semantics and keep
challenge, idempotency and version-conflict checks on writes. A copy-only
read-only Codex client template is available at
`docs/project/mcp/codex-product-mcp.toml.example`; active CyberOS MCP config
files remain unchanged. The template is a client handoff aid, not operator
configuration or acceptance evidence.

The repository-configured CyberOS machine gates also exited `0` on this
checkout: build, lint, backend test, contentfactory (200 generated evaluation
cases), Flutter test and coverage all passed. The doctor step was skipped
because the CyberOS memory CLI was not importable. The output remains machine
evidence only and explicitly retains human review/final acceptance; evidence
is in `docs/project/agent-runs/2026-08-30-cyberos-gates.md`.

The single current Sol Ultra review follow-up ended at the platform usage limit
before returning a terminal verdict. No replacement reviewer was spawned and
no historical approval was reused; the attempt is recorded in
`docs/project/agent-runs/2026-08-30-sol-review-usage-limit.md`.

## Product target

```text
Today | Work | Knowledge | Learning
```

The canonical flow is work capture → versioned work data → sourced knowledge →
grounded assistant → preview/confirm/receipt → English overlay. External Drive
and GitHub data remain read-only in V1 except for the explicitly confirmed,
limited GitHub issue/comment/label actions.

## Current gate snapshot — 2026-08-29

The latest source changes bind all full-application MCP writes to the
authenticated token principal, harden Streamable HTTP negotiation, add the
text-first voice composer to the canonical assistant surfaces and make usage
accounting explicit when provider metadata is unavailable. The regressions
cover Project, Task, Decision and manual Knowledge import and prevent an
accidental write into the development default user workspace; the voice path
keeps transcript editing and text submission as the canonical flow. MCP POSTs
now require both JSON and SSE response types in `Accept`, validate an optional
protocol-version header, and return `202` for notifications; GET remains an
intentional `405` because the server does not emit unsolicited SSE. Assistant
conversation metadata can be listed and a selected transcript can be reopened
inside the authenticated workspace.

The continuation hardening also binds configured REST legacy trash routes for
Project, Task and Decision to the same `work.entity.trash` challenge,
expected-version check, replay boundary and receipt used by the canonical
actions endpoint and MCP. Servers without the action service keep the direct
route only as an explicit compatibility test seam.

The compiled module manifest is now enforced at runtime. `DEVENGLISH_MODULES`
accepts `all`/`*` or a dependency-complete comma-separated allowlist; startup
rejects unknown or incomplete graphs. The selected manifest gates V2 and MCP
surface registration, and Work trash/purge routes are not registered without
the Actions module, so disabling a module cannot create a direct-write escape
hatch. The V2 bootstrap now loads only selected Work, Knowledge and Assistant
projections and returns stable empty arrays for disabled modules, so a partial
manifest cannot query or leak disabled data. `/api/v1/*` remains the
intentional compatibility surface.

The read-only Google Drive connector now supports both a short-lived static
access token for bounded local smoke and an OAuth refresh source configured by
client ID, client secret and refresh token. Refreshed access tokens stay in
memory, refresh/provider errors are redacted, Drive plus OAuth token requests
share a bounded 30-second HTTP client, and request cancellation reaches the
refresh exchange; live consent, quota and rate-limit behavior remain
unverified.

Workspace resolution now fails closed around legacy topology: a fresh user may
receive one deterministic workspace, but an owner mismatch, deleted target,
multiple active mappings or any ambiguous historical mapping returns an
explicit unavailable/operator-resolution error. The repository locks the owner
row before resolving and the HTTP boundary maps these cases to `503`; no
production backfill, automatic workspace selection or migration policy was
invented.

The latest HTTP boundary refresh also verifies method/`Allow` contracts,
workspace isolation and query normalization, connector input rejection before
provider I/O, product and connector error mappings, and generic error redaction
at the HTTP boundary. Credential-shaped values are not returned from generic
connector or provider errors. A route-wide workspace-initialization failure
matrix now exercises `42` V2 cases and verifies that every route fails closed
instead of falling back to an implicit workspace.

The web entrypoint now has a static boot-state layer before Flutter
initializes: it shows a branded loading card, respects reduced-motion
preferences, removes itself when the Flutter host surface appears and shows a
reloadable failure state after 30 seconds. This prevents a failed or slow web
engine startup from being presented as a blank page; it does not replace the
real-device U0 walkthrough.

The CyberOS product/improvement task lint is also clean after normalizing eight
acceptance-criteria trace fields from the non-canonical `tests:` spelling to
`test:`. The repair is recorded in
`docs/project/agent-runs/2026-08-29-task-traceability-repair.md`; informational
`TRACE-001` messages remain by design.

The current task reconciliation evidence is now co-located for
`TASK-PRODUCT-001` and `TASK-OPS-001`: both pass R1 spec integrity and R2
phase-artifact checks. R3 has no ship manifest, and R4 remains red because the
declared delivery objects are not in `HEAD`; the OPS R5 test-mode check also
refuses its untracked verifier citations. These are intentional HITL/Git
boundaries in the dirty development checkout, not hidden implementation
failures.

The latest local gates are: PostgreSQL-enabled Go `1060` tests across 23
packages, PostgreSQL-enabled race `1060` tests, and default-profile Go `989`
tests for both normal and race runs; vet and build are clean; the official MCP
Go SDK conformance test exits `0`; production-shaped smoke with
19 migrations and all Work/MCP/Knowledge/auth/CORS/provider-failure/history
checks passing; Flutter analyze and web release build passing with
`--no-wasm-dry-run`, and Android debug plus release artifacts generated. The latest full
Flutter suite passed `54` tests with `2` environment skips after restoring the
local Dart AOT runtime path; no Flutter test failure remains. The canonical app
shell now imports public Today/Work/Knowledge/Learning feature boundaries; the
screen implementations now live under their owning feature directories, while
`workspace.dart` remains a private composition library and the old workspace
screen is only a compatibility facade. The authenticated production-shell test
now explicitly traverses Today → Work → Knowledge → Learning → Today. A fresh
browser smoke rendered Today and Learning
from the rebuilt release artifact with no console warnings/errors, while the
Chrome smoke test covered navigation through all four canonical destinations;
mobile layout is covered by the 390x844 and 430x932 goldens. This is not
real-device acceptance.

The Android packaging helper now supports the actual cross-network
development setup: it prefers an explicit API endpoint, then a Tailscale IPv4
address, then the local Wi-Fi address; it verifies `/healthz` before building
and supports a caller-selected APK output path. The release APK has also been
installed and walked through on the local `Ledgerly_Pixel_8` Android Emulator:
Today, Work, Knowledge and Learning render backend-backed content. The phone
must run Tailscale in the same tailnet as the Mac. Emulator success is not
physical-device routing, microphone or production deployment evidence.

The repeatable `scripts/android_emulator_smoke.sh` harness now installs a
caller-selected APK, launches `com.devenglish.devenglish/.MainActivity`, checks
the Today accessibility contract, walks all four canonical destinations,
verifies the assistant text/voice composer markers, captures
screenshot/UI/logcat evidence and filters crash detection to the app PID. It
passed on `emulator-5554` both directly and through Make with the Tailscale
APK; the evidence directory and exact commands are recorded in
`docs/project/TEST_LOG.md`. A preceding Make attempt exposed a temporary
backend bootstrap failure and is retained as diagnostic evidence. This remains
emulator evidence, not physical-phone or release acceptance.

The local Compose backend was rebuilt from the current source and recreated
without changing the PostgreSQL volume. The live health, bootstrap and
assistant-conversation-list checks returned HTTP `200` before the post-rebuild
emulator smoke rerun. This confirms local runtime alignment only; it is not CI
or production deployment evidence.

The current PostgreSQL-enabled all-backend coverage is `77.40%`
(`9354/12085` statements). The strict raw-count checker
`scripts/backend_coverage_gate.mjs` passes the seven V1 core packages:
Application `752/830`, Assistant `274/295`, Connectors `1133/1258`, HTTP API
`1169/1297`, Knowledge `833/923`, MCP `1358/1495` and Work `1581/1756`.
The checker is wired into the disposable PostgreSQL integration workflow and
compares integer counts, so rounded `90.00%` cannot pass when the raw ratio is
below 90%; the same invocation compares the total against the machine-readable
R0 manifest. The R0 baseline is recorded in
`docs/project/agent-runs/2026-08-29-r0-baseline.md` as
`4269/6393 = 66.78%`, measured from the declared baseline commit in a detached
worktree. The current `77.40%` total is therefore above the R0 floor. This is
an implementation checkpoint, not a release approval.

## Slice status

| Slice | Status | Evidence / remaining boundary |
| --- | --- | --- |
| R0 baseline + UX lock | Implemented as reference | Baseline coverage is recorded and current total is above its floor; U0 audit, flows, wireframes, tokens, high-fi spec and prototype exist; operator/device verdict remains open |
| S1 walking skeleton | Verified | `/api/v2/bootstrap`, Flutter Workspace shell, API/domain/controller contracts and S1 report |
| S2 Work depth | Implemented and tested | Project/Task/Decision CRUD, version checks, history, trash/restore/purge and Today next-action path; broader UX and production migration acceptance remain |
| S3 Knowledge + sync | Implemented and tested | Source/item/revision/chunk/claim/evidence, FTS + optional 384-dim hybrid retrieval, Drive/GitHub incremental sync; live provider provisioning and capacity evidence are local-only |
| S4 Assistant + safe actions | Implemented and tested | DeepSeek routing, grounded response contract, usage reservation, retries, challenges and receipts; live billing reconciliation and production credentials are not release evidence |
| S5 Learning + voice | Implemented and tested | Work-derived overlay, observations, editable transcript, push-to-talk composer and on-demand TTS; full UX replacement and second-device review remain |
| S6 MCP | Implemented and tested | `/mcp`, separate bearer token/scopes, Streamable HTTP negotiation, tools/resources, replay and REST parity checks; official Go SDK conformance passes. Product-client handoff is documented in `docs/project/MCP_CLIENT_SETUP.md`; operator-side Codex/Claude configuration remains open |
| S7 cutover + hardening | Evidence complete locally; release gate open | Workspace backfill, rollback flag, Docker migration dry-run/application/rollback and capacity harnesses, production-shaped auth/provider smoke, responsive/state goldens and final local build gates are present; operator/device sign-off, lifecycle acceptance and release authorization remain open |

## Implemented capabilities

### Work

- workspace-scoped Project, Task and Decision records;
- create/update with optimistic `expectedVersion` checks;
- mutation history, soft trash, restore and guarded purge;
- canonical Project/Task/Decision detail sheets with version transitions and
  backend activity history;
- Today priority/next-action read model with truthful empty state;
- Today Quick capture for tasks, decisions and reviewed manual sources;
- conflict responses preserve the canonical record and expose reload context.

### Knowledge and connectors

- immutable source revisions and evidence-linked claims;
- source-detail timeline shared by HTTP/application read paths;
- exact PostgreSQL full-text search plus optional multilingual-e5-small
  384-dimensional sidecar retrieval and RRF ranking;
- FTS fallback with degraded state when the sidecar is unavailable;
- read-only Drive and GitHub incremental sync with cursor/run records,
  full-bootstrap/removal tombstones, workspace-scoped revision deduplication
  and safe provider error handling.

### Assistant, usage and actions

- fast/smart DeepSeek routing with strict JSON response normalization;
- evidence/unknown/stale response fields and citation verification;
- provider-reported usage when available, explicit unavailable usage otherwise;
- usage summaries expose unavailable provider records instead of presenting
  missing billing metadata as zero spend;
- per-feature monthly reservation/commit/release guard with concurrency-safe
  PostgreSQL path and in-memory test fallback;
- transient-only retry for network/429/5xx failures;
- one-time, TTL-bound action challenges and receipts for limited GitHub writes
  and canonical Work trash actions;
- configured REST legacy trash routes cannot bypass confirmation and return the
  same redacted action receipt semantics as the generic action endpoint.

### Learning and client

- Today/Work/Knowledge/Learning navigation shell;
- work-derived missions, Vietnamese help plus an English starter;
- observation persistence isolated from canonical work/knowledge data;
- canonical assistant push-to-talk with an editable transcript, lazy
  microphone initialization and on-demand read-aloud controls;
- structured API errors and retryable loading/error states in the Flutter
  controller/UI.
- bootstrap capability state is decoded from the canonical manifest and shown
  in Today as a compact readiness summary for Work, Knowledge, Assistant and
  Learning;
- connector sync status with explicit cursor/no-change/read-only evidence;
- responsive surface and state goldens for Today, Work, Knowledge, Assistant
  and Learning at the supported viewports, plus loading, empty, stale/degraded,
  provider-error, offline, conflict and connector-sync states.

### MCP

- Streamable HTTP endpoint at `/mcp`;
- independent digest-only bearer token persistence, expiry/revoke and scopes;
- read/write/confirmed tool registry and Work resources;
- POST response negotiation for JSON/SSE, optional protocol-version validation
  and `202` notification responses; GET remains an explicit no-unsolicited-SSE
  `405` boundary;
- replay/idempotency and version-conflict semantics aligned with application
  services and REST.

## Current verification refresh — 2026-08-29

- PostgreSQL-enabled `rtk go test ./... -count=1`: `1060` tests passed across
  `23` packages;
- PostgreSQL-enabled `rtk go test -race ./... -count=1`: `1060` tests passed
  across `23` packages;
- default-profile `rtk go test ./... -count=1` and race run: `985` tests
  passed across `23` packages for each run;
- module-aware bootstrap suite (`rtk go test ./backend/internal/application
  ./backend/internal/httpapi ./backend/internal/platform -count=1`): exit `0`;
  `308` tests passed, including proof that disabled repositories are not
  queried, disabled projections remain empty arrays, and bootstrap errors are
  returned safely;
- runtime module-manifest targeted suite: `254` tests passed across the
  platform, HTTP API and server packages, including explicit module disable
  and Work trash/purge fail-closed assertions;
- local PostgreSQL drift repair: migration `018_assistant_conversation_context.sql`
  was pending on the development volume; the append-only migration applied
  successfully and the two previously failing conversation persistence tests
  then passed;
- `rtk go vet ./...` and `rtk go build ./...`: exit 0;
- `rtk git diff --check`: exit 0;
- runtime module-manifest targeted suite: exit `0`; `254` tests passed across
  platform, HTTP API and server packages, including explicit disabled-module
  and Work trash/purge fail-closed assertions;
- binary smoke with `DEVENGLISH_MODULES=platform,work`: exit `0`; Work
  projects returned `200`, while Knowledge search, Work trash and MCP returned
  `404` as disabled surfaces; bootstrap returned `200` with stable empty
  `sources` and `conversations` arrays;
- production-shaped Docker smoke rerun from the current source: exit `0`; `19`
  migrations and the complete auth/Work/Knowledge/MCP/CORS/provider-failure
  sequence passed;
- `rtk go test ./backend/internal/httpapi -count=1`: exit `0`; `232` tests
  passed, including workspace
  isolation, query normalization, connector pre-provider validation,
  method/error mapping, credential-redaction checks, the empty-input 400
  boundary and the route-wide workspace-initialization failure matrix;
- `rtk go test ./backend/internal/httpapi -run
  TestV2RoutesFailClosedWhenWorkspaceInitializationFails -count=1`: exit `0`;
  all `42` V2 route cases failed closed with the original initialization error;
- `rtk go test ./backend/internal/connectors -count=1`: exit `0`; `85` tests
  passed, including the safe-write entity-target validation boundaries;
- PostgreSQL-enabled coverage profile: exit `0`; `77.40%` statement coverage
  (`9354/12085` statements); application `90.60%`, assistant `92.88%`,
  connectors `90.06%`, HTTP API `90.12%`, knowledge `90.25%`, MCP `90.84%`
  and work `90.03%`;
- R0 baseline measurement: detached worktree from
  `104304f12f411dd534aa4b385e964b3b8c11ae43`; PostgreSQL-enabled Go suite
  exit `0` with `395` tests and raw coverage `4269/6393 = 66.78%`;
- `rtk node scripts/backend_coverage_gate.mjs --profile
  /tmp/devenglish-go-db-20260829-workspace-resolution2.cov --min 90
  --baseline-manifest
  docs/project/coverage/2026-08-29-r0-backend-coverage-baseline.json`: exit
  `0`; the R0 total floor and all seven required V1 core packages passed the
  raw-count gate;
- `rtk node --test scripts/backend_coverage_gate_test.mjs`: exit `0`; exact
  threshold, baseline-floor, malformed-profile and malformed-manifest cases
  passed;
- the profile is the disposable PostgreSQL run at
  `/tmp/devenglish-go-db-20260829-workspace-resolution2.cov`; the R0 comparison record is
  `docs/project/agent-runs/2026-08-29-r0-baseline.md`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh`: exit `0`; the rebuilt Docker smoke
  passed 19 migrations and all Work/MCP/Knowledge/auth/CORS/provider-failure/
  history checks;
- disposable migration dry-run/rollback harness: exit `0`; dry-run reported all
  `19` migrations pending without creating `schema_migrations`, the real run
  applied `000..018`, the second run was idempotent and the injected failure
  rolled back; evidence is in
  `docs/project/agent-runs/2026-08-29-migration-dry-run.md`;
- disposable legacy-upgrade harness: exit `0`; migrations `000..018`,
  idempotency, legacy snapshot preservation and workspace isolation passed;
- disposable Knowledge capacity harness: exit `0`; `100000` chunks, FTS lookup
  and bounded last-page pagination passed;
- the latest boundary tests cover workspace/user isolation, include/exclude
  trash query flags, bounded query parsing, connector validation before
  provider I/O, active-child purge rejection, early-purge lifecycle responses
  and Project/Task/Decision history routes; Flutter also verifies canonical
  Work detail/activity presentation.
- Generic HTTP errors now pass through the shared credential redactor, and the
  boundary test verifies secret-like values do not escape in connector or
  provider error responses.
- MCP Streamable HTTP tests cover required JSON/SSE `Accept` negotiation,
  optional supported/unsupported `MCP-Protocol-Version`, `202` notification
  responses and the intentional GET `405` no-SSE boundary.
- `rtk flutter analyze`: exit `0`; no issues found. `rtk flutter test
  --reporter compact`: exit `0`; `54` tests passed and `2` environment skips.
- `rtk flutter test --dart-define=DEVENGLISH_ENV=production
  test/production_shell_test.dart`: exit `0`; `2` tests passed, including the
  authenticated Today → Work → Knowledge → Learning → Today regression guard.
- `rtk flutter build apk --debug`: exit `0`; generated
  `build/app/outputs/flutter-apk/app-debug.apk`.
- `rtk flutter test -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true
  test/browser_smoke_test.dart --reporter compact`: exit `0`; the canonical
  Today → Work → Knowledge → Learning navigation smoke passed.
- `rtk flutter build web --release --no-wasm-dry-run`: exit `0`; `build/web`
  rebuilt without the previous WASM dry-run warning.
- `rtk bash scripts/android_local_apk.sh`: exit `0`; release APK generated at
  `build/app/outputs/flutter-apk/app-release.apk` and copied to
  `/Users/ryantruong/Desktop/DevEnglish-local.apk`.
- Official MCP Go SDK conformance: `62/62` passed, including modern protocol
  negotiation, tool/resource discovery and a bearer-authenticated tool call;
  operator-side Codex/Claude configuration remains unverified.
- The package coverage gate is also wired into
  `.github/workflows/postgres-integrations.yml`; CI has not been run from this
  local checkout.
- Fresh in-app browser visual smoke rendered the rebuilt Today and Learning
  surfaces with no console warnings/errors; the Flutter Chrome smoke covers
  all four destinations and the mobile goldens cover 390x844/430x932. This is
  visual smoke, not real-device/operator acceptance.
- Fresh web reload at `http://localhost:8093/?boot=verification` rendered the
  `DevEnglish` shell with `bootNodes=0` and `flutterPanes=1`; no warning/error
  entries were captured. The static boot card and its 30-second reload fallback
  are in `web/index.html`.
- A fresh in-app browser session at `http://localhost:8093/` loaded the release
  artifact as `DevEnglish` at a 2560x1440 viewport, rendered Today, navigated to
  Work, Knowledge and Learning, and returned two canonical results for the
  Knowledge search `canonical`; captured tab logs contained only the bootstrap
  debug entry and no error/warning entries.
- The same browser session sent a real text-assistant question, kept the user
  transcript visible while the source was checked, and returned a grounded
  response with an explicit unknown boundary after about `16.4s`; the backend
  recorded the corresponding message request at `16448ms` and the browser logs
  still contained no error/warning entries. This is functional smoke evidence,
  not a latency SLO or real-device acceptance.
- Usage accounting hardening tests confirm provider-reported DeepSeek usage is
  preserved, unavailable speech usage is not estimated, and preflight
  reservations are not written as final usage records; the Settings summary
  reports the unavailable-record count.
- Configured REST legacy Project/Task/Decision trash routes and the full MCP
  `entity_trash` path now share the Work action challenge, version, replay and
  receipt boundary; focused HTTP/MCP evidence is `228`/`132` tests passed.
- Google Drive OAuth refresh uses the same bounded 30-second HTTP client as
  Drive API calls through `oauth2.HTTPClient`; targeted integrations/connectors
  coverage is `101` tests, and no live Google request was made. Evidence is in
  `docs/project/agent-runs/2026-08-29-drive-oauth-context-cancellation.md`.
- Workspace resolution now rejects owner mismatch, deleted targets, multiple
  active mappings and ambiguous legacy history; fresh creation is serialized
  by the owner lock and HTTP maps operator-resolution/unavailable cases to
  `503`. Evidence is in
  `docs/project/agent-runs/2026-08-29-workspace-resolution-guard.md`.

The previous local evidence below is retained as historical context.

## Historical verification evidence — 2026-08-28

- `go test ./...`: 438 tests passed across 23 packages;
- `go test -race ./...`: 438 tests passed across 23 packages;
- `go vet ./...`, `go build ./...`, Dart format, Flutter analyze and
  `git diff --check` passed;
- Flutter tests: 43 passed and 2 skipped; browser smoke, web release build
  and Android debug build passed;
- A fresh browser opened the release artifact at `http://localhost:8094/` and
  rendered the canonical Today surface; the Flutter view was non-empty and
  browser logs contained no error or warning entries;
- task lint and the multi-agent protocol verifier passed with no errors;
- disposable legacy-upgrade harness: migration chain `000..018`, legacy
  snapshot preservation, workspace backfill and second-run idempotency;
- disposable knowledge-capacity harness: 100,000 chunks, pagination and FTS
  index query;
- production-shaped Docker smoke passed: migrations `19`, embedding `200`,
  login/session `200`, MCP issue/initialize/revoke/revoked `201/200/204/401`,
  knowledge import/search `201/200`, CORS `204/403`, provider fail-closed
  `500`, logout `204`;
- MCP replay, version conflict, persistent-token and knowledge-evidence parity
  tests passed where their configured database fixture was available.

At the historical 2026-08-28 checkpoint, the DB-enabled profile was `73.1%`
statement coverage, below the plan threshold of `90%`. The current profile is
recorded above. Database-only paths require the configured integration
fixture for their coverage to be represented. The CyberOS-only verifier passes
its static checks but reports the protected pre-existing
`.github/workflows/ci.yml` WIP snapshot finding; that WIP was preserved and not
reverted. A passing command is evidence for that command only; it is not a
product-wide release approval.

## Open acceptance items

- operator accepts the U0 high-fi specification/prototype after mobile and
  desktop walkthrough;
- split the shared private Dart composition into fully independent feature
  libraries only if strict file-level/package ownership is required; the
  implementation files are now feature-owned and public
  Today/Work/Knowledge/Learning boundaries are in place;
- run the mandated high-fi walkthrough on a real mobile device and record the
  operator U0 verdict;
- confirm the recorded R0 overall-coverage baseline and total-coverage floor
  gate on GitHub CI from the delivery commit; the local workflow gate and the
  separate raw `>=90%` package gate already pass for all seven V1 core
  packages;
- decide the authoritative mapping for zero, multiple, deleted or restored
  legacy workspaces before enabling any production backfill or automatic
  resolver migration;
- request one new bounded Sol Ultra review only when reviewer quota is
  available, because the latest attempt ended at the usage limit without a
  verdict; then record human release authorization before commit, push, merge
  or deployment;
- advance the product-reset task chain sequentially: only reopen the next
  `on_hold` successor after its predecessor reaches `done` through HITL;
- complete compatibility adapter/rollback review and any live-provider billing
  reconciliation required for production.

## Runtime notes

- local backend: `http://localhost:8080`;
- local embedding sidecar: `http://localhost:8090`;
- local PostgreSQL is exposed on port `5433` by development Compose;
- Flutter release web has previously been served on port `8093`;
- provider secrets stay in ignored local environment/configuration and must not
  be copied into reports, screenshots or logs.

## Historical compatibility

The previous learning-first features (diagnostic, SRS, vocabulary, roleplay,
Copilot, speaking and analytics) are retained where their logic is useful, but
their old navigation is not the product target. They should be reached through
Learning or the assistant overlay during the cutover, then retired only after
compatibility and data migration evidence is accepted.
