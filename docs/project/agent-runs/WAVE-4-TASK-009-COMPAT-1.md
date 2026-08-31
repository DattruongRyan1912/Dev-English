# WAVE-4 / TASK-009-COMPAT-1 — legacy user-scoped compatibility guard

## Status

The test-only implementation is complete in the Wave 3 integration worktree
and is ready for the same Sol Ultra reviewer as `WAVE-4-REVIEW-2`. Production
backfill and workspace resolution remain intentionally untouched.

## Scope

The implementation cone is exactly one file:

- `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3/backend/internal/store/postgres_integration_test.go`

The new test is
`TestPostgresLegacyContextCompatibilityAcrossWorkspaceAmbiguity`.
Controller-owned evidence is outside the implementation diff:

- `docs/project/agent-runs/WAVE-4.json`
- `docs/project/agent-runs/WAVE-4-TASK-009-COMPAT-1.md`

No migration, workflow, runtime resolver, HTTP/auth, canonical Knowledge,
Flutter, provider, connector, MCP or cumulative Wave 3 file was changed for
this task. No file was staged, committed, pushed, merged, integrated or
deployed.

## Implemented guard

The test uses synthetic, unique IDs and a bounded context deadline. It creates:

- user A with two active workspaces;
- user B with no workspace;
- one distinct legacy `work_context` row for each user through
  `PostgresStore.SaveWorkContext`;
- one distinct `imported_sources` row for each user through parameterized SQL,
  because that table has no canonical runtime repository.

It then verifies exact title, content, source, domain, identity and timestamp
preservation in each user's scope, and explicitly checks that the other user's
scope sees zero rows. It never selects a workspace, relies on workspace order,
writes `knowledge_sources`/`source_items`/`source_revisions`, defines a
resolver, or calls a provider. Cleanup deletes both synthetic users so the
existing foreign-key cascades remove their test rows.

## Impact and source checks

GitNexus was up-to-date at the parent commit. Impact checks were run with the
`Dev-English-WAVE-3` repository selected:

- `PostgresStore.SaveWorkContext`: low risk; no indexed upstream callers.
- `PostgresStore.AllWorkContexts`: low risk; no indexed upstream callers.
- `WithUser`: high risk with five direct callers and fourteen impacted symbols
  across HTTP/auth/startup paths. It was not modified; the test only consumes
  the existing user-scoping contract.

## Runtime evidence

The test used a newly named disposable Compose project
`devenglish-task009-compat`, healthy on host port 55433. The database password
and connection credential are intentionally omitted here.

| Gate | Result |
| --- | --- |
| Fresh migration runner over `000`–`006` | exit 0; all migrations applied |
| Second migration run | exit 0; all migrations reported already applied |
| Migration count and intentional rollback probe | exit 0; failed migration rolled back and was not recorded |
| Focused compatibility test with race and coverage | exit 0; `RUN` and `PASS` observed |
| Six-package DB-enabled race/coverage run | exit 0; compatibility test discovered automatically |
| Full `go test -mod=readonly -count=1 ./...` | exit 0; 389 passed in 16 packages |
| Full `go test -mod=readonly -count=1 -race ./...` | exit 0; 389 passed in 16 packages |
| `go vet ./...` | exit 0; no issues found |
| `gofmt -l` for the implementation file | exit 0; no output |
| `git diff --check` | exit 0 |

Focused compatibility output:

```text
=== RUN   TestPostgresLegacyContextCompatibilityAcrossWorkspaceAmbiguity
--- PASS: TestPostgresLegacyContextCompatibilityAcrossWorkspaceAmbiguity
coverage: 4.5% of statements
```

The six-package run recorded:

| Package | Coverage |
| --- | ---: |
| `backend/internal/store` | 15.3% |
| `backend/internal/knowledge` | 96.4% |
| `backend/internal/work` | 93.3% |
| `backend/internal/connectors` | 91.4% |
| `backend/internal/mcp` | 92.2% |
| `backend/internal/assistant` | 84.2% |

The pre-existing Knowledge, Work, Connectors and MCP thresholds remain above
90%; store coverage improved from the previously recorded 13.8% to 15.3% in
the full DB-enabled package run.

## Diagnostic correction

The first run of the new test failed at the assertion because PostgreSQL
returned the same timestamp instant with a different timezone location. The
assertion initially compared the complete `time.Time` struct; it was corrected
to compare all scalar fields and `time.Time.Equal`. The rerun passed. This was
a test assertion issue, not a data-isolation failure.

## Migration/backfill boundary

This task adds no migration and does not prove a production snapshot backfill.
The following still require a separate human-approved product/data decision:

- authoritative explicit user-to-workspace mapping;
- behavior for zero, multiple, deleted or restored workspaces;
- deterministic canonical source/item/revision IDs and collision policy;
- whether legacy endpoints remain legacy-only or later dual-write.

The test deliberately encodes none of those decisions.

## Review handoff

Freeze this one-file change for `WAVE-4-REVIEW-2` in the same Sol Ultra
session. Review must inspect the actual source and rerun focused evidence
before any migration, backfill, workspace resolver, HTTP/runtime adapter,
commit, push, merge, integration or deployment. `actionlint`, remote GitHub
Actions, production/shared databases and live providers remain unverified.

## Independent review result

`WAVE-4-REVIEW-2` was completed by the same sole Sol Ultra reviewer in
read-only mode at turn
`01a03fb8-3f2f-7ce0-aae1-09724aee39a6`.

Verdict: `approved`. No blocking, high, medium or actionable-low findings were
reported. Sol independently inspected the source and reran the fresh isolated
migration chain, focused compatibility test, six-package PostgreSQL race/
coverage run and repository gates. The reviewer confirmed the one-file cone,
exact user scoping, bounded cleanup, no workspace selection, and no canonical
Knowledge/backfill/runtime changes.

The task may hand off to later bounded compatibility/backfill planning. A
production backfill or resolver must remain blocked until the authoritative
user-to-workspace mapping and zero/multiple/deleted-workspace policy are
defined. The reviewer removed its disposable Compose database after testing.
