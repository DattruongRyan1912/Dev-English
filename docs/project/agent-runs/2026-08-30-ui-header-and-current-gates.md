# UI header correction and current frontend gates — 2026-08-30

## Scope

- correct the compact Today header so its Settings action remains visible in
  the top-right slot at mobile width;
- keep the multi-action controls on Work and Knowledge from competing with
  their page heading;
- refresh the frontend evidence after the source and golden changes.

## Implementation

- `WorkspacePageFrame` accepts `compactActionInHeader` for a single compact
  action; Today opts into it and Work/Knowledge keep their wrapped action row;
- `test/workspace_ui_test.dart` asserts the Settings control is in the compact
  header for a `390×844` viewport;
- the affected workspace surface and loading/offline state goldens were
  regenerated from the intentional layout change.

## Verified commands

| Check | Result |
| --- | --- |
| `rtk flutter test test/production_shell_test.dart --reporter compact` | exit `0`; `10` passed, `1` environment skip |
| `rtk flutter test --dart-define=DEVENGLISH_ENV=production test/production_shell_test.dart --reporter compact` | exit `0`; `11` passed |
| `rtk flutter test test/workspace_ui_test.dart test/workspace_golden_test.dart test/workspace_state_golden_test.dart --reporter compact` | exit `0`; `31` focused UI/golden tests passed |
| `rtk flutter test --reporter compact` | exit `0`; `62` passed, `2` environment skips |
| `rtk flutter analyze` | exit `0`; no issues |
| `rtk flutter build web --release --no-wasm-dry-run` | exit `0` |
| Chrome canonical navigation smoke | exit `0`; Today → Work → Knowledge → Learning passed |
| release APK + `scripts/android_emulator_smoke.sh` | exit `0` on `emulator-5554`; evidence retained under `/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.twHJEx` |
| `rtk git diff --check` | exit `0` |

## Boundary

The local frontend and emulator evidence is current, but it is not a real
device/operator U0 verdict. No task lifecycle state, commit, push, merge,
deployment or release authorization was changed.
