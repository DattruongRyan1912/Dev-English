# Dev-English Changelog

Project changes and verification milestones are recorded here with the newest entries first.

## [Unreleased]

## [1.0.1] — 2026-09-01

### V1 delivery package

- published the reviewed DevEnglish V1 consolidation as Android build `2`;
- retained the local-development backend endpoint selection for Tailscale or
  same-network testing;
- this release package is for local/operator acceptance and does not imply
  production deployment or live-provider validation.

### Final V1 consolidation — 2026-08-30

- created the isolated `integration/final-v1` worktree from `origin/main`
  (`104304f`) and reconciled the current canonical product-reset WIP on top;
  older Wave 2/3 worktrees remain untouched as reference snapshots;
- verified the consolidated backend with Go unit, PostgreSQL integration,
  race, migration, Docker runtime and coverage gates; the PostgreSQL-enabled
  coverage gate is `77.42%` overall with every new core package above `90%`;
- verified the Flutter shell with analyze, tests, web build, Android debug
  build and Chrome navigation smoke against a disposable local backend;
- this entry records repository evidence only; one independent Sol Ultra
  review, final commit identity, push and human release acceptance remain
  explicit gates for this branch.


### Knowledge import mutation boundary — 2026-08-30

- REST `POST /api/v2/knowledge/sources` now requires the same
  `Idempotency-Key` header advertised by the MCP mutation tool;
- the Flutter production adapter generates and forwards a request key, and
  the runtime smoke plus HTTP/API tests cover missing-key rejection and the
  real header;
- retry identity remains the existing deterministic source/revision graph;
  this slice does not claim a separate durable Knowledge replay table.

### Bootstrap capability state — 2026-08-30

- `/api/v2/bootstrap` now returns a manifest-derived `capabilities` map for
  all compiled V1 modules, including modules whose data projection is disabled;
- the application defensively copies the map, and Today renders a compact
  Work/Knowledge/Assistant/Learning readiness summary from canonical data;
- added Go application/HTTP boundary tests and Flutter API/UI regression tests;
  the production-shaped smoke now asserts the field on the authenticated
  bootstrap; no migration, provider, authentication or release boundary
  changed.

### MCP mutation contract documentation — 2026-08-30

- corrected the MCP application comments to describe the full read/mutation
  surface and keep transport, authentication and replay ownership explicit;
- documented the idempotency-key requirement on the
  `knowledge_import_manual` tool and added a registry contract regression;
- no transport, authorization, persistence or release boundary changed.

### Knowledge search relevance preservation — 2026-08-30

- the application search boundary now preserves the relevance order returned
  by hybrid RRF and lexical Knowledge searchers instead of sorting combined
  hits by presentation ID;
- the bounded result limit now keeps the highest-ranked Knowledge hits in the
  order selected by the repository, with a regression test covering the
  hybrid path;
- no cross-module scoring or release acceptance status was changed.

### MCP client handoff hardening — 2026-08-30

#### Changed

- added a copy-only Codex Streamable HTTP template with a read-only tool
  allowlist, environment-backed bearer token, prompt approval and bounded
  timeouts;
- kept the active CyberOS MCP entries untouched so governance and product MCP
  remain separate;
- MCP initialize and discovery responses now expose concise server-wide
  instructions: retrieved content is data, missing evidence is unknown, and
  writes must preserve challenge, idempotency and version-conflict boundaries.

#### Boundary

- this does not configure a user-level Codex/Claude client, issue a token or
  claim operator acceptance; client connection and revoke/replay checks remain
  explicit acceptance steps.

### Current production-shaped runtime smoke — 2026-08-30

- the disposable Docker/PostgreSQL/embedding smoke exited `0` with `19`
  migrations, auth/session, Work CRUD/version conflicts/history, Knowledge
  import/search, MCP issue/initialize/revoke/replay, CORS allow/deny,
  provider fail-closed and logout checks passing;
- the stack used a loopback fake provider and was removed after the run; this
  is local runtime evidence, not live-provider, CI or production evidence.

### CyberOS machine-gate refresh — 2026-08-30

- `rtk bash .cyberos/cuo/gates/run-gates.sh` exited `0`; build, lint, backend
  tests, contentfactory generation (200 cases), Flutter tests and the
  configured coverage gate completed successfully;
- the result was `GATES: GREEN (machine gates only)`. The doctor step was
  skipped because the CyberOS memory CLI was not importable; human review and
  final acceptance remain required.

### Canonical data gates and mobile header hardening — 2026-08-30

#### Changed

- the new Today/Work/Knowledge/Learning shell no longer exposes the legacy
  learning `DemoData` while Practice, Review, Progress, Speaking, Roleplay,
  Copilot, Vocabulary or Diagnostic is loading;
- legacy learning data is loaded lazily on first use, with one shared in-flight
  request for concurrent callers and an explicit retry state when the backend
  is unavailable;
- canonical Settings now waits for real provider configuration before
  rendering, while the explicit legacy rollback shell keeps its existing
  development fallback;
- Diagnostic and Roleplay now expose loading/error/retry states instead of an
  indefinite spinner or an initial sample state.
- logout and session expiry now increment a session generation before
  invalidating user state; late legacy-learning, Settings or workspace
  bootstrap, conversation, history, search, sync, mutation or action
  responses are ignored instead of restoring the previous user;
- session invalidation notifies the workspace controller, clears in-flight
  UI flags and resets all user-scoped learning/work/assistant snapshots.
- Today now keeps Settings in the compact mobile header instead of placing it
  below the page heading; Work and Knowledge retain their multi-action row.
- push-to-talk is now keyboard-focusable: holding Space or Enter starts the
  recording lifecycle, releasing it ends transcription, and focus is visibly
  indicated without changing the pointer gesture path.

#### Verified

- `rtk flutter analyze` — exit `0`;
- `rtk flutter test test/production_shell_test.dart --reporter compact` — exit
  `0`, including unavailable-state, real lazy-load, concurrent-load and
  late-response session-boundary tests;
- `rtk flutter test test/production_shell_test.dart
  --dart-define=DEVENGLISH_ENV=production --reporter compact` — exit `0`;
- focused Roleplay, widget and Workspace UI tests — exit `0`.
- full `rtk flutter test --reporter compact` — exit `0`, `63` passed and `2`
  environment skips;
- focused Workspace UI tests — exit `0`, `19` passed; responsive surface and
  header-position assertions include the 390×844 mobile layout;
- workspace surface and state goldens — exit `0`, `32` focused UI/golden/state
  tests, after updating the intentional header layout change;
- `rtk flutter test --dart-define=DEVENGLISH_ENV=production
  test/production_shell_test.dart --reporter compact` — exit `0`, `11` passed;
  late assistant, history and opened-conversation responses remain isolated
  after a session reset;
- the current Tailscale-configured release APK passed
  `scripts/android_emulator_smoke.sh` on `emulator-5554`; evidence:
  `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.Saifwv`;
- the current source passed the Chrome canonical navigation smoke with exit
  `0`.

#### Boundary

- this change is local source/test evidence only; no commit, push, merge,
  deployment or release authorization was performed.

### Production-shaped runtime smoke refresh — 2026-08-30

#### Verified

- reran `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES
  scripts/production_runtime_smoke.sh` from the current source; it exited
  `0` with `19` migrations, embedding `200`, authenticated session `200`,
  Project/Task/Decision create/update/history and `409` stale-version checks,
  MCP issue/initialize/revoke/replay checks, Knowledge import/search, CORS
  allow/deny checks, provider fail-closed `500` and logout `204`;
- the run used an isolated disposable PostgreSQL stack and a loopback fake
  provider endpoint, then cleaned the stack automatically.
- Flutter Chrome smoke also passed against the current local backend, covering
  the canonical Today → Work → Knowledge → Learning navigation.

#### Boundary

- this is repository-owned runtime evidence only; live provider billing,
  physical-device routing, CI delivery-commit evidence, independent review
  and human release authorization remain open;
- no commit, push, merge, deployment or production data was changed.

### Repeatable Android emulator smoke — 2026-08-30

#### Changed

- added `scripts/android_emulator_smoke.sh` and the `make
  android-emulator-smoke` target;
- the harness selects or accepts an ADB device, waits for boot, installs a
  supplied APK, launches the Android activity, captures a screenshot and
  accessibility tree, and filters crash checks to the app process;
- documented `ANDROID_APK_PATH`, `ANDROID_DEVICE_ID` and
  `ANDROID_SMOKE_OUTPUT` overrides for repeatable local checks.
- the timeout path now distinguishes the app's
  `Workspace is temporarily unavailable` state from a genuine screen-render
  timeout, so backend failures are actionable in the captured UI evidence.

#### Verified

- the first run correctly exposed and fixed a harness-only false negative: the
  four navigation labels are separate accessibility content descriptions,
  not one combined string;
- the second run exposed and fixed a launch race where the UI could be ready
  before `pidof` returned the app process;
- the corrected harness passed on `emulator-5554` with the release APK;
- the same check passed through `make android-emulator-smoke`;
- Today rendered `Canonical workspace`, `Live`, `1 open task`, `1 source
  connected` and all four navigation destinations; the assistant transcript,
  citation, editable text composer and `Hold to talk` control were also
  visible; no app-process crash signature was found;
- the successful Make run retained `today.png`, `work.png`,
  `knowledge.png`, `learning.png`, `today-assistant.png`, `window.xml` and
  PID-filtered `logcat.txt` under `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.u6g1HV`;
- the local Compose backend was rebuilt from the current source and recreated
  without changing the PostgreSQL volume; live `/healthz`, `/api/v2/bootstrap`
  and `GET /api/v2/assistant/conversations` then returned `200`, followed by a
  successful smoke rerun under `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.TgRC7j`;
- one earlier Make attempt was retained as a diagnostic failure because the
  backend bootstrap was temporarily unavailable; the subsequent endpoint
  check and rerun passed, so that transient runtime signal is not counted as
  a release gate.

#### Boundary

- this is repeatable emulator evidence only; it does not replace physical
  device, microphone, cross-network phone, provider, CI or release acceptance;
- no commit, push, merge, deployment or release authorization was performed.

### Self-contained S7 gate refresh — 2026-08-30

#### Verified

- migration dry-run, full application, idempotency and rollback harness passed
  against an isolated PostgreSQL Compose project;
- legacy upgrade preserved synthetic legacy data, performed deterministic
  workspace backfill and passed idempotency/isolation checks;
- Knowledge capacity passed with exactly `100000` chunks, FTS lookup and
  bounded last-page pagination;
- production-shaped configuration failed closed for an undersized auth secret;
- all four raw-count coverage gate tests passed.

#### Boundary

- human/device acceptance, Tailscale routing, current independent review,
  provider/billing, CI delivery-commit evidence and release authorization are
  intentionally deferred; no commit, push, merge or deployment was performed.

### Cross-network Android development packaging — 2026-08-29

#### Changed

- extended `scripts/android_local_apk.sh` to prefer an explicit API URL, then
  a Tailscale IPv4 address, then the local Wi-Fi address;
- added `ANDROID_BACKEND_PORT` and `DEVENGLISH_APK_OUTPUT` overrides;
- rejected non-HTTP(S) endpoints and verify `/healthz` before invoking the
  Flutter release build;
- added a shell test for endpoint precedence, normalization and invalid input.

#### Verified

- `bash -n scripts/android_local_apk.sh scripts/android_local_apk_test.sh`
  exits `0`;
- `scripts/android_local_apk_test.sh` passes all four endpoint-selection
  cases;
- the configured local Tailscale endpoint returns backend `/healthz` HTTP
  `200`.
- the release APK installs and launches on the `Ledgerly_Pixel_8` Android
  Emulator; `Today → Work → Knowledge → Learning` navigation and backend-backed
  content render successfully with no app crash.

#### Boundary

- this packages a development APK that uses a private tailnet endpoint; the
  phone must run Tailscale in the same tailnet; emulator success does not prove
  physical-device routing, microphone behavior, production HTTPS or release
  evidence.

### Web boot-state hardening — 2026-08-29

#### Changed

- replaced the default Flutter web metadata with DevEnglish product metadata;
- added a visible HTML boot card with reduced-motion support while the Flutter
  engine initializes;
- added a 30-second startup failure message with a reload action so a failed
  web-engine load is no longer presented as a blank page;
- automatically removes the boot card when Flutter creates its first host
  surface or emits its first-frame event.

#### Verified

- web release build with `--no-wasm-dry-run` exits `0`;
- fresh local browser reload renders the canonical Today surface, the boot
  node is removed and one Flutter glass pane is present;
- Chrome route smoke passes through Today, Work, Knowledge and Learning with
  no console warning/error entries.

#### Boundary

- this is local web-startup evidence only; real-device/operator U0 acceptance,
  CI on a delivery commit and release authorization remain open.

### Workspace resolution guard — 2026-08-29

#### Changed

- legacy workspace resolution now fails closed for owner mismatch, deleted
  targets, multiple active mappings and ambiguous historical mappings;
- fresh workspace creation is deterministic and serialized by an owner-row
  lock;
- unresolved topology maps to an explicit operator-resolution/unavailable
  boundary instead of silently selecting a workspace, with HTTP status `503`.

#### Verified

- targeted workspace tests cover fresh/idempotent/concurrent creation, legacy
  ambiguity, owner mismatch, deleted targets and missing owners;
- default normal/race runs pass `985` tests each, PostgreSQL normal/race runs
  pass `1060` each, and the raw coverage gate passes at `9354/12085`
  (`77.40%`);
- production-shaped Docker smoke, vet, build, diff check and CyberOS machine
  gates pass.

#### Boundary

- no production backfill, automatic legacy mapping, commit, push, merge or
  deployment was performed; an authoritative mapping decision remains needed
  before production migration.

### Google Drive OAuth request-context cancellation — 2026-08-29

#### Changed

- OAuth refresh now keeps its in-memory token cache while accepting the
  caller's request context for token exchange;
- cancelled Drive requests can stop an in-flight OAuth refresh instead of
  waiting on a background context.

#### Verified

- injected cancellation test passes without a live Google request;
- default normal/race runs pass `981` tests each, PostgreSQL normal/race runs
  pass `1049` each, and the raw coverage gate passes at `9308/12034`
  (`77.35%`);
- production-shaped Docker smoke and CyberOS machine gates pass.

#### Boundary

- no live Google request was made; OAuth consent, token validity, quotas,
  production provisioning and release authorization remain open.

### Google Drive OAuth refresh timeout hardening — 2026-08-29

#### Changed

- Drive API calls and OAuth refresh exchanges now share a bounded 30-second
  HTTP client through `oauth2.HTTPClient`;
- kept the static access-token compatibility path and read-only OAuth scope.

#### Verified

- injected OAuth token endpoint test proves the configured client is used;
- default normal/race runs pass `980` tests each, PostgreSQL normal/race runs
  pass `1048` each, and the raw coverage gate passes at `9292/12014` (`77.34%`);
- production-shaped Docker smoke and CyberOS machine gates pass.

#### Boundary

- no live Google request was made; OAuth consent, token validity, quotas,
  production provisioning and release authorization remain open.

### Google Drive read-only OAuth refresh source — 2026-08-29

#### Changed

- added an in-memory OAuth refresh-token source for the Drive connector using
  only the `drive.readonly` scope;
- kept static access-token configuration for bounded local smoke and existing
  fixtures;
- documented both modes in `.env.example`,
  `infra/production.env.example` and `README.md`.

#### Verified

- the integrations/connectors target suite passes `99` tests;
- refresh configuration, token forwarding and generic redacted refresh errors
  are covered without calling Google or logging credential values.

#### Boundary

- live OAuth consent, refresh-token validity, quota/rate-limit behavior and
  production secret provisioning remain open; no Drive write scope was added.

### Product task audit coverage completed — 2026-08-29

#### Changed

- added hash-bound, post-hoc spec-correctness audits for
  `TASK-PRODUCT-002` through `TASK-PRODUCT-008`;
- recorded the exact reconcile evidence and kept lifecycle, independent review
  and Git/release authorization separate.

#### Verified

- task-lint exited `0`; all seven newly audited Product tasks reconcile with
  `R1/R2: pass`;
- `R3` remains absent and `R4` remains red for deliverables not present in
  `HEAD`; P007/P008 `R5` correctly refuse untracked production smoke suites.

#### Boundary

- these are manual spec audits, not Sol verdicts or lifecycle transitions;
  no commit, push, merge or deployment was performed.

### Compiled module manifest is now enforced at runtime — 2026-08-29

#### Changed

- added the built-in registry and `DEVENGLISH_MODULES` parser with dependency
  validation; the manifest selects compiled code only and cannot load runtime
  plugins;
- wired the selected manifest into Go server route registration so V2/MCP
  surfaces are disabled when their module is not selected;
- made the V2 bootstrap load only selected Work, Knowledge and Assistant
  projections, with stable empty arrays and no repository query for disabled
  modules;
- kept Work trash/purge behind the Actions module and preserved `/api/v1/*`
  as the compatibility surface.

#### Verified

- targeted platform/HTTP/server suite: `254` tests passed across `3`
  packages; the module-aware application/bootstrap suite passed `308` tests
  across `3` packages;
- default Go profile: `976` tests passed for both normal and race runs;
- PostgreSQL-enabled profile: `1044` tests passed for both normal and race
  runs; the raw coverage gate passed at `77.27%` overall with all seven core
  packages at or above `90%`; vet, build and `git diff --check` also exited
  `0`;
- production-shaped Docker smoke and a real binary smoke with
  `DEVENGLISH_MODULES=platform,work` passed, including disabled Knowledge,
  trash and MCP surfaces returning `404`.

#### Boundary

- this is local implementation evidence; independent review, CI on the
  delivery commit, task lifecycle acceptance and Git/release authorization
  remain open.

### End-to-end plan status alignment — 2026-08-29

#### Changed

- aligned the authoritative rebuild-plan header with the recorded S1–S7
  implementation evidence;
- made the remaining acceptance and release gates explicit so local evidence
  is not mistaken for operator acceptance or release approval.

#### Boundary

- documentation-only change; no task lifecycle state, code, commit, push,
  merge or deployment was changed.

### Task evidence reconciliation — 2026-08-29

#### Changed

- backfilled `context-map.md`, `edge-case-matrix.md`, `impl-plan.md` and
  `obs-injection.md` plus hash-bound manual spec audits for Product UX lock and
  OPS orchestration guard;
- preserved the distinction between spec correctness, implementation proof,
  independent review, human acceptance and Git/release state.

#### Verified

- both reconciled tasks now pass R1 spec-integrity and R2 phase-artifact
  checks;
- normal and `--run-tests` reconcile results still show the real R3/R4/R5
  boundaries: no ship manifest, uncommitted deliverables, and untracked OPS
  verifier citations at `HEAD`.

#### Boundary

- the manual audits are post-hoc spec audits, not Sol Ultra verdicts;
- no task status, commit, push, merge or deployment was changed.

### PostgreSQL development-volume drift repair — 2026-08-29

#### Changed

- applied the pending append-only conversation-context migration to the local
  development PostgreSQL volume;
- refreshed the PostgreSQL-backed verification evidence after the runtime
  schema caught up with the current code.

#### Verified

- the two previously failing conversation persistence tests pass;
- PostgreSQL-backed Go and race suites pass with `1024` tests across `23`
  packages; vet, build and health checks remain green;
- the local schema now records `19` migrations through
  `018_assistant_conversation_context.sql`.

#### Boundary

- this changed only the local development database state; it is not a
  production migration or release authorization.

### Product task file-cone metadata repair — 2026-08-29

#### Changed

- corrected four stale Flutter paths in the Product task specifications to the
  canonical `lib/src/features/` implementation files;
- recorded a filesystem audit confirming every declared Product
  `new_files`/`modified_files` path exists.

#### Boundary

- this repairs task metadata only; no lifecycle status, audit verdict, commit,
  push, merge or deployment was performed.

### Task traceability repair — 2026-08-29

#### Changed

- normalized eight product-task acceptance criteria to the CyberOS `test:`
  trace format;
- recorded the repair and exact linter/protocol evidence in
  `docs/project/agent-runs/2026-08-29-task-traceability-repair.md`.

#### Boundary

- no task lifecycle or human acceptance state was changed.

### Today quick capture — 2026-08-29

#### Changed

- added a Today Quick capture surface for new tasks, recorded decisions and
  reviewed manual knowledge sources;
- task capture requires an active project and supports priority plus an
  optional description, while reusing the canonical workspace mutation path;
- made section trailing labels wrap or ellipsize safely on compact viewports;
- recorded implementation and browser evidence in
  `docs/project/agent-runs/2026-08-29-today-quick-capture.md`.

#### Verified

- the quick-capture interaction test creates a task through a mocked
  `WorkspaceApi`;
- full Flutter tests pass (`52` passed, `2` environment skips), selected
  responsive/state goldens pass, analyze, protocol verification, diff check and
  web release build pass;
- the rebuilt local release browser smoke renders all three actions without
  captured console warnings or errors.

#### Boundary

- local and mocked evidence only; operator/device acceptance, live provider
  acceptance and release authorization remain open;
- no commit, push, merge or deployment was performed.

### Feature-owned Flutter workspace implementation — 2026-08-29

#### Changed

- moved the canonical Today, Work, Knowledge, Learning and assistant screen
  implementation parts under their owning `lib/src/features/` directories;
- kept `lib/src/features/workspace/workspace.dart` as the private composition
  boundary for shared state/dialog primitives;
- reduced `lib/src/screens/workspace_screen.dart` to a compatibility facade for
  legacy callers and tests;
- recorded the source-path and gate evidence in
  `docs/project/agent-runs/2026-08-29-feature-ownership-refactor.md`.

#### Verified

- Flutter analyze, full Flutter tests (`51` passed, `2` environment skips), web
  release build and Android debug build passed;
- the CyberOS machine gate exited `0` with `GATES: GREEN (machine gates only)`;
- no legacy workspace part-file references remain in `lib/` or `test/`.

#### Boundary

- shared private composition is intentionally still one Dart library; splitting
  it into independent libraries remains an optional strict-ownership follow-up;
- no commit, push, merge or deployment was performed.

### Authenticated navigation regression guard — 2026-08-29

#### Changed

- added a production-shell regression test that authenticates a fixture and
  traverses Today, Work, Knowledge and Learning, preventing the historical
  Home-only production-path regression from returning;
- recorded the exact test output in
  `docs/project/agent-runs/2026-08-29-wave3-navigation-regression-guard.md`.

#### Boundary

- this is local evidence, not a Sol re-review, device U0 verdict or release
  authorization; no commit, push, merge or deployment was performed.

### MCP client handoff documentation — 2026-08-29

#### Changed

- added `docs/project/MCP_CLIENT_SETUP.md` with the product `/mcp`
  endpoint, token lifecycle, least-privilege scopes, Codex setup, Claude
  transport guidance, network boundary and operator acceptance checklist;
- linked the handoff from the project documentation index, README and status;
- explicitly separated product MCP from the tracked CyberOS stdio MCP config.
- rechecked the HTTP and MCP packages: `360` tests passed, and the
  multi-agent protocol verifier passed protocol, entrypoint and manifest checks.

#### Boundary

- client-side Codex/Claude configuration and the final operator checklist
  remain unverified; no token or credential was added to the repository;
- no commit, push, merge or deployment was performed.

### Production-shaped Docker smoke refresh — 2026-08-29

#### Verified

- the current checkout passed the disposable PostgreSQL/backend/embedding
  smoke with `19` migrations, authenticated Work history and conflict checks,
  Knowledge import/search, MCP issue/initialize/revoke/revoked,
  CORS `204/403`, provider fail-closed `500` and logout `204`.

#### Boundary

- this is local runtime evidence only; it does not prove live-provider billing,
  production deployment or release authorization;
- no commit, push, merge or deployment was performed.

### Acceptance trace reconciliation — 2026-08-29

#### Changed

- reconciled the eight product task specifications with the actual Go test
  functions and Flutter test titles;
- linked the replacement evidence in the product acceptance matrix without
  changing task lifecycle state.

#### Verified

- seven focused Go packages: `524` tests passed;
- focused HTTP acceptance set: `8` tests passed;
- focused Flutter shell/state/UI/roleplay selection exited `0` with `1`
  environment skip.

#### Boundary

- traceability is not an acceptance verdict; human U0, independent review,
  CI and release authorization remain open;
- no commit, push, merge or deployment was performed.

### CyberOS machine gate rerun — 2026-08-29

#### Verified

- local CyberOS machine gates exited `0` for build, lint, backend tests,
  content-factory (`200` cases), Flutter tests (`51` passed and `2` environment
  skips), coverage and status-page generation;
- the gate reported `GATES: GREEN (machine gates only)` and retained the HITL
  requirement.
- a fresh in-app browser smoke loaded `http://localhost:8093/` with title
  `DevEnglish` and rendered the Today workspace shell.

#### Boundary

- CyberOS doctor was skipped because the local memory CLI is unavailable;
- this does not close real-device/operator acceptance, independent review,
  GitHub CI, provider/billing checks or release authorization;
- no commit, push, merge or deployment was performed.

### Work trash action boundary — 2026-08-29

#### Changed

- configured REST legacy trash routes for Project, Task and Decision now use
  the canonical `work.entity.trash` challenge, expected-version validation,
  replay handling and receipt boundary;
- added a production-path MCP test proving `entity_trash` returns an accepted
  Work receipt and removes the canonical record from the active view;
- preserved direct mutation only for intentionally unconfigured compatibility
  fixtures.

#### Verified

- HTTP package: `228` tests passed;
- MCP package: `132` tests passed;
- `rtk git diff --check`: exit `0`.

#### Boundary

- purge remains a separate lifecycle operation;
- no commit, push, merge or deployment was performed.

### Acceptance audit and release-web smoke refresh — 2026-08-29

#### Verified

- refreshed `http://localhost:8093/` from the rebuilt release artifact;
- Today rendered the canonical workspace shell, next safe action, assistant
  composer and source-backed evidence;
- the browser capture contained no warning/error log entries;
- coverage-gate regression tests (`4`) and the multi-agent protocol verifier
  passed after the documentation refresh.

#### Boundary

- this is local web smoke evidence, not real-device/operator U0 acceptance;
- independent current review, GitHub CI from the delivery commit, provider and
  billing reconciliation, and human release authorization remain open;
- no commit, push, merge or deployment was performed.

### Migration dry-run and production runbook hardening — 2026-08-29

#### Changed

- added `DRY_RUN=YES` to `scripts/db_migrate.sh`; it validates the migration
  set, reports applied/pending versions and does not create or mutate
  `schema_migrations`;
- extended `scripts/db_migrate_test.sh` to prove fresh-database dry-run,
  ordered application, idempotent re-application and rollback of a failing
  migration;
- updated `docs/project/PRODUCTION_RUNBOOK.md` with PostgreSQL readiness,
  migration preview and disposable-volume verification steps.

#### Verified

- disposable PostgreSQL migration harness exited `0`; all `19` migrations
  were reported pending during dry-run, then applied as `000..018`, reported
  already applied on the second run and passed the failure-rollback check;
- invalid `DRY_RUN` input failed closed with exit `2` before any Docker/database
  operation; shell syntax checks and `git diff --check` exited `0`.

#### Boundary

- dry-run is an operator preview only; it does not replace backup, migration
  review or production release authorization;
- no production database was changed, and no commit, push, merge or deploy
  was performed.

### Strict backend coverage gate and acceptance refresh — 2026-08-29

#### Changed

- added `scripts/backend_coverage_gate.mjs`, which validates raw Go statement
  counts with an integer comparison, refuses missing or under-threshold core
  packages and can enforce an overall R0 floor;
- added the machine-readable R0 baseline manifest
  `docs/project/coverage/2026-08-29-r0-backend-coverage-baseline.json`;
- added executable tests for coverage-gate threshold, baseline-floor and
  fail-closed input behavior, and run them before the CI profile gate;
- wired the gate into `.github/workflows/postgres-integrations.yml` and added
  CI coverage artifact upload plus total-floor enforcement;
- added Work service boundary coverage for nil repository/options and the
  default clock path.
- moved the R0 coverage JSON to `docs/project/coverage/` so the multi-agent
  manifest verifier only scans agent-run manifests; the verifier and coverage
  checker regression tests pass again.

#### Verified

- PostgreSQL-enabled Go and race suites: `1016` tests passed across `23`
  packages for each run;
- total backend coverage: `77.45%` (`9107/11759` statements);
- raw `>=90%` package gate passed for Application `700/773`, Assistant
  `274/295`, Connectors `1110/1232`, HTTP API `1138/1260`, Knowledge `833/923`,
  MCP `1351/1482` and Work `1581/1756`;
- the latest local profile is
  `/tmp/devenglish-go-db-20260829-final.cov`.
- coverage-gate regression tests: `4` passed; malformed profile and baseline
  inputs fail closed.
- recorded the R0 baseline from detached commit
  `104304f12f411dd534aa4b385e964b3b8c11ae43`: PostgreSQL-enabled Go `395`
  tests across `16` packages and raw coverage `4269/6393 = 66.78%`;
  current overall coverage is above that floor.

#### Boundary

- the baseline comparison is recorded in
  `docs/project/agent-runs/2026-08-29-r0-baseline.md`; this is not a release
  approval;
- U0/device, lifecycle/HITL, independent current review and human release
  authorization remain open;
- no commit, push, merge or deployment was performed.

### Local acceptance gate refresh — 2026-08-29

#### Verified

- Go unit suite: `837` passed across `23` packages;
- Go race suite: `837` passed across `23` packages;
- PostgreSQL-enabled coverage profile: `75.1%` total statement coverage
  (`8832/11759`); application `90.6%`, assistant `92.9%`, knowledge `90.2%`,
  MCP `91.2%`, work `85.5%`, connectors `80.9%` and HTTP API `83.8%`;
- Flutter analyze, full suite (`49` passed, `2` environment skips), Chrome
  route smoke, web release build and Android debug build all exited `0`;
- fresh in-app browser smoke loaded Today, navigated Work/Knowledge/Learning,
  returned canonical Knowledge search results and captured no console
  error/warning entries;
- Go vet, gofmt, build and diff check exited `0`;
- production-shaped Docker smoke exited `0` with migrations `19`, Work/MCP/
  Knowledge/auth/CORS/provider-fail-closed/history checks passing;
- disposable legacy-upgrade and 100,000-chunk capacity gates exited `0`;
- official MCP Go SDK conformance exited `0`; the multi-agent protocol verifier
  exited `0` and the CyberOS-only verifier retained its known protected WIP
  finding in `.github/workflows/ci.yml`.

#### Boundary

- strict `90%` touched/new package coverage remains open for HTTP API, Work and
  connectors; total coverage is tracked against the R0 baseline. Real-device/
  operator U0, lifecycle/HITL, independent review and human release
  authorization also remain open;
- no commit, push, merge or deployment was performed.

### Persistence and service boundary test refresh — 2026-08-29

#### Changed

- added PostgreSQL MCP token persistence coverage for invalid replay metadata,
  conflicting idempotency, expiry/revoke behavior and corrupt scope rows;
- added Knowledge service boundary coverage for unsupported capabilities,
  repository error propagation, blank-search short-circuiting and invalid
  workspace/entity identifiers;
- added Application factory coverage for missing durable pools and safe
  database error mapping.

#### Verified

- Go unit suite: `749` passed across `23` packages;
- Go race suite: `749` passed across `23` packages;
- PostgreSQL-enabled suite: `770` passed; `73.4%` total statement coverage
  (`8635/11759`); application `84.3%`, knowledge `84.1%`, MCP `85.0%` and
  HTTP API `83.8%`;
- vet, build, diff check and multi-agent protocol verifier passed.

#### Boundary

- strict `90%` coverage, U0/device, lifecycle/HITL, independent review and
  human release authorization remain open;
- CyberOS-only verifier still reports the protected pre-existing
  `.github/workflows/ci.yml` WIP snapshot; it was preserved;
- no commit, push, merge or deployment was performed.

### HTTP action boundary test refresh — 2026-08-29

#### Changed

- added boundary coverage for action target projection, idempotency validation,
  action/MCP error mapping, missing services, malformed bodies, confirmation
  keys and empty token scopes;
- refreshed the evidence to `719` no-DB / `733` PostgreSQL tests, with `73.1%`
  total coverage and HTTP API at `83.8%`.

#### Boundary

- strict `90%` coverage, U0/device, lifecycle/HITL, independent review and
  human release authorization remain open;
- no commit, push, merge or deployment was performed.

### V2 workspace-scope failure matrix — 2026-08-29

#### Changed

- added `42` route-level regression cases proving V2 adapters fail closed when
  workspace initialization fails;
- recorded the current `691` no-DB / `705` PostgreSQL test evidence and
  `73.0%` total coverage, with HTTP API at `82.5%`.

#### Boundary

- strict `90%` coverage, U0/device, lifecycle/HITL, independent review and
  human release authorization remain open;
- no commit, push, merge or deployment was performed.

### Acceptance refresh and Learning browser-smoke repair — 2026-08-29

#### Changed

- kept the Learning data-separation invariant visible even when legacy learning
  metrics are unavailable;
- refreshed the Learning responsive goldens after the copy contract fix;
- recorded the current local acceptance evidence in
  `docs/project/agent-runs/2026-08-29-acceptance-refresh.md`.

#### Verified

- Flutter full suite: `49` passed, `2` environment-skipped, exit `0`;
- Flutter Chrome navigation smoke: exit `0`, Today → Work → Knowledge →
  Learning passed;
- rebuilt web release with `--no-wasm-dry-run`: exit `0`; in-app browser
  rendered Today and Learning with no console warnings/errors;
- Go tests/race: `649` passed across `23` packages; PostgreSQL-enabled run:
  `663` passed, `72.4%` total coverage;
- focused HTTP API: `89` passed, `77.1%` package coverage; connectors: `34`
  PostgreSQL-enabled tests, `80.9%` coverage;
- Android release APK and production-shaped Docker smoke remain green.

#### Boundary

- the strict `90%` coverage target, real-device/operator U0, lifecycle
  acceptance, independent review and human release authorization remain open;
- no commit, push, merge or deployment was performed.

### HTTP error boundary and coverage refresh — 2026-08-29

#### Changed

- added v2 method/`Allow` contract regression coverage;
- added workspace/user isolation, bounded query parsing and connector
  pre-provider input validation coverage;
- verified product and connector error mappings at the HTTP boundary;
- redacted credential-shaped values from generic HTTP error responses;
- added bounded assistant, application and MCP validation tests without
  changing canonical data or action semantics.

#### Verified

- Go tests and race tests: `606` passed across `23` packages;
- DB-enabled coverage run: `620` tests passed, `70.8%` total statement
  coverage; application `83.4%`, assistant `92.9%`, knowledge `82.6%`, MCP
  `84.1%`, work `85.5%`, connectors `78.2%` and HTTP API `64.1%`;
- Flutter full suite: `49` passed, `2` environment-skipped, exit `0` after
  repairing the local Dart AOT runtime path;
- vet, build and diff check passed;
- production-shaped Docker smoke passed with `19` migrations and the complete
  Work/MCP/Knowledge/auth/CORS/provider-fail-closed flow.

#### Boundary

- the plan's strict `90%` coverage threshold, real-device/operator acceptance,
  lifecycle acceptance, independent review
  and human release authorization remain open;
- no commit, push, merge or deployment was performed.

### Usage accounting hardening — 2026-08-29

#### Changed

- preserved provider-reported DeepSeek token usage and model identity;
- stopped persisting preflight reservation amounts as final assistant usage;
- marked STT, pronunciation and TTS records explicitly unavailable when no
  provider billing metadata is returned, while retaining known TTS character
  metadata;
- exposed `unavailableRecords` in the usage summary and Settings so incomplete
  billing data is visible.

#### Verified

- focused application/learning/HTTP tests: `76` passed;
- Go tests and race tests: `550` passed across `23` packages;
- vet, build and diff check passed;
- Flutter analyze passed;
- production-shaped Docker smoke passed with `19` migrations and the complete
  Work/MCP/Knowledge/auth/CORS/provider-fail-closed flow.

#### Boundary

- cost is still an application estimate based on provider-reported tokens and
  model rates, not a provider invoice;
- coverage is `68.6%` against the `90%` target, and Flutter full-suite,
  real-device/operator, lifecycle, independent-review and release-authorization
  gates remain open;
- no commit, push, merge or deployment was performed.

### Conversation history, MCP SDK conformance and mobile visual smoke — 2026-08-29

#### Added

- lightweight assistant conversation history with lazy transcript reopening in
  REST and Flutter;
- an independent official MCP Go SDK conformance test covering modern
  negotiation, discovery, resource read and bearer-authenticated tool use.

#### Verified

- Go tests and race tests: `546` passed across `23` packages;
- official MCP Go SDK conformance: `62/62` passed;
- production-shaped Docker smoke: `19` migrations and the complete
  Work/MCP/Knowledge/auth/CORS/provider-fail-closed flow passed;
- Flutter analyze and web release build passed with
  `--no-wasm-dry-run`;
- a fresh browser visual smoke rendered Today, Work, Knowledge and Learning at
  mobile viewport `390×844` with no browser warnings/errors.

#### Boundary

- Flutter full-suite execution remains inconclusive at compiler startup,
  coverage is `68.6%` against the `90%` target, and real-device/operator,
  lifecycle and release-authorization gates remain open;
- no commit, push, merge or deployment was performed.

### Legacy upgrade harness aligned with migration 018 — 2026-08-29

#### Changed

- extended the disposable legacy upgrade and idempotency harness through
  `018_assistant_conversation_context.sql`;
- added a schema assertion for the assistant conversation context columns,
  constraint and workspace-scoped index after upgrade;
- kept legacy source snapshots and workspace backfill/isolation checks as the
  compatibility boundary.

#### Verified

- disposable Compose upgrade passed from a blank database: migrations
  `000..018`, second-run idempotency, legacy snapshot preservation and the
  assistant context schema assertion all passed;
- shell syntax and diff checks passed.

#### Boundary

- this evidence is local/disposable only; no production migration or release
  approval is implied.

### MCP Streamable HTTP negotiation hardening — 2026-08-29

#### Changed

- enforced the Streamable HTTP POST `Accept` contract: clients must allow both
  `application/json` and `text/event-stream`, including parameterized media
  types and case-insensitive tokens;
- validated an optional `MCP-Protocol-Version` header against the server's
  supported versions while retaining the protocol's backwards-compatible
  behavior when the header is omitted;
- returned `202 Accepted` with an empty body for MCP notifications;
- kept GET explicitly unsupported with `405 Allow: POST` because this server
  does not publish unsolicited SSE events;
- updated application fixtures and the production-shaped smoke to send the
  required negotiation header.

#### Verified

- MCP focused tests: `61` passed; HTTP API + MCP focused tests: `100` passed;
- Go tests: `544` passed across 23 packages; race suite: `544` passed; vet,
  build and diff check passed;
- production-shaped Docker smoke passed with `19` migrations and the complete
  Work/MCP/Knowledge/auth/CORS/provider-fail-closed flow;
- no Flutter source changed in this slice; the latest full Flutter compiler
  rerun remains inconclusive at exit `130` and is not counted as a pass.

#### Boundary

- independent MCP client conformance, strict 90% coverage, real-device U0,
  lifecycle acceptance and human release authorization remain open;
- no commit, push, merge or deployment was performed.

### Grounded assistant context continuation — 2026-08-29

#### Changed

- added a typed `ContextRef` boundary for Project, Task, Decision and Knowledge
  Source references across assistant retrieval, generation and conversation
  persistence;
- added `POST /api/v2/assistant/conversations` and message support for
  `contextType`/`contextId`, with a 409 when a conversation attempts to switch
  its pinned canonical context;
- resolved pinned context from the authenticated workspace before creating a
  conversation, so a missing entity cannot leave an orphan conversation;
- persisted conversation context through migration `018_assistant_conversation_context.sql`
  and restored it on bootstrap/continuation;
- exposed Work and Knowledge "Ask assistant with this context" actions and a
  visible pinned-context indicator in the assistant surface;
- kept pinned canonical evidence first while bounding the final assistant
  retrieval set to 20 items, and labeled it separately from lexical/hybrid
  retrieval;
- bounded provider prompt snippets by Unicode code points and added a regression
  check so long Vietnamese evidence remains valid UTF-8 and within the limit;

#### Verified

- Go tests: `543` passed across 23 packages; race suite: `543` passed; vet,
  build and formatting passed;
- Flutter analyze passed. The latest full/isolated Flutter test rerun stayed at
  the frontend compiler loading phase and was stopped with exit `130`; it is
  recorded as inconclusive, not as a pass. Earlier focused workspace tests
  remain the available Flutter test evidence; web release build passed;
- disposable PostgreSQL migration run applied migration `018` successfully;
- Android debug build generated `build/app/outputs/flutter-apk/app-debug.apk`;
- DB-enabled coverage profile measured `68.6%` total statement coverage;
  application `82.0%`, assistant `84.1%`, Knowledge `82.1%`, MCP `82.0%`, Work
  `84.5%`, connectors `72.2%` and HTTP API `62.8%`;
- production-shaped Docker smoke passed with `19` migrations and the complete
  Work/MCP/Knowledge/auth/CORS/provider-fail-closed flow;
- fresh browser visual smoke rendered the non-empty Today surface from
  `build/web` at `http://localhost:8094/`.

#### Boundary

- the strict 90% coverage gate, real-device/operator U0 acceptance, independent
  Sol Ultra verdict for this latest source change, lifecycle acceptance and
  human authorization for commit/push/merge/deploy remain open;
- the Flutter full-suite rerun remains inconclusive; no Flutter release approval
  is inferred from the successful analyze and web-build gates;
- no commit, push, merge or deployment was performed by this continuation.

### Product reset continuation — 2026-08-29

#### Changed

- added canonical Work detail sheets for Project, Task and Decision, including
  version/status/context fields and backend activity history from the v2
  history endpoints;
- kept audit `before`/`after` snapshots in the backend boundary instead of
  rendering raw snapshot data in the client;
- added an optional voice composer to Today, Work and Knowledge assistant
  surfaces: push-to-talk captures PCM audio, transcribes into the existing
  editable text field, and keeps sending text as the canonical submit path;
- added on-demand read-aloud controls for assistant turns; TTS failure leaves
  the text conversation usable and never replaces the transcript;
- bound full-application MCP Project, Task, Decision and manual Knowledge
  writes to the authenticated token principal, even when the transport
  context has no caller identity;
- aligned memory Work reads with PostgreSQL by hiding soft-deleted records
  from ordinary Get operations;
- rejected orphan Knowledge topics, claims, evidence revisions and chunks in
  the memory adapter;
- added application, HTTP v2, Knowledge, MCP and cross-module contract tests
  for blank input, malformed JSON, workspace isolation, version conflicts,
  source evidence, replay and principal binding;
- exposed safe lifecycle errors consistently for v2 action and purge routes;
- moved canonical Workspace composition behind `features/workspace/` and made
  the app shell import the public Today/Work/Knowledge/Learning feature
  boundaries; the old workspace screen is now a compatibility facade;

#### Verified

- Go test suite: 527 passed across 23 packages; race suite: 527 passed;
  vet and build passed;
- Flutter analyze passed; Flutter tests: 49 passed, 2 skipped; web release
  build and Android debug build passed;
- production-shaped disposable smoke passed with 18 migrations, Work
  create/update/conflict/history 201/200/409/200 for Project, Task and Decision,
  MCP issue/initialize/revoke/revoked 201/200/204/401, Knowledge
  import/search 201/200, allowed/forbidden CORS 204/403,
  provider fail-closed 500 and logout 204;
- current DB-enabled all-backend statement coverage measured 68.4%; application
  81.3%, Knowledge 82.1%, MCP 82.0% and Work 84.5%, so the plan's 90% coverage
  gate remains open.
- Flutter release web smoke at `http://localhost:8094/` rendered Today and
  Work from the rebuilt artifact after a coordinate navigation click; no app
  warning/error logs were observed. The browser semantic tree exposed only
  Flutter's accessibility placeholder, so this is visual smoke evidence rather
  than a full DOM-semantic assertion.

#### Known risks

- the latest bounded Sol Ultra review attempt terminated at the usage limit
  before a verdict; it is neither approved nor rejected;
- U0 operator/device acceptance, lifecycle acceptance, coverage threshold and
  human authorization for commit, push, merge or deployment remain open;
- no commit, push, merge or deployment was performed by this continuation.

### Product reset hardening — 2026-08-28

#### Added

- explicit Knowledge connector sync status showing checked/imported/unchanged
  counts, cursor state and read-only boundaries;
- a connector-sync state golden in addition to the loading, empty,
  stale/degraded, provider-error, offline and conflict state fixtures;
- durable safe-write reservations and digest-only PostgreSQL MCP token
  persistence, including restart/revoke coverage;
- full initial Drive bootstrap/removal tombstones and workspace-scoped
  connector identity; product-reset task specs `TASK-PRODUCT-001` through
  `TASK-PRODUCT-008` with traceable acceptance criteria;
- guarded production-shaped Docker smoke coverage for embedding readiness,
  authenticated workspace import/search, MCP token issue/initialize/revoke,
  allowed and forbidden CORS, fail-closed provider errors and session
  revocation.
- idempotent manual-source retry handling that compares stable canonical
  identity instead of import timestamps;
- workspace-scoped safe-write receipt identity and Drive trashed-file
  tombstone/restore coverage;
- an opt-in PostgreSQL application integration test proving repeated manual
  imports preserve one revision/chunk and the source-item version.

#### Verified

- full Flutter test suite: 43 passed, 2 skipped; analyze, web release build and
  Android debug build passed;
- canonical browser smoke reached Today, Work, Knowledge and Learning against
  the local backend;
- Go tests: 438 passed in 23 packages; race suite: 438 passed; vet, build,
  formatting and diff checks passed;
- production runtime smoke passed with migrations=17, embedding=200,
  login=200, session=200, mcp-issue=201, mcp-initialize=200,
  mcp-revoke=204, mcp-revoked=401, knowledge-import=201,
  knowledge-search=200, cors=204, forbidden-cors=403,
  provider-fail-closed=500 and logout=204;
- all-backend statement coverage measured `55.5%`, so the plan threshold of
  `90%` remains open; database-only paths need the configured fixture to
  contribute coverage.

#### Known risks

- backend package coverage remains below the 90% statement-coverage threshold
  in the plan and must be raised before release acceptance;
- U0 real-device verdict, human lifecycle acceptance and human release
  authorization are still required; the previous bounded Sol Ultra repair
  review is approved and the latest source repair is awaiting its bounded
  re-review. Local green gates do not authorize Git operations or deployment.

### Product reset implementation — 2026-08-28

#### Added

- Product-reset status and roadmap for the Today/Work/Knowledge/Learning V1;
- U0 high-fidelity mobile/desktop specification and prototype walkthrough;
- disposable legacy-upgrade migration harness with workspace backfill,
  legacy-snapshot preservation and second-run idempotency checks;
- disposable 100,000-knowledge-chunk capacity harness with pagination and FTS
  index checks;
- MCP replay/version-conflict/evidence-parity coverage and usage reservations
  scoped by feature.

#### Verified

- local PostgreSQL migrations `000..015`, embedding sidecar readiness and
  384-dimensional embedding response;
- backend, Flutter, browser-smoke, migration, capacity and MCP focused gates
  for the current implementation state.

#### Known risks

- U0 still needs operator/device verdict and final golden screenshots;
- full S7 race/coverage, production-shaped auth/provider failure and
  independent release review are still open;
- the legacy-sized Flutter screen remains a staged migration seam even though
  the new Workspace shell is functional.

### Added

- A vendor-neutral `.agents` operating system that assigns implementation to
  Luna Max and independent review/decision authority to Sol Ultra.
- Standard change-control, quality-gate and hotfix workflows plus reusable
  implementation brief, handoff and review-report templates.
- Adaptive roleplay fallback guidance for Vietnamese or help-seeking answers:
  brief Vietnamese context, one immediately usable English starter and one
  easy scenario-aware follow-up question without claiming a technical answer
  was supplied.

### Changed

- Reconciled the four approved Wave 2 cones into the dedicated, uncommitted
  `integration/wave-2` worktree without touching the canonical WIP checkout.
- Completed the bounded Wave 2 Connectors repair in the isolated
  `TASK-002C-connectors` worktree: revision deduplication is workspace-scoped,
  and GitHub safe-write receipts reserve `pending` before provider mutation
  with fail-closed `uncertain` replay handling.
- Orchestration now keeps the controller alive while a worker/reviewer thread is
  active: it waits on the exact Codex thread or leaves a bounded heartbeat
  wake-up, then reads the authoritative result before changing the manifest.
- Added a single-controller multi-agent protocol and run-manifest verifier that
  blocks duplicate active task assignments, descendant reviewers, excess
  reviewer concurrency and timeout-triggered replacement workflows.
- Added evidence-first subagent reconciliation: lifecycle timeouts do not imply
  missing output; the controller must read and record the authoritative message
  or evidence artifact before changing orchestration state or selecting work.
- Applied the RC1 review-fix batch for the production migration boundary,
  example-secret sentinels, Chrome smoke documentation and release evidence.
- Added an internal guidance-only roleplay signal so the learning service skips
  final technical scoring and removes technical credit from help-only turns.

### Verified

- The multi-agent protocol verifier passed its required-rule, entrypoint and
  run-manifest checks; the existing CyberOS-only verifier still passed all five
  entrypoint, ownership, runtime and WIP-preservation checks.
- Phase 1 local verification passed: Go tests/race/vet, Flutter format/analyze/
  tests/production tests/release build, backend-aware Chrome smoke, disposable
  PostgreSQL migration first-run/idempotency/rollback and repository integration,
  local/CI/production Compose validation, and backend `/healthz`/`/readyz`.
- `rtk go test ./backend/internal/ai ./backend/internal/learning` passed with
  `30 passed in 2 packages`; `rtk go test ./...` passed with
  `55 passed in 11 packages`.
- Sol Ultra independently reran the final diff: all 55 Go tests and all 55
  race-detector tests passed, `go vet` found no issues, backend shell syntax
  checks passed, and content-factory generated 200 cases without changing the
  generated-file checksum.
- Full-diacritic Vietnamese detection, unaccented markers, scenario-aware
  starters, English regression, service-level zero-score gating and the
  fake-server DeepSeek prompt contract passed without live provider access.
- Sol Ultra R6 independently approved TASK-005-R2 after source inspection,
  focused race/coverage review and repository gates: 31 connector tests at
  91.5% coverage, a repaired-case `-race -count=50` stress run, 89 repository
  tests across 12 packages and repository vet all exited 0.
- Sol Ultra R7 independently reviewed the staged integration branch: the four
  packages passed race tests, per-package coverage was 91.5% or higher, the
  repository test and vet gates exited 0, and the exact 32-file cone was clean.

### Known risks

- Wave 2 integration is blocked: `005_knowledge.sql` and `006_work.sql`
  require `003_platform_foundation.sql` to create `workspaces`, but that
  migration is outside the currently authorized integration cones. The
  canonical checkout contains foundation files as WIP; they were not copied.
- PostgreSQL migration runtime and live providers remain unverified. The
  isolated migration test database was not configured, and the shared
  development database was not used.

### Next actions

- [ ] Authorize the exact foundation migration target that owns `workspaces`,
  then rerun the complete migration chain and Sol Ultra R8 review.
- [ ] After R8 verifies the integration branch, request explicit human
  authorization before commit or push.
- [ ] Commit and push the review-fix, then obtain CI on the exact new SHA and one independent approval before merge.
- [ ] Obtain one independent approval, merge PR #1, and record the exact merge and CI evidence.
- [ ] After PR #1 is merged, start UsageGuard in a separate branch and PR: 300,000 VND monthly hard cap, provider/feature quotas, actual DeepSeek/Groq/Azure usage, reservation/commit/release, concurrency safety, structured quota errors and regression tests.

### Deferred

- Live DeepSeek roleplay behavior remains unverified; the adaptive prompt
  contract is covered by the local fake server only.
- Real HTTPS deployment, DNS, certificate and public-origin verification.
- Production backup/restore rehearsal on the real target.
- Private GitHub repository provisioning and deeper project indexing.
- Shared session storage for a future multi-instance deployment.

## [2026-08-23] — RC1 hardening and release controls

### Added

- Backend-aware Chrome smoke coverage for Home, Practice, Review and Progress.
- A 90-second client timeout for provider-backed Work Context requests and a deterministic timeout regression test.
- PostgreSQL-backed `/readyz`, production Flutter/Caddy packaging and hardened production Compose topology.
- Versioned migration tracking, transactional migration execution, guarded backup/restore scripts and disposable PostgreSQL integration coverage.
- Production authentication boundaries with server-issued `Secure`, `HttpOnly`, `SameSite=Strict` sessions, login/logout, session restoration and expiry handling.
- Encrypted server-side DeepSeek secret storage with metadata-only settings responses and safe capability-specific provider probes.
- Atomic PostgreSQL and in-memory writing-outcome persistence.
- Repeatable provider-backed API/PostgreSQL smoke coverage for diagnostic, mission/writing, work import, review, roleplay, Copilot and GitHub import flows.

### Verified

- Go tests, race tests, vet, Flutter analyze/tests/build, browser smoke, API smoke and disposable PostgreSQL integration checks pass in CI.
- Local production-mode smoke passed for `/healthz`, `/readyz`, login, session lookup/logout and HTTPS-origin CORS validation.
- Live local provider checks passed for DeepSeek, Groq Whisper, Azure Pronunciation and Azure Neural TTS.
- Protected `main` requires backend, Flutter, PostgreSQL and production checks plus one approving review.
- PR #1 is open on branch `chore/rc1-timeout-browser-smoke`; it remains draft pending independent review and merge.

### Known risks

- The review-fix remains uncommitted and unpushed; CI on its exact SHA and one independent approval are still pending.
- Secure-cookie behavior on a real HTTPS origin and public deployment evidence remain unverified.
- AI usage values are currently service-side estimates; provider billing reconciliation belongs to UsageGuard.

## [2026-08-22] — Provider-backed learning and recovery flows

### Added

- Provider-backed speaking flow: Groq transcription, persisted speaking session, Azure pronunciation/prosody assessment and Azure TTS playback.
- Atomic writing evaluation, mistake memory, skill update and mission-completion flow.
- Release-web serving and local Docker development topology.
- Explicit Speaking permission, recording, upload, transcription, assessment, TTS success and retry states.

### Verified

- Diagnostic → daily mission → writing feedback → review/progress learning loop against PostgreSQL.
- Roleplay and Copilot requests with persisted usage records.
- PostgreSQL backup/restore rehearsal with guarded restore confirmation.
- Local browser rendering and navigation for Home, Practice, Review, Progress and Settings.

### Known limitations

- The local in-app browser did not complete microphone permission flow; real Chrome/device capture was verified separately.
- Real HTTPS deployment and private GitHub access were not yet provisioned.

## Changelog rules

- Keep entries newest first.
- Use `Added`, `Changed`, `Fixed`, `Verified`, `Known risks` and `Deferred` sections where useful.
- Record the verification boundary and exact follow-up instead of marking an item complete without evidence.
- Never place API keys, passwords, tokens or other secret values in this file.
