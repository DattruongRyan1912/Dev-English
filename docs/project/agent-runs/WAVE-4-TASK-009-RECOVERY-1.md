# WAVE-4 TASK-009-RECOVERY-1

## Status

Approved in WAVE-4-REVIEW-5 by the existing Sol Ultra reviewer. This is a
disposable local recovery harness only. No migration, CI workflow, application
code, provider configuration, production backfill, commit, push, merge, or
deploy was performed.

## Scope

- Base commit: ea3bd05f92f4209e927cfbc9dd571acc4200ec28
- Implementation worktree: /Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3
- Branch: integration/wave-3
- New file: scripts/db_backup_restore_test.sh
- File hash after implementation: 81d947e270fd75e57717beaf46ec738b9f6c3e73
- Existing dependencies inspected and invoked without modification:
  scripts/db_backup.sh, scripts/db_restore.sh, scripts/db_migrate.sh,
  infra/docker-compose.ci.yml, and migrations 000 through 006.

## Implementation

The executable accepts an explicit disposable opt-in and rejects unsafe
configuration before Docker or PostgreSQL mutation. It requires the repository
CI Compose file, a unique project name matching
devenglish-backup-restore-*, the exact postgres service/user/database
postgres/devenglish/devenglish, and an explicit non-5432 port.

The positive path starts an isolated Compose project, waits for PostgreSQL,
checks that the source database is blank, applies exactly migrations 000
through 006 through the existing migration runner, and seeds deterministic
synthetic data:

- legacy work_context and imported_sources rows for users A and B;
- two workspaces for A, no workspace for B, and one workspace for C;
- Knowledge source, item, revision, chunk, topic, canonical claim, and
  evidence records;
- Work project, task, decision, history, and idempotency records;
- a consumed action challenge and completed action receipt.

It then calls the existing db_backup.sh script, verifies a non-empty
custom-format archive with mode 600, creates a disposable restore database, and
calls the existing db_restore.sh script. Missing confirmation and a wrong
restore target are checked before mutation. Source and restored snapshots, data
invariants, required relations, indexes, and triggers are compared. Migrations
are rerun on the restored database and must report every version as already
applied without changing the snapshot. The cleanup trap removes the restore
database and the named Compose project resources.

## Runtime evidence

| Check | Result |
| --- | --- |
| Bash syntax for recovery harness and frozen scripts | exit 0 |
| Positive disposable round-trip | exit 0 |
| Positive project and isolated port | devenglish-backup-restore-20260827-3 / 55446 |
| Migration set | all versions 000 through 006 applied exactly once |
| Backup archive | non-empty custom format; file mode 600 |
| Negative missing-confirmation preflight | exact destructive-action guard; exit 1; no target mutation |
| Negative wrong-target preflight | exact destructive-action guard; exit 1; no target mutation |
| Source/restore deterministic snapshot groups | 19 groups identical |
| Required schema objects | 71/71 present, including required indexes and triggers |
| Restore migration rerun | all seven versions already applied; snapshot unchanged |
| Existing migration regression test | exit 0 |
| Go unit/integration tests | 389 passed across 16 packages |
| Go race tests | 389 passed across 16 packages |
| Go vet | exit 0 |
| Diff check | exit 0 |
| Agent-run manifest JSON and protocol verifier | exit 0; protocol, entrypoints and manifests passed |
| Pre-Docker negative configuration checks | four cases rejected with exit 2 |
| Shellcheck | unavailable on this host; not claimed |
| Live providers or credentials | not used |
| Independent Sol Ultra re-review | approved; fresh project devenglish-backup-restore-review5-rereview / port 55447; 19 groups and 71/71 objects |

The positive run ended with:

Recovery round-trip passed: 71/71 schema objects and exact source/restore
snapshots.

The run also verified that the exact disposable Compose project left no
containers or volumes after cleanup.

## Review result

The same Sol Ultra reviewer independently inspected the source and confirmed
the exact restore guard text and exit 1, canonical accepted receipt status,
19 snapshot groups, frozen dependency hashes, one-file scope, cleanup, and all
listed gates. Verdict: approved with no blocking, high, medium, or actionable
low findings. Approval is limited to TASK-009-RECOVERY-1 and does not
authorize migration 007, production backfill, CI wiring, application changes,
providers, integration, commit, push, merge, or deployment.
