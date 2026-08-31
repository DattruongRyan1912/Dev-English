# impl-plan@1 — reconciliation plan

task_id: TASK-PRODUCT-001-ux-lock
generated_at: 2026-08-29T18:56:12+07:00
phase: post-implementation evidence reconciliation
owner: main controller
status: evidence_recorded

## Objective

Make the product-reset shell auditable without changing the existing WIP: Today is the default authenticated surface, Work/Knowledge/Learning are reachable from the same shell, and state behavior is covered at mobile and browser targets.

## Work packages

1. Read the current shell entry point and shared workspace contract.
2. Verify the four navigation destinations and the canonical/legacy boundary from source and widget tests.
3. Verify loading, empty, provider-error, offline and stale/degraded states from the golden suite.
4. Record the actual feature paths and browser/build evidence.
5. Leave U0 visual acceptance and Git integration as explicit human gates.

## File cone

- `lib/main.dart`
- `lib/src/design_system/`
- `lib/src/features/{today,work,knowledge,learning}/`
- `lib/src/workspace/`
- `test/production_shell_test.dart`
- `test/workspace_golden_test.dart`
- `test/workspace_state_golden_test.dart`
- `test/browser_smoke_test.dart`

## Verification evidence

- `flutter test`: 52 passed, 2 environment skips.
- `flutter analyze`: exit 0.
- `flutter build web --release --no-wasm-dry-run`: exit 0.
- `flutter build apk --debug`: exit 0.
- Backend-aware Chrome smoke: exit 0, all cases passed.

## Explicit non-claims

This artifact is not a Sol Ultra review, not a visual U0 acceptance, and not a commit/push/merge/deploy authorization.
