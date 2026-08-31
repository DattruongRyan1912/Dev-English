# WAVE-3-REVIEW-9 — TASK-008 re-review

## Verdict

`changes_requested`

Sol Ultra independently re-reviewed the TASK-008 R1 repair in the existing reviewer thread. The citation/evidence boundary, provenance labels, current workspace smoke coverage and successful-authentication guard were verified. One HIGH regression remains: the authenticated production path no longer preserves the complete legacy navigation shell.

## HIGH finding

`lib/main.dart:135` returned a bare `HomeScreen` for authenticated production. The base shell had an `IndexedStack` and `NavigationBar` for Home, Practice, Review and Progress. A production user would therefore lose direct access to the latter three destinations. The production test only asserted that Home rendered and did not detect this navigation loss.

Required bounded repair:

- restore the complete legacy shell for authenticated production;
- extend `test/production_shell_test.dart` to navigate through Home, Practice, Review and Progress;
- run the same fresh gates and submit one same-thread re-review.

## Repairs independently verified

- Explicit evidence-ID citation selection and evidence-free unknown fallback: `lib/src/workspace_controller.dart:22`.
- Matching citation and unsupported-prompt tests: `test/workspace_ui_test.dart:183`.
- Explicit demo/canonical provenance and preview rendering: `lib/src/workspace_models.dart:1`, `lib/src/screens/workspace_screen.dart:641`.
- Compact navigation, legacy callbacks and in-session conversation retention: `test/workspace_ui_test.dart:65`.
- Backend-aware current-shell browser smoke: `test/browser_smoke_test.dart:12`.
- Durable reload/database persistence remains explicitly unclaimed; only controller-lifetime/session navigation is covered.

## Commands independently observed

- `rtk git diff --check` — exit 0.
- `rtk dart format --output=none --set-exit-if-changed lib test` — exit 0; 34 files, 0 changed.
- `rtk flutter analyze` — exit 0; no issues.
- `rtk flutter test` — exit 0; 10 passed, 2 skipped.
- `rtk flutter test --dart-define=DEVENGLISH_ENV=production test/production_shell_test.dart` — exit 0; 1 passed.
- `rtk flutter build web --release` — exit 0.
- `rtk flutter build web --release --dart-define=DEVENGLISH_ENV=production` — exit 0.
- `rtk sh scripts/verify_multi_agent_protocol.sh` from the canonical repo — exit 0; protocol, entrypoints and manifests passed.

## Boundary

The reviewer made no source or manifest edits, created no worker/reviewer, and did not commit, push, merge or deploy. TASK-008 remains unapproved and `TASK-008A` remains blocked until the navigation repair is independently re-reviewed.

Reviewer thread: `01a03c8f-5f2d-7833-ad4c-94557dcca88f`.

Review turn/submission: `01a03eda-f408-7423-b097-e3bf3270caa5`.
