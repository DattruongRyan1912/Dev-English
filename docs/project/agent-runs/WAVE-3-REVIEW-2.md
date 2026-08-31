# WAVE-3-REVIEW-2 — Sol Ultra re-review

Date: 2026-08-26  
Reviewer: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)  
Review turn: `01a03e32-ce88-7d41-8f9b-05c1c9660fbb`  
Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`  
Frozen cone: `backend/internal/assistant/`  
Base: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

## Verdict

`changes_requested`

## Finding

### High

The receipt challenge and idempotency identity is only checked for presence.
Operation, target, and action hash are still taken from the model-facing
`SuggestedAction` in `Draft`, so a caller could relabel a receipt with a
different challenge or idempotency key and retain an existing action ID.

Repair by separating model suggestions from a trusted challenged-action
binding and comparing receipt scope, challenge, idempotency, operation, target,
and action hash against that binding.

### Medium

- `ConversationID` is trimmed but not bounded or control-character safe before
  it is sent to retriever and generator dependencies. Validate it as an
  optional identifier.
- Receipt action IDs are looked up and interpolated into an error before
  identifier validation. Validate first and avoid echoing untrusted values.
- Focused coverage is 82.1%; challenge/idempotency mismatch, generator error
  classification, conversation-ID bounds, cancellation/deadline propagation,
  and receipt transition/replay edge cases are not covered.

## Repairs verified

- Quotes must occur in both evidence and answer.
- Workspace and user scope are explicit and cross-scope evidence is rejected.
- Empty retrieval does not invoke generation or retain actions.
- Dependency bodies are redacted while cancellation/deadline sentinels are
  preserved.
- Receipt statuses are `pending`, `uncertain`, and `accepted`, with replay as
  separate outcome metadata.

## Commands independently run

All requested commands exited `0`: focused race/coverage test, focused vet,
repository test, repository vet, gofmt diff, git diff check, and final git
status. Focused coverage was 82.1%; the cone still contained exactly three
untracked files and no tracked changes outside the cone.

## Release recommendation

Hold TASK-006 at the review gate. Do not begin REST/MCP wiring until the
trusted receipt binding and remaining input-boundary repairs are independently
re-reviewed. No commit, integration, push, merge, or deployment is approved.
