# WAVE-4 / TASK-009-UPGRADE-1 — legacy-schema upgrade preservation harness

## Status

The one-file implementation is complete in the Wave 3 integration worktree
and was approved by the second `WAVE-4-REVIEW-3` pass from the same sole Sol
Ultra reviewer. The first review found and rejected a high-severity
false-positive path; the repair is limited to this script.
Production backfill, workspace resolution and runtime compatibility remain
intentionally untouched.

## Scope

The implementation cone is exactly one new executable script:

- `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3/scripts/db_legacy_upgrade_test.sh`

The script uses temporary symlinked migration directories and delegates schema
application to the existing `scripts/db_migrate.sh`; it does not change that
runner or any migration file. Controller-owned evidence is outside the
implementation diff:

- `docs/project/agent-runs/WAVE-4.json`
- `docs/project/agent-runs/WAVE-4-TASK-009-UPGRADE-1.md`

No file was staged, committed, pushed, merged, integrated or deployed.

## Implemented harness

The harness is fail-closed and requires:

- `DEVENGLISH_LEGACY_UPGRADE_DISPOSABLE=1`;
- the repository `infra/docker-compose.ci.yml` only;
- a uniquely prefixed `devenglish-legacy-upgrade-*` Compose project;
- the expected `postgres`/`devenglish` disposable service and database.

It rejects missing configuration, unsafe project names and non-CI Compose
configuration before connecting. It cleans only the explicitly named
disposable project and its own temporary directory.

Before the first migration or seed, it requires the disposable database to
have no user relations. A dedicated
`DEVENGLISH_LEGACY_UPGRADE_NEGATIVE_CONTROL=1` mode is used against a
pre-migrated disposable database and must reject it before any seed. Migration
assertions match an exact full output line, so `already applied ...` cannot
satisfy an `applied ...` first-run assertion.

The run is split into two migration views made from symlinks:

1. Apply `000_schema_migrations.sql` through `002_provider_secrets.sql`.
2. Seed two fixed synthetic users and representative `work_context` and
   `imported_sources` rows using parameterized `psql` variables.
3. Capture row counts and stable checksums containing IDs, user IDs, source
   fields, content and timestamps.
4. Apply `003_platform_foundation.sql` through `006_work.sql`.
5. Run the second phase again and require every migration to report
   `already applied`.
6. Add two active workspaces for user A and none for user B without selecting
   or inferring a legacy workspace mapping.
7. Compare the legacy snapshot, verify the exact schema version list, confirm
   canonical Knowledge tables remain empty, and clean up the disposable DB.

## Runtime evidence

The successful controller harness run used the fresh disposable Compose
project `devenglish-legacy-upgrade-20260827-repair1` on an isolated host port.
The negative control used the separately named pre-migrated project
`devenglish-legacy-upgrade-20260827-negative1`. The reviewer independently
reran fresh positive and pre-migrated negative projects. Database credentials
are intentionally omitted.

| Gate | Result |
| --- | --- |
| `db_legacy_upgrade_test.sh` fresh two-phase harness | exit 0 |
| Phase 1 migration application (`000`–`002`) | applied |
| Phase 2 migration application (`003`–`006`) | applied |
| Phase 2 second run | all four reported already applied |
| Pre-migrated negative control before seed | rejected; harness exit 0 in explicit negative-control mode |
| First-run output matching | exact full-line `applied <migration>` guard |
| Initial database blankness guard | fresh run accepted; pre-migrated run rejected before seed |
| Legacy snapshot before/after | identical counts and checksums |
| Ambiguous topology | user A has 2 active workspaces; user B has 0 |
| Canonical Knowledge row check | `knowledge_sources`, `source_items` and `source_revisions` all empty |
| Existing `db_migrate_test.sh` on separate fresh disposable DB | exit 0; idempotency and rollback passed |
| `go test -mod=readonly -count=1 ./...` | exit 0; 389 passed in 16 packages |
| `go test -mod=readonly -count=1 -race ./...` | exit 0; 389 passed in 16 packages |
| `go vet ./...` | exit 0; no issues |
| `bash -n scripts/db_legacy_upgrade_test.sh` | exit 0 |
| `git diff --check` | exit 0 |
| multi-agent protocol verifier | exit 0; protocol, entrypoints and manifests passed |
| Sol Ultra `WAVE-4-REVIEW-3` re-review | approved; no blocking, high, medium or actionable-low findings |

Successful harness snapshot output:

```text
Legacy snapshot before upgrade: imported_sources|2|197050571dbd335cae73c3ab819e41d0
work_context|2|3ffcc75483c7333a930469088058224b
Legacy snapshot after upgrade: imported_sources|2|197050571dbd335cae73c3ab819e41d0
work_context|2|3ffcc75483c7333a930469088058224b
Legacy schema upgrade preservation and isolation checks passed.
```

Safety checks also passed: missing opt-in/configuration, an unsafe project
name and a non-CI database were rejected with exit 2 before database work.
`shellcheck` is unverified because the executable is not installed on this
machine.

## Diagnostic correction

The first harness attempt reached Phase 1 and seeded rows, then exposed that
`psql` variable substitution was not applied when the snapshot SQL was passed
through `-c`. The script was corrected to send variable-bearing queries over
stdin through `psql_query`; the failed disposable project was cleaned, and a
fresh retry passed all checks. This was a harness transport issue, not a
migration or data-preservation failure.

The first `WAVE-4-REVIEW-3` pass then demonstrated a separate false positive:
on a pre-migrated database, `already applied 003...` contained the substring
`applied 003...`, so the harness incorrectly reported success without running
either migration phase. The repair adds the blank-database preflight, exact
full-line output matching and the explicit negative-control mode. A fresh
positive run and a pre-migrated negative run now both produce the expected
outcome.

The same Sol Ultra re-review confirmed the repaired control flow at lines 134,
147 and 201, reran both database modes and all repository gates, and reported
the executable hash as
`25a717d633ef1f1fa18cc7b323e75170068d5ed3`. All disposable projects,
containers, volumes and networks from the review were removed.

## Boundary and review handoff

This task proves an additive upgrade path for the current migrations and
legacy-table preservation on a fresh disposable database. It does not prove a
production snapshot backfill, old-binary rollback, workspace mapping,
resolver, runtime adapter, CI execution on GitHub, or provider behavior.

The `WAVE-4-REVIEW-3` re-review is approved and this task may hand off to the
next bounded CI planning gate. Do not infer production snapshot backfill,
old-binary rollback, runtime compatibility, migration 007 or workspace
resolver work from this approval. Commit, push, merge and deploy remain
human-only.
