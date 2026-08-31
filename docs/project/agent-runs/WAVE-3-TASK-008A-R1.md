# WAVE-3-TASK-008A-R1 — bounded repair evidence

## Scope

This is the repair requested by `WAVE-3-REVIEW-11`. It remains a Flutter
voice/learning slice and does not change backend schema, provider credentials,
migrations or external mutations.

The amended file cone is:

- `lib/src/api.dart`
- `lib/src/app_controller.dart`
- `lib/src/components.dart`
- `lib/src/models.dart`
- `lib/src/screens/roleplay_screen.dart`
- `pubspec.yaml`
- `pubspec.lock`
- `test/api_test.dart`
- `test/roleplay_voice_learning_test.dart`
- `test/workspace_ui_test.dart`

The existing TASK-008A UI files remain part of the cumulative Wave 3 worktree;
the repair only touches the files above. `AGENTS.md` and `CLAUDE.md` were
checked and are unchanged in the repair worktree.

## Implemented repair

- `RoleplayAudioRecorder` is injectable for deterministic tests.
- Push-to-talk uses `AudioEncoder.pcm16bits`, 16 kHz mono, and wraps captured
  PCM in a WAV container for both web and native targets.
- Recording is bounded to 60 seconds and 2 MiB of PCM; hitting the byte/time
  limit stops and transcribes the bounded capture.
- `onTapCancel` calls a discard path that stops, cancels the subscription and
  clears audio without calling STT.
- Start/stop/speech operations have bounded timeouts and cleanup is attempted
  on permission, stream, stop, cancellation and disposal paths.
- STT multipart audio now sends an explicit MIME type and matching filename;
  the request has a configurable 30-second default timeout.
- Development guidance matching uses explicit Vietnamese/English request
  phrases, avoids `helper` and ordinary technical sentences, and guidance
  feedback is marked `scored: false` with no technical score.
- Focused tests cover release transcript insertion, cancel without STT,
  permission denial, the recording cap, fallback intent regressions,
  multipart MIME/filename and STT timeout.

## Verification

Run in `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3` on
2026-08-26T17:46:58Z:

- `rtk dart format lib test` — exit 0; 35 files formatted, 0 changed after
  the final pass.
- `rtk flutter analyze` — exit 0; no issues found.
- `rtk flutter test test/roleplay_voice_learning_test.dart test/api_test.dart`
  — exit 0; 9 tests passed.
- `rtk flutter test` — exit 0; 18 passed, 2 environment-skipped.
- `rtk flutter build web --release` — exit 0; `build/web` created.
- `rtk go test ./...` — exit 0; 389 passed in 16 packages.
- `rtk go test -race ./...` — exit 0; 389 passed in 16 packages.
- `rtk git diff --check` — exit 0.

## Browser smoke

The rebuilt release web shell was opened at
`http://localhost:8094/?qa=task-008a` in the in-app browser. Visual smoke
confirmed the Roleplay screen renders the assistant opening, selected scenario
context, editable transcript hint, learning overlay and `Hold to talk` control
with `Voice: Ready`. This does not claim live microphone, STT or TTS provider
success.

## Handoff

The repair is ready for `WAVE-3-REVIEW-12` by the same Sol Ultra reviewer.
TASK-009 remains blocked until that review returns an accepted verdict.
