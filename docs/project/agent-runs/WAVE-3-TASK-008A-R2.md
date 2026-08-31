# WAVE-3-TASK-008A-R2 — lifecycle and audio-drain repair evidence

## Scope

This bounded repair addresses the HIGH and MEDIUM findings from
`WAVE-3-REVIEW-12`. It is limited to the roleplay recording implementation and
its deterministic tests:

- `lib/src/screens/roleplay_screen.dart`
- `test/roleplay_voice_learning_test.dart`

No backend schema, migration, provider credential, MCP, or external mutation
was changed. Existing cumulative Wave 3 WIP remains untouched.

## Implemented repair

- Added synchronous disposal/session invalidation. An incrementing recording
  session token is checked after permission, recorder start, stop, stream drain,
  cleanup and transcription awaits before the session can perform side effects.
- Made stream errors terminal synchronously: recording state is invalidated
  before asynchronous discard cleanup begins, so a concurrent release cannot
  invoke STT.
- Added delayed-start disposal handling and force-stop cleanup for a recorder
  whose `startStream` completes after the widget is gone.
- Kept collecting bounded chunks through recorder stop, waited for the stream's
  `onDone` drain, and copied PCM only after stop and drain completed.
- Serialized recorder stop calls and bounded the stop wait; cleanup cancels the
  timer/subscription and clears the captured buffer on every terminal path.
- Added deterministic coverage for delayed start plus unmount, stream error plus
  release with zero STT, and sentinel tail bytes appearing in the WAV multipart
  body.

## Verification

Run in `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`; the
controller observed the browser smoke and command results at
`2026-08-26T18:51:37Z`.

- `rtk flutter test test/roleplay_voice_learning_test.dart test/api_test.dart --reporter expanded` — exit 0; 11 passed.
- `rtk dart format --output=none --set-exit-if-changed lib test` — exit 0; 35 files, 0 changed.
- `rtk flutter analyze` — exit 0; no issues found.
- `rtk flutter test --reporter expanded` — exit 0; 20 passed, 2 environment-skipped.
- `rtk flutter build web --release` — exit 0; `build/web` created.
- `rtk go test -count=1 ./...` — exit 0; 389 passed in 16 packages.
- `rtk go test -race -count=1 ./...` — exit 0; 389 passed in 16 packages.
- `rtk git diff --check` — exit 0.
- GitNexus status was up to date at the stated base; impact analysis for
  `_RoleplayScreenState` was LOW with direct impact in `lib/main.dart` and
  widget-test imports at depth two. The attempted recorder-symbol lookup had
  no indexed target and was not used as evidence.

## Browser smoke

The rebuilt release shell at `http://localhost:8094/?qa=task-008a` was opened
in the in-app browser. Visual smoke confirmed Today, Learning, Roleplay
scenario selection, the opening prompt, editable reply field, collapsible
Vietnamese help, `Hold to talk` and `Voice: Ready`. Browser logs for the final
Roleplay smoke were `[]` for error and warning levels. This does not claim live
microphone permission, STT or TTS provider success.

## Handoff

The implementation slice is frozen for `WAVE-3-REVIEW-13` by the same Sol Ultra
reviewer. TASK-009 remains blocked until that review returns an accepted
terminal verdict. No commit, push, merge or deploy was performed.

