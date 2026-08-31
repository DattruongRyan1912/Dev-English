# WAVE-3-REVIEW-4 — Sol Ultra review of TASK-007

Date: 2026-08-26  
Reviewer: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)  
Review turn: `01a03e65-9493-7811-827b-bed29d1f9acf`  
Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`  
Frozen base: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

## Verdict

`changes_requested`

The review found no blocking finding, but TASK-007 must remain frozen before
TASK-008 or runtime composition. One high and two medium findings require a
bounded same-cone repair.

## Findings

### HIGH — MCP identity and argument validation was weaker than canonical validation

`auth.go` and `application.go` used the ASCII-only MCP `isControl` predicate.
Embedded Unicode controls such as U+0085 and embedded whitespace could pass
the MCP boundary even though downstream Assistant/Work validation rejects
them. This violated the fail-before-service criterion for invalid identities
and arguments.

Repair: use one Unicode-aware strict identifier validator for token identity,
conversation IDs and entity IDs, and reject Unicode controls in text before a
service call. Add pre-service tests for embedded whitespace, Unicode/C1
controls, oversize values and malformed arguments.

### MEDIUM — Public wiring could bypass application services

The exported constructor accepted structural `KnowledgeReader` and
`WorkReader` interfaces. Repository contracts implement the same read method
subsets, so future composition could inject repositories directly and bypass
service validation.

Repair: make the exported constructor accept concrete canonical service
types; retain only a package-private interface seam for tests.

### MEDIUM — Isolation and forwarding tests were incomplete

The existing tests did not prove two identity-bound tokens with the same
entity IDs, individual-get mismatch rejection for every read type,
deterministic resource ordering, or successful forwarding of trusted action
and receipt fields.

Repair: add those tests without introducing mutation or action execution.

## Verified by Sol

All required review gates exited `0`, including MCP coverage at `91.3%`,
Knowledge `96.4%`, Assistant `84.2%`, Work `93.3%`, repository tests, vet,
formatting, diff check and the protocol verifier. The complete command output
is recorded in the Sol review turn above; this artifact records the durable
verdict and repair boundary.

## Repair boundary

The repair is limited to the existing TASK-007 cone:

- `backend/internal/mcp/auth.go`
- `backend/internal/mcp/application.go`
- `backend/internal/mcp/application_test.go`
- `backend/internal/knowledge/service.go`
- `backend/internal/knowledge/service_read_test.go`

No runtime route, database, migration, provider, Flutter/UI, commit, push,
merge or deployment is authorized by this review.
