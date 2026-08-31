# Keyboard accessibility hardening — 2026-08-30

## Scope

The canonical push-to-talk control previously accepted only pointer press and
release events. The V1 quality gate also requires keyboard navigation for the
web surface, so the control now exposes a focusable target and a bounded
keyboard equivalent without changing the existing pointer lifecycle.

## Implementation

- `lib/src/components.dart` makes `HoldToTalkButton` stateful and focusable;
- holding Space or Enter calls `onPressStart`, and releasing the same key calls
  `onPressEnd`;
- repeated key-down events are ignored while the current keyboard press is
  active; focus loss cancels the active keyboard recording;
- disabled controls are removed from traversal and retain the existing disabled
  visual state;
- the semantic label/hint documents both pointer and keyboard interaction, and
  the focused control receives a visible two-pixel focus ring.

## Verification

```text
rtk flutter test test/workspace_ui_test.dart --reporter compact
exit 0 — 19 tests passed

rtk flutter test test/workspace_ui_test.dart test/workspace_golden_test.dart
test/workspace_state_golden_test.dart --reporter compact
exit 0 — 32 focused UI/golden/state tests passed

rtk flutter test --reporter compact
exit 0 — 63 tests passed, 2 environment skips

rtk flutter analyze
exit 0 — No issues found!

rtk flutter build web --release --no-wasm-dry-run
exit 0 — Built build/web

rtk flutter test -d chrome --dart-define=DEVENGLISH_BROWSER_SMOKE=true
test/browser_smoke_test.dart --reporter compact
exit 0 — canonical Today → Work → Knowledge → Learning navigation passed

rtk env TAILSCALE_IP=100.126.52.73
DEVENGLISH_APK_OUTPUT=/tmp/devenglish-current-keyboard.apk
scripts/android_local_apk.sh
exit 0 — rebuilt the current release APK (52.2 MB)

rtk env ANDROID_APK_PATH=/tmp/devenglish-current-keyboard.apk
ANDROID_DEVICE_ID=emulator-5554 scripts/android_emulator_smoke.sh
exit 0 — current APK installed and canonical Android smoke passed; evidence:
/var/folders/wd/txjr4_k51yj009f68scyqyz00000gn/T/devenglish-android-smoke.Saifwv

rtk git diff --check
exit 0
```

## Boundary

This closes the source-level keyboard/focus regression and revalidates the
current web artifact/navigation smoke. Browser keyboard walkthrough,
physical-device U0 and human release acceptance remain separate gates. No task
state, commit, push, merge or deployment was changed.
