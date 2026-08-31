# WAVE-3-REVIEW-8 — TASK-008 Review

## Verdict

`changes_requested`

Sol Ultra completed one independent read-only review of TASK-008 in the existing review thread. No blocker was found, but two HIGH and two MEDIUM findings prevent approval. The reviewer did not modify the worktree.

## Findings

### HIGH

1. Unsupported or task-status answers can receive an unrelated first citation. `workspace_controller.dart` always selects the first evidence item, while unmatched prompts only become unknown when they contain a small keyword set. Repair must select evidence by explicit claim/evidence identity and return an evidence-free unknown response for every unmatched prompt. Add tests proving that the cited excerpt supports the answer and unrelated prompts cannot receive a `Source-backed` citation.
2. Successful authenticated production bootstrap still exposes the default local demo `WorkspaceController`. Repair must restrict the preview workspace to development, preserve the authenticated production shell until canonical workspace composition exists, or inject an authenticated canonical workspace gateway. Add a successful-authentication production test.

### MEDIUM

1. Demo provenance is contradictory: the UI calls the data read-only demo in one place but labels it canonical and source-backed elsewhere. Model provenance explicitly and render fixture data as `Demo`/`Preview`; reserve canonical/source-backed labels for verified application-service data.
2. Runtime/test coverage is stale or incomplete: the opt-in browser smoke still targets the removed Home/Practice/Review/Progress shell; current workspace tests do not prove compact navigation, legacy callback reachability, or conversation retention after navigation. Clarify whether persistence means session navigation or durable reload storage.

## Direct evidence

- Reviewer thread: `01a03c8f-5f2d-7833-ad4c-94557dcca88f`.
- Review turn/submission: `01a03ebd-65fd-7431-8f50-4d1af5a627d3`.
- Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`.
- The reviewer independently observed the required gates green: diff check exit 0, format check exit 0, Flutter analyze exit 0, Flutter tests exit 0 with 8 passed/1 skipped, release web build exit 0, production-define widget/runtime checks exit 0, and canonical protocol verification exit 0.
- The reviewer confirmed the TASK-008 implementation files match the listed UI cone and the backend entries are pre-existing out-of-scope WIP.

## Next bounded repair

The main controller may repair only the four findings within the UI/test cone, with the browser smoke test explicitly added to the repair cone because the independent review identified it as stale. No worker or additional reviewer is spawned. After fresh gates and evidence are recorded, submit exactly one same-thread re-review. `TASK-008A` remains blocked until approval.
