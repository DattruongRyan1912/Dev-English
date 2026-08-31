# Bootstrap capability state — 2026-08-30

## Scope

The canonical `/api/v2/bootstrap` envelope now exposes a stable
`capabilities` object keyed by the compiled V1 module IDs. The HTTP transport
derives the values from the active platform manifest, while the application
copies the map defensively. This keeps optional data projections and module
readiness state in one response without coupling the application package to
the platform manifest type.

Flutter decodes the additive field into `WorkspaceData.capabilities` and Today
renders a compact readiness summary for Work, Knowledge, Assistant and
Learning. Missing capability data remains an empty map for older fixtures.

## Files

- `backend/internal/application/application.go`
- `backend/internal/httpapi/v2.go`
- `lib/src/workspace_models.dart`
- `lib/src/features/today/workspace_today.dart`
- `lib/src/features/workspace/workspace_shared.dart`
- `scripts/production_runtime_smoke.sh`
- focused application/HTTP/Flutter tests and project status logs

## Evidence

- `rtk go test ./backend/internal/application ./backend/internal/httpapi -count=1`
  — exit `0`, `294` tests passed;
- `rtk go test ./backend/... -count=1` and race — exit `0`, `988` tests
  passed in `23` packages for each run;
- `rtk go vet ./backend/...` and backend build — exit `0`;
- `rtk flutter analyze` — exit `0`;
- focused workspace API/UI tests — exit `0`, `28` passed;
- full Flutter suite — exit `0`, `64` passed and `2` environment skips;
- `rtk flutter build web --release --no-wasm-dry-run` — exit `0`;
- production runtime smoke — exit `0`; `19` migrations, authenticated
  bootstrap capability assertion, Work/Knowledge/MCP/auth/CORS/provider
  failure checks all passed. The disposable stack was removed by the script.

## Boundary

This is an additive contract/UI change. It does not enable modules, alter
canonical data, add a migration, or authorize commit, push, merge, deployment,
live-provider billing or human/device acceptance.
