# WAVE-3-TASK-008-R2 — Repair evidence

## Status

`ready_for_review`

The main controller completed the single HIGH repair requested by `WAVE-3-REVIEW-9`. The authenticated production path now restores the complete legacy Home/Practice/Review/Progress shell, and the production test navigates through all four destinations. The change remains limited to the existing UI/test worktree. No commit, push, merge or deployment was performed.

## Changed file cone

Worktree: `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3`

Base: `ea3bd05f92f4209e927cfbc9dd571acc4200ec28`

R2 source/test changes:

- `lib/main.dart`: extracted the legacy authenticated shell with `IndexedStack` and `NavigationBar` for Home, Practice, Review and Progress.
- `test/production_shell_test.dart`: successful authenticated production test now selects and asserts Practice, Review, Progress and Home, and still asserts no demo workspace/preview UI.

All earlier R8/R1 repairs remain in the same `lib/` and `test/` cone. Pre-existing backend WIP is untouched and out of scope.

## R9 finding addressed

R9 identified that the authenticated production branch returned only `HomeScreen`, dropping direct access to the existing Practice, Review and Progress destinations. R2 replaces that bare return with the complete legacy shell and verifies the four navigation destinations under `DEVENGLISH_ENV=production`.

## Verification evidence

Observed after the repair on `2026-08-26`:

| Gate | Result |
| --- | --- |
| `rtk git diff --check` | exit 0 |
| `rtk dart format --output=none --set-exit-if-changed lib test` | exit 0; 34 files, 0 changed |
| `rtk flutter analyze` | exit 0; no issues found |
| `rtk flutter test --dart-define=DEVENGLISH_ENV=production test/production_shell_test.dart` | exit 0; 1 passed |
| `rtk flutter test` | exit 0; 10 passed, 2 skipped |
| `rtk flutter build web --release --dart-define=DEVENGLISH_ENV=production` | exit 0 |
| `rtk flutter build web --release` | exit 0 |
| rtk sh scripts/verify_multi_agent_protocol.sh from canonical repo | exit 0; protocol, entrypoints and manifests passed for the final R2 state |

## Browser runtime evidence

Local static server: `http://localhost:8094/`, serving the final default release build.

Verified in the Codex in-app browser at `http://localhost:8094/?qa=task-008-r2-final`:

- Today renders the read-only `Local preview` boundary.
- An unsupported quick prompt produces an evidence-free `Unknown` response.
- `Show my current tasks` produces the matching task evidence card with `Demo preview` provenance.
- Browser console contained 0 error and 0 warning entries during the interaction.

The production navigation branch is verified by the production widget test because the local browser runs the development preview and has no authenticated backend session.

## Known limitations

- The demo gateway is deterministic preview data; no live provider, REST composition or MCP runtime is claimed by TASK-008.
- Conversation retention is session/controller-lifetime only; durable reload/database persistence remains outside TASK-008.
- Push-to-talk, editable transcript, TTS and learning overlay remain `TASK-008A`, blocked until approval.

## Handoff

Submit exactly one same-thread Sol Ultra re-review as `WAVE-3-REVIEW-10`. The reviewer must inspect the source and fresh outputs, verify all four production destinations, and return `approved`, `changes_requested` or `blocked`. No replacement reviewer or additional worker is allowed.
