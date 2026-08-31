# Final V1 — four-finding repair evidence

Date: 2026-08-31  
Worktree: /Users/ryantruong/Project/Orther/.worktrees/Dev-English-final-v1  
Branch: integration/final-v1  
Review target: origin/main at 104304f  
Reviewer to re-check: Sol Ultra thread 01a05194-f696-76f2-8629-584fc1a657dd

## Findings addressed

1. Knowledge manual import now uses ManualImportRepository.CommitManualImport
   so the PostgreSQL idempotency reservation and complete source graph commit
   share one transaction. Concurrent requests with the same key create one
   graph; a different payload returns a conflict.
2. Flutter project/task/decision purge now routes through
   createActionChallenge → confirmAction and records the returned receipt.
   Expected-version mismatch is covered.
3. scripts/production_web_artifact_test.sh,
   scripts/db_backup_restore_test.sh and
   scripts/db_legacy_upgrade_test.sh are executable (100755).
4. Migration 019 removes the legacy pending-challenge index before retaining
   the workspace-scoped canonical index. The legacy-upgrade and
   backup/restore harnesses now assert the canonical name and the absence of
   the legacy duplicate.

## Verification

| Check | Result |
| --- | --- |
| rtk bash -n scripts/db_backup_restore_test.sh scripts/db_legacy_upgrade_test.sh scripts/production_web_artifact_test.sh scripts/db_migrate.sh | exit 0 |
| rtk git diff --check | exit 0 |
| rtk go test -race -count=1 ./... | exit 0; 999 tests in 23 packages |
| rtk flutter test --reporter compact | exit 0; 71 passed, 2 environment skips |
| rtk flutter test test/workspace_api_test.dart test/workspace_ui_test.dart --reporter compact | exit 0; 29 passed |
| rtk bash scripts/production_web_artifact_test.sh | exit 0; artifact scan, negative sentinel control and repository-preservation check passed |
| Disposable legacy upgrade harness on PostgreSQL | exit 0; migrations 003..019, preservation, idempotency and canonical-index assertion passed |
| Disposable backup/restore harness on PostgreSQL | exit 0; 71/71 schema objects, mode 600, negative restore guards and exact snapshots passed |
| PostgreSQL-enabled rtk go test -mod=readonly -count=1 -race ./... | exit 0; all packages passed with DEVENGLISH_TEST_DATABASE_URL set to the disposable database on host port 15433 |
| rtk bash scripts/verify_multi_agent_protocol.sh | exit 0; protocol, entrypoints, file-cone, preservation manifest and agent manifests passed |
| rtk bash .cyberos/cuo/gates/run-gates.sh | exit 0; GATES GREEN (machine gates only) |

The first PostgreSQL attempt used host port 5432 and reached an unrelated
host PostgreSQL role, so it failed before exercising the tests. The test was
repeated on isolated port 15433 and passed. All disposable Compose projects
were removed after the runs; no repository files were changed by the tests.

## Remaining boundary

This evidence does not authorize commit, push, merge, deploy, production
migration, or human/device acceptance. Sol Ultra must independently re-review
the current worktree and return a new approved, changes_requested, or blocked
verdict.

## Review observation

The requested re-review used the existing Sol Ultra thread. Turn
01a05425-1133-7f80-a2c3-163f5a952b5a completed without an assistant message;
the reconciliation turn 01a05428-17d1-70b1-94f4-c6a955de0eb3 also completed
with no visible items. No verdict was accepted. The controller therefore
leaves the review state as unknown/HOLD rather than treating an empty response
as approval.
