# HTTP boundary and coverage refresh — 2026-08-29

## Scope

This continuation added focused regression coverage for the v2 HTTP method and
error contracts, generic credential redaction, assistant validation boundaries,
application wrapper propagation, MCP token-revoke behavior, connector provider
boundaries, workspace/user isolation, query normalization and pre-provider
input rejection. It did not alter
the canonical Work, Knowledge, connector, assistant-action or MCP semantics.

## Source changes

- `backend/internal/httpapi/server.go` now redacts credential-shaped values in
  generic serialized errors through the shared connector redactor.
- `backend/internal/httpapi/v2_error_contract_test.go` covers method/`Allow`
  headers, product/connector status mapping and redaction.
- `backend/internal/httpapi/v2_isolation_and_connector_boundary_test.go` covers
  workspace/user isolation, trash query flags, bounded query parsing and
  invalid connector input being rejected before provider I/O.
- `backend/internal/assistant/grounding_test.go`,
  `backend/internal/application/application_test.go`,
  `backend/internal/ai/provider_http_test.go` and
  `backend/internal/mcp/application_boundary_extra_test.go` add bounded
  behavior tests for validation, context, provider usage and token boundaries.
- `backend/internal/knowledge/embedding_test.go`,
  `backend/internal/work/postgres_error_test.go` and
  `backend/internal/connectors/pure_boundaries_test.go` add environment,
  database-error and connector-constructor/safe-write boundary coverage.

## Verification evidence

| Check | Result |
| --- | --- |
| HTTP focused tests | 57 passed, exit 0 |
| Go unit suite | 606 passed across 23 packages, exit 0 |
| Go race suite | 606 passed across 23 packages, exit 0 |
| Go vet/build/diff check | exit 0 |
| DB-enabled suite | 620 passed, exit 0 |
| DB-enabled total coverage | 70.8% statements |
| Package coverage | application 83.4%; assistant 92.9%; knowledge 82.6%; MCP 84.1%; work 85.5%; connectors 78.2%; HTTP API 64.1% |
| Production-shaped Docker smoke | exit 0; 19 migrations and Work/MCP/Knowledge/auth/CORS/provider-failure/history checks passed |
| Multi-agent protocol verifier | exit 0 |
| CyberOS-only verifier | exit 1 at the protected pre-existing `.github/workflows/ci.yml` WIP snapshot check |

Coverage profile: `/tmp/devenglish-db-enabled-20260829-final.cov`.

## Boundary

The strict 90% coverage gate is still open. Flutter full-suite compiler
startup, real-device/operator U0 acceptance, lifecycle acceptance, independent
review and human Git/release authorization are also open. This evidence does
not authorize commit, push, merge or deployment. The protected WIP finding was
preserved and not reverted.
