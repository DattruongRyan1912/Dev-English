# WAVE-4 / TASK-009 — PostgreSQL migration-chain and invariant CI gate

## Status

Implementation is complete in the Wave 3 integration worktree and is waiting
for the single Sol Ultra reviewer to return `approved`, `changes_requested`, or
`blocked`. This is not a release or integration approval.

## Scope and ownership

The implementation cone was exactly:

- `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3/.github/workflows/ci.yml`

The workflow PostgreSQL job now:

- runs the existing migration runner before package tests;
- uses a disposable PostgreSQL service;
- runs race-enabled, uncached, verbose coverage tests for store, Knowledge,
  Work, Connectors, MCP and Assistant;
- writes the coverage profile and test log under `/tmp`;
- asserts that Knowledge, Work, Connectors and MCP each report at least 90%
  coverage;
- keeps the `if: always()` PostgreSQL cleanup step.

No migration, backend, Flutter, provider, connector, MCP runtime, compatibility
or cumulative Wave 3 source file was changed for TASK-009. The existing
uncommitted Wave 3 work remains in place and was not staged, integrated,
committed, pushed, merged or deployed.

## Runtime evidence

All commands below were executed locally from the implementation worktree with
the test database credential redacted in this document.

| Gate | Result |
| --- | --- |
| Disposable Compose PostgreSQL health check | healthy on host port 55432 |
| `scripts/db_migrate_test.sh` | exit 0 |
| Focused PostgreSQL race/coverage command over six packages | exit 0 |
| Coverage threshold assertion over the captured log | exit 0 |
| `go test -mod=readonly -count=1 ./...` | exit 0; 389 passed in 16 packages |
| `go test -mod=readonly -count=1 -race ./...` | exit 0; 389 passed in 16 packages |
| `go vet ./...` | exit 0; no issues found |
| `git diff --check` | exit 0 |
| `scripts/verify_multi_agent_protocol.sh` from canonical checkout | exit 0; protocol, entrypoints and manifests passed |
| Ruby YAML parse of `.github/workflows/ci.yml` | exit 0 |

The focused PostgreSQL run recorded these required tests as executed and
passing:

- `TestPostgresRepositoryIntegration`
- `TestPostgresKnowledgeWorkspaceAndDeferredEvidenceIntegrity`
- `TestPostgresKnowledgeRevisionsAreImmutable`
- `TestPostgresWorkWorkspaceOwnerIntegrity`
- `TestPostgresSafeWriteChallengeAndReceiptScope`

Actual per-package coverage from the captured verbose run:

| Package | Coverage |
| --- | ---: |
| `backend/internal/store` | 13.8% |
| `backend/internal/knowledge` | 96.4% |
| `backend/internal/work` | 93.3% |
| `backend/internal/connectors` | 91.4% |
| `backend/internal/mcp` | 92.2% |
| `backend/internal/assistant` | 84.2% |

The migration runner evidence showed migrations `000` through `006` applying
on a fresh disposable database, a second run reporting every migration as
`already applied`, and the intentional rollback check failing with the
expected division-by-zero error before reporting that idempotency and rollback
checks passed.

## Diagnostic note

An initial local test attempt targeted host port 5432 and reached the native
host PostgreSQL instance, which rejected the disposable test role. The test
was not treated as product evidence. The dedicated Compose database was then
started on host port 55432 and the complete focused command above passed. CI
continues to use the service's normal runner-side port 5432.

## Review limitations and handoff

`actionlint` was not available locally, so GitHub Actions execution was not
claimed. YAML syntax was independently parsed, and the shell/Go commands were
run against the same source change. No live provider, credential, shared
database or external mutation was used.

The frozen handoff was `WAVE-4-REVIEW-1` to the same Sol Ultra reviewer. Sol
Ultra independently inspected the workflow, reran the relevant gates and
returned `approved`: no blocking, high, medium or actionable-low findings.
This approval allows a bounded next-plan handoff only; it does not authorize
migration 007, backfill, runtime wiring, load test, commit, push, merge or
deploy work.

## Independent review — WAVE-4-REVIEW-1

- Reviewer: the same Sol Ultra session
  `01a03c8f-5f2d-7833-ad4c-94557dcca88f`.
- Verdict: `approved`.
- Review turn: `01a03f9f-3edb-73f0-aba2-fcd10eb8900a`.
- Scope result: exactly `.github/workflows/ci.yml`, 45 additions and 2
  deletions; cumulative Wave 3 status entries were not attributed to TASK-009.
- Runtime result: five named PostgreSQL invariants passed independently;
  migration fresh/second-run/rollback checks passed; six-package
  race/coverage, whole-repository tests, race, vet, YAML/JSON and protocol
  gates all passed.
- Review limitations: `actionlint`, remote GitHub Actions, production/shared
  databases and live providers were not exercised.
- Follow-up boundary: do not invent the missing legacy user-to-workspace
  mapping or start migration 007 without a new bounded plan.
