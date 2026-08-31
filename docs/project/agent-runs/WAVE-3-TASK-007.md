# TASK-007 implementation plan

Date: 2026-08-26
Wave: WAVE-3
Planner: Sol Ultra / Huygens (`01a03c8f-5f2d-7833-ad4c-94557dcca88f`)
Planning turn: `01a03e4a-ab1c-7251-a43c-efb0bb74e8b5`
Base commit: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

## Objective

Reuse the approved Wave 2 MCP transport/security implementation and add the
narrow identity-bound, read-only application-service adapter. Derive
`WorkspaceID` and `UserID` only from the authenticated MCP token. Do not
reimplement the protocol or mount a production route in this slice.

## Dependency order

1. Add minimal read methods to the Knowledge application service.
2. Bind canonical workspace/user identity to MCP token issuance and verified
   `Principal`.
3. Register read-only Assistant, Knowledge and Work tools/resources over the
   existing MCP registry.
4. Prove tenant isolation and protocol behavior with `httptest` JSON-RPC
   requests.

## Allowed implementation cone

- `backend/internal/mcp/auth.go`
- `backend/internal/mcp/application.go` (new)
- `backend/internal/mcp/application_test.go` (new)
- `backend/internal/mcp/mcp_test.go`
- `backend/internal/knowledge/service.go`
- `backend/internal/knowledge/service_read_test.go` (new)

Forbidden for this slice:

- `backend/internal/assistant/**` (approved and frozen dependency)
- `backend/internal/mcp/protocol.go`
- `backend/internal/mcp/registry.go`
- `backend/internal/mcp/replay.go`
- `backend/internal/mcp/scope.go`
- `backend/internal/work/**`
- `backend/internal/connectors/**`
- `backend/internal/httpapi/**`
- `backend/cmd/server/**`
- `infra/migrations/**`, `go.mod`, `go.sum`, and Flutter/UI files

If implementation genuinely needs a forbidden file, stop and return for
replanning rather than silently widening the cone.

## Required surface

- `assistant.ask`, requiring `assistant:use`;
- `knowledge.get_claim` and `knowledge.get_chunk`, requiring
  `knowledge:read`;
- `work.get_project`, `work.get_task`, and `work.get_decision`, requiring
  `work:read`;
- exact-URI, bounded Work list resources for projects, tasks and decisions;
- no registered tool has `Mutating: true`; no Work or connector write.

## Acceptance criteria

- Identity-bound token issuance preserves the lower-level token API and never
  places bearer secrets in `Principal`, logs, errors or responses.
- Missing/invalid token workspace or user identity is rejected before any
  application service is invoked.
- Request JSON rejects unknown fields, trailing data, client-supplied
  workspace/user/scope, control characters and oversized values.
- MCP identity maps directly to canonical Assistant, Knowledge and Work scope
  types; Knowledge reads go through service methods, never its repository.
- Assistant output exposes only the approved trusted binding/receipt shape; the
  MCP adapter neither manufactures action identity nor executes suggestions.
- Same entity IDs under different workspace/user tokens cannot cross-disclose.
- Lists are deterministic and capped at 50 or less.
- Dependency/provider errors remain classifiable and redact underlying detail.
- Existing nonce, replay, current-request-ID replay, revocation, expiry and
  scope behavior remains unchanged.
- MCP package coverage remains at least 90%.

## Gates and runtime evidence

Run all commands with real exit codes after implementation:

```text
rtk proxy go test -mod=readonly -count=1 -race -cover ./backend/internal/mcp ./backend/internal/knowledge ./backend/internal/assistant ./backend/internal/work
rtk proxy go vet ./backend/internal/mcp ./backend/internal/knowledge
rtk proxy go test -mod=readonly -count=1 ./...
rtk proxy go vet ./...
rtk gofmt -d <all changed Go files>
rtk git diff --check
rtk git status --porcelain=v2 -uall
rtk bash scripts/verify_multi_agent_protocol.sh
```

Runtime evidence is bounded to `httptest` requests through the existing MCP
HTTP handler with fake services. No live provider, shared database or
production route is used. No migration is required for this slice.

## Review point

Freeze the exact amended cone and submit it to the same Sol thread as
`WAVE-3-REVIEW-4` after the evidence artifact and all gates are complete, and
before changing HTTP/server composition, adding a mutating tool, integrating,
or starting TASK-008.

## Evidence classification

Observed from the current checkpoint:

- Wave 2 already supplies tested JSON-RPC/Streamable HTTP handling, scoped
  registry, digest-only tokens, expiry/revocation, nonce/idempotency replay,
  and current-request-ID replay.
- MCP `Principal` currently lacks application workspace/user identity.
- Knowledge repositories have reads, but `knowledge.Service` exposes only
  writes.
- MCP is not mounted by the current HTTP server.
- Baseline MCP coverage is 95.3%; repository tests, vet, formatting,
  diff-check and protocol verifier exited 0.
- No TASK-007 diff exists at planning time.

Inferred:

- Reusing the MCP core and adding a narrow adapter is safer than replacing
  already-tested protocol/security behavior.
- Read-only wiring is the largest safe slice because durable action execution
  and runtime composition are not yet present.

Unknown and intentionally deferred:

- production MCP route and token-issuance lifecycle;
- concrete persistent Knowledge/Work repositories in server composition;
- real Assistant retriever/generator and canonical action executor;
- whether persistent MCP identities eventually need schema changes;
- live-provider and production-runtime behavior.

This is a plan, not approval of TASK-007 code. Commit, push, merge and deploy
authorization remain absent.

## Implementation evidence

Recorded at: 2026-08-26T14:05:11Z

Implementation is complete in the isolated Wave 3 worktree and is pending the
single Sol Ultra review. The exact amended cone was preserved:

- `backend/internal/mcp/auth.go`
- `backend/internal/mcp/application.go`
- `backend/internal/mcp/application_test.go`
- `backend/internal/knowledge/service.go`
- `backend/internal/knowledge/service_read_test.go`

Observed behavior:

- `TokenStore.IssueForIdentity` binds validated workspace/user identity to the
  verified `Principal`; the existing `Issue` API remains protocol-only.
- The adapter registers six read-only tools: `assistant.ask`, two Knowledge
  reads, and three Work reads.
- It registers three exact Work resource URIs for bounded projects, tasks and
  decisions. Service calls receive the authenticated Work scope and responses
  are filtered, sorted by ID and capped at 50.
- Application arguments use strict JSON decoding with unknown-field,
  trailing-data, control-character, identifier, message and size checks.
- Knowledge reads go through service methods. Assistant responses and trusted
  action/receipt fields are forwarded from the approved Assistant contract;
  the adapter creates no action identity and exposes no write tool.
- Returned entities and Assistant response scope are checked against the
  authenticated identity; dependency errors keep a safe operation label while
  retaining an `errors.Is` classification without exposing provider detail.

Gate results with real command exits:

- Focused `go test -mod=readonly -count=1` across MCP, Knowledge, Assistant and
  Work: 295 tests passed.
- Focused race/coverage MCP run: 17 tests passed; 91.3% statement coverage.
- Focused race tests for Knowledge, Assistant and Work: 278 tests passed.
- Full repository `go test -mod=readonly -count=1 ./...`: 385 tests passed in
  16 packages.
- `go vet ./...`: exit 0.
- `gofmt -d` on all changed Go files: clean.
- `git diff --check`: exit 0.
- `scripts/verify_multi_agent_protocol.sh`: `PASS protocol`, `PASS
  entrypoints`, `PASS manifests`.

The runtime proof is bounded to in-process `httptest` JSON-RPC requests with
fake services. No production route, live provider, shared database, migration,
commit, push, merge or deployment was performed. The next required action is
to freeze this evidence and submit exactly one `WAVE-3-REVIEW-4` request to the
existing Sol Ultra thread before TASK-008 or runtime composition.

## Sol review and repair record

Recorded at: 2026-08-26T14:28:01Z

Sol Ultra returned `changes_requested` in `WAVE-3-REVIEW-4`. The review found
no blocking issue, but identified one high and two medium findings:

- MCP identity and argument validation did not reject all Unicode controls or
  embedded identifier whitespace before service calls.
- The exported MCP application constructor accepted structural reader
  interfaces that could be implemented directly by repositories.
- Isolation tests did not cover two identity-bound tokens with reused entity
  IDs, every individual response-scope mismatch, deterministic ordering, and
  successful trusted action/receipt forwarding.

The main controller is applying a bounded same-cone repair. TASK-007 remains
unapproved and TASK-008 remains blocked until the repaired cone is independently
reviewed by the same Sol Ultra session.

## TASK-007-R1 implementation evidence

Recorded at: 2026-08-26T14:32:00Z

The bounded repair is complete in the same Wave 3 worktree. It addresses all
three WAVE-3-REVIEW-4 findings without changing the application surface beyond
the approved TASK-007 cone:

- `validMCPIdentifier` now rejects invalid UTF-8, leading/trailing or embedded
  Unicode whitespace, Unicode control characters and oversized identifiers.
  Token identity issuance, conversation IDs and entity IDs share this strict
  validator; message validation rejects Unicode controls before dependencies.
- `NewApplication` now accepts only canonical `*assistant.Service`,
  `*knowledge.Service` and `*work.Service` values. Structural service seams
  are package-private and used only by tests, so a repository cannot be passed
  through the public constructor.
- Tests now cover two identity-bound tokens reusing the same entity IDs,
  individual response-scope mismatches for Assistant/Knowledge/Work reads,
  deterministic resource ordering and successful action/receipt field
  forwarding.

Fresh gates with real command exits:

- `rtk go test -mod=readonly -count=1 ./backend/internal/mcp`: 20 passed.
- `rtk go test -mod=readonly -count=1 -race -cover ./backend/internal/mcp ./backend/internal/knowledge ./backend/internal/assistant ./backend/internal/work`: 298 passed.
- MCP race/coverage profile: 20 passed; 91.9% statement coverage.
- `rtk go vet ./backend/internal/mcp ./backend/internal/knowledge`: exit 0.
- `rtk go test -mod=readonly -count=1 ./...`: 388 passed in 16 packages.
- `rtk go vet ./...`: exit 0.
- `rtk gofmt -d` on the full TASK-007 cone: exit 0, clean.
- `rtk git diff --check`: exit 0.
- `scripts/verify_multi_agent_protocol.sh`: `PASS protocol`, `PASS
  entrypoints`, `PASS manifests`.

The proof remains bounded to in-process fake-service `httptest` MCP requests;
no production route, live provider, database, migration, commit, push, merge
or deployment was performed. This repair is pending the same Sol Ultra
reviewer's `WAVE-3-REVIEW-5`; TASK-008 and runtime composition remain frozen.

## Sol review and repair record — WAVE-3-REVIEW-5

Recorded at: 2026-08-26T14:45:32Z

Sol Ultra returned `changes_requested` after independently reviewing the
TASK-007-R1 source and evidence. One HIGH finding remains: malformed UTF-8 in
the raw JSON arguments can be normalized by Go's JSON decoder before the
application's string validators run. The affected boundary covers message,
conversation ID and entity ID arguments and could allow malformed bytes to
reach a dependency.

The required repair is bounded to the existing cone: reject invalid UTF-8 in
`decodeApplicationArguments` before any decoder/object validation, add raw-byte
tests for those three argument classes, and assert `InvalidParams` with zero
dependency calls. Sol also verified the previous constructor, identity,
scope-isolation, ordering/cap and trusted action-data repairs. TASK-007 is
therefore in a same-cone R2 repair state; TASK-008 and runtime composition
remain frozen until `WAVE-3-REVIEW-6`.

## TASK-007-R2 implementation evidence

Recorded at: 2026-08-26T14:49:18Z

The R5 HIGH finding is repaired in the same Wave 3 worktree and existing
TASK-007 file cone:

- `decodeApplicationArguments` now rejects invalid UTF-8 in the raw argument
  bytes before `requireJSONObject` or `json.Decoder` can normalize or inspect
  the payload.
- `TestApplicationRejectsMalformedUTF8BeforeDependencies` sends malformed
  raw bytes in a message, conversation ID and Knowledge entity ID. Every case
  returns `ErrInvalidParams`, and the assistant/Knowledge fake dependency call
  counters remain zero.

Fresh gates with real command exits:

- `rtk go test -mod=readonly -count=1 ./backend/internal/mcp`: 21 passed.
- `rtk go test -mod=readonly -count=1 -race -coverprofile=/tmp/devenglish-wave3-task007-r2-mcp.cover ./backend/internal/mcp`: 21 passed; 92.2% statement coverage.
- `rtk go test -mod=readonly -count=1 -race -cover ./backend/internal/mcp ./backend/internal/knowledge ./backend/internal/assistant ./backend/internal/work`: 299 passed.
- `rtk go vet ./backend/internal/mcp ./backend/internal/knowledge`: exit 0.
- `rtk go test -mod=readonly -count=1 ./...`: 389 passed in 16 packages.
- `rtk go vet ./...`: exit 0.
- `rtk gofmt -d` over the TASK-007 cone: clean; `rtk git diff --check`: exit 0.
- `rtk sh scripts/verify_multi_agent_protocol.sh`: `PASS protocol`, `PASS entrypoints`, `PASS manifests`.

The runtime proof remains bounded to in-process fake-service tests. Production
MCP route composition, live providers, shared database/migrations, commit,
push, merge and deployment remain unverified or unauthorized. This evidence
is ready for exactly one `WAVE-3-REVIEW-6` request to the existing Sol Ultra
reviewer; TASK-008 remains frozen until that verdict.

## WAVE-3-REVIEW-6 attention

Recorded at: 2026-08-26T15:04:06Z

The single Sol Ultra review request was submitted as
`01a03e91-47b9-7cd2-9b87-6a0b2a1ee755`. The reviewer independently started
the governance, source and frozen-cone checks, then the task failed with the
Sol Ultra usage-limit error before returning a terminal verdict. Its last
message is commentary only and does not approve or reject TASK-007-R2.

TASK-007 remains unapproved and TASK-008 remains frozen. The controller does
not create a replacement reviewer or infer a verdict from the green local
gates. Resume only after quota reset, in the same Sol thread, with one fresh
read-only review attempt.

## Sol approval — WAVE-3-REVIEW-7

Recorded at: 2026-08-26T15:13:25Z

After the quota reset, the same Sol Ultra session completed one fresh
read-only review of TASK-007-R2 and returned `approved`. It independently
verified the raw UTF-8 ordering and zero-call malformed-input tests, then
rechecked the prior identity, constructor, isolation, scope, replay, redaction,
resource and read-only boundaries. The worktree-local protocol command's
missing-script exit 127 was reported separately; the canonical governance
script returned exit 0 with all three PASS lines.

TASK-007 is approved for the dependency-ordered TASK-008 UI work. The
production route/database/provider behavior is still outside this approval, and
commit, push, merge and deploy remain unauthorized.
