# WAVE-4 / TASK-009-CI-UPGRADE-1 — CI gate for the legacy-schema upgrade harness

## Status

The bounded CI implementation is complete in the Wave 3 integration worktree
and is ready for `WAVE-4-REVIEW-4` by the same sole Sol Ultra reviewer. The
task adds one `legacy-upgrade` job only; the approved harness, migrations,
application source and existing CI jobs remain unchanged.

No file was staged, committed, pushed, merged, integrated or deployed.

## Scope and frozen inputs

Implementation worktree:

- `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`

Exact implementation file:

- `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3/.github/workflows/ci.yml`

The task-specific patch appends one `legacy-upgrade` job at lines 169–257.
The pre-task workflow blob hash was `e92c7fa43eef9be057cfcf772addcd1d5e027a3a`;
the post-task blob hash is `567784d41978648a13e041d55abffd25b8ff819f`. The
approved harness remains executable mode `100755` with blob hash
`25a717d633ef1f1fa18cc7b323e75170068d5ed3`.

Forbidden changes were preserved: no migration, script, Compose file,
backend, Flutter, test, provider, workflow-trigger or existing-job changes
were made for this task. Cumulative earlier Wave 3/Wave 4 WIP remains in the
worktree and was not cleaned up.

## Implemented CI job

The job:

1. Checks out the repository with `actions/checkout@v4` and runs `bash -n` on
   both the approved harness and the existing migration runner before starting
   PostgreSQL.
2. Uses a non-default host port (`55432`) and two sequential project names
   derived from numeric `${{ github.run_id }}` and `${{ github.run_attempt }}`:
   `devenglish-legacy-upgrade-<run_id>-<run_attempt>-positive` and
   `devenglish-legacy-upgrade-<run_id>-<run_attempt>-negative`.
3. Starts and health-checks the positive disposable database, runs the
   harness with explicit disposable opt-in, and requires the normal harness
   exit path.
4. Stops the positive project before starting the separately named negative
   project, pre-applies migrations `000..006` through the existing runner, and
   runs the harness in explicit negative-control mode.
5. Requires the exact negative success line and rejects any seed marker.
6. Uses an unconditional cleanup step to remove both explicitly named
   projects, volumes and orphans, including partial-start and failed-test
   paths.

No provider is called and no credential or source secret is introduced.

## Runtime and static evidence

The local sequence used the exact numeric naming shape with host port `55440`
to avoid local collisions. The harness itself also cleaned each project; the
final resource check found no matching containers, volumes or networks.

| Gate | Result |
| --- | --- |
| Ruby YAML parse of `.github/workflows/ci.yml` | exit 0 |
| Positive CI-equivalent sequence | exit 0; fresh `000..002` and `003..006`, idempotent Phase 2, snapshots preserved |
| Negative CI-equivalent sequence | exit 0; pre-migrated DB rejected before seed |
| Exact negative success-line assertion | passed |
| Negative seed-marker absence assertion | passed |
| `bash -n scripts/db_legacy_upgrade_test.sh scripts/db_migrate.sh` | exit 0 |
| Existing `db_migrate_test.sh` on separate disposable DB | exit 0; fresh, idempotent and rollback checks passed |
| `go test -mod=readonly -count=1 ./...` | exit 0; 389 passed in 16 packages |
| `go test -mod=readonly -count=1 -race ./...` | exit 0; 389 passed in 16 packages |
| `go vet ./...` | exit 0; no issues |
| `git diff --check` | exit 0 |
| multi-agent protocol verifier and manifest JSON validation | exit 0 |
| `actionlint` | unavailable; unverified |
| GitHub-hosted runner execution | not run; unverified |

Positive local snapshots remained:

```text
Legacy snapshot before upgrade: imported_sources|2|197050571dbd335cae73c3ab819e41d0
work_context|2|3ffcc75483c7333a930469088058224b
Legacy snapshot after upgrade: imported_sources|2|197050571dbd335cae73c3ab819e41d0
work_context|2|3ffcc75483c7333a930469088058224b
```

The negative run reported the exact line:

```text
Negative gate passed: pre-migrated disposable database was rejected before seed.
```

## Boundary and review handoff

This task proves the local command sequence and statically adds the CI gate;
it does not claim GitHub-hosted execution, production backfill, workspace
mapping, migration 007, runtime compatibility or provider behavior.

Freeze the one-file patch for `WAVE-4-REVIEW-4` in the same Sol Ultra session.
Review must inspect the actual workflow and rerun the positive and negative
disposable sequences before any further planning or integration. Commit, push,
merge and deploy remain human-only.
