# Acceptance continuation — strict coverage gate and full verification — 2026-08-29

## Scope

- add an executable backend package coverage gate that compares raw Go
  statement counts instead of rounded percentages;
- close the final Work package boundary uncovered by the strict gate;
- refresh the full backend profile and record the implementation evidence
  without changing task lifecycle state or performing release operations.

## Implementation

- Added `scripts/backend_coverage_gate.mjs`.
  It parses a Go `coverprofile`, aggregates statements by backend-relative
  package, requires every requested package to be present, and exits non-zero
  when `covered * 100 < total * threshold`. It also compares the overall raw
  ratio with the recorded R0 manifest when requested.
- Added `scripts/backend_coverage_gate_test.mjs` and wired it into the
  PostgreSQL integration workflow so threshold, baseline, and malformed-input
  behavior are tested before the live profile gate runs.
- Added `docs/project/coverage/2026-08-29-r0-backend-coverage-baseline.json`
  as the machine-readable R0 floor (`4269/6393`), including the baseline
  commit and profile digest.
- Kept the coverage artifact outside `docs/project/agent-runs/` so the
  multi-agent manifest verifier cannot misclassify it as an agent run.
- Added the same gate to
  `.github/workflows/postgres-integrations.yml` after the disposable
  PostgreSQL migrations and focused integration checks. The workflow also
  enforces the R0 floor and uploads the generated profile as an artifact.
- Added `backend/internal/work/service_boundary_test.go` to verify nil
  repository rejection, nil option handling and the default clock path.
  This raised Work from a rounded-but-failing `1580/1756` to a strict passing
  `1581/1756`.

## Verified commands

| Check | Result |
| --- | --- |
| PostgreSQL-enabled full Go suite | exit `0`; `1016` tests across `23` packages |
| PostgreSQL-enabled full Go race suite | exit `0`; `1016` tests across `23` packages |
| Full backend coverage profile | exit `0`; `77.45%` (`9107/11759`) statements |
| Raw package coverage gate | exit `0`; Application `700/773`, Assistant `274/295`, Connectors `1110/1232`, HTTP API `1138/1260`, Knowledge `833/923`, MCP `1351/1482`, Work `1581/1756` — all `PASS` at `>=90%` |
| R0 total coverage floor | exit `0`; current `9107/11759 = 77.45%` is above baseline `4269/6393 = 66.78%` using the JSON manifest |
| Coverage checker syntax | exit `0`; `node --check scripts/backend_coverage_gate.mjs` |
| Go quality | `gofmt`, `go vet`, `go build` and `git diff --check` — exit `0` after the final Work boundary test |
| Frontend gates | latest verified analyze exit `0`, Flutter `49` passed with `2` environment skips, Chrome route smoke exit `0`, web release and Android debug builds exit `0` |
| Runtime smoke | production-shaped Docker smoke exit `0`; migrations `19`, Work/MCP/Knowledge/auth/CORS/provider-failure/history checks passed |
| Migration/scale | disposable legacy upgrade exit `0`; Knowledge capacity exit `0` with `100000` chunks and bounded pagination |
| MCP | official Go SDK conformance test exit `0`; multi-agent protocol verifier exit `0` |
| Coverage-gate regression | `node --test scripts/backend_coverage_gate_test.mjs` — `4` tests passed; malformed profiles/manifests fail closed |
| CyberOS machine gates | `.cyberos/cuo/gates/run-gates.sh` exit `0`; build, lint, test and coverage passed; doctor skipped because the local CyberOS memory CLI is unavailable |
| Browser smoke | `http://localhost:8093/` rendered Today; Work/Knowledge/Learning navigation and canonical Knowledge search passed with no captured console errors/warnings; text assistant returned evidence plus explicit unknown boundary |

## Acceptance boundary

- The raw package threshold and the recorded R0 total floor are now technically
  passing. The machine gate is present in the PostgreSQL integration workflow,
  but GitHub CI has not been run from this local checkout.
- The R0 baseline is now recorded in
  `docs/project/agent-runs/2026-08-29-r0-baseline.md`: detached baseline
  commit `104304f12f411dd534aa4b385e964b3b8c11ae43`, PostgreSQL-enabled Go
  `395` tests, raw `4269/6393 = 66.78%`. The current `77.45%`
  (`9107/11759`) is above that floor; this still must not be described as a
  release approval.
- Operator/device U0, human lifecycle acceptance, an independent current
  reviewer verdict, provider/billing acceptance and human authorization for
  commit/push/merge/deploy remain open.
- `verify_cyberos_only.sh` still reports the protected pre-existing
  `.github/workflows/ci.yml` WIP snapshot; that file was preserved.
- No task status, commit, push, merge or deployment was performed in this
  continuation.
