# DevEnglish test log

## 2026-08-30 — CyberOS WIP snapshot rebaseline

- The operator confirmed that all post-baseline paths in the protected-WIP
  manifest are valid user-owned WIP outside `TASK-CYB-001`.
- The external protected-WIP manifest was re-baselined to the confirmed
  current hashes; no protected file content was changed.
- `rtk bash scripts/verify_cyberos_only.sh` — exit `0`; `entrypoints`,
  `.agents`, `.claude/skills`, `static_runtime` and `backup_and_scope` passed.
- A full manifest comparison reported `0` hash mismatches.
- No application source, CyberOS config, task status, commit, push, merge or
  deployment was changed.

## 2026-08-30 — Knowledge import mutation boundary

- `rtk go test ./backend/internal/httpapi -count=1` — exit `0`; `235` tests
  passed, including missing-key rejection for REST Knowledge import and all
  existing HTTP boundary/contract tests;
- `rtk flutter test test/workspace_api_test.dart test/workspace_ui_test.dart`
  — exit `0`; `28` tests passed, including the real Flutter API request
  carrying `Idempotency-Key` for manual source import;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk bash -n scripts/production_runtime_smoke.sh` — exit `0`;
- `rtk go test ./backend/... -count=1` and
  `rtk go test -race ./backend/... -count=1` — both exit `0`; `989` tests
  passed in `23` packages;
- `rtk go vet ./backend/...` and
  `rtk go build -o /tmp/devenglish-server-import-key ./backend/cmd/server`
  — both exit `0`;
- `rtk flutter test` — exit `0`; `64` tests passed and `2` environment skips;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES ./scripts/production_runtime_smoke.sh`
  — exit `0`; the disposable stack passed migrations, auth/session,
  workspace mutation/conflict/history, MCP, Knowledge import/search, CORS and
  provider fail-closed checks;
- deterministic source/revision identity remains the import retry mechanism;
  no migration or durable Knowledge replay record was added;
- no commit, push, merge, deployment or production data changed.

## 2026-08-30 — Bootstrap capability state

- `rtk go test ./backend/internal/application ./backend/internal/httpapi
  -count=1` — exit `0`; `294` tests passed in the two focused packages;
- `rtk go test ./backend/... -count=1` and
  `rtk go test -race ./backend/... -count=1` — both exit `0`; `988` tests
  passed in `23` packages;
- `rtk go vet ./backend/...` and
  `rtk go build -o /tmp/devenglish-server-capabilities ./backend/cmd/server`
  — both exit `0`;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk flutter test test/workspace_api_test.dart test/workspace_ui_test.dart`
  — exit `0`; `28` tests passed, including canonical capability parsing and
  Today readiness rendering;
- `rtk flutter test` — exit `0`; `64` tests passed and `2` environment skips;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`; `build/web`
  generated successfully;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES ./scripts/production_runtime_smoke.sh`
  — exit `0`; the disposable stack verified `19` migrations, embedding,
  secure auth/session, bootstrap capability state (`work=true`), Work
  version/conflict/history, Knowledge import/search, MCP revoke/replay, CORS
  and provider fail-closed behavior;
- additive REST/Flutter contract and UI state only; no migration, commit,
  push, merge, deployment or production data changed.

## 2026-08-30 — MCP mutation contract documentation

- `rtk go test ./backend/internal/mcp -count=1` — exit `0`; `133` tests
  passed. The focused run covers the full application registry contract,
  including the mutating
  `knowledge_import_manual` description and idempotency-key requirement;
- `rtk go test ./backend/... -count=1` — exit `0`; `987` tests passed in `23`
  packages;
- `rtk go test -race ./backend/... -count=1` — exit `0`; `987` tests passed
  in `23` packages;
- `rtk go vet ./backend/...` and `rtk go build -o /tmp/devenglish-server
  ./backend/cmd/server` — both exit `0`;
- source-only documentation/comment alignment; no migration, task status,
  commit, push, merge, deployment or production data changed.

## 2026-08-30 — Preserve Knowledge relevance through application search

- `rtk go test ./backend/internal/application -count=1` — exit `0`; `59`
  tests passed, including a regression that keeps the repository-provided
  hybrid ranking order and applies the two-item limit without ID reordering;
- `rtk go test ./backend/... -count=1` — exit `0`; `986` tests passed in `23`
  packages;
- `rtk go test -race ./backend/... -count=1` — exit `0`; `986` tests passed in
  `23` packages;
- `rtk go vet ./backend/...` — exit `0`, no issues;
- `rtk go build -o /tmp/devenglish-server ./backend/cmd/server` — exit `0`;
- `rtk git diff --check` — exit `0`;
- the change removes only the application-level ID sort. PostgreSQL RRF and
  memory lexical ranking remain the owners of Knowledge relevance; Work list
  order remains the repository-defined order.

Boundary:

- this is a local regression and correctness hardening slice; no task status,
  commit, push, merge, deployment or production data changed.

## 2026-08-30 — Sol Ultra review attempt

- one bounded follow-up was sent to the existing reviewer thread;
- it ended with the platform usage-limit error before a terminal verdict;
- no replacement/parallel reviewer was created and no source/task/Git state was
  changed. Evidence:
  `docs/project/agent-runs/2026-08-30-sol-review-usage-limit.md`.

## 2026-08-30 — MCP handshake instructions and client template

- `rtk go test ./backend/internal/mcp` — exit `0`; `132` tests passed,
  including configured instructions in `initialize`, default instruction
  fallback, the under-512-rune guidance limit and `server/discover` parity;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES
  scripts/production_runtime_smoke.sh` — exit `0`; rebuilt the backend from
  the current source and passed the disposable 19-migration runtime sequence,
  including MCP issue/initialize-with-instructions/revoke/revoked
  `201/200/204/401`;
- `rtk git diff --check` — exit `0`;
- the copy-only Codex TOML template was inspected for the local endpoint,
  environment-backed bearer token, read-only allowlist, prompt approval and
  bounded startup/tool timeouts; no token or credential value is present;
- the active `.mcp.json`, `.codex/config.toml` and `.agents/mcp_config.json`
  remain CyberOS-only. No user-level client was changed and no token was
  issued.

Boundary:

- this proves the server-side guidance and safe handoff artifact only; actual
  Codex/Claude operator connection, scoped discovery and revoke/401 acceptance
  remain open.

## 2026-08-30 — Current production-shaped runtime smoke

- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES scripts/production_runtime_smoke.sh`
  — exit `0`; the disposable stack applied `19` migrations and passed
  authentication/session, Project/Task/Decision create-update-history and
  stale-version `409` checks, MCP issue/initialize/revoke/replay, Knowledge
  import/search, CORS allow/deny, provider fail-closed `500` and logout;
- the run used PostgreSQL, the embedding sidecar and a loopback fake provider,
  then removed its disposable stack. It is local runtime evidence, not live
  provider, CI, production or release evidence.

## 2026-08-30 — CyberOS machine gates

- `rtk bash .cyberos/cuo/gates/run-gates.sh` — exit `0`; build, lint, backend
  test, contentfactory (200 generated evaluation cases), Flutter test and
  configured coverage gates completed successfully;
- the final output was `GATES: GREEN (machine gates only)`; doctor was skipped
  because the CyberOS memory CLI was not importable;
- the command explicitly retained human review/final acceptance, and no task
  was set to `done`. Evidence:
  `docs/project/agent-runs/2026-08-30-cyberos-gates.md`.

## 2026-08-30 — Keyboard-accessible push-to-talk

Scope:

- make the canonical push-to-talk control reachable through keyboard focus;
- keep a held Space/Enter key aligned with the existing start/release lifecycle;
- preserve a visible focus state and cancel on focus loss;
- revalidate the web build and canonical Chrome navigation after the change.

Verified commands and observed results:

- `rtk flutter test test/workspace_ui_test.dart --reporter compact` — exit `0`;
  `19` tests passed, including Space and Enter press/release activation;
- `rtk flutter test test/workspace_ui_test.dart test/workspace_golden_test.dart
  test/workspace_state_golden_test.dart --reporter compact` — exit `0`; `32`
  focused UI/golden/state tests passed;
- `rtk flutter test --reporter compact` — exit `0`; `63` passed and `2`
  environment skips;
- `rtk flutter analyze` — exit `0`, no issues;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`, `build/web`
  generated;
- `rtk flutter test -d chrome
  --dart-define=DEVENGLISH_BROWSER_SMOKE=true test/browser_smoke_test.dart
  --reporter compact` — exit `0`; Today → Work → Knowledge → Learning passed.
  The integration-test plugin warning is informational for this direct
  `flutter test` invocation.
- `rtk env TAILSCALE_IP=100.126.52.73
  DEVENGLISH_APK_OUTPUT=/tmp/devenglish-current-keyboard.apk
  scripts/android_local_apk.sh` — exit `0`; rebuilt the current release APK
  at `build/app/outputs/flutter-apk/app-release.apk` (52.2 MB).
- `rtk env ANDROID_APK_PATH=/tmp/devenglish-current-keyboard.apk
  ANDROID_DEVICE_ID=emulator-5554 scripts/android_emulator_smoke.sh` — exit
  `0`; current APK installed and canonical Android smoke passed. Evidence:
  `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.Saifwv`.

Boundary:

- this verifies source-level keyboard behavior and the web navigation smoke;
  it does not replace a browser keyboard walkthrough, physical-device U0 or
  human release acceptance;
- no task status, commit, push, merge, deployment or production data changed.

## 2026-08-30 — Canonical data gates for compatibility learning

Scope:

- keep the new canonical shell independent from legacy learning bootstrap;
- load compatibility learning data only when a user opens the relevant tool;
- prove unavailable states do not render initial demo data and concurrent
  callers share one load operation;
- invalidate late user-scoped responses after logout or session expiry;
- keep late assistant, conversation-history, conversation-open, search, sync,
  mutation and action responses from repopulating the reset controller;
- prevent canonical Settings, Diagnostic and Roleplay surfaces from showing
  sample data while their backend request is pending or unavailable.

Verified commands and observed results:

- `rtk flutter analyze` — exit `0`, no issues;
- `rtk flutter test test/production_shell_test.dart --reporter compact` —
  exit `0`; the suite covered canonical-shell independence, strict Learning
  unavailable state, strict Settings unavailable state, successful real-data
  lazy-load, concurrent-load deduplication and late-response invalidation;
- `rtk flutter test test/production_shell_test.dart
  --dart-define=DEVENGLISH_ENV=production --reporter compact` — exit `0`;
  `11` tests passed, including assistant, history, opened-conversation and
  mutation late-response invalidation cases;
- `rtk flutter test test/roleplay_test.dart test/widget_test.dart
  test/workspace_ui_test.dart --reporter compact` — exit `0`.
- full `rtk flutter test --reporter compact` — exit `0`; `62` passed and `2`
  environment skips;
- `rtk flutter test test/workspace_ui_test.dart test/workspace_golden_test.dart
  test/workspace_state_golden_test.dart --reporter compact` — exit `0`; `31`
  focused UI/golden tests passed, including the 390×844 compact-header
  Settings assertion and loading/offline state goldens;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`;
- `rtk flutter build apk --debug` — exit `0`;
- a release APK built from the current source with
  `API_BASE_URL=http://100.126.52.73:8080` passed
  `rtk env ANDROID_APK_PATH=/Users/ryantruong/Project/Orther/Dev-English/build/app/outputs/flutter-apk/app-release.apk
  ANDROID_DEVICE_ID=emulator-5554 scripts/android_emulator_smoke.sh` — exit
  `0`; evidence is retained at
  `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.twHJEx`;
- `rtk flutter test -d chrome
  --dart-define=DEVENGLISH_BROWSER_SMOKE=true
  test/browser_smoke_test.dart --reporter compact` — exit `0`.

Boundary:

- this proves the source-level gate behavior and focused widget paths only;
  full repository gates and physical-device/release acceptance remain separate;
- no task status, commit, push, merge, deployment or production data was
  changed.

## 2026-08-30 — Production-shaped runtime smoke refresh

Scope:

- rerun the production-shaped Docker flow from the current source after the
  local backend image rebuild;
- verify authentication, workspace-scoped Work/Knowledge/MCP behavior,
  provider fail-closed behavior, CORS and logout/session revocation;
- use an isolated disposable PostgreSQL stack and a loopback provider endpoint,
  so this check does not send a live provider request or use production data.

Verified command and observed result:

- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES
  scripts/production_runtime_smoke.sh` — exit `0`;
- exact result: `migrations=19, embedding=200, login=200, session=200,
  project-create=201, project-update=200, project-conflict=409,
  project-history=200, task-create=201, task-update=200, task-conflict=409,
  task-history=200, decision-create=201, decision-update=200,
  decision-conflict=409, decision-history=200, mcp-issue=201,
  mcp-initialize=200, mcp-revoke=204, mcp-revoked=401, knowledge-import=201,
  knowledge-search=200, cors=204, forbidden-cors=403,
  provider-fail-closed=500, logout=204`;
- the disposable stack and database were cleaned automatically after the
  run; no commit, push, merge, deployment or production data was changed.
- `rtk flutter test -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true
  test/browser_smoke_test.dart --reporter compact` — exit `0`; the canonical
  Today → Work → Knowledge → Learning navigation smoke passed against the
  running local backend.

Boundary:

- this proves the repository-owned production-shaped path only; it does not
  replace live-provider billing/quota reconciliation, physical-device
  acceptance, CI delivery-commit evidence, current independent review or
  human release authorization.

## 2026-08-30 — Repeatable Android emulator smoke

Scope:

- turn the manually verified Android walkthrough into a repeatable local
  command without sending a provider request;
- prove APK installation, app launch, canonical Today rendering, navigation
  accessibility markers and app-process crash absence;
- keep emulator evidence separate from physical-device and release evidence.

Verified commands and observed results:

- `rtk bash -n scripts/android_emulator_smoke.sh` — exit `0`;
- `rtk env ANDROID_APK_PATH=/Users/ryantruong/Desktop/DevEnglish-tailscale.apk
  ANDROID_DEVICE_ID=emulator-5554 scripts/android_emulator_smoke.sh` — exit
  `0`; APK installation and activity launch succeeded;
- `rtk env ANDROID_APK_PATH=/Users/ryantruong/Desktop/DevEnglish-tailscale.apk
  ANDROID_DEVICE_ID=emulator-5554 make android-emulator-smoke` — exit `0`;
- after rebuilding and recreating the local Compose backend from the current
  source, `/healthz`, `/api/v2/bootstrap` and
  `GET /api/v2/assistant/conversations?limit=20` each returned HTTP `200`;
- the post-rebuild Make smoke also exited `0` and retained evidence at
  `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.TgRC7j`;
- the harness found `DEVENGLISH / TODAY`, `Canonical workspace`, `Live`, `1
  open task`, `1 source connected`, and separate `Today`, `Work`, `Knowledge`
  and `Learning` accessibility markers;
- `Work` exposed Projects/Open tasks, `Knowledge` exposed Connected sources,
  `Learning` exposed the workflow overlay, and the returned Today view exposed
  the text composer plus `Hold to talk`;
- screenshots, UI hierarchy and PID-filtered logcat were captured at
  `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.u6g1HV`
  (`today.png`, `work.png`, `knowledge.png`, `learning.png`,
  `today-assistant.png`, `today-return.png`, `window.xml` and `logcat.txt`);
- app-process logcat contained no `FATAL EXCEPTION`, `Dart Error` or
  `Unhandled exception` signature.
- the harness syntax-checks cleanly and reports the explicit unavailable-
  workspace state separately from a generic render timeout.

The first harness attempt was intentionally retained as a debugging signal:
it failed only because it expected a non-existent combined navigation marker.
After that assertion was corrected, a second run exposed a launch race in PID
collection; the harness now waits for the rendered UI before polling the app
process. A later Make attempt also exposed a temporary backend bootstrap
failure; the endpoint recovered and the complete smoke was rerun successfully
both directly and through Make.

Boundary:

- this proves local emulator startup and backend-backed shell rendering only;
  it does not prove a physical phone, microphone, real cross-network phone
  routing, provider usage, CI or production release readiness;
- no task status, commit, push, merge, deployment or production data was
  changed.

## 2026-08-30 — Self-contained S7 recovery and capacity gate refresh

Scope:

- verify the remaining repository-owned migration, legacy-upgrade, recovery and
  capacity behavior without depending on human/device acceptance or Tailscale;
- use isolated disposable PostgreSQL Compose projects and retain exact command
  exit codes as evidence;
- keep production/provider, physical-device and release-authorization gates
  explicitly out of this local-only result.

Verified commands and observed results:

- `rtk env COMPOSE_FILE=infra/docker-compose.ci.yml
  COMPOSE_PROJECT_NAME=devenglish-migrate-20260830 DEVENGLISH_CI_PORT=55434
  POSTGRES_SERVICE=postgres POSTGRES_USER=devenglish POSTGRES_DB=devenglish
  scripts/db_migrate_test.sh` — exit `0`; dry-run did not mutate the database,
  all `19` migrations applied, a second run reported them already applied, and
  the injected failing migration rolled back without being recorded;
- `rtk env COMPOSE_FILE=infra/docker-compose.ci.yml
  COMPOSE_PROJECT_NAME=devenglish-legacy-upgrade-20260830
  DEVENGLISH_CI_PORT=55438 POSTGRES_SERVICE=postgres POSTGRES_USER=devenglish
  POSTGRES_DB=devenglish DEVENGLISH_LEGACY_UPGRADE_DISPOSABLE=1
  scripts/db_legacy_upgrade_test.sh` — exit `0`; two synthetic legacy users
  and their `work_context`/`imported_sources` digests were preserved, the
  deterministic workspace backfill passed, phase-two migrations were
  idempotent, and no Knowledge rows were created implicitly;
- `rtk env COMPOSE_FILE=infra/docker-compose.ci.yml
  COMPOSE_PROJECT_NAME=devenglish-capacity-20260830 DEVENGLISH_CI_PORT=55439
  POSTGRES_SERVICE=postgres POSTGRES_USER=devenglish POSTGRES_DB=devenglish
  DEVENGLISH_CAPACITY_DISPOSABLE=1 scripts/knowledge_capacity_test.sh` — exit
  `0`; exactly `100000` chunks were inserted, analyzed, found through FTS and
  read through bounded last-page pagination;
- `rtk bash scripts/production_example_test.sh` — exit `0`; production-shaped
  configuration rejected an undersized auth secret before startup;
- `rtk node --test scripts/backend_coverage_gate_test.mjs` — exit `0`; all
  `4` raw-count, baseline-floor and fail-closed profile checks passed;
- each disposable Compose project was isolated by an explicit project name;
  the migration project was removed after the test and the legacy/capacity
  harnesses cleaned their own named projects.

Boundary:

- these are repository-owned local gates only; they do not replace operator
  U0/device acceptance, physical cross-network Tailscale testing, a current
  independent review, live provider/billing reconciliation, CI evidence on a
  delivery commit or human release authorization;
- no task status, commit, push, merge, deployment or production data was
  changed.

## 2026-08-29 — Android Emulator APK walkthrough

Scope:

- verify that the cross-network release APK starts on an Android virtual
  device;
- verify the canonical mobile shell and backend-backed state across all four
  destinations;
- keep physical-device and emulator evidence separate.

Verified commands and observed results:

- `rtk flutter emulators --launch Ledgerly_Pixel_8` followed by
  `rtk adb wait-for-device` — emulator booted as `emulator-5554`;
- `rtk adb -s emulator-5554 install -r
  /Users/ryantruong/Desktop/DevEnglish-tailscale.apk` — exit `0`, streamed
  install `Success`;
- `rtk adb -s emulator-5554 shell am start -n
  com.devenglish.devenglish/.MainActivity` — exit `0`; activity reported
  `RESUMED` and first window drawn;
- screenshot and accessibility dump — `Today` rendered with `Canonical
  workspace / Live`, `1 open task` and `1 source connected`; the four tab
  buttons were present;
- input navigation and accessibility dumps — `Today → Work → Knowledge →
  Learning` all rendered the expected screen identity and backend-backed
  content;
- app-process logcat — no `FATAL`, `AndroidRuntime` crash or Dart exception;
  the only app-process warning was the emulator's denied `max_map_count`
  inspection, not a rendering or request failure;
- `rtk adb -s emulator-5554 shell 'toybox nc -z -w 3
  100.126.52.73 8080'` — exit `0`; the emulator reached the backend TCP port.

Boundary:

- emulator UI/navigation and backend-backed rendering are verified locally;
- no physical Android device was attached (`adb devices` had no physical
  device), so microphone hardware, real Tailscale routing across networks,
  background behavior and physical-device performance remain open;
- no provider request, production deployment, commit, push, merge or release
  authorization was performed.

## 2026-08-29 — Cross-network Android packaging helper

Scope:

- support a phone on a different network through the Mac's Tailscale address;
- preserve explicit endpoint and same-Wi-Fi fallbacks;
- fail before APK generation when the selected backend cannot be reached.

Verified commands and observed results:

- `rtk bash -n scripts/android_local_apk.sh scripts/android_local_apk_test.sh`
  — exit `0`;
- `rtk bash scripts/android_local_apk_test.sh` — exit `0`; explicit
  `API_BASE_URL`, explicit `TAILSCALE_IP`, Wi-Fi fallback and invalid-scheme
  rejection all passed;
- `rtk curl -fsS --connect-timeout 3 http://<tailscale-ip>:8080/healthz` —
  exit `0`; the running Docker backend returned HTTP `200`;
- `rtk env TAILSCALE_IP=100.126.52.73 DEVENGLISH_APK_OUTPUT=/Users/ryantruong/Desktop/DevEnglish-tailscale.apk scripts/android_local_apk.sh`
  — exit `0`; Flutter release APK built successfully (`52.2 MB`) and was
  copied to `/Users/ryantruong/Desktop/DevEnglish-tailscale.apk`;
- `rtk git diff --check` — exit `0`.

The helper now writes the APK to `DEVENGLISH_APK_OUTPUT` when supplied and
otherwise keeps the historical Desktop output path. No credential, token or
secret is embedded or logged by the helper.

Boundary:

- this is local development connectivity evidence only; Tailscale must also
  be installed and connected on the phone;
- no public port exposure, production deployment or release authorization was
  performed.

## 2026-08-29 — Final machine-gate refresh after web boot hardening

Verified commands and observed results:

- `rtk bash .cyberos/cuo/gates/run-gates.sh` — exit `0`; build, lint, backend
  tests, content factory, Flutter tests, coverage and status generation passed;
  the CyberOS doctor step remained an environment skip because the local
  memory CLI is unavailable;
- the gate reported `52` Flutter tests passed with `2` environment skips and
  `GATES: GREEN (machine gates only)`;
- `rtk git diff --check`, task lint and the multi-agent protocol verifier — exit
  `0`.

Boundary:

- machine green is not a human lifecycle, U0, independent-review, CI,
  provider/billing or production-release verdict;
- no task status, commit, push, merge, deployment or release authorization was
  changed.

## 2026-08-29 — Web boot-state hardening

Scope:

- make the web artifact visibly loading before Flutter initializes;
- show a recoverable startup error instead of leaving the page blank after a
  prolonged engine/bootstrap failure;
- keep the boot layer outside the Flutter application and remove it once the
  Flutter host surface appears.

Verified commands and observed results:

- `rtk git diff --check` — exit `0`;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`; release
  artifact rebuilt successfully;
- fresh in-app browser reload at `http://localhost:8093/?boot=verification`
  — page title `DevEnglish`, Flutter UI rendered, `bootNodes=0`,
  `flutterPanes=1`, and no captured warning/error logs;
- `rtk flutter test -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true
  test/browser_smoke_test.dart --reporter compact` — exit `0`; canonical
  Today → Work → Knowledge → Learning navigation passed.

Boundary:

- the local browser was not used to claim real-device acceptance;
- no commit, push, merge, deployment or release authorization was performed.

## 2026-08-29 — Workspace resolution guard

Scope:

- fail closed when legacy workspace topology is missing, ambiguous, deleted or
  owned by another user;
- allow deterministic creation only for a genuinely fresh owner, serialize it
  with an owner-row lock and map unresolved topology to an explicit HTTP `503`;
- keep production backfill, workspace switching and migration policy outside
  this slice.

Verified commands and observed results:

- targeted default workspace/application/http tests — exit `0`; `292` tests
  passed across `2` packages;
- targeted PostgreSQL workspace/application/http tests — exit `0`; `301`
  tests passed across `2` packages;
- full default-profile Go normal/race runs — exit `0`; `985` tests passed
  across `23` packages for each run;
- full PostgreSQL-enabled Go normal/race runs — exit `0`; `1060` tests passed
  across `23` packages for each run;
- PostgreSQL-backed raw coverage — exit `0`; `9354/12085` statements
  (`77.40%`), with the seven V1 core package raw-count gate passing:
  Application `752/830`, Assistant `274/295`, Connectors `1133/1258`, HTTP
  API `1169/1297`, Knowledge `833/923`, MCP `1358/1495` and Work `1581/1756`;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`;
- production-shaped Docker smoke — exit `0`; migrations, auth, Work conflict
  and history, Knowledge, MCP replay/revoke, CORS and provider fail-closed
  checks passed;
- CyberOS machine gates, task lint and multi-agent protocol verifier — exit
  `0`; CyberOS doctor remains an environment skip because its memory CLI is
  unavailable.

Evidence: `docs/project/agent-runs/2026-08-29-workspace-resolution-guard.md`.

Boundary:

- this is local implementation evidence only; no production backfill or
  automatic legacy mapping was performed;
- no task lifecycle, commit, push, merge, deployment or release authorization
  was changed.

## 2026-08-29 — Continuation checkpoint refresh

- Google Drive OAuth context-cancellation hardening — exit `0`; the refresh
  source preserves in-memory token caching, uses the bounded 30-second HTTP
  client and stops the injected token exchange when the caller cancels. No
  live Google request was made. Evidence:
  `docs/project/agent-runs/2026-08-29-drive-oauth-context-cancellation.md`;
- Full default-profile Go normal/race runs — exit `0`; `981` tests passed
  across `23` packages for each run;
- Full PostgreSQL-enabled Go normal/race runs — exit `0`; `1049` tests passed
  across `23` packages for each run;
- PostgreSQL-backed raw coverage (`9308/12034`, `77.35%`) — exit `0`; the
  strict seven-package `>=90%` checker passed with the recorded R0 floor;

- Google Drive OAuth timeout hardening — exit `0`; the refresh token source
  uses the same bounded 30-second HTTP client as Drive API calls through
  `oauth2.HTTPClient`, and the injected token endpoint test passed without a
  live Google request. Evidence:
  `docs/project/agent-runs/2026-08-29-drive-oauth-timeout.md`;
- Google Drive OAuth refresh-source tests — exit `0`; static-token and
  read-only refresh configuration are covered with `httptest`/in-memory token
  sources, including redacted refresh failure. No live Google request was
  made; live consent, quota and rate-limit behavior remain open.
- Product-task audit backfill — exit `0` for task-lint and the seven
  reconciliation commands; `TASK-PRODUCT-002` through `TASK-PRODUCT-008` now
  pass R1/R2 with hash-bound manual spec audits. R3 remains absent and R4/R5
  retain the uncommitted/untracked evidence boundaries documented in
  `docs/project/agent-runs/2026-08-29-product-task-audit-backfill.md`;
- the runtime module-manifest boundary suite
  (`rtk go test ./backend/internal/platform ./backend/internal/httpapi
  ./backend/cmd/server -count=1`) — exit `0`; `254` tests passed across `3`
  packages, including disabled-module and delete-like-route fail-closed
  assertions;
- the module-aware application/bootstrap suite
  (`rtk go test ./backend/internal/application ./backend/internal/httpapi
  ./backend/internal/platform -count=1`) — exit `0`; `308` tests passed across
  `3` packages, including disabled-repository, stable-array, and bootstrap
  error-path assertions;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk curl -fsS http://localhost:8080/healthz` and
  `rtk curl -fsS http://localhost:8090/healthz` — both local services ready;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`.
- production-shaped Docker smoke from the current source — exit `0`; `19`
  migrations and the complete auth/Work/Knowledge/MCP/CORS/provider-failure
  sequence passed;
- runtime binary smoke with `DEVENGLISH_MODULES=platform,work` — exit `0`;
  Work projects remained available (`200`), while Knowledge search, Work
  trash and MCP were disabled (`404`); bootstrap remained available (`200`)
  with stable empty `sources` and `conversations` arrays.

Boundary:

- this refresh revalidates the current local checkout only; it does not close
  U0/operator acceptance, independent review, CI-on-delivery-commit or any
  Git/release authorization gate.

## 2026-08-29 — Task evidence reconciliation

Scope:

- backfill phase evidence and hash-bound manual spec audits for the Product UX
  lock and OPS orchestration-loop tasks;
- verify that the evidence ladder distinguishes spec integrity and phase
  artifacts from committed-object and release gates.

Verified commands and observed results:

- `rtk node .cyberos/docs-tools/task-lint.mjs docs/tasks/product
  docs/tasks/improvement` — exit `0`; only the documented informational
  `TRACE-001` messages remain;
- `rtk node .cyberos/docs-tools/task-reconcile.mjs
  TASK-PRODUCT-001-ux-lock --run-tests --json` — R1/R2 pass, R3 absent, R4
  route-back because declared delivery objects are not in `HEAD`;
- `rtk node .cyberos/docs-tools/task-reconcile.mjs
  TASK-OPS-001-agent-orchestration-loop-guard --run-tests --json` — R1/R2
  pass, R3 absent, R4 route-back, R5 refuses the untracked verifier citations;
- `rtk git diff --check` — exit `0`.

Boundary:

- the co-located audits are post-hoc spec audits because no executable
  `task-audit` runner is present in this checkout; they are not independent Sol
  verdicts or lifecycle acceptance;
- no task status, commit, push, merge or deployment was changed.

## 2026-08-29 — Authenticated canonical navigation regression guard

Scope:

- close the missing local regression assertion from the historical Wave 3
  review, where authenticated production could be reduced to a legacy Home
  screen;
- verify the current production fixture traverses all four canonical shell
  destinations.

Verified commands and observed results:

- `rtk flutter test --dart-define=DEVENGLISH_ENV=production
  test/production_shell_test.dart` — exit `0`; `2` tests passed, including
  Today → Work → Knowledge → Learning → Today with surface-specific content
  assertions;
- `rtk flutter test test/workspace_golden_test.dart
  test/browser_smoke_test.dart test/widget_test.dart` — exit `0`; `9` tests
  passed and `1` environment skip;
- `rtk dart format --output=none --set-exit-if-changed
  test/production_shell_test.dart` and `rtk git diff --check` — exit `0`.

Boundary:

- this is local regression evidence only; the same-thread Sol re-review,
  real-device U0, lifecycle acceptance and release authorization remain open;
- no task lifecycle, commit, push, merge or deployment was changed.

## 2026-08-29 — MCP client handoff documentation

Scope:

- document the product Streamable HTTP endpoint, separate from the CyberOS
  stdio MCP configuration;
- document least-privilege token issuance, Codex configuration, Claude
  transport fields, network boundaries and the operator acceptance checklist.

Verified against source:

- product endpoint is `/mcp` and token lifecycle is
  `POST /api/v2/mcp/tokens` plus authenticated revoke;
- stable V1 underscore tools and the five allow-listed scopes match
  `backend/internal/mcp/application.go` and `scope.go`;
- Codex example uses the documented `bearer_token_env_var` pattern;
  tracked CyberOS `.mcp.json`, `.codex/config.toml` and
  `.agents/mcp_config.json` were not changed.
- after the documentation update, `rtk go test ./backend/internal/httpapi
  ./backend/internal/mcp -count=1` exited `0` with `360` tests
  passed; `rtk bash scripts/verify_multi_agent_protocol.sh` exited
  `0` with protocol, entrypoint and manifest checks passing.

Boundary:

- this closes the documentation gap, not the operator acceptance gate;
- Codex/Claude must still be connected and tested from the actual client,
  including scoped discovery and revoke/401.

## 2026-08-29 — Production-shaped Docker smoke refresh

Scope:

- run the disposable PostgreSQL, backend and embedding stack from the current
  checkout after the action-boundary and acceptance-trace updates;
- exercise the authenticated Work, Knowledge, MCP, CORS and provider-failure
  paths end to end.

Observed result:

- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh` — exit `0`; migrations `19`, embedding
  `200`, login/session `200`, Project/Task/Decision create/update/conflict/
  history, MCP issue/initialize/revoke/revoked `201/200/204/401`, Knowledge
  import/search `201/200`, CORS `204/403`, provider fail-closed `500` and
  logout `204` all passed.

Boundary:

- this is disposable local runtime evidence, not production deployment,
  live-provider billing reconciliation or release authorization; the smoke
  stack was torn down by the harness.

## 2026-08-29 — Acceptance trace reconciliation

Scope:

- reconcile the eight product task specifications with the actual Go test
  functions and Flutter test titles;
- verify the referenced Work, Knowledge, Assistant, Learning, MCP and shell
  coverage without changing task lifecycle state.

Verified commands and observed results:

- `rtk go test ./backend/internal/work ./backend/internal/knowledge
  ./backend/internal/assistant ./backend/internal/learning
  ./backend/internal/learningoverlay ./backend/internal/integrations
  ./backend/internal/mcp -count=1` — exit `0`; `524` tests passed across
  seven packages;
- focused HTTP acceptance set — exit `0`; `8` tests passed, including the
  walking skeleton, grounded unknown response, learning isolation, action
  replay and configured Work-trash challenge boundary;
- `rtk flutter test test/production_shell_test.dart
  test/workspace_state_golden_test.dart test/workspace_ui_test.dart
  test/roleplay_test.dart --reporter compact` — exit `0`; all other tests
  passed with `1` environment skip in this focused selection;
- old/nonexistent acceptance references were removed from the product specs;
  the replacement links are recorded in
  `docs/project/agent-runs/2026-08-29-product-ac-completion-matrix.md`.

Boundary:

- this is traceability and focused regression evidence only; it does not
  check AC boxes, change CyberOS lifecycle, replace independent review or
  grant commit/push/merge/deploy authorization.

## 2026-08-29 — CyberOS machine gate rerun

Scope:

- rerun the repository machine gates after the Work action-boundary
  continuation and documentation refresh;
- verify build, lint, tests, coverage and generated status evidence from the
  same local working tree.

Verified commands and observed results:

- `rtk bash .cyberos/cuo/gates/run-gates.sh` — exit `0`; build, lint, backend
  tests, content-factory (`200` generated evaluation cases), Flutter tests
  (`51` passed and `2` environment skips), coverage and status-page generation
  all completed successfully;
- the gate reported `GATES: GREEN (machine gates only)`;
- the CyberOS doctor step was skipped because the local CyberOS memory CLI is
  unavailable;
- the gate retained the human-in-the-loop requirement; no lifecycle status was
  changed.
- a fresh in-app browser smoke at `http://localhost:8093/` loaded title
  `DevEnglish` and rendered the Today workspace shell; the screenshot was
  captured after the gate rerun.

Boundary:

- this is a local machine-gate result, not independent Sol review, real-device
  U0 acceptance, GitHub CI, provider/billing reconciliation or release
  authorization;
- the protected pre-existing `.github/workflows/ci.yml` WIP remains untouched;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — Work action-boundary continuation

Scope:

- bind configured REST legacy Project/Task/Decision trash routes to the
  canonical `work.entity.trash` challenge and receipt service;
- verify the full MCP application path uses a Work receipt rather than the
  low-level compatibility mutation path;
- keep the unconfigured REST/MCP fixtures available only for compatibility and
  boundary tests.

Verified commands and observed results:

- `rtk go test ./backend/internal/httpapi` — exit `0`; `228` tests passed,
  including challenge-required legacy trash for all three entity types;
- `rtk go test ./backend/internal/mcp` — exit `0`; `132` tests passed,
  including production-path `entity_trash` with an accepted Work receipt;
- `rtk git diff --check` — exit `0`.

Boundary:

- this continuation does not authorize commit, push, merge or deployment;
- permanent purge remains a separate lifecycle operation and is not silently
  treated as a trash confirmation.

## 2026-08-29 — Follow-up release web smoke

Scope:

- reload the freshly rebuilt release web artifact after the acceptance-audit
  documentation and migration-runner changes;
- verify that the canonical Today surface is rendered instead of a blank page.

Observed result:

- `http://localhost:8093/` loaded with document title `DevEnglish`;
- the rendered surface showed the canonical workspace shell, Today summary,
  next safe action, assistant composer and source-backed evidence;
- captured browser logs contained `0` warning/error entries.

Boundary:

- this is a fresh web rendering smoke only; Flutter Chrome navigation,
  responsive goldens and APK builds are recorded separately;
- it does not constitute real-device, operator U0, production or release
  acceptance.

## 2026-08-29 — Migration dry-run and rollback harness

Scope:

- add a non-mutating migration preview for the production-shaped runner;
- prove that previewing a fresh database does not create
  `schema_migrations`;
- retain coverage for ordered application, idempotent re-application and
  rollback when a migration fails.

Verified commands and observed results:

- `rtk bash -n scripts/db_migrate.sh` — exit `0`;
- `rtk bash -n scripts/db_migrate_test.sh` — exit `0`;
- invalid `DRY_RUN` input — exit `2` with the expected validation message,
  before Docker/database access;
- disposable PostgreSQL `scripts/db_migrate_test.sh` harness — exit `0`;
  `DRY_RUN=YES` reported all `19` migrations as pending and
  `schema_migrations` remained absent; the real run applied `000..018`, the
  second run reported every migration as already applied, and the injected
  failing migration rolled back successfully;
- `rtk git diff --check` — exit `0`.

Boundary:

- the dry-run proves the local migration workflow only; it does not apply a
  production migration or grant release authorization;
- the disposable Docker volume was removed after the test; no production
  database was changed.

## 2026-08-29 — Strict coverage gate and final local verification

Scope:

- add a deterministic raw-count gate for the seven V1 backend core packages;
- close the Work package's rounded-but-below-90% boundary with a default-clock
  service test;
- rerun the PostgreSQL-enabled backend suite and race suite before updating
  current acceptance evidence.

Verified commands and observed results:

- `rtk env DEVENGLISH_TEST_DATABASE_URL=<disposable-local-DSN> rtk go test
  ./... -count=1` — exit `0`; `1024` tests passed across `23` packages;
- `rtk env DEVENGLISH_TEST_DATABASE_URL=<disposable-local-DSN> rtk go test
  -race ./... -count=1` — exit `0`; `1024` tests passed across `23` packages;
- the persistent local development database was at migration `017`; running
  the append-only migration runner applied
  `018_assistant_conversation_context.sql`, after which the two PostgreSQL
  conversation persistence tests passed and the full profile was rerun;
- PostgreSQL-enabled coverage profile — exit `0`; `77.45%` total statement
  coverage (`9107/11759`); Application `90.56%`, Assistant `92.88%`,
  Connectors `90.10%`, HTTP API `90.32%`, Knowledge `90.25%`, MCP `91.16%`
  and Work `90.03%`;
- `rtk node scripts/backend_coverage_gate.mjs --profile
  /tmp/devenglish-go-db-20260829-final.cov --min 90` — exit `0`; all seven
  required packages passed using raw integer statement counts;
- `rtk node --check scripts/backend_coverage_gate.mjs` — exit `0`;
- `rtk node --test scripts/backend_coverage_gate_test.mjs` — exit `0`; raw
  threshold, baseline-floor, malformed-profile and malformed-manifest cases
  passed;
- focused Work profile — exit `0`; `221` tests and raw Work coverage
  `1581/1756` (`90.03%`);
- the package gate is wired into `.github/workflows/postgres-integrations.yml`
  and uploads the CI profile artifact; GitHub CI has not been run from this
  local checkout.
- R0 baseline rerun in detached worktree from
  `104304f12f411dd534aa4b385e964b3b8c11ae43`: PostgreSQL-enabled Go suite
  exit `0`, `395` tests across `16` packages, raw coverage
  `4269/6393 = 66.78%`; evidence is in
  `docs/project/agent-runs/2026-08-29-r0-baseline.md`.

Boundary:

- the current overall profile is above the recorded R0 floor (`77.45%` vs
  `66.78%`); this does not grant release approval;
- U0 operator/device acceptance, lifecycle/HITL, independent current review,
  provider/billing acceptance and human release authorization remain open;
- the protected pre-existing `.github/workflows/ci.yml` WIP finding remains
  untouched;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — Local acceptance gate refresh

Scope:

- re-run the full backend and frontend gates after the MCP/application boundary
  additions;
- verify the rebuilt release web artifact in the in-app browser, including the
  canonical navigation and Knowledge search path;
- replace stale current metrics with the latest observed coverage profile.

Verified commands and observed results:

- `rtk go test ./... -count=1` — exit `0`; `837` tests passed across `23`
  packages;
- `rtk go test -race ./... -count=1` — exit `0`; `837` tests passed across
  `23` packages;
- PostgreSQL-enabled coverage profile — exit `0`; `75.1%` total statement
  coverage (`8832/11759`); Application `90.6%`, Assistant `92.9%`, Knowledge
  `90.2%`, MCP `91.2%`, Work `85.5%`, Connectors `80.9%` and HTTP API `83.8%`;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk flutter test --reporter compact` — exit `0`; `49` passed and `2`
  environment skips;
- Flutter Chrome navigation smoke — exit `0`; Today → Work → Knowledge →
  Learning passed (the integration-test plugin advisory is non-failing);
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`; `build/web`
  rebuilt;
- `rtk flutter build apk --debug` — exit `0`; Android debug APK generated at
  `build/app/outputs/flutter-apk/app-debug.apk`;
- fresh in-app browser at `http://localhost:8093/` — title `DevEnglish`,
  Today rendered at 2560x1440, Work/Knowledge/Learning navigation succeeded,
  Knowledge search `canonical` returned two results, and captured tab logs had
  no error/warning entries.
- the same in-app browser session sent `What is the current next safe action?`
  through the canonical text assistant; the transcript stayed visible during
  source checking and the response returned with evidence plus an explicit
  unknown boundary after about `16.4s`. Backend request logging recorded the
  corresponding message at `16448ms`; no browser error/warning was captured.
- production-shaped Docker smoke — exit `0`; migrations `19`, embedding `200`,
  Work create/update/conflict/history `201/200/409/200`, MCP
  issue/initialize/revoke/revoked `201/200/204/401`, Knowledge import/search
  `201/200`, CORS `204/403`, provider fail-closed `500`, logout `204`;
- disposable legacy-upgrade harness — exit `0`; migrations `000..018`,
  idempotency, legacy snapshot preservation and workspace isolation passed;
- disposable Knowledge capacity gate — exit `0`; `100000` chunks, FTS lookup
  and bounded last-page pagination passed;
- official MCP Go SDK conformance — exit `0`; Streamable HTTP handshake,
  discovery, resource read and bearer-authenticated tool call passed;
- multi-agent protocol verifier — exit `0`; protocol, entrypoints and
  manifests passed; CyberOS-only verifier retains the protected pre-existing
  `.github/workflows/ci.yml` WIP finding and exits `1` by design.

Boundary:

- strict `90%` touched/new package coverage remains open for HTTP API, Work and
  connectors; total coverage is tracked against the R0 baseline. Real-device/
  operator U0, lifecycle/HITL, independent review and release authorization
  also remain open;
- the disposable profile is `/tmp/devenglish-db-continuation-coverage-3.cov`;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — Persistence and service boundary test refresh

Scope:

- cover PostgreSQL MCP token persistence invalid replay metadata, conflicting
  idempotency, expiry/revoke behavior and corrupt scope rows;
- cover Knowledge service capability/error/identifier boundaries and
  Application durable-pool/error mapping boundaries;
- refresh full, race and PostgreSQL-enabled evidence after the additions.

Verified commands and observed results:

- focused MCP persistence suite — exit `0`; `7` tests passed;
- focused Knowledge suite — exit `0`; `135` tests passed;
- focused Application suite — exit `0`; `28` tests passed;
- `rtk go test ./... -count=1` — exit `0`; `749` tests passed across `23`
  packages;
- `rtk go test -race ./... -count=1` — exit `0`; `749` tests passed across
  `23` packages;
- PostgreSQL-enabled coverage run — exit `0`; `770` tests passed; `73.4%`
  total statement coverage (`8635/11759`); HTTP API `83.8%`, Application
  `84.3%`, Knowledge `84.1%`, MCP `85.0%`;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`;
- `rtk bash scripts/verify_multi_agent_protocol.sh` — exit `0`; protocol,
  entrypoints and manifests passed.
- `rtk flutter analyze` and `rtk flutter test --reporter compact` — exit `0`;
  `49` tests passed and `2` environment skips;
- Flutter Chrome navigation smoke — exit `0`; Today → Work → Knowledge →
  Learning passed;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`; `build/web`
  rebuilt;
- production-shaped Docker smoke — exit `0`; `19` migrations and the complete
  Work/Knowledge/MCP/auth/CORS/provider-fail-closed/history flow passed.
- disposable legacy-upgrade harness — exit `0`; migrations `000..018`,
  idempotency, legacy snapshot preservation and workspace isolation passed;
- disposable Knowledge capacity gate — exit `0`; `100000` chunks, FTS lookup
  and bounded last-page pagination passed.

Boundary:

- strict `90%` coverage, U0/device, lifecycle/HITL, independent review and
  release authorization remain open;
- CyberOS-only verifier still reports the protected pre-existing
  `.github/workflows/ci.yml` WIP snapshot; it was preserved;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — HTTP action boundary test refresh

Scope:

- cover action target projection, idempotency-key validation and action/MCP
  error mapping;
- verify missing action/MCP services, malformed bodies, missing confirmation
  keys and empty token scopes fail at the HTTP boundary;
- refresh the full and PostgreSQL-enabled coverage evidence after the new
  boundary cases.

Verified commands and observed results:

- `rtk go test ./backend/internal/httpapi -count=1` — exit `0`; `159` tests
  passed and the package measured `83.8%` coverage;
- `rtk go test ./... -count=1` and `rtk go test -race ./... -count=1` — exit
  `0`; `719` passed across `23` packages for each run;
- PostgreSQL-enabled coverage run — exit `0`; `733` passed; `73.1%` total;
  HTTP API `83.8%`;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`.

Boundary:

- strict `90%` coverage, U0/device, lifecycle/HITL, independent review and
  release authorization remain open;
- CyberOS-only verifier still reports the protected pre-existing
  `.github/workflows/ci.yml` WIP snapshot; it was preserved;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — V2 workspace-scope failure matrix

Scope:

- exercise every V2 route family with a workspace initializer that fails;
- prove HTTP adapters fail closed and never fall back to an implicit scope.

Verified commands and observed results:

- `rtk go test ./backend/internal/httpapi -run TestV2RoutesFailClosedWhenWorkspaceInitializationFails -count=1` — exit `0`; `42` route cases passed;
- `rtk go test ./backend/internal/httpapi -count=1` — exit `0`; `131` tests passed and the package measured `82.5%` coverage;
- `rtk go test ./... -count=1` and `rtk go test -race ./... -count=1` — exit `0`; `691` passed across `23` packages for each run;
- PostgreSQL-enabled coverage run — exit `0`; `705` passed; `73.0%` total; HTTP API `82.5%`;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit `0`.

Boundary:

- strict `90%` coverage, U0/device, lifecycle/HITL, independent review and release authorization remain open;
- CyberOS-only verifier still reports the protected pre-existing `.github/workflows/ci.yml` WIP snapshot; it was preserved;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — Acceptance refresh and Learning browser-smoke repair

Scope:

- reproduce the canonical Flutter Chrome smoke after the local web rebuild;
- repair the Learning screen contract exposed by the smoke when legacy metrics
  are unavailable;
- refresh responsive goldens and verify the rebuilt browser artifact;
- reconcile the latest backend, connector, Flutter and runtime evidence.

Observed failure and fix:

- the first Chrome smoke rendered the app but failed on the Learning invariant
  because the screen replaced the canonical data-separation sentence with a
  legacy-metrics fallback;
- `lib/src/screens/workspace_learning.dart` now always renders
  `Learning observations stay separate from canonical work data.` and shows
  legacy-metrics availability as a separate status line;
- the affected 390x844, 430x932 and 1440x900 Learning goldens were regenerated
  from the corrected widget output.

Verified commands and observed results:

- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk flutter test --reporter compact` — exit `0`; `49` tests passed and `2`
  environment skips;
- `rtk flutter test -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true
  test/browser_smoke_test.dart --reporter compact` — exit `0`; Today → Work →
  Knowledge → Learning navigation passed;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`; `build/web`
  rebuilt cleanly;
- fresh in-app browser visual check — Today and Learning rendered from the
  rebuilt artifact; console warnings/errors were empty;
- `rtk go test ./... -count=1` and `rtk go test -race ./... -count=1` — exit
  `0`; `649` tests passed across `23` packages for each run;
- PostgreSQL-enabled coverage run — exit `0`; `663` tests passed, `72.4%`
  total coverage, connectors `80.9%`, HTTP API `77.1%`;
- `rtk bash scripts/android_local_apk.sh` — exit `0`; release APK generated;
- production-shaped Docker smoke, legacy-upgrade and 100,000-chunk capacity
  harnesses — exit `0` in their disposable environments.

Boundary:

- the strict `90%` coverage target, real-device/operator U0, lifecycle
  acceptance, independent review and human release authorization remain open;
- the CyberOS-only verifier still reports the protected pre-existing
  `.github/workflows/ci.yml` WIP snapshot finding; it was preserved;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — HTTP error boundary and coverage refresh

Scope:

- verify v2 method and `Allow` header contracts at the HTTP boundary;
- verify product/connector error status mapping and generic credential
  redaction;
- verify workspace/user isolation, query flag normalization and connector
  validation before provider I/O;
- extend assistant/application/MCP boundary coverage without changing the
  canonical data or action semantics;
- rebuild and rerun the production-shaped Docker smoke after the HTTP
  redaction hardening.

Verified commands and observed results:

- `rtk go test ./backend/internal/httpapi -count=1` — exit `0`; `57` tests
  passed, including isolation, query normalization, pre-provider validation,
  method/error mapping and secret-redaction assertions;
- `rtk go test ./... -count=1` — exit `0`; `606` tests passed across `23`
  packages;
- `rtk go test -race ./... -count=1` — exit `0`; `606` tests passed across
  `23` packages;
- `rtk go vet ./...` and `rtk go build ./...` — exit `0`;
- `rtk git diff --check` — exit `0`;
- DB-enabled coverage run — exit `0`; `620` tests passed and the profile at
  `/tmp/devenglish-db-enabled-20260829-final.cov` measured `70.8%` total
  statement coverage. Package results: application `83.4%`, assistant `92.9%`,
  knowledge `82.6%`, MCP `84.1%`, work `85.5%`, connectors `78.2%` and HTTP
  API `64.1%`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh` — exit `0`; rebuilt Docker images,
  `19` migrations, embedding `200`, Work create/update/conflict/history,
  MCP issue/initialize/revoke/revoked, Knowledge import/search,
  allowed/forbidden CORS, provider fail-closed and logout all passed.
- `bash scripts/verify_multi_agent_protocol.sh` — exit `0`; protocol and run
  manifest invariants passed;
- `bash scripts/verify_cyberos_only.sh` — exit `1` at the protected WIP
  snapshot check for `.github/workflows/ci.yml`; this is the pre-existing WIP
  finding recorded by the CyberOS boundary, and the file was preserved rather
  than reverted.

Boundary:

- generic HTTP errors now redact credential-shaped values before serialization;
- the strict `90%` coverage gate remains open; real-device/operator U0,
  lifecycle acceptance, independent review and human release authorization
  remain open;
- no commit, push, merge or deployment was performed.

Latest Flutter recovery verification:

- `rtk flutter test --reporter compact` — exit `0`; `49` tests passed and `2`
  environment-skipped after replacing the path-level local Dart AOT runtime
  executable with a byte-identical, recoverable copy.

## 2026-08-29 — Usage accounting hardening

Scope:

- preserve provider-reported DeepSeek usage when available;
- prevent preflight reservations and guessed STT/TTS/pronunciation values from
  being written as final usage;
- surface unavailable provider usage in the usage summary and Settings;
- rerun backend, static, build and production-shaped smoke gates.

Verified commands and observed results:

- `rtk go test ./backend/internal/learning ./backend/internal/httpapi
  ./backend/internal/application -count=1` — exit `0`; `76` tests passed;
- `rtk go test ./... -count=1` — exit `0`; `550` tests passed across `23`
  packages;
- `rtk go test -race ./... -count=1` — exit `0`; `550` tests passed across
  `23` packages;
- `rtk go vet ./...` and `rtk go build ./...` — exit `0`;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk git diff --check` — exit `0`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh` — exit `0`; `19` migrations and the
  complete Work/MCP/Knowledge/auth/CORS/provider-fail-closed flow passed.

Boundary:

- provider-reported tokens are preserved for DeepSeek; speech usage remains
  explicitly unavailable when the provider interface has no billable usage
  payload, and Settings reports the count rather than treating it as zero
  spend;
- reservations are preflight estimates only and are not final usage records;
- the `68.6%` DB-enabled coverage profile, Flutter full-suite compiler
  startup, real-device/operator U0, lifecycle acceptance, independent review
  and human release authorization remain open;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — Conversation history, MCP SDK conformance and mobile visual smoke

Scope:

- expose lightweight assistant conversation history and lazy transcript
  reopening through REST and Flutter;
- validate the Streamable HTTP adapter with the official MCP Go SDK;
- verify the rebuilt canonical Flutter shell visually across Today, Work,
  Knowledge and Learning at a mobile viewport;
- refresh production-shaped runtime evidence after the latest backend changes.

Verified commands and observed results:

- `rtk go test ./backend/internal/mcp -count=1` — exit `0`; `62` tests
  passed, including the official SDK conformance test;
- `rtk go test ./backend/internal/httpapi ./backend/internal/application
  -count=1` — exit `0`; `62` tests passed across 2 packages;
- `rtk go test ./... -count=1` — exit `0`; `546` tests passed across 23
  packages;
- `rtk go test -race ./... -count=1` — exit `0`; `546` tests passed;
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`;
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh` — exit `0`; `migrations=19`,
  embedding/login/session `200`, Work create/update/conflict/history
  `201/200/409/200`, MCP issue/initialize/revoke/revoked `201/200/204/401`,
  Knowledge import/search `201/200`, CORS `204/403`, provider fail-closed
  `500` and logout `204`;
- `rtk flutter analyze` — exit `0`; no issues found;
- `rtk flutter build web --release --no-wasm-dry-run
  --dart-define=DEVENGLISH_ENV=production
  --dart-define=DEVENGLISH_WORKSPACE_SHELL=new
  --dart-define=API_BASE_URL=http://127.0.0.1:8080` — exit `0`; release
  artifact built in `18.5s`;
- fresh in-app-browser visual smoke against the rebuilt artifact served on
  `http://127.0.0.1:8095/` — Today, Work, Knowledge and Learning all rendered
  at `390×844`; browser log inspection returned no warnings/errors.

Boundary:

- Flutter full/isolated tests remain inconclusive because the frontend
  compiler stayed in its loading phase and was stopped with exit `130`;
- the DB-enabled statement coverage profile is `68.6%`, below the plan's
  `90%` threshold;
- real-device/operator U0, operator-side Codex/Claude configuration, lifecycle
  acceptance and human release authorization remain open;
- no commit, push, merge or deployment was performed.

## 2026-08-29 — Legacy upgrade harness aligned with migration 018

Scope:

- include `018_assistant_conversation_context.sql` in the disposable legacy
  upgrade chain and idempotency checks;
- assert that the assistant conversation context columns, constraint and
  workspace-scoped index exist after upgrading a legacy-shaped database;
- keep the synthetic legacy snapshot and workspace backfill assertions intact.

Verified commands and observed results:

- `bash -n scripts/db_legacy_upgrade_test.sh` — exit `0`;
- disposable Compose project `devenglish-legacy-upgrade-local-20260829-r2`
  with `infra/docker-compose.ci.yml` — exit `0`; migrations `000..018`
  applied, second run reported every migration already applied, legacy
  `imported_sources`/`work_context` snapshots were unchanged, workspace
  backfill/isolation checks passed, and assistant context schema checks passed;
- `rtk git diff --check` — exit `0`.

Boundary:

- this is disposable local migration evidence; it does not authorize a
  production migration or release;
- strict `90%` coverage, U0/device acceptance, lifecycle acceptance and human
  release authorization remain open. No commit, push, merge or deployment was
  performed.

## 2026-08-29 — MCP Streamable HTTP negotiation hardening

Scope:

- enforce the required JSON/SSE `Accept` negotiation for MCP POST requests;
- validate optional `MCP-Protocol-Version` headers with backwards-compatible
  omission handling;
- return `202 Accepted` without a body for notifications;
- keep the no-unsolicited-SSE GET boundary explicit and update runtime smoke
  callers.

Verified commands and observed results:

- `rtk go test ./backend/internal/mcp -count=1` — exit `0`; `61` tests passed.
- `rtk go test ./backend/internal/httpapi ./backend/internal/mcp -count=1` —
  exit `0`; `100` tests passed across 2 packages.
- `rtk go test ./... -count=1` — exit `0`; `544` tests passed across 23
  packages.
- `rtk go test -race ./... -count=1` — exit `0`; `544` tests passed across 23
  packages.
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check` — exit
  `0`.
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES bash
  scripts/production_runtime_smoke.sh` — exit `0`; `19` migrations,
  embedding/login/session `200`, Work create/update/conflict/history
  `201/200/409/200`, MCP issue/initialize/revoke/revoked `201/200/204/401`,
  Knowledge import/search `201/200`, allowed/forbidden CORS `204/403`,
  provider fail-closed `500` and logout `204`.

Boundary:

- the server does not publish unsolicited SSE, so GET `/mcp` remains
  `405 Allow: POST`; independent MCP client conformance remains open;
- the latest full Flutter compiler rerun remains inconclusive at exit `130`;
  no Flutter source changed in this slice;
- strict `90%` coverage, U0/device acceptance, lifecycle acceptance and human
  release authorization remain open. No commit, push, merge or deployment was
  performed.

## 2026-08-29 — Grounded assistant context continuation

Scope:

- pin an assistant conversation to one canonical Project, Task, Decision or
  Knowledge Source reference;
- resolve and validate that reference inside the authenticated workspace before
  conversation creation;
- persist the reference through PostgreSQL migration `018`, expose it through
  v2 HTTP and Flutter Work/Knowledge assistant actions, and keep final evidence
  bounded to 20 items;
- make the pinned context visible in the assistant UI and distinguish it from
  lexical, hybrid and degraded retrieval.

Verified commands and observed results:

- `rtk go test ./... -count=1` — exit `0`; `543` tests passed across 23
  packages.
- `rtk go test -race ./... -count=1` — exit `0`; `543` tests passed.
- `rtk go vet ./...`, `rtk go build ./...` and Dart format — exit `0`.
- `rtk flutter analyze` — exit `0`; no issues found.
- `rtk flutter test` full and isolated reruns stayed at the frontend compiler
  loading phase and were stopped with exit `130`; no full-suite pass is claimed.
- `rtk flutter build web --release` — exit `0`; `build/web` generated.
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES ./scripts/production_runtime_smoke.sh`
  — exit `0`; `migrations=19`, embedding/login/session `200`, Work
  create/update/conflict/history `201/200/409/200`, MCP
  issue/initialize/revoke/revoked `201/200/204/401`, Knowledge import/search
  `201/200`, allowed/forbidden CORS `204/403`, provider fail-closed `500` and
  logout `204`.
- `rtk flutter build apk --debug` — debug APK artifact was generated at
  `build/app/outputs/flutter-apk/app-debug.apk`; the latest parallel invocation
  did not yield a captured exit code.
- DB-enabled coverage profile at
  `/tmp/devenglish-db-enabled-20260829-refresh-5.cov` — exit `0`; total `68.6%`
  statement coverage; application `82.0%`, assistant `84.1%`, Knowledge
  `82.1%`, MCP `82.0%`, Work `84.5%`, connectors `72.2%` and HTTP API `62.8%`.
- disposable migration chain — exit `0`; migration `018_assistant_conversation_context.sql`
  applied successfully.
- fresh browser visual smoke at `http://localhost:8094/` — title `DevEnglish`
  and non-empty Today surface rendered from the rebuilt `build/web` artifact;
  Flutter exposes only its accessibility placeholder to the DOM snapshot, so
  this remains visual smoke evidence rather than a DOM-semantic assertion.

Boundary:

- the strict `90%` coverage gate, real-device/operator U0 acceptance,
  independent Sol Ultra re-review of this latest source change, lifecycle
  acceptance and human Git/release authorization remain open;
- existing WIP was preserved; no commit, push, merge or deployment was done.

## 2026-08-29 — Work detail/history slice and refreshed gates

Scope:

- expose canonical Project/Task/Decision detail views from the Work action
  menus;
- load immutable activity history from the corresponding v2 history routes;
- keep raw before/after audit snapshots out of the presentation layer.

Verified commands and observed results:

- `rtk flutter test --reporter compact` — exit `0`; `49` passed, `2` skipped.
- `rtk flutter analyze` — exit `0`; no issues found.
- `rtk flutter build web --release` — exit `0`; `build/web` generated.
- `rtk flutter build apk --debug` — exit `0`; debug APK generated.
- `rtk go test ./...` — exit `0`; `527` tests passed across 23 packages.
- `rtk go test -race ./...` — exit `0`; `527` tests passed.
- DB-enabled coverage refresh — exit `0`; `541` tests passed and `68.4%`
  total statement coverage. Package results remain application `81.3%`,
  knowledge `82.1%`, MCP `82.0%`, work `84.5%`, connectors `72.2%` and HTTP
  API `62.7%`.
- production runtime smoke — exit `0`; Project/Task/Decision history routes
  each returned `200` after their versioned update flow, alongside the
  existing migration, auth, MCP, connector, CORS and provider-fail-closed
  checks.

Release boundary:

- The plan's `90%` touched/new-package coverage threshold remains open.
- Real-device/operator U0 acceptance, lifecycle acceptance, independent
  reviewer verdict and human release authorization remain open.

## 2026-08-29 — DB-enabled coverage refresh and HTTP lifecycle boundary

Scope:

- refresh coverage with the configured disposable PostgreSQL fixture after the
  latest HTTP purge-boundary test;
- keep task/decision purge behavior explicit when active children exist or the
  retention window has not elapsed.

Verified commands and observed results:

- `rtk go test ./...` — exit `0`; `527` tests passed across 23 packages.
- `rtk go test -race ./...` — exit `0`; `527` tests passed.
- `rtk go vet ./...` and `rtk go build ./...` — exit `0`.
- DB-enabled `rtk go test -coverprofile=/tmp/devenglish-db-enabled-20260829-refresh.cov ./...` — exit `0`; `541` tests passed, `68.4%` total statement coverage.
- Package results: application `81.3%`, knowledge `82.1%`, MCP `82.0%`, work
  `84.5%`, connectors `72.2%` and HTTP API `62.7%`.
- Focused HTTP lifecycle test — exit `0`; active-child purge returns conflict
  and early purge returns the retention error without deleting data.

Release boundary:

- The plan's `90%` touched/new-package coverage threshold remains open.
- The profile uses a disposable database and is evidence for the commands
  above; it does not establish release approval.

## 2026-08-29 — Wave 3 text-first voice composer

Scope:

- expose push-to-talk in the canonical Today/Work/Knowledge assistant;
- transcribe audio into the existing editable assistant composer;
- keep TTS opt-in through a read-aloud control on assistant turns;
- keep the recorder lazy so rendering the workspace does not require a native
  microphone plugin in widget tests.

Verified commands and observed results:

- `rtk flutter analyze` — exit `0`; no issues found.
- `rtk flutter test` — exit `0`; `43` passed, `2` skipped.
- `rtk flutter test test/workspace_ui_test.dart` — exit `0`; `15` passed,
  including the voice-composer presence assertion.
- `rtk flutter build web --release` — exit `0`; `build/web` generated.
- `rtk flutter build apk --debug` — exit `0`; debug APK generated.
- Fresh browser visual smoke at `http://localhost:8094/` — exit `0`; Today
  rendered the `Text + voice assistant` surface and the `Hold to talk`
  control, while existing evidence/citation cards remained visible.

Boundary:

- microphone permission and live STT/TTS provider behavior still require an
  operator/device walkthrough; no live provider call is made by this test.

## 2026-08-29 — Wave 3 canonical feature boundary slice

Scope:

- move canonical Workspace composition behind a feature-owned library;
- make the app shell import public Today/Work/Knowledge/Learning boundaries;
- retain the old workspace screen only as a compatibility facade while its
  internal part-file implementation is migrated incrementally.

Verified commands and observed results:

- `rtk flutter analyze` — exit `0`; no issues found.
- `rtk flutter test --reporter compact` — exit `0`; `43` passed, `2` skipped.
- `rtk flutter build web --release` — exit `0`; `build/web` generated.
- Fresh browser visual smoke at `http://localhost:8094/` — rendered Today,
  then Work after a coordinate navigation click; no app warning/error logs
  were observed. The browser semantic tree exposed only Flutter's accessibility
  placeholder, so this is visual smoke evidence rather than a DOM-semantic
  assertion.

This slice changes no intended runtime behavior and does not establish the
real-device U0 acceptance gate.

## 2026-08-29 — Product reset continuation and MCP identity boundary

Scope:

- MCP full-application write identity binding for Project, Task, Decision and
  manual Knowledge import;
- v2 HTTP boundary validation, lifecycle error mapping and canonical Work
  conflict behavior;
- Knowledge memory foreign-reference validation and soft-delete parity;
- full local Go/Flutter gates and production-shaped disposable runtime smoke.

Verified commands and observed results:

- go test ./... — exit 0; 494 tests passed across 23 packages.
- go test -race ./... — exit 0; 494 tests passed.
- go vet ./... and go build ./... — exit 0.
- Flutter analyze — exit 0; no issues found.
- Flutter test — exit 0; 43 passed, 2 skipped.
- Flutter web release build — exit 0; build/web generated.
- production runtime smoke — exit 0; 18 migrations, embedding 200,
  login/session 200, Project/Task/Decision create-update-conflict
  201/200/409, MCP issue/initialize/revoke/revoked 201/200/204/401,
  Knowledge import/search 201/200, CORS 204/403,
  provider fail-closed 500 and logout 204.
- focused MCP identity tests — exit 0; full-application writes use the
  authenticated token principal when request context has no caller identity.
- full-backend coverage profile — exit 0; the earlier profile measured 68.3%
  statement coverage overall, with application 81.3%, Knowledge 82.1%, MCP
  78.3% and Work 84.5%. The current DB-enabled refresh is recorded above.

Release boundary:

- The 90% plan coverage threshold remains open.
- The latest bounded Sol Ultra review attempt terminated at the usage limit
  before a verdict; it is not treated as approval or rejection.
- U0 operator/device acceptance, lifecycle acceptance and human Git/release
  authorization remain open.
- Existing WIP was preserved; no commit, push, merge or deployment was done.

## 2026-08-28 — Product reset follow-up and S5–S7 evidence

Scope:

- Learning/voice overlay, canonical Workspace shell and assistant state
  surfaces;
- connector sync status with cursor/no-change/read-only evidence;
- responsive and state goldens, production-shaped Docker smoke and final local
  build gates;
- manual-source retry idempotency, workspace-scoped receipts and Drive trash
  restore behavior.

Verified commands and observed results:

- `rtk flutter test --reporter compact` — exit `0`; `43` passed, `2` skipped.
- `rtk flutter analyze` — exit `0`.
- `rtk flutter test test/workspace_golden_test.dart
  test/workspace_state_golden_test.dart --reporter compact` — exit `0`; the
  surface and runtime-state golden suites passed.
- `rtk flutter test test/browser_smoke_test.dart -d chrome
  --dart-define=DEVENGLISH_BROWSER_SMOKE=true
  --dart-define=INTEGRATION_TEST_SHOULD_REPORT_RESULTS_TO_NATIVE=false
  --dart-define=DEVENGLISH_ENV=development
  --dart-define=API_BASE_URL=http://localhost:8080` — exit `0`; browser smoke
  passed against the local backend.
- `rtk flutter build web --release
  --dart-define=DEVENGLISH_ENV=development
  --dart-define=API_BASE_URL=http://localhost:8080` — exit `0`.
- `rtk flutter build apk --debug` — exit `0`.
- `rtk go test ./...` — exit `0`; `438` tests passed across `23` packages.
- `rtk go test -race ./...` — exit `0`; `438` tests passed.
- `rtk go vet ./...` — exit `0`; no issues.
- `rtk gofmt -l backend` — exit `0`; no files listed.
- `rtk git diff --check` — exit `0`.
- `rtk env DEVENGLISH_PRODUCTION_SMOKE=YES
  ./scripts/production_runtime_smoke.sh` — exit `0`; migrations `17`,
  embedding `200`, login `200`, session `200`, MCP issue/initialize/revoke/
  revoked `201/200/204/401`, knowledge import/search `201/200`, allowed CORS
  `204`, forbidden CORS `403`, provider fail-closed `500`, logout `204`.
- `DEVENGLISH_TEST_DATABASE_URL=... rtk go test
  ./backend/internal/application -run TestPostgresApplicationManualImportIsIdempotent
  -count=1` — exit `0`; repeated manual import preserved one immutable
  revision/chunk and the source-item version.
- Fresh release artifact browser smoke at `http://localhost:8094/` — rendered
  canonical Today; the Flutter view was non-empty and no browser error/warning
  entries were observed.
- Disposable legacy-upgrade and knowledge-capacity harnesses passed with
  isolated projects and were cleaned up; existing user containers were
  preserved.

Coverage boundary and release status:

- The current all-backend profile is `55.5%` statement coverage. This does **not**
  meet the plan threshold of `90%` and remains an acceptance item; database-only
  paths need the configured fixture to contribute coverage.
- The CyberOS-only verifier passes its static checks but reports the protected,
  pre-existing `.github/workflows/ci.yml` WIP snapshot finding. The WIP was
  preserved intentionally and not reverted.
- The product task graph `TASK-PRODUCT-001` through `TASK-PRODUCT-008` is
  authored and lint-clean. `TASK-PRODUCT-001` remains `ready_to_review`;
  successors 002–008 are `on_hold` behind sequential human review gates, so
  the graph cannot be dispatched as parallel work.
- The previous Sol Ultra read-only repair review is `approved` with no P0/P1
  findings; the latest bounded review attempt terminated at the usage limit
  before a verdict. It is not treated as approval or rejection.
  Real-device/operator U0 walkthrough, human lifecycle acceptance and human
  release authorization remain open. No commit, push, merge or deployment was
  performed by this evidence batch.

## 2026-08-22 — Provider-backed speaking and database recovery

Environment:

- Docker backend and PostgreSQL/pgvector running locally with the configured providers.
- A generated 16 kHz mono WAV was used for the deterministic API portion of the voice test; no secret values were logged.

Verified:

- `POST /api/v1/speaking/transcribe` reached Groq Whisper Large V3 and returned a transcript.
- The transcript was persisted as a `SpeakingSession`.
- `POST /api/v1/speaking/assess` reached Azure pronunciation/prosody and changed the session to `evaluated` with a non-null pronunciation result.
- `POST /api/v1/speaking/synthesize` returned a playable MP3 file.
- The browser reached the Speaking screen, but the in-app browser did not transition after `Start recording`; real microphone permission/device capture remains unproven.
- `./scripts/db_migrate.sh` recorded `000_schema_migrations.sql` and `001_initial.sql` in `schema_migrations`.
- `db_backup.sh` produced a PostgreSQL custom-format dump of 50,565 bytes with mode `600`.
- `db_restore.sh` restored that dump into disposable database `devenglish_restore_test`; `users|7` and `schema_migrations|2` were observed before the disposable database was removed.
- Restore without `CONFIRM_RESTORE=YES` was rejected before any database command ran.

Implementation note:

- Groq requires a supported audio extension in the multipart filename. The STT client now derives the filename extension from the uploaded MIME type, so WAV/WebM/MP3-style requests are identified correctly.

Remaining evidence:

- A real HTTPS domain, DNS, certificate and deployment target are still required for production verification.

## 2026-08-22 — P0 production-boundary hardening

Implemented and statically verified:

- Production rejects missing `DATABASE_URL`, auth secret, login secret or CORS allowlist.
- Production skips local demo seeding and disables deterministic AI fallback.
- Configured CORS rejects origins outside the explicit allowlist.
- `/api/v1/settings/test` now performs authenticated provider probes instead of reporting configuration only.
- Runtime `vietnameseFallback` fields were removed because no persisted source-of-truth existed.

Checks:

- `go test ./...` — passed (`36` tests across `10` packages).
- `go vet ./...` — passed.
- `flutter analyze` — passed.

Not yet proven in this batch:

- Production process startup with real secrets and PostgreSQL.
- Live provider probe responses with the configured DeepSeek/Groq/Azure credentials.
- HTTPS deployment.

## 2026-08-23 — P0.1 production web authentication

Implementation and verification boundary:

- Removed the Flutter `API_TOKEN` compile-time path. The client now uses the browser credentialed HTTP client and never receives `DEVENGLISH_BOOTSTRAP_KEY`.
- Added `POST /api/v1/auth/login` with a server-side `DEVENGLISH_LOGIN_SECRET`; successful login sets a `Secure`, `HttpOnly`, `SameSite=Strict` cookie without returning a bearer token.
- Added `POST /api/v1/auth/logout`, server-side session revocation, session restore through `/api/v1/auth/me`, and an expiry/401 transition back to the login screen.
- Kept `POST /api/v1/auth/session` explicitly development/API-tooling-only; production returns `404` for that bootstrap route.
- Configured CORS credentials only for an explicit origin allowlist.

Checks:

- `go test ./...` — passed after adding anonymous rejection, cookie login, reload, logout revocation and production bootstrap-route tests.
- `go vet ./...` — passed.
- `flutter analyze` — passed.
- `flutter test` — passed.
- `go test -race ./internal/httpapi` — passed.
- `flutter build web --release --dart-define=DEVENGLISH_ENV=production --dart-define=API_BASE_URL=https://app.example.com` — passed; the resulting `build/web` artifact contained no `API_TOKEN`, `DEVENGLISH_BOOTSTRAP_KEY` or login-secret marker.
- `flutter build web --release --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://localhost:8080` — passed.
- `docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build` — passed; `GET /healthz` returned `200` and unauthenticated development `GET /api/v1/home` returned `200` without a client bearer token.

Not yet proven in this batch:

- Real browser login over HTTPS with a deployed certificate and domain; `Secure` cookie behavior still belongs to the P0.5 HTTPS verification gate.
- Multi-instance/shared persistence for session revocation; the current RC1 target is one production backend instance.

## 2026-08-22 — Local runtime smoke

Environment:

- Docker backend and PostgreSQL/pgvector running locally.
- Flutter release web client served on port `8093`.

Verified:

- `GET /healthz` returned HTTP `200`.
- `GET /api/v1/auth/me` returned HTTP `200` with a development session.
- `GET /api/v1/home` returned HTTP `200`.
- `GET /api/v1/practice` returned HTTP `200`.
- `GET /api/v1/review/due` returned HTTP `200`.
- `GET /api/v1/progress` returned HTTP `200`.
- `GET /api/v1/settings/test` returned HTTP `200`.
- `flutter build web --release` completed successfully.
- A fresh browser tab rendered the Home screen with the daily mission, review section, progress score and navigation bar.

Not yet verified end-to-end:

- Live Groq transcription and Azure pronunciation/TTS flow.
- Roleplay turns, Copilot generation and GitHub import.
- Review actions changing SRS state and progress metrics.

## 2026-08-22 — Writing mission continuation fix

Flow:

- Opened the daily technical writing mission in the release web client.
- Submitted a structured answer containing observed behavior, expected behavior, impact and next step.
- Waited for the backend evaluation response and inspected the resulting focus screen.

Observed:

- Backend persisted the attempt for `user-1` with score `89.30`.
- UI rendered `89 / 100`, changed progress from `1 / 2` to `2 / 2`, displayed `Next action: Continue to the next mission.`, and changed the bottom action to `Continue to next mission`.
- A fresh reload no longer showed the false demo warning while the slow initial mission request was still completing.

Code checks:

- `flutter analyze` — passed.
- `flutter test` — passed (`1` test).
- `flutter build web --release` with local API base URL and development session token — passed.

## Test evidence format

For future entries record:

- Date and environment.
- Exact command or user flow.
- Expected result.
- Observed result.
- Follow-up issue or link, if any.

## 2026-08-23 — RC1 hardening continuation

Implementation and verification boundary:

- Added AES-GCM envelope encryption for the DeepSeek secret, PostgreSQL `provider_secrets` storage, metadata-only settings endpoints and Flutter Settings controls.
- Added safe capability probes: DeepSeek configured model list, Groq configured STT model, Azure Pronunciation assessment endpoint and Azure TTS voice list.
- Added atomic PostgreSQL/memory writing outcome persistence for attempt, evaluation, mission completion, mistakes and learning-state update.
- Applied `002_provider_secrets.sql` to the existing local PostgreSQL volume.

Checks:

- `go test ./...` — passed.
- `flutter analyze` — passed (`No issues found`).
- `flutter test` — passed (`1` test).
- `bash -n scripts/db_migrate.sh scripts/db_restore.sh scripts/db_backup.sh` — passed.
- `git diff --check` — passed.
- `./scripts/db_migrate.sh` — passed; `schema_migrations` contains `000_schema_migrations.sql`, `001_initial.sql` and `002_provider_secrets.sql`; `provider_secrets` and `user_settings.deepseek_status` exist.
- Capability probe HTTP tests — passed with safe error-code assertions and no provider response-body leakage.

## 2026-08-23 — RC1 local runtime and browser smoke

Environment:

- Docker backend and PostgreSQL/pgvector running locally.
- `.env.local` now contains an ignored 32-byte `DEVENGLISH_SECRET_ENCRYPTION_KEY`; the value was not printed or committed.
- Flutter release web client served at `http://localhost:8093`.

Verified:

- `docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build backend` — passed; the rebuilt backend started with the encryption key present.
- `GET /healthz` — `200`; `GET /api/v1/settings` — DeepSeek connected and speech configured.
- `GET /api/v1/settings/test` — DeepSeek text generation, Groq speech-to-text, Azure Pronunciation assessment and Azure Neural TTS all returned healthy capability-specific checks with safe metadata.
- DeepSeek Settings lifecycle — set/test/remove, no raw key in responses, encrypted secret survived a backend restart, and the provider-secret row was removed cleanly.
- Live PostgreSQL learning loop — generated a daily mission, submitted a writing attempt, returned score `95.5`, and persisted one completed mission, one attempt and one evaluation.
- Live roleplay and Copilot requests — completed and persisted one roleplay usage record and one Copilot usage record.
- Fresh in-app browser tab — Home rendered after the Flutter web rebuild; Practice, Review, Progress and Settings navigation rendered; Settings eventually displayed `Connected` after provider checks completed.
- Speaking screen rendered with manual transcript fallback and a microphone entry point.

Not yet proven:

- The in-app browser microphone click did not transition from `Ready`; no permission prompt was accepted. A real Chrome/device permission test is still required.
- Real HTTPS/domain/certificate deployment, production deployment and private GitHub import remain outside the local boundary.

## 2026-08-23 — RC1 provider-backed API and PostgreSQL smoke

Environment:

- Docker backend and PostgreSQL/pgvector running locally with `.env.local` provider configuration.
- Command: `DEVENGLISH_LIVE_SMOKE=YES scripts/smoke_local.sh`.
- The script created one disposable authenticated user and removed it in an exit trap.

Verified:

- Diagnostic: 12 questions, CEFR `B2`, overall score `72`.
- Daily mission → writing attempt: evaluation score `89.3`.
- Work Context import generated vocabulary; Review submission persisted successfully.
- Roleplay conversation and one provider-backed turn returned a reply.
- Copilot returned `simple`, `natural` and `professional` outputs.
- Public GitHub README import succeeded for `octocat/Spoon-Knife`; private repository provisioning was not attempted.
- PostgreSQL counts for the disposable user included learning state, diagnostic result, missions, attempt, evaluation, work context, vocabulary, review, conversation and roleplay/Copilot AI usage rows.
- Web Speaking capture now selects the `record_web`-supported PCM16 stream and wraps the raw samples in a 16 kHz mono WAV before STT/pronunciation upload.

Additional checks:

- `flutter analyze` — passed.
- `flutter test` — passed (`3` tests, including Speaking controls/manual fallback and runtime guard flags).
- `flutter test --dart-define=DEVENGLISH_ENV=production` — passed (`3` tests; production requires auth and disables demo fallback).
- `bash -n scripts/db_backup.sh scripts/db_migrate.sh scripts/db_restore.sh scripts/smoke_local.sh` — passed.
- Targeted production CORS tests — passed; HTTP origins, URL decorations, userinfo, wildcard and extra preflight headers are rejected or not advertised.

Remaining:

- Real browser microphone permission/capture and playback evidence.
- Real HTTPS/domain/certificate deployment and production Postgres/auth/secret/backup/readiness verification.

## 2026-08-23 — RC1 P1/P2 Chrome and PostgreSQL verification

Environment:

- Docker backend and PostgreSQL/pgvector running locally.
- Flutter release web rebuilt and served at `http://localhost:8093`.
- Real Chrome tab with microphone permission available; no secrets were printed or committed.

Verified:

- Speaking browser E2E: `Ready` → `Recording` → `Success — transcript ready` → persisted `SpeakingSession` → `Success — pronunciation assessed` → `Playing feedback` → `Success — feedback played`. The persisted session was `evaluated`, contained the transcript and had pronunciation assessment data.
- Diagnostic UI/API/PostgreSQL: 12-question flow returned A1 and `22.5`; `learning_state` and `diagnostic_results` rows were present.
- Writing UI/API/PostgreSQL: the browser displayed `89 / 100` feedback. A deliberately malformed answer then produced three structured corrections, persisted three `mistakes`, and updated the user's skill profile.
- Work Context: the first browser attempt exposed a real 30-second client timeout while the provider request took about 34 seconds. The Flutter endpoint now uses a 90-second timeout; the browser then rendered `Suggested mission` successfully. PostgreSQL contained work-context, mission and vocabulary rows.
- GitHub UI import: public `https://github.com/octocat/Spoon-Knife` rendered a suggested mission with source URL and backend domain; the import was persisted in PostgreSQL.
- Review UI/API/PostgreSQL: `observed behavior` was revealed and marked `Got it`; `vocabulary_reviews` recorded success `true`, score `90`, mastery advanced to `0.31` and `next_review` moved forward.
- Roleplay UI/API/PostgreSQL: a real Chrome turn rendered the AI follow-up question; the conversation and roleplay usage rows were persisted.
- Copilot UI/API/PostgreSQL: Simple, Natural and Professional outputs rendered; Copilot usage was persisted.

Checks:

- `flutter test` — passed (`5` tests, including API client and widget smoke tests).
- `flutter test test/browser_smoke_test.dart -d chrome --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://localhost:8080` — passed.
- `flutter analyze` — passed.
- `go test -race ./...` and `go vet ./...` — passed.

Remaining:

- Real HTTPS/domain/certificate deployment and production Postgres/auth/secret/backup/readiness verification.
- Draft PR #1 and protected `main` are in place; merge still requires one independent approval.

## 2026-08-23 — Production packaging and local production-mode smoke

Implemented:

- Added `GET /readyz`, backed by `Repository.Ready`; PostgreSQL readiness uses `Pool.Ping` and the memory store remains testable.
- Added CA certificates and `wget` to the backend runtime image for outbound TLS and container health checks.
- Added `Dockerfile.web`, `infra/docker-compose.production.yml`, `infra/Caddyfile.production`, `infra/production.env.example` and `docs/project/PRODUCTION_RUNBOOK.md`.

Checks:

- `go test ./...`, `go test -race ./...` and `go vet ./...` — passed.
- `docker compose --env-file infra/production.env.example -f infra/docker-compose.production.yml config --quiet` — passed.
- Backend production image build — passed.
- Flutter/Caddy production image build with `API_BASE_URL=https://english.example.com` — passed.
- Caddy production configuration validation — passed.
- Local `DEVENGLISH_ENV=production` container smoke — `/healthz` 200, `/readyz` 200, login 200, authenticated `/auth/me` 200, logout 204 and configured HTTPS-origin CORS 200.

Boundary:

- This is local production-mode evidence only. The real domain, DNS, certificate, production host/database, off-host backup and external logging still require operator-provided infrastructure.

## 2026-08-23 — Disposable PostgreSQL repository regression

Implemented:

- Added opt-in `TestPostgresRepositoryIntegration`, enabled with `DEVENGLISH_TEST_DATABASE_URL` so the default unit suite remains database-free.
- Added a GitHub Actions `postgres` job using disposable `pgvector/pg16`, applying all versioned migrations before the test.

Verified locally against the running PostgreSQL/pgvector container:

- `DEVENGLISH_TEST_DATABASE_URL=... go test ./backend/internal/store -run '^TestPostgresRepositoryIntegration$' -count=1` — passed.
- The test verified per-user mission isolation, persisted mission completion, persisted mistake extraction, learning-state/skill update and cleanup of disposable users.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, `git diff --check` and shell syntax checks — passed.

## 2026-08-23 — PR #1 review follow-up: backend-aware Chrome smoke and migration runner

Implemented:

- Added an injectable `AppController` to the Flutter root so the browser smoke can inspect the same state that the rendered screens use without changing production ownership or disposal behavior.
- Changed the Chrome smoke from demo-only navigation to a backend-backed check. It requires the generated mission from PostgreSQL, verifies Practice/Review/Progress data, and fails when the backend silently falls back to demo data.
- Used Flutter's integration-test binding only for the explicitly enabled Chrome smoke so real browser HTTP requests are allowed; ordinary `flutter test` remains demo-safe and keeps the smoke skipped.
- Made the Work Context 90-second timeout an explicit API policy and added a short injected timeout regression test that proves a hanging request is interrupted.
- Added a disposable Compose PostgreSQL definition and `scripts/db_migrate_test.sh`, which invokes the tracked `scripts/db_migrate.sh`, checks first-run application, second-run idempotency and transactional rollback of a failing migration.
- Updated CI so the Flutter job starts a disposable backend for the Chrome smoke and the PostgreSQL job exercises the actual migration runner before the repository integration test.

Local checks:

- `flutter test test/browser_smoke_test.dart -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true --dart-define=INTEGRATION_TEST_SHOULD_REPORT_RESULTS_TO_NATIVE=false --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://127.0.0.1:18081` — passed against a disposable PostgreSQL-backed Go backend.
- `COMPOSE_FILE=infra/docker-compose.ci.yml COMPOSE_PROJECT_NAME=devenglish-ci POSTGRES_SERVICE=postgres POSTGRES_USER=devenglish POSTGRES_DB=devenglish ./scripts/db_migrate_test.sh` — passed: all tracked migrations applied, rerun was idempotent and the failing fixture left no table or migration record.

Release boundary:

- RC1 P4 remains `PARTIAL` while PR #1 is awaiting independent review and merge. UsageGuard is intentionally not included in this PR.

## 2026-08-23 — RC1 bounded production review-fix documentation

Verified/documented:

- Production Compose no longer mounts `./migrations` into `docker-entrypoint-initdb.d`; the production runbook waits for the PostgreSQL healthcheck and confirms `pg_isready` before `scripts/db_migrate.sh`.
- `infra/production.env.example` uses `CHANGE_ME` sentinels. Existing production startup validation rejects the unchanged example; no real secrets are present.
- README documents the backend/PostgreSQL prerequisite and the complete backend-aware Chrome smoke flags. `All tests skipped.` is explicitly not a pass.
- RC1 P4 remains `PARTIAL`: the review-fix is still uncommitted in the current worktree; local and remote branch HEAD remain `d39019a`; CI run 32632728390 covers `d39019a` only. Independent approval and merge are still required.

Bounded checks run for this patch:

- `ls -l scripts/production_example_test.sh` — passed; the script is executable.
- `bash -n scripts/production_example_test.sh` — passed.
- `./scripts/production_example_test.sh` — passed; output confirmed that the unchanged production example is rejected by startup validation.
- `git diff --check` — passed.
- YAML and diff inspection — passed; the production job now installs the Go toolchain from `go.mod` before running the Go-based validation script.

At the time of this bounded documentation check, full test suites and live-provider checks had not been run by design. Subsequent Phase 1 verification is recorded below; live-provider checks remain intentionally unrun.

## 2026-08-23 — Phase 1 local evidence correction and Chrome smoke

Environment and boundary:

- Branch: `chore/rc1-timeout-browser-smoke`.
- Local and remote branch HEAD: `d39019abb93b333654b14e068a1ae0d476b1cae2`.
- Review-fix remains uncommitted in the current worktree; no commit, push, PR mutation, merge or deployment was performed.
- Backend and PostgreSQL were rebuilt/running locally from the current worktree. No live DeepSeek, Groq or Azure smoke was run.

Documentation correction:

- `RC1_EVIDENCE.md` keeps P4 as `PARTIAL` and no longer states that the review-fix was pushed or that CI run 32632728390 validates the current worktree.
- CI run 32632728390 is recorded as evidence for `d39019a` only. CI on the review-fix and independent approval remain pending.

Runtime checks:

- `rtk docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build backend` — passed; `infra-backend-1` was recreated from the current worktree.
- `rtk curl -sS -o /tmp/devenglish-healthz-phase1.out -w 'healthz HTTP %{http_code}\\n' http://127.0.0.1:8080/healthz` — `healthz HTTP 200`.
- `rtk curl -sS -o /tmp/devenglish-readyz-phase1.out -w 'readyz HTTP %{http_code}\\n' http://127.0.0.1:8080/readyz` — `readyz HTTP 200`.

Required local gates:

- `rtk git diff --check`, `rtk gofmt -l backend`, `rtk bash -n scripts/*.sh`, local/CI/production Compose config and CI YAML parse — passed.
- `rtk go test ./...` — passed, 47 tests in 11 packages.
- `rtk go test -race ./...` — passed, 47 tests in 11 packages.
- `rtk go vet ./...` — passed, no issues.
- `rtk dart format --output=none --set-exit-if-changed lib test` — passed, 28 files and 0 changed.
- `rtk flutter analyze` — passed, no issues found.
- `rtk flutter test` — passed, 5 passed and 1 skipped.
- `rtk flutter test --dart-define=DEVENGLISH_ENV=production` — passed, 5 passed and 1 skipped.
- `rtk flutter build web --release` — passed, `Built build/web`.
- `COMPOSE_FILE=infra/docker-compose.ci.yml COMPOSE_PROJECT_NAME=devenglish-ci POSTGRES_SERVICE=postgres POSTGRES_USER=devenglish POSTGRES_DB=devenglish rtk ./scripts/db_migrate_test.sh` — passed on fresh disposable PostgreSQL; first application, idempotent rerun and failing migration rollback were verified.
- `DEVENGLISH_TEST_DATABASE_URL=postgres://devenglish:devenglish-ci@127.0.0.1:25432/devenglish?sslmode=disable rtk go test ./backend/internal/store -run '^TestPostgresRepositoryIntegration$' -count=1` — passed, 1 test.

Backend-aware Chrome smoke:

- Command: `rtk flutter test test/browser_smoke_test.dart -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true --dart-define=INTEGRATION_TEST_SHOULD_REPORT_RESULTS_TO_NATIVE=false --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://127.0.0.1:8080`.
- Result: passed; output ended with `00:02 +1: All tests passed!`.
- Boundary: this proves the current local backend-backed browser navigation smoke over HTTP. It does not prove the new review-fix on CI, independent review, HTTPS, production deployment or live-provider behavior.

## 2026-08-25 — Adaptive roleplay bilingual guidance

Implementation and test scope:

- Deterministic roleplay now detects the full Vietnamese diacritic set plus common unaccented Vietnamese markers and general help/guidance requests.
- Guidance-only results carry an internal signal through both deterministic and DeepSeek providers. The learning service keeps score at `0` and removes technical credit instead of applying `FinalWritingScore`.
- Adaptive deterministic replies keep exactly one immediately usable English starter sentence and choose simple starter/question pairs by `technical-interview`, `system-design` or general scenario type.
- Normal English deterministic roleplay keeps its existing response path.
- DeepSeek roleplay now carries an adaptive bilingual system-prompt contract that preserves normal English behavior and adapts to scenario level.
- The HTTP test uses `httptest.NewServer`; it captures the outgoing DeepSeek system/user messages and does not contact the network.

Checks run:

- `rtk gofmt -w backend/internal/ai/provider.go backend/internal/ai/provider_test.go backend/internal/ai/provider_http_test.go backend/internal/learning/features.go backend/internal/learning/features_test.go && rtk go test ./backend/internal/ai ./backend/internal/learning` — passed; output: `Go test: 30 passed in 2 packages`.
- `rtk go test ./...` — passed; output: `Go test: 55 passed in 11 packages`.
- `rtk gofmt -l backend/internal/ai/provider.go backend/internal/ai/provider_test.go backend/internal/ai/provider_http_test.go backend/internal/learning/features.go backend/internal/learning/features_test.go` — passed; no files reported.
- `rtk git diff --check` — passed; no whitespace errors reported.

Boundary:

- This proves deterministic provider behavior, the service-level score gate and the DeepSeek prompt contract against a local fake server. No live DeepSeek request, Flutter test or Flutter source change was made for this scoped backend task.

Sol Ultra independent review on the final local diff:

- `rtk go test ./...` — passed; `55 passed in 11 packages`.
- `rtk go test -race ./...` — passed; `55 passed in 11 packages`.
- `rtk go vet ./...` — passed; no issues found.
- `rtk gofmt -l backend` and `git diff --check` — passed; no output.
- `rtk bash -n scripts/db_backup.sh scripts/db_migrate.sh scripts/db_restore.sh scripts/smoke_local.sh` — passed; no syntax errors.
- `rtk go run ./backend/cmd/contentfactory --root . --write-evaluation-cases tests/evaluation_cases.generated.json --target 200` — passed; generated `200` evaluation cases and left the file checksum unchanged at `282728854032e5715b203d6b693faf02f84a1bb74dacb7724f479981e7db04d5`.
- `rtk docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build backend` — passed; the local backend image was rebuilt from the final worktree and `infra-backend-1` was recreated.
- The first immediate health requests raced container startup and were reset. After the server logged that it was listening on port `8080`, `/healthz` and `/readyz` both returned HTTP `200`.
- GitNexus final impact review classified `RoleplayResult`, `Service.RoleplayTurn` and `DeterministicProvider` as `LOW`, and `DeepSeekProvider` as `MEDIUM`; no `HIGH` or `CRITICAL` impact was reported. The installed CLI does not provide the repository-requested `detect-changes` command.
- Decision: `APPROVED` for the reviewed uncommitted local diff. It is not `READY_TO_MERGE`: there is no exact new commit SHA or CI run, and live DeepSeek behavior remains intentionally unverified.
