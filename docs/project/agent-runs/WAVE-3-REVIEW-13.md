# WAVE-3-REVIEW-13 — TASK-008A R2 re-review record

## Review identity

- Reviewer: same sole Sol Ultra session `01a03c8f-5f2d-7833-ad4c-94557dcca88f`
- Review turn: `01a03f6c-3d64-7482-823d-4e723ad34496`
- Review scope: TASK-008A R2 implementation and tests only
- Implementation worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`

## Terminal verdict

`changes_requested`

One MEDIUM finding remained. TASK-009 stayed blocked. The reviewer remained
read-only and did not modify, stage, commit, push, merge, deploy or integrate
any worktree.

## Finding

- **MEDIUM — stop serialization failed after timeout.** The `_stopRecorderBounded`
  implementation tracked the timeout wrapper, so its completion handler could
  clear the shared future while the underlying recorder `stop()` was still
  running. Cleanup could then issue a second underlying stop. The fake recorder
  completed immediately, so this behavior had no test coverage. Required
  repair: retain the raw stop future until it actually completes, apply a
  timeout separately to every wait, and add a hanging-stop test proving one
  underlying stop call, bounded completion, cleanup/disposal and zero STT.

## Verified by Sol

- Synchronous session invalidation protected delayed startup/unmount and
  stream-error/release paths; both tests passed with zero STT.
- Normal release waited for stream completion before snapshot; sentinel tail
  PCM reached the multipart WAV body.
- PCM16 16 kHz mono, WAV header, `audio/wav`, `recording.wav`, capture limits
  and STT timeout remained aligned.
- Editable transcript, explicit TTS controls, Vietnamese help intent,
  negative `helper` matching and unscored guidance regressions passed.
- No staged files; `AGENTS.md`, `CLAUDE.md`, migrations, provider and HTTP
  backend paths were unchanged. No secret markers appeared in the R2 cone.

## Commands observed by Sol

- Format check — exit 0; 35 files, 0 changed.
- `rtk flutter analyze` — exit 0.
- Focused Flutter tests — exit 0; 11 passed.
- Full Flutter tests — exit 0; 20 passed, 2 environment-skipped.
- Web release build — exit 0.
- Go tests — exit 0; 389 passed in 16 packages.
- Go race tests — exit 0; 389 passed in 16 packages.
- `rtk git diff --check` — exit 0.
- Manifest JSON validation — exit 0.
- Protocol verifier — exit 0; protocol, entrypoints and manifests passed.

Live microphone/native PCM, live STT/TTS, the real 60-second timer and
hanging platform-stop behavior remained unverified. This record was written by
the main controller from the completed read-only Sol turn.

