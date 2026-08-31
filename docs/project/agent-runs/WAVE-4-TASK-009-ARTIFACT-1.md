# WAVE-4 TASK-009-ARTIFACT-1

## Status

The first WAVE-4-REVIEW-6 returned changes_requested with three findings.
Those findings were repaired in the same one-file cone. The existing Sol
Ultra reviewer then independently re-reviewed the source and runtime
evidence in WAVE-4-REVIEW-6 and approved the slice with no blocking, high,
medium, or actionable-low findings. This bounded slice adds one offline
production web artifact security harness only. No application, test, CI,
Dockerfile, database, migration, provider, integration, commit, push, merge,
or deploy operation was performed.

## Scope

- Base commit: ea3bd05f92f4209e927cfbc9dd571acc4200ec28
- Implementation worktree: /Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3
- Branch: integration/wave-3
- New file: scripts/production_web_artifact_test.sh
- File mode: 755
- File hash after repair: 096f171641974b2532cb0e4b3f9e01988b346fd0
- Existing dependencies inspected without modification:
  Dockerfile.web, lib/src/api.dart, lib/src/app_controller.dart,
  test/runtime_guard_test.dart, and test/production_shell_test.dart.

## Implementation

The executable resolves and verifies its own Flutter repository root, then
builds the Flutter web release inside that root into a mktemp directory with
an isolated environment. Only a fixed synthetic public origin under .invalid
is passed as the API base; no real credential value is read, printed, or
forwarded. The cleanup trap removes the temporary build and
negative-control directories.

The binary-safe scanner checks every regular artifact file. It requires the
synthetic origin and rejects the local development origin, known compile-time
credential marker names, the synthetic negative-control sentinel, and any
.map source-map artifact. A copied artifact with only the synthetic sentinel
appended is required to fail while scanner output remains private.

The harness captures Git status, the complete tracked diff against HEAD, and
content plus mode digests for every non-ignored untracked file before and
after the build. It fails if any of those snapshots changes. Artifact
traversal is captured and checked before scanning, and grep no-match is
distinguished from read/I/O failure so the scanner fails closed. It prints
only file count, a per-run artifact fingerprint, generic negative-control
confirmation, and the unchanged-state confirmation.

## WAVE-4-REVIEW-6 repair record

The first independent review found:

1. Flutter was invoked from the caller's current directory rather than the
   resolved repository root.
2. Repository preservation compared only Git status text, which could miss
   content or mode changes in already modified or untracked paths.
3. Process-substitution traversal and grep read errors were not checked
   distinctly, allowing a possible fail-open scan.

The repair anchors the build in the resolved root, snapshots tracked and
non-ignored untracked content/modes, captures the artifact file list with a
checked find exit code, and treats grep statuses other than no-match as
scan failures.

## Runtime evidence

| Check | Result |
| --- | --- |
| Bash syntax for new harness | exit 0 |
| Post-repair production artifact run from worktree root | exit 0; 38 regular files; fingerprint c6d9c4a30a073597e64ffdab31291303dc4a980b0543c770aac728596d3b7df1 |
| Consecutive post-repair run from /tmp via absolute script path | exit 0; 38 regular files; fingerprint 6f570b4de4933ad120de801ed17f94ef4b21e885a4cbb1e9ca4444165bfa0c7c |
| Negative artifact control | exit 1 inside the harness as expected; synthetic sentinel rejected without content disclosure |
| Source-map control | no .map artifact accepted; production scan passed |
| Temporary cleanup and repository state | both runs exited through the cleanup trap and reported unchanged Git status, tracked diff, and untracked content/mode snapshots |
| Production runtime guard and shell tests | exit 0; 2 tests passed |
| Flutter analyzer | exit 0; no issues found |
| Full Flutter tests | exit 0; 21 passed, 2 skipped |
| Go unit/integration tests | exit 0; 389 passed across 16 packages |
| Go race tests | exit 0; 389 passed across 16 packages |
| Go vet | exit 0; no issues found |
| Diff check | exit 0 |
| Agent-run manifest JSON and protocol verifier | exit 0; protocol, entrypoints and manifests passed |
| Shellcheck | unavailable on this host; not claimed |
| Live providers or credentials | not used |

The fingerprints are recorded as per-run evidence only. They differed between
successful Flutter builds, so this task makes no claim that the generated
artifact bytes are deterministically identical across runs.

## Final review result

The same Sol Ultra reviewer independently verified the repaired source,
mode 755, hash 096f171641974b2532cb0e4b3f9e01988b346fd0, root invocation,
absolute invocation from /tmp, negative sentinel rejection, repository
content/mode snapshots, and the listed regression gates. The reviewer
reported fresh successful invocations with 38 files and approved
TASK-009-ARTIFACT-1 in WAVE-4-REVIEW-6 (turn
01a041c3-624b-7f21-a7e5-3bbde9067985). No blocking, high, medium, or
actionable-low findings remain.

The next handoff is WAVE-4-NEXT-PLAN-6: request one new bounded Sol Ultra
plan before additional implementation or integration. Commit, push, merge,
deployment, CI wiring, runtime/provider work, migration 007, and production
backfill remain outside this task.
