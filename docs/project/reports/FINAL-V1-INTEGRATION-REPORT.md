# DevEnglish V1 final integration report

Date: 2026-08-30  
Branch: `integration/final-v1`  
Repository: `/Users/ryantruong/Project/Orther/Dev-English`

## Purpose

This report records the final consolidation candidate before the independent
Sol Ultra review and the authorized push. It is an evidence ledger, not a
release approval.

## Source and integration decision

| Input | Ref/state | Decision |
| --- | --- | --- |
| Remote delivery baseline | `origin/main` at `104304f` | Used as the clean integration base. |
| Canonical development checkout | `chore/rc1-timeout-browser-smoke` at `d39019a` plus its current working tree | Treated as the current product-reset source of truth; tracked diff and non-ignored WIP were overlaid onto the integration worktree. |
| Older Wave 2/3 worktrees | `integration/wave-2`, `integration/wave-3` and task worktrees | Preserved in place as reference/rollback snapshots; not blindly merged over the newer canonical source. |
| Integration candidate | `integration/final-v1` in `.worktrees/Dev-English-final-v1` | Isolated candidate for review, commit and push. |

No reset, cleanup or overwrite was performed on the canonical checkout or the
older worktrees.

## Machine evidence

All commands below were run from the integration worktree and their exit code
was observed.

| Area | Command/result |
| --- | --- |
| Go correctness | `rtk go test ./...` — exit `0`; 989 tests passed across 23 packages. |
| Go race | `rtk go test -race ./...` — exit `0`; all packages passed. |
| Go static/build | `rtk go vet ./...`, `rtk go build ./...`, `rtk gofmt -l backend` — exit `0`; no vet issue and no unformatted Go file. |
| PostgreSQL integration | Disposable PostgreSQL Compose database with the real migration runner and integration test suite — exit `0`; all packages passed. |
| PostgreSQL race | Same disposable database with `DEVENGLISH_TEST_DATABASE_URL` — exit `0`; all packages passed. |
| Coverage | `backend_coverage_gate.mjs` with the PostgreSQL-enabled profile — exit `0`; overall `77.42%` (`9362/12093`), R0 floor `66.78%`, and application/assistant/connectors/httpapi/knowledge/mcp/work all passed their `90%` package gates. |
| Migration safety | Real disposable database — 19 migrations applied, second run idempotent, injected failure/rollback check passed. |
| Flutter static/tests | `rtk flutter analyze` — exit `0`; `rtk flutter test` — exit `0`, 64 passed and 2 environment skips. |
| Flutter artifacts | Release web build and debug APK build — exit `0`. The web build emitted a non-blocking wasm dry-run warning before completing the fallback build. |
| Browser smoke | `rtk flutter test -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true --dart-define=INTEGRATION_TEST_SHOULD_REPORT_RESULTS_TO_NATIVE=false --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://127.0.0.1:8080 test/browser_smoke_test.dart --reporter compact` — exit `0`; Today → Work → Knowledge → Learning passed against a disposable local backend. |
| Production-shaped runtime | Disposable Docker/PostgreSQL/embedding stack — exit `0`; 19 migrations, auth/session, Work CRUD/version conflicts/history, Knowledge import/search, MCP initialize/revoke/replay, CORS allow/deny, provider fail-closed and logout checks passed. |
| Supporting scripts | Backend coverage raw-count tests, Android local APK endpoint tests, production-example fail-closed test and multi-agent protocol verification — exit `0`. |

The first browser-smoke attempt without a running backend was intentionally not
counted as a product failure: `127.0.0.1:8080` was closed and the test was
correctly rerun with the required disposable backend and explicit API URL.

## Review focus before push

Sol Ultra must independently inspect the complete diff against `origin/main`,
with particular attention to:

- the replacement of the legacy learning shell with the canonical
  Today/Work/Knowledge/Learning composition;
- data ownership, migration ordering, workspace isolation, idempotency and
  optimistic version conflicts;
- MCP authentication, scope, replay and shared REST/application semantics;
- provider failure and secret boundaries;
- the intentional removal/relocation of older coverage, production-artifact,
  backup/restore and roleplay voice test files;
- whether the linked-worktree CyberOS hook check is only a path-topology issue
  or requires a source change. The canonical checkout's CyberOS gate passes;
  the linked worktree cannot expose `.git/hooks` as a directory because its
  `.git` entry is a worktree pointer.

Required verdict: `approved`, `changes_requested` or `blocked`, with exact
file-level findings. No commit, push, merge or deployment is implied by this
report.

## Known non-gates

- no live DeepSeek, Groq or Azure request was made;
- no physical phone, cross-network Tailscale route or microphone session was
  claimed from these checks;
- CI delivery-commit evidence and human release acceptance are still separate
  gates;
- the browser smoke requires a reachable backend; a blank/error state is not a
  substitute for canonical workspace evidence.

