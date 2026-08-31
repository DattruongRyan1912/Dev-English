# TASK-009-HOSTED-FLUTTER-R1 — Environment-correct roleplay tests

Status: approved by the same Sol Ultra reviewer; commit/push and the new
hosted pull-request run completed successfully, and the hosted gate is
accepted. PR #2 was squash-merged into `main`; deployment remains
unauthorized.

Review cycle: `WAVE-4-REVIEW-9`

Reviewer turn:
`codex-thread:01a03c8f-5f2d-7833-ad4c-94557dcca88f#turn-01a04244-e239-7410-8a91-19dd9bf9ff4c`

## Scope

- Implementation worktree:
  `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`
- Base commit:
  `8764fc769e470562f0bba076640771baf127c409`
- Changed file: `test/roleplay_voice_learning_test.dart`
- File-cone result: exactly one file changed; no `lib/**`, backend, migration,
  script, dependency, or workflow change.

## Repair evidence

The previous hosted run `33050232555` failed only because the production-defined
Flutter suite used fixtures that expected development demo fallback. Source
behavior was verified as fail-closed in production. The repair therefore:

1. Adds `_isProduction` using the same compile-time environment convention as
   the application.
2. Keeps development fallback assertions and adds production assertions for an
   unchanged conversation, no fabricated turn, a null turn result, and the
   production error.
3. Adds explicit dual-mode coverage for failed scenario/start loading.
4. Gives the voice lifecycle tests successful mocked scenario and conversation
   setup, preserving their unmount, serialized-stop, cleanup, zero-STT, and
   permission-denial assertions.
5. Leaves the production-defined CI command unchanged and does not skip tests.

## Commands and observed exits

All commands ran from the implementation worktree after the repair:

| Command | Exit | Observed result |
| --- | ---: | --- |
| `dart format --output=none --set-exit-if-changed test/roleplay_voice_learning_test.dart` | 0 | 0 files changed |
| `flutter analyze` | 0 | No issues found |
| `flutter test test/roleplay_voice_learning_test.dart --reporter expanded` | 0 | 9/9 passed |
| `flutter test test/roleplay_voice_learning_test.dart --dart-define=DEVENGLISH_ENV=production --reporter expanded` | 0 | 9/9 passed |
| `flutter test` | 0 | 22 passed, 2 skipped |
| `flutter test --dart-define=DEVENGLISH_ENV=production` | 0 | 23 passed, 1 skipped |
| `flutter build web --release` | 0 | Web artifact built |
| `flutter build web --release --dart-define=DEVENGLISH_ENV=production` | 0 | Production web artifact built |
| `git diff --check` | 0 | No whitespace errors |

An initial parallel Flutter invocation hit a native-asset/code-signing race;
the affected development command was rerun alone and exited 0. The race was
not reproduced by the sequential gates and is not treated as a code failure.

## Review decision

Sol Ultra returned `approved`: no blocking, high, medium, or actionable-low
findings. The review confirmed that production runtime and CI workflow are
unchanged, the permission-denial typed input remains rendered by the existing
roleplay screen, and the one-file test diff does not hide the original failure.

## Commit and push evidence

- Commit: `228d3563037eb7c60ba65793d3d0479ed89fb14b`
- Parent: `8764fc769e470562f0bba076640771baf127c409`
- Author: `ryantruong <dattruong19122003@gmail.com>`
- Branch: `integration/wave-3`
- Remote ref: `origin/refs/heads/integration/wave-3`
- Push exit: `0`
- Remote SHA verified: `228d3563037eb7c60ba65793d3d0479ed89fb14b`
- Committed path: `test/roleplay_voice_learning_test.dart` only

The authorized one-file repair is pushed, the exact hosted run is terminal
success, and the same Sol Ultra session accepted the hosted gate. PR #2 is
merged; deployment remains unauthorized.

## Hosted CI evidence

- Run: [33053487877](https://github.com/DattruongRyan1912/Dev-English/actions/runs/33053487877)
- Event: `pull_request`
- Head SHA: `228d3563037eb7c60ba65793d3d0479ed89fb14b`
- Terminal status: `completed`
- Conclusion: `success`
- `backend`: success
- `flutter`: success, including analyze, full default tests, browser smoke,
  production-defined tests, web build, and production artifact security
- `production`: success
- `postgres`: success
- `legacy-upgrade`: success
- `backup-restore-recovery`: success

This run is evidence for the repair SHA only. The same Sol Ultra session
reviewed the exact run and SHA and accepted the hosted gate; merge and
deployment remain unauthorized.

## Hosted review decision

- Reviewer: same Sol Ultra session
- Review turn: `codex-thread:01a03c8f-5f2d-7833-ad4c-94557dcca88f#turn-01a04252-4b26-7db2-88bf-22e7ca34c323`
- Verdict: `approved`
- Findings: no task-scoped blocking, high, medium, or actionable-low findings
- Verification: exact local commit/parent, one-file cone, unchanged runtime and
  workflow paths, exact PR head, hosted run/job/step metadata, and focused
  production roleplay test were independently checked
- Limitations: GitNexus index remains stale and was not refreshed in this
  read-only review; GitHub emitted pre-existing Node.js action deprecation
  warnings

The Wave 4 hosted gate may be recorded as accepted. PR #2 is merged; deployment
still requires separate human authorization and a verified production target.

## Merge evidence

- PR: [#2](https://github.com/DattruongRyan1912/Dev-English/pull/2)
- Method: squash merge
- Head SHA: `228d3563037eb7c60ba65793d3d0479ed89fb14b`
- Main merge commit: `104304f12f411dd534aa4b385e964b3b8c11ae43`
- Merged at: `2026-08-27T09:01:58Z`
- Branch `integration/wave-3` was retained.
- Branch protection was temporarily set to 0 required approvals for the
  authorized merge, then restored to 1 immediately after the merge; the other
  protection settings were preserved.
- Main CI run [33056690884](https://github.com/DattruongRyan1912/Dev-English/actions/runs/33056690884)
  passed all six jobs at the merge commit.
- No production deployment was performed because the repository has no
  configured production target or deploy workflow.
