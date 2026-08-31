# WAVE-3-REVIEW-1 — Sol Ultra review

Date: 2026-08-26  
Reviewer: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)  
Review turn: `01a03e20-f19c-70c0-9421-b70aaf6a8e6e`  
Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`  
Frozen cone: `backend/internal/assistant/`  
Base: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

## Verdict

`changes_requested`

TASK-006 must remain frozen. REST/MCP/UI wiring must not begin until the
bounded repairs below are implemented and this same Sol session re-reviews
the repaired cone.

## Findings

### High

1. Grounding can be asserted using only an evidence ID. An empty quote passes
   citation normalization and any known evidence ID marks the entire answer as
   `grounded`. Repair by requiring a non-empty quote supported by the evidence
   and the answer, then add a negative unrelated-answer test.
2. The request contract carries `WorkspaceID` but no authenticated `UserID`,
   and evidence has no scope binding. The assistant cannot reject evidence
   returned for another workspace or user. Repair with an explicit
   `{WorkspaceID, UserID}` scope through ask, retrieval, generation, evidence,
   and response validation, plus cross-workspace and cross-user tests.
3. Assistant receipts are not compatible with the Wave 2 safe-write receipt
   contract. They lack workspace, user, challenge, operation, target, action
   hash, and idempotency binding, and expose statuses that do not represent
   `pending`, `uncertain`, and `accepted`. Repair by accepting only a fully
   scoped trusted action-service result and keeping replay as outcome metadata.

### Medium

1. Empty retrieval still invokes the generator and may preserve suggested
   actions in an unknown response. Fail closed before generation and return no
   actions when there is no evidence.
2. Retriever and generator errors are returned verbatim. Return classified,
   redacted dependency errors while preserving cancellation/deadline semantics.
3. Identifiers and text have no size/control-character bounds. Add bounded
   validation before REST/MCP wiring.
4. Focused coverage is 89.0%, but the unsafe cases above are not covered.

## Commands independently run

All commands exited `0` unless noted otherwise:

- `rtk git status --porcelain=v2 -uall`
- `rtk git diff --name-only ea3bd05f92f4209e927cfbc9dd571acc4200ec28 --`
- `rtk proxy go test -mod=readonly -count=1 -race -cover ./backend/internal/assistant`
- `rtk proxy go vet ./backend/internal/assistant`
- `rtk gofmt -d backend/internal/assistant/errors.go backend/internal/assistant/types.go backend/internal/assistant/grounding_test.go`
- `rtk proxy go test -mod=readonly -count=1 ./...`
- `rtk proxy go vet ./...`
- `rtk git diff --check`

The focused package measured 89.0% statement coverage. Persistence, REST/MCP/
Flutter wiring, semantic entailment, and live providers remain unverified.

## Required next handoff

Main Luna may repair only `backend/internal/assistant/` in the existing Wave 3
worktree. After fresh tests and evidence are recorded, freeze the cone and send
exactly one read-only re-review request to this same Sol session. No commit,
integration, push, merge, or deployment is approved by this review.
