# TASK-006 implementation evidence

Date: 2026-08-26
Wave: WAVE-3
Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`
Base commit: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

## Scope delivered

The main Luna controller implemented and repaired the bounded TASK-006 slice
in the new `backend/internal/assistant/` package:

- provider-neutral `Retriever` and `Generator` interfaces;
- authenticated workspace+user-bound `Service.Ask` orchestration boundary;
- stable `AssistantResponse` JSON fields: `answer`, `evidence`, `unknowns`,
  `staleSources`, `suggestedActions`, and `actionReceipts`;
- exact evidence-ID and quote validation against both evidence and answer;
- cross-workspace and cross-user evidence rejection;
- fail-closed unknown response when no citation/evidence is supplied, without
  invoking the generator or retaining model actions;
- stale/unknown evidence is labeled `inferred` and surfaced by source ID;
- suggested actions are declarative and always confirmation-gated;
- action receipts are not accepted from the model draft and can only be
  attached through a trusted `ActionReceiptAttachment` constructed from a
  canonical `ActionBinding`; receipt scope, provider, operation, challenge,
  idempotency, target and action hash must match that binding and canonical
  pending, uncertain, or accepted status;
- dependency failures are classified without returning provider error bodies;
- identifiers and text are bounded and reject unsafe control characters,
  including the optional conversation identifier.

## Files

- `backend/internal/assistant/errors.go`
- `backend/internal/assistant/types.go`
- `backend/internal/assistant/grounding_test.go`

No existing learning, store, provider, MCP or Flutter symbols were modified.

## Verification evidence

All commands ran in the Wave 3 worktree. Exit code was `0` for each command.

| Check | Result |
| --- | --- |
| `rtk go test -race -count=1 -coverprofile=/tmp/devenglish-wave3-task006-repair2.cover ./backend/internal/assistant` | 25 tests passed |
| `rtk go tool cover -func=/tmp/devenglish-wave3-task006-repair2.cover \| tail -1` | 84.2% statements |
| `rtk go vet ./backend/internal/assistant` | no issues |
| `rtk go test -count=1 ./...` | 377 tests passed in 16 packages |
| `rtk go vet ./...` | no issues |
| `rtk gofmt -d backend/internal/assistant/errors.go backend/internal/assistant/types.go backend/internal/assistant/grounding_test.go` | clean |
| `rtk git diff --check` | clean |

Live providers, network calls, credentials and external mutations were not
used.

## Sol review and repair

Sol Ultra completed `WAVE-3-REVIEW-1` and `WAVE-3-REVIEW-2` in the existing
reviewer thread. R1's three high findings and relevant medium fail-closed
branches were repaired in the same file cone. R2 verified the grounding,
scope, empty-retrieval, dependency-redaction and status repairs, but found one
remaining high receipt-binding gap plus ConversationID/error-boundary gaps.
The same cone now separates the model-facing suggestion from the trusted
challenged-action binding, validates conversation identifiers and adds focused
error/replay/mismatch tests. Review evidence is recorded in
`WAVE-3-REVIEW-1.md` and `WAVE-3-REVIEW-2.md`; the repaired cone is held for
`WAVE-3-REVIEW-3`.

The repaired cone was independently re-reviewed by the same Sol thread in
`WAVE-3-REVIEW-3.md` and approved for the REST/MCP wiring gate. No commit,
integration, push, merge or deployment is claimed here.

## Known limitations

- This is the contract/grounding slice only; REST/MCP registration, provider
  routing, conversation persistence wiring and Flutter UI remain unimplemented.
- Exact quote matching proves that a citation points to supplied retrieval
  text, but does not semantically prove that the whole generated answer is
  entailed by the quote. A later retrieval/evaluation layer must address that.
- Existing roleplay conversation persistence was intentionally left untouched
  because `Conversation` has a broad Flutter call graph and this task's file
  cone is isolated.

## Review status

`WAVE-3-REVIEW-1`: `changes_requested`  
`WAVE-3-REVIEW-2`: `changes_requested`  
`WAVE-3-REVIEW-3`: `approved` (same Sol reviewer; evidence in
`WAVE-3-REVIEW-3.md`).
