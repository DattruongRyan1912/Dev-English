# WAVE-4 TASK-009-CI-RECOVERY-1

## Status

Implementation is complete and approved in WAVE-4-REVIEW-8 by the same Sol
Ultra reviewer. This bounded slice appends one blocking disposable
PostgreSQL backup/restore recovery job to the existing CI workflow. No
application source, recovery script, migration, Docker Compose file,
provider, credential, integration, commit, push, merge, or deployment was
changed or performed.

## Scope

- Base commit: ea3bd05f92f4209e927cfbc9dd571acc4200ec28
- Implementation worktree: /Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3
- Branch: integration/wave-3
- Exact implementation file: .github/workflows/ci.yml
- Pre-task workflow blob: bed46952fee1b786049ca92ed1c7e095c6fffdc5
- Post-task workflow blob: 498fcb9309ddcf23306e83f7a6bb4550b49db9d6
- Recovery harness: scripts/db_backup_restore_test.sh
- Recovery harness hash/mode: 81d947e270fd75e57717beaf46ec738b9f6c3e73 / 755
- Backup script hash: 0bfbcc1c9ef364577a15b853f4b180d28440afed
- Restore script hash: 8d85da0e4f58044c45203863b7d2e3f4c1fb0bc0
- Migration runner hash: 696cd692082155c20892b41e589202549ef2ae98
- CI Compose file hash: 50a096f90e49ced3c542b522718d8aae25494022

## Implementation

The workflow appends exactly one top-level job named
backup-restore-recovery. It uses only actions/checkout@v4, binds the
required disposable port 55433 and a unique backup-restore project derived
from the run id and attempt, then:

1. validates the recovery, backup, restore and migration scripts with bash -n;
2. checks a generic clean Git status;
3. runs the approved recovery harness once with the approved Compose file,
   project, postgres service, user and database;
4. checks the generic clean Git status again;
5. runs a separate if always cleanup for only that project with
   down --volumes --remove-orphans.

The recovery step has timeout-minutes 20. It does not manually start
PostgreSQL outside the harness. No secret expression, external endpoint,
artifact upload, production Compose file, shared devenglish-ci project, or
provider call was introduced.

## Runtime evidence

| Check | Result |
| --- | --- |
| Ruby YAML and one-job structural assertion | exit 0; exactly one appended job, exact pre-task blob reconstruction, contract and command order |
| Bash syntax for four recovery scripts | exit 0 |
| First local run with required port 55433 | exit 1 at Compose startup because existing WIP container devenglish-task009-compat-postgres-1 already owned host port 55433; harness cleanup ran; no WIP container was stopped or deleted |
| Docker daemon and Compose config read-only checks | exit 0; Docker available and CI Compose config valid |
| Fresh local recovery run with isolated port 55434 | exit 0; source and restore snapshots identical; archive non-empty and mode 600; negative restore guards rejected without mutation; 71/71 schema objects recovered |
| Recovery cleanup inspection | exit 0; no containers, volumes, networks or Compose resources remained for the disposable project |
| Go unit/integration tests | exit 0; 389 passed across 16 packages |
| Go race tests | exit 0; 389 passed across 16 packages |
| Go vet | exit 0; no issues found |
| Git diff check in implementation worktree | exit 0 |
| Manifest JSON and multi-agent protocol verifier | exit 0 |
| Frozen dependency hashes and harness mode | unchanged; recovery harness mode 755 |
| actionlint | unavailable on this host; not claimed |
| GitHub-hosted CI | unverified because no push was authorized |

The local port 55434 run is CI-equivalent lifecycle evidence only; the
workflow contract remains 55433 for a clean GitHub-hosted runner.

## Final review result

The same Sol Ultra reviewer independently inspected the workflow, reconstructed
the pre-task blob after removing only the appended job, verified all five
frozen hashes and executable modes, and ran a fresh recovery cycle on isolated
port 55435. The reviewer returned approved in WAVE-4-REVIEW-8 (turn
01a041f4-9523-7923-82b9-e3c3faee4b86) with no blocking, high, medium, or
actionable-low findings. The review confirmed exact source/restore equality,
mode-600 archive, negative restore guards, 71/71 schema objects, empty
post-cleanup inspections, and all repository gates. actionlint was unavailable
(exit 127); GitHub-hosted execution remains unverified because no push
occurred.

## Review boundary

The task is complete at the review gate. The next handoff is a new bounded
Sol Ultra planning cycle before additional implementation or integration. No
integration, commit, push, merge, or deployment is authorized by this task.
