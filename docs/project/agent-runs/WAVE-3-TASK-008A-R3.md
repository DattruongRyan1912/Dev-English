# WAVE-3-TASK-008A-R3 — serialized stop timeout repair evidence

## Scope

This bounded repair addresses the single MEDIUM finding from
`WAVE-3-REVIEW-13`. It remains limited to:

- `lib/src/screens/roleplay_screen.dart`
- `test/roleplay_voice_learning_test.dart`

No backend, migration, provider, MCP or external mutation changed. Existing
cumulative Wave 3 WIP remains untouched.

## Implemented repair

- `_recorderStopFuture` now stores the raw recorder `stop()` operation rather
  than a timeout wrapper.
- Every caller applies `widget.recordingStopTimeout` to its own wait, while
  callers that arrive during a pending stop share the same raw operation.
- The shared future is cleared only from the raw operation's completion handler,
  so a timed-out cleanup cannot start a second underlying stop.
- The default stop timeout remains 10 seconds; the widget accepts a bounded
  timeout seam so deterministic tests can exercise timeout behavior quickly.
- Added a fake recorder with a gated hanging stop and a test that verifies one
  underlying stop call, bounded finish, cleanup/disposal, and zero STT. The test
  releases the gate after disposal so the raw operation is completed cleanly.

## Verification

Run in `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`; final
controller checks were observed at `2026-08-26T19:07:09Z`.

- `rtk dart format lib/src/screens/roleplay_screen.dart test/roleplay_voice_learning_test.dart` — exit 0.
- `rtk flutter test test/roleplay_voice_learning_test.dart test/api_test.dart --reporter expanded` — exit 0; 12 passed.
- `rtk dart format --output=none --set-exit-if-changed lib test` — exit 0; 35 files, 0 changed.
- `rtk flutter analyze` — exit 0; no issues found.
- `rtk flutter test --reporter expanded` — exit 0; 21 passed, 2 environment-skipped.
- `rtk flutter build web --release` — exit 0; `build/web` created.
- `rtk go test -count=1 ./...` — exit 0; 389 passed in 16 packages.
- `rtk go test -race -count=1 ./...` — exit 0; 389 passed in 16 packages.
- `rtk git diff --check` — exit 0.

The browser smoke from R2 remains valid for the unchanged UI contract; no
browser interaction was repeated after this internal stop-timeout-only repair.
No commit, push, merge or deploy was performed.

## Handoff

The R3 repair is frozen for `WAVE-3-REVIEW-14` by the same Sol Ultra reviewer.
TASK-009 remains blocked until that review returns an accepted terminal verdict.
