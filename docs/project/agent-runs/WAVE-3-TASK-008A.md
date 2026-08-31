# WAVE-3-TASK-008A — implementation evidence

## Scope

TASK-008A adds the first bounded voice and learning layer to the roleplay flow:

- push-to-talk recording with microphone permission and failure states;
- editable transcript insertion before the user sends a reply;
- on-demand TTS for assistant messages and the learning starter;
- collapsible Vietnamese help with a short English starter;
- beginner/help-aware roleplay fallback guidance without changing canonical work data.

Implementation was limited to the existing Flutter UI/test cone in the Wave 3
worktree:

- `lib/src/components.dart`
- `lib/src/screens/roleplay_screen.dart`
- `lib/src/screens/workspace_screen.dart`
- `lib/src/app_controller.dart`
- `test/workspace_ui_test.dart`
- `test/roleplay_voice_learning_test.dart`

The cumulative worktree also contains earlier Wave 3 changes. No backend schema,
provider credential, migration, or external mutation was added by this slice.

## Verification

Run in `/Users/ryantruong/Project/Orther/.worktrees/Dev-English-WAVE-3` on
2026-08-26T17:13:56Z:

- `rtk dart format --output=none --set-exit-if-changed lib test` — exit 0;
  35 files, 0 changed.
- `rtk flutter analyze` — exit 0; no issues found.
- `rtk flutter test` — exit 0; 13 passed, 2 skipped by environment.
- `rtk flutter build web --release` — exit 0; `build/web` created.
- `rtk git diff --check` — exit 0.

The focused tests cover the roleplay screen's editable answer, voice controls,
learning help and on-demand TTS affordance, plus the Vietnamese help fallback
when a roleplay turn receives a provider error. The support-card test verifies
the hold/release callback lifecycle without opening a live microphone.

## Browser smoke

The locally served release build was opened at
`http://localhost:8094/?qa=task-008a` through the in-app browser. Visual
interaction verified:

1. Today navigates to Learning.
2. Learning opens the `Need a little help?` overlay and renders Vietnamese
   guidance plus the English starter.
3. Learning opens Roleplay, a scenario opens, and the screen displays the
   editable transcript hint, `Hold to talk`, `Voice: Ready`, assistant `Read
   aloud`, and the learning overlay.
4. The Roleplay learning overlay renders the selected scenario context and goal
   as runtime values rather than literal interpolation placeholders.

The browser check is a UI smoke test only. It does not claim live microphone
permission, provider STT/TTS success, or authenticated backend persistence.

## Review handoff

Implementation is complete and awaiting the single accepted Sol Ultra reviewer
for `WAVE-3-REVIEW-11`. The reviewer must inspect source and test output directly
and decide `approved`, `changes_requested`, or `blocked`. Commit, push, merge and
deploy remain outside this handoff.
