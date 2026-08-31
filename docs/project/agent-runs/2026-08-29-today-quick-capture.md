# Today quick capture evidence — 2026-08-29

## Scope

Today now exposes a canonical quick-capture entry point for the smallest useful
piece of work. The controls are available only when a live `WorkspaceApi` is
connected; demo mode keeps the same surface visible as preview-only and does
not write data.

## Source result

- `lib/src/features/today/workspace_today.dart` adds the Quick capture surface
  with New task, Record decision and Add source actions.
- `lib/src/features/workspace/workspace_dialogs_import.dart` adds the task
  dialog with active-project selection, priority and optional description.
- Task creation reuses `WorkspaceController.createTask`, including the
  existing idempotency and error handling boundary.
- Decision creation reuses the existing reviewed Work dialog.
- Manual source creation reuses the existing review-before-import flow.
- `lib/src/components.dart` makes SectionTitle trailing text wrap or ellipsize
  safely on compact viewports.

## Verification

Commands were run from the repository root with the repository `rtk` wrapper:

```text
rtk flutter test test/workspace_ui_test.dart --plain-name
'Today quick capture creates a task through the workspace API'
exit 0 — 1 test passed

rtk flutter test --reporter compact
exit 0 — 52 tests passed, 2 environment skips

rtk flutter test test/workspace_golden_test.dart
test/workspace_state_golden_test.dart --reporter compact
exit 0 — all selected golden tests passed

rtk flutter analyze
No issues found!

rtk flutter build web --release --no-wasm-dry-run
exit 0 — Built build/web

rtk bash scripts/verify_multi_agent_protocol.sh
PASS protocol
PASS entrypoints
PASS manifests

rtk git diff --check
exit 0
```

The rebuilt release artifact was served from the existing local static server
at `http://localhost:8093/?v=20260829-quick-capture`. A fresh browser smoke
rendered the canonical Today screen, including all three Quick capture actions,
with title `DevEnglish` and no captured console warnings or errors. The
browser surface is local rendering evidence; the physical-device U0 verdict,
live provider acceptance and release authorization remain open.

## Boundary

No commit, push, merge or deployment was performed. Quick capture is verified
against a mocked Workspace API and local release rendering; it does not replace
the required authenticated device walkthrough or production end-to-end write
acceptance.
