# repo-context-map@1

task_id: TASK-PRODUCT-001-ux-lock
generated_at: 2026-08-29T18:56:12+07:00
task_module: product
repo_root: /Users/ryantruong/Project/Orther/Dev-English

## Existing patterns

- kind: error_type; value: `ApiException` plus explicit workspace loading/error state; pinned_in: `lib/src/workspace/workspace_controller.dart:1`
- kind: state_management; value: `ChangeNotifier` controller owns canonical workspace loading, refresh and mutation state; pinned_in: `lib/src/workspace/workspace_controller.dart:1`
- kind: logging; value: UI surfaces provider/offline/degraded state through typed `ApiException` and visible banners rather than raw provider output; pinned_in: `lib/src/features/workspace/workspace.dart:1`
- kind: test_framework; value: `flutter test` with widget, golden and Chrome smoke tests; pinned_in: `test/production_shell_test.dart:1`

## Database and type surface

- table_or_type: `WorkspaceData`; defined_in: `lib/src/workspace/workspace.dart:1`; consumed_by: Today, Work, Knowledge and Learning screens
- table_or_type: `WorkspaceSnapshot`; defined_in: `lib/src/workspace/workspace.dart:1`; consumed_by: authenticated production bootstrap and controller load
- table_or_type: `WorkspaceDataOrigin`; defined_in: `lib/src/workspace/workspace.dart:1`; consumed_by: preview/degraded-state notices
- table_or_type: `WorkspaceController`; defined_in: `lib/src/workspace/workspace_controller.dart:1`; consumed_by: app shell and feature screens

## Sampled surface and outside-domain files

The scan sampled `lib/main.dart`, `lib/src/design_system/`, `lib/src/features/{today,work,knowledge,learning}/`, `lib/src/workspace/`, the production-shell and golden tests, and the browser smoke test. No database migration, provider implementation or external connector is part of this task's implementation cone.

- path: `lib/main.dart`; reason: selects the canonical production shell and navigation; risk: medium
- path: `lib/src/workspace/`; reason: supplies the shared data/controller contract used by every surface; risk: medium
- path: `test/workspace_state_golden_test.dart`; reason: proves the five required state presentations; risk: low
- path: `test/browser_smoke_test.dart`; reason: verifies the built shell in a real browser target; risk: low

## Blast radius

```yaml
files_in_immediate_domain: 9
files_outside_immediate_domain: 4
modules_touched: 5
cross_module_edges: 4
score: 42
```

module_placement_warning: null

## Integrity notes

- The actual feature paths are under `lib/src/features/`; stale `lib/src/screens/` paths were corrected in the sibling Product task specifications before this map was recorded.
- The map is a static evidence artifact. It does not execute application code and does not authorize integration or release actions.
