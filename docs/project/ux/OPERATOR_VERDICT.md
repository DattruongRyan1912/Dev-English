# U0 — Operator verdict

## Checklist

- [x] Screenshot audit đã ghi nhận mobile và desktop target.
- [x] User-flow map bao phủ task, source, assistant, citation, action và learning.
- [x] Low-fi wireframe bao phủ Today, Work, Knowledge, Conversation, Learning.
- [x] Token sheet và component state sheet đã khóa.
- [x] High-fidelity mobile/desktop specification đã khóa viewport, layout và state matrix.
- [x] Prototype flow đã khóa các path grounded, unknown, conflict, action, connector và learning.
- [ ] High-fidelity review trên thiết bị thật.
- [ ] Operator verdict: approved / changes_requested.

## Current recommendation

approved for incremental implementation with review after the first mobile
golden pass.

Đây là recommendation của implementation run, không thay thế human acceptance
của operator. Nếu operator chọn changes_requested, giữ nguyên backend contracts
và chỉ điều chỉnh presentation tokens/layout trước khi mở rộng surface tiếp theo.

## Local evidence available for the walkthrough

Last checked: **2026-08-30**, local release artifact at
`http://localhost:8093/`.

- The fresh browser smoke loaded the page with title `DevEnglish` and rendered
  the canonical Today surface; the previous blank-page symptom was not
  reproduced.
- The route smoke covered `Today → Work → Knowledge → Learning → Today`.
- Flutter goldens cover `390×844`, `430×932` and `1440×900`; the full suite
  passed `64` tests with `2` environment skips.
- CyberOS machine gates report `GATES: GREEN`; the multi-agent protocol
  verifier reports `PASS`. The disposable production-shaped smoke also passed
  authentication/session, Work conflict/history, Knowledge import/search, MCP
  revoke/replay, CORS and provider fail-closed checks.
- These are local/browser-sized checks only. They do not replace a physical
  Android/iOS device walkthrough or the operator verdict below.

## Suggested operator walkthrough

Run this against the local app or the intended staging URL and record any
finding before choosing a verdict:

1. Open **Today** and confirm the first prominent item is one concrete next
   action, with its source/context and one primary action.
2. From **Today → Quick capture**, create a task for an active project and
   confirm the new task is written through the workspace boundary; also open
   the decision and manual-source flows to verify their review step.
3. Open **Work**, create or edit a Project/Task/Decision, then confirm the
   resulting version/history is visible and a stale update shows a conflict
   instead of overwriting data.
4. Open **Knowledge**, search an exact phrase, open the source timeline and
   verify the excerpt, revision and freshness label are visible.
5. Ask the assistant in Vietnamese, reopen the conversation, and verify the
   answer keeps evidence, unknowns and suggested actions distinct.
6. Trigger the voice affordance if permission is available; edit the
   transcript before sending and confirm the text remains usable if recording
   or TTS fails.
7. Check narrow layout, keyboard/focus order, tap targets, safe-area spacing,
   long text wrapping and the offline/provider-error states.

## Operator decision record

Fill this section only after the walkthrough. Do not infer the verdict from a
green local build.

- Reviewer:
- Date/time and timezone:
- Device and OS/browser:
- App URL/build/ref:
- Findings or screenshots:
- U0 verdict: `[ ] approved`  `[ ] changes_requested`
- Follow-up task IDs (if any):
- Human acceptance recorded by:
