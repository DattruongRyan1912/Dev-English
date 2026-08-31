# WAVE-3-REVIEW-12 — TASK-008A re-review record

## Review identity

- Reviewer: same sole Sol Ultra session `01a03c8f-5f2d-7833-ad4c-94557dcca88f`
- Review turn: `01a03f32-86c8-7c31-9b6f-ba8a3500b1a1`
- Scope: bounded read-only re-review of the TASK-008A R1 repair
- Implementation worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`

## Terminal verdict

`changes_requested`

TASK-009 remained blocked. The reviewer did not modify, stage, commit, push,
merge, deploy or integrate any worktree.

## Findings

- **HIGH — recorder terminal states were not atomic.** Disposal did not
  synchronously invalidate an in-flight permission/start continuation, so it
  could subscribe and schedule a timer after the widget was gone. Stream
  failure also started asynchronous discard while `_recording` remained true;
  a concurrent release could therefore reach STT. Required repair: a
  synchronous disposed/discarding session token, checks after awaited startup,
  and delayed-start plus stream-error/release tests proving zero STT.
- **MEDIUM — final PCM could be truncated.** The implementation rejected
  chunks while finishing and copied the buffer before `stop()` and stream
  closure. The installed `record` contract requires waiting for stream close
  to obtain the final bytes. Required repair: drain after stop, snapshot after
  drain, and assert sentinel tail bytes in the multipart WAV request.

## Verified by Sol

- PCM16, 16 kHz mono, WAV fields, `audio/wav`, `recording.wav` and the
  30-second STT timeout were source-correct.
- Gesture cancel had a distinct discard path and the focused test observed no
  additional STT call.
- Transcript editing, explicit TTS controls and guidance matching were intact.
- `AGENTS.md`, `CLAUDE.md` and migrations matched the base; no files were
  staged. Cumulative Wave 3 status entries outside the amended cone were not
  re-reviewed.

## Commands observed by Sol

- `rtk flutter test test/roleplay_voice_learning_test.dart test/api_test.dart --reporter expanded` — exit 0; 9 passed.
- `rtk dart format --output=none --set-exit-if-changed lib test` — exit 0; 35 files, 0 changed.
- `rtk flutter analyze` — exit 0; no issues.
- `rtk flutter test --reporter expanded` — exit 0; 18 passed, 2 environment-skipped.
- `rtk flutter build web --release` — exit 0.
- `rtk go test -count=1 ./...` — exit 0; 389 passed in 16 packages.
- `rtk go test -race -count=1 ./...` — exit 0; 389 passed in 16 packages.
- `rtk git diff --check` — exit 0.
- `rtk sh scripts/verify_multi_agent_protocol.sh` — exit 0.

Live microphone permission/capture, native-device PCM runtime, live STT/TTS
providers and the actual 60-second timer path were unverified. The reviewer
did not write this artifact; it records the exact terminal result observed by
the main controller.

