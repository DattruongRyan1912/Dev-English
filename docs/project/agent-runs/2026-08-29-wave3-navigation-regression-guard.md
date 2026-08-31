# Wave 3 authenticated navigation regression guard — 2026-08-29

## Context

The historical `WAVE-3-REVIEW-9` finding reported that the authenticated
production path could replace the canonical workspace shell with a single
legacy `HomeScreen`. The current source already routes an authenticated,
canonical workspace through `WorkspaceShell`; this run adds the missing
production-test assertion so the regression cannot be hidden by a Home-only
check.

## Change

`test/production_shell_test.dart` now uses the authenticated production
fixture to navigate and assert the complete canonical sequence:

```text
Today → Work → Knowledge → Learning → Today
```

The assertions check each destination's surface-specific content (`Projects`,
`Connected sources`, `Choose a focused practice` and `Ask your assistant`).
The test does not change the CyberOS lifecycle or treat the historical review
as a new independent approval.

## Verification

- `rtk dart format --output=none --set-exit-if-changed test/production_shell_test.dart` — exit `0`; no changes required.
- `rtk flutter test --dart-define=DEVENGLISH_ENV=production test/production_shell_test.dart` — exit `0`; `2` tests passed.
- `rtk flutter test test/workspace_golden_test.dart test/browser_smoke_test.dart test/widget_test.dart` — exit `0`; `9` tests passed and `1` environment skip.
- `rtk git diff --check` — exit `0`.

## Boundary

This closes the missing local regression assertion for the historical
navigation finding. It does not constitute the required same-thread Sol
re-review, real-device U0 acceptance, lifecycle acceptance or release
authorization. No commit, push, merge or deployment was performed.
