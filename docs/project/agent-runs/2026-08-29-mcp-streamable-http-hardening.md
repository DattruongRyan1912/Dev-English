# MCP Streamable HTTP negotiation hardening — 2026-08-29

## Scope

This bounded continuation hardened the `/mcp` HTTP adapter without changing
application semantics or the Flutter client. The adapter now enforces the
required JSON/SSE `Accept` negotiation on POST, validates an optional
`MCP-Protocol-Version` header against supported versions, and returns `202`
with no body for notifications. GET remains `405 Allow: POST` because the
server has no unsolicited SSE stream to publish.

## Files

- `backend/internal/mcp/protocol.go`
- `backend/internal/mcp/mcp_test.go`
- `backend/internal/mcp/application_test.go`
- `backend/internal/httpapi/actions_test.go`
- `scripts/production_runtime_smoke.sh`

## Verification evidence

- MCP focused tests: `61` passed.
- HTTP API + MCP focused tests: `100` passed across 2 packages.
- Full Go tests: `544` passed across 23 packages.
- Full Go race tests: `544` passed across 23 packages.
- `go vet`, `go build` and `git diff --check`: exit `0`.
- Production-shaped Docker smoke: exit `0`; `19` migrations and all existing
  Work/MCP/Knowledge/auth/CORS/provider-fail-closed checks passed.

## Release boundary

This is implementation evidence, not an approval. Independent MCP client
conformance, strict 90% coverage, real-device/operator U0, lifecycle
acceptance and human authorization for Git/release operations remain open.
No commit, push, merge or deployment was performed.
