# WAVE-3 / TASK-008 Evidence

## Scope

- Task: `TASK-008` — Today, Work, Knowledge and text assistant UI.
- Implementation owner: main controller (`gpt-5.6-luna`).
- Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`.
- Branch: `integration/wave-3`.
- Base commit: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`.
- Allowed implementation cone: `lib/`, `test/`.
- `TASK-008A` remains out of scope and blocked until this task is independently reviewed.

## Implemented

- Added the responsive workspace shell with `Today`, `Work`, `Knowledge` and `Learning` destinations.
- Added Today briefing cards, text-first assistant composer, quick prompts, conversation history and source citation cards.
- Added explicit unknown/no-match states and a visible local-preview boundary for demo data.
- Added Work project/task/decision views and Knowledge source/claim/evidence views.
- Added a deterministic demo assistant gateway that exercises grounded and unknown response paths without live provider calls.
- Preserved the existing learning screens behind Learning navigation callbacks.
- Changed development startup so the self-contained workspace shell renders immediately while the optional legacy bootstrap runs; authenticated production flows still wait for bootstrap.
- Added widget coverage for navigation, source-backed responses, citation rendering, unknowns and empty Knowledge results.

## Changed files

```text
lib/main.dart
lib/src/screens/workspace_screen.dart
lib/src/workspace_controller.dart
lib/src/workspace_demo.dart
lib/src/workspace_models.dart
test/demo_navigation_smoke.dart
test/widget_test.dart
test/workspace_ui_test.dart
```

## Verification evidence

Recorded at `2026-08-26T15:37:26Z` or later after the final formatting pass:

| Gate | Result |
| --- | --- |
| `rtk dart format --output=none --set-exit-if-changed lib test` | exit `0`, 33 files checked, 0 changed |
| `rtk flutter analyze` | exit `0`, `No issues found!` |
| `rtk flutter test` | exit `0`, 8 passed, 1 skipped by environment condition |
| `rtk flutter build web --release` | exit `0`, `build/web` created |
| `rtk git diff --check` | exit `0` |

## Browser smoke evidence

- Release build served from the implementation worktree at `http://localhost:8094/`.
- Final desktop check loaded the Today shell in approximately 1.2 seconds after navigation; no blank page or framework error was observed.
- CUA navigation from Today to Work and back to Today succeeded.
- The `Show current tasks` quick prompt produced a user turn and assistant response.
- The response rendered the source-backed citation `DevEnglish V1 delivery plan` with `Wave 3 / TASK-007` evidence.
- Browser console query returned no `error` or `warn` entries during final startup and interaction checks.
- Flutter web content is canvas-rendered in this environment, so the DOM snapshot was not used as content evidence; screenshots and actual interactions were used for the visual/runtime check.

## Dependency and migration impact

- No backend, database, migration, provider credential or MCP files were changed by TASK-008.
- The UI currently uses local deterministic preview data and does not claim production REST/MCP/provider composition is live.
- No commit, merge, push or deploy was performed.

## Known limitations / review points

- Production wiring from the workspace UI to the existing REST/application services is a later composition step.
- Push-to-talk, editable transcript, on-demand TTS and the learning overlay belong to `TASK-008A` and were not implemented here.
- The temporary `8094` static server is only browser evidence infrastructure; it is not a production deployment.

## Review handoff

This evidence is submitted for exactly one read-only independent review by the existing Sol Ultra reviewer. The reviewer must inspect source and test output directly, verify file-cone compliance and regression risk, and return a terminal verdict of `approved`, `changes_requested` or `blocked` before `TASK-008A` is started.
