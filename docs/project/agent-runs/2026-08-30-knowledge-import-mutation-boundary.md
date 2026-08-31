# Knowledge import mutation boundary — 2026-08-30

## Scope

Manual Knowledge import is a write operation. The REST endpoint now requires
the same `Idempotency-Key` header already required by the MCP
`knowledge_import_manual` mutation, and the Flutter production adapter creates
and forwards a request key from the workspace controller.

The application keeps its existing deterministic source/revision/chunk
identity. That makes an identical retry converge on the same immutable graph,
but this slice does not add a separate durable replay table or claim
cross-payload key-conflict detection for Knowledge imports.

## Files

- `backend/internal/httpapi/v2.go`
- `backend/internal/httpapi/v2_boundary_test.go`
- `backend/internal/httpapi/v2_contract_test.go`
- `backend/internal/httpapi/v2_isolation_and_connector_boundary_test.go`
- `backend/internal/httpapi/v2_scope_failure_test.go`
- `backend/internal/httpapi/server_test.go`
- `lib/src/api.dart`
- `lib/src/workspace_api.dart`
- `lib/src/workspace_controller.dart`
- `test/workspace_api_test.dart`
- `test/workspace_ui_test.dart`
- `scripts/production_runtime_smoke.sh`

## Evidence

- HTTP API focused suite: exit `0`, `235` tests passed;
- Flutter workspace API/UI suite: exit `0`, `28` tests passed;
- Flutter analyze: exit `0`;
- smoke script syntax check: exit `0`.
- Full backend suite: exit `0`, `989` tests passed in `23` packages;
- Full backend race suite: exit `0`, `989` tests passed in `23` packages;
- `go vet` and server build: exit `0`;
- Full Flutter suite: exit `0`, `64` tests passed and `2` environment skips;
- Flutter web release build: exit `0`;
- Production-shaped smoke: exit `0`; authenticated bootstrap capability
  assertion, `19` migrations, Work conflict/history, MCP revoke/replay,
  Knowledge import/search, CORS and provider fail-closed checks passed.

## Boundary

This is a transport/client contract hardening slice. It does not add a
Knowledge replay migration, alter canonical data, change MCP auth, or
authorize commit, push, merge, deployment, live-provider billing or human
acceptance.
