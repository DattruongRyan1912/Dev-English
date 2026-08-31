# WAVE-3-REVIEW-5 — Sol Ultra re-review of TASK-007-R1

Recorded at: 2026-08-26T14:45:32Z

- Reviewer: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)
- Review turn: `01a03e7d-6511-7162-88d3-20c852c10fd2`
- Submission: `01a03e7d-6511-7162-88d3-20c852c10fd2`
- Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`
- Base commit: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`
- Verdict: `changes_requested`

## Finding

One HIGH finding remains in the application argument boundary. The adapter
decodes raw arguments before the post-decode string checks. Go's JSON decoder
can replace malformed UTF-8 with U+FFFD, and `json.Valid` accepts malformed
non-UTF-8 bytes inside a JSON string. Consequently malformed raw bytes in a
message, conversation ID or entity ID could reach a dependency.

Required same-cone repair:

1. Validate `utf8.Valid(raw)` before any JSON decode or object validation.
2. Add raw-byte tests for message, conversation ID and entity ID arguments.
3. Assert `InvalidParams` and zero dependency calls for each malformed input.

Sol independently verified that the prior constructor-boundary,
identity-validation, response-scope, token-isolation, ordering/cap,
trusted-action-forwarding and read-only surface repairs were present. The
review also observed the focused race/coverage, vet, repository test,
formatting, diff and protocol gates as green for TASK-007-R1.

## Boundary and disposition

The main controller is applying only the requested raw-argument validation and
tests in the existing TASK-007 file cone. TASK-007 remains unapproved and
TASK-008 remains frozen until the same Sol Ultra session performs one fresh
R6 review. No new reviewer, worker, worktree, commit, push, merge or deploy is
authorized by this record.
