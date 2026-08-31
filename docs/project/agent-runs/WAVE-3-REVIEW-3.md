# WAVE-3-REVIEW-3 — Sol Ultra approval

Date: 2026-08-26  
Reviewer: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)  
Review turn: `01a03e44-88d0-7112-b615-34e13c9de345`  
Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`  
Frozen cone: `backend/internal/assistant/`  
Base: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

## Verdict

`approved`

No blocking, high, or medium findings remain in the bounded TASK-006
contract slice.

## Repairs verified

- Model-facing `SuggestedAction` no longer supplies operation, target identity,
  or action hash.
- Canonical action identity is separated into `ActionBinding`; attachments
  carry a private trust marker and raw attachments are rejected.
- Scope, provider, operation, challenge, idempotency key, action hash, target,
  and action ID are compared against the trusted binding.
- Missing-action errors validate before lookup and do not echo supplied IDs.
- `ConversationID` is bounded and control-safe before either dependency runs.
- Retrieval/generation errors are redacted while cancellation and deadline
  sentinels remain intact.
- Focused tests cover challenge/idempotency mismatch, raw attachments,
  replay/status, generator errors, cancellation/deadline, conversation bounds,
  and cross-tenant scope.

## Commands and results

All commands independently exited `0`:

- `rtk proxy go test -mod=readonly -race -count=1 -cover ./backend/internal/assistant` — 84.2% coverage.
- `rtk proxy go vet ./backend/internal/assistant` — no issues.
- `rtk proxy go test -mod=readonly -count=1 ./...` — all repository packages passed.
- `rtk proxy go vet ./...` — no issues.
- `rtk gofmt -d backend/internal/assistant/errors.go backend/internal/assistant/types.go backend/internal/assistant/grounding_test.go` — clean.
- `rtk git diff --check` — clean.
- Final status/base checks — HEAD and merge-base remain
  `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`; exactly three untracked files
  remain, all under the permitted assistant cone.

## Release recommendation

TASK-006 may release to the REST/MCP wiring gate. The next wiring must create
receipt attachments only from canonical action-service outputs, never from
decoded client or model payloads.

Persistence, canonical action-service runtime provenance, REST/MCP/Flutter
adapters, live providers, and semantic entailment beyond exact quote support
remain unverified because they are outside this slice.
