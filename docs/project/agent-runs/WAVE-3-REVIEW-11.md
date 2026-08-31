# WAVE-3-REVIEW-11 — TASK-008A review record

## Decision

`changes_requested` from the single Sol Ultra reviewer
(`01a03c8f-5f2d-7833-ad4c-94557dcca88f`) at turn
`01a03f11-a0d4-7fa3-80df-563c122aa42f`.

No integration, commit, push, merge or deploy was performed. The reviewer
required a bounded repair before TASK-008A can be accepted.

## Findings

1. HIGH — The frozen evidence listed a six-file Flutter cone while regenerated
   `AGENTS.md` and `CLAUDE.md` also differed from the base. Those generated
   out-of-cone changes were reconciled before the repair; the current worktree
   no longer contains those two changes.
2. HIGH — Native recording selected Opus although the installed `record`
   stream contract supports PCM16 for this path. The multipart request also
   omitted the explicit audio content type.
3. HIGH — Gesture cancellation could upload audio, recording had no duration
   or byte bound, and terminal cleanup was incomplete around stop timeouts.
   Speech requests had no bounded timeout and tests did not prove cancel-safe
   behavior.
4. MEDIUM — The development help fallback matched bare `help` (including
   `helper`) and assigned a normal technical score to a guidance request.

## Required repair

- Use PCM16 stream recording and wrap it as WAV on every platform.
- Align multipart MIME and filename, and add a bounded STT request timeout.
- Add an injectable recorder seam, separate cancel handling, 60-second/2 MB
  recording limits, and cleanup on every terminal path.
- Make guidance detection explicit, exclude `helper` and normal technical
  answers, and mark guidance feedback as unscored.
- Add focused tests for MIME/filename, timeout, permission denial, release,
  cancel-without-STT, recording cap, transcript insertion and fallback intent.

## Reviewer evidence

The reviewer independently ran the protocol verifier, formatting, analysis,
focused/all Flutter tests and web build. The exact final message is retained
in the Codex task turn above; this file records the durable verdict and repair
boundary only.
