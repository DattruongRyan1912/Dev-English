# WAVE-3-TASK-008-R1 — Repair evidence

## Status

`ready_for_review`

The main controller completed the bounded repair requested by `WAVE-3-REVIEW-8`. The repair stays inside the Flutter UI/test cone and does not integrate, commit, push, merge or deploy anything. `TASK-008A` remains blocked until the same Sol Ultra reviewer returns an approved verdict.

## Review findings addressed

1. Assistant responses now select citations by explicit evidence ID (`evidence-wave3-task`, `evidence-wave3-ui`, `evidence-wave3-mcp`). Unsupported prompts return an evidence-free unknown response instead of inheriting the first source item.
2. The authenticated production bootstrap keeps the existing canonical `HomeScreen`; the local workspace preview is only used when the app is not in the production authentication mode or when the development fallback is explicitly active.
3. Workspace data now carries an explicit `WorkspaceDataOrigin`. Fixture data is rendered as `Demo preview`; `Source-backed`/`Stale source` labels are reserved for canonical-origin data.
4. Runtime coverage was updated for the current shell: backend-aware browser smoke, compact navigation, legacy learning callbacks, conversation retention across navigation, citation matching, and successful authenticated production bootstrap.

## Changed file cone

Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`

Base: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

Implementation and test changes are limited to `lib/` and `test/`:

- `lib/src/workspace_models.dart`
- `lib/src/workspace_demo.dart`
- `lib/src/workspace_controller.dart`
- `lib/src/screens/workspace_screen.dart`
- `lib/main.dart`
- `test/workspace_ui_test.dart`
- `test/browser_smoke_test.dart`
- `test/production_shell_test.dart`

The pre-existing backend WIP is not part of this repair.

## Verification evidence

Observed at `2026-08-26T16:13:09Z`:

| Gate | Result |
| --- | --- |
| `rtk git diff --check` | exit 0 |
| `rtk dart format --output=none --set-exit-if-changed lib test` | exit 0; 34 files, 0 changed |
| `rtk flutter analyze` | exit 0; no issues found |
| `rtk flutter test` | exit 0; 10 passed, 2 skipped |
| `rtk flutter test --dart-define=DEVENGLISH_ENV=production test/production_shell_test.dart` | exit 0; 1 passed |
| `rtk flutter build web --release` | exit 0 |
| `rtk flutter build web --release --dart-define=DEVENGLISH_ENV=production` | exit 0 |

The default release web build was run again after the production build so the local browser smoke used the development preview artifact.

## Browser runtime evidence

Local static server: `http://localhost:8094/`, serving the final default release build.

Verified in the Codex in-app browser at `http://localhost:8094/?qa=task-008-r1-final`:

- Today renders the text-first assistant and the explicit read-only `Local preview` notice.
- `What should I do next?` renders an evidence-free `Unknown` response because the preview source does not support that claim.
- `Show my current tasks` renders the task answer and the matching `DevEnglish V1 delivery plan` evidence card with `Demo preview` provenance.
- Browser console contained 0 error and 0 warning entries during this interaction.

## Known limitations

- The current demo gateway is deterministic preview data; it is not a REST/MCP/provider composition.
- Conversation retention is verified across in-session navigation only. Durable reload/database persistence is not claimed by `TASK-008` and belongs to the application-service/runtime integration work.
- Push-to-talk, editable transcript, TTS and learning overlay remain `TASK-008A` and are blocked pending review approval.

## Handoff

Submit exactly one same-thread Sol Ultra re-review as `WAVE-3-REVIEW-9`. The reviewer must inspect source and fresh outputs, then return `approved`, `changes_requested` or `blocked`. No replacement reviewer or additional worker is allowed.
