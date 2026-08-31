# U0 — Token and component state sheet

## Tokens

| Token | Value | Use |
| --- | --- | --- |
| background | #F7F8FA | Application canvas |
| surface | #FFFFFF | One meaningful grouped surface |
| textPrimary | #111827 | Work/canonical content |
| textSecondary | #6B7280 | Supporting copy |
| accent | #2563EB | Primary navigation and action |
| success | #16A34A | Verified/live/succeeded |
| warning | #D97706 | Attention/stale/priority |
| error | #DC2626 | Failed/conflict/blocked |
| space | 4, 8, 12, 16, 20, 24, 32, 40 | Consistent rhythm |
| radius | 8, 12, 16, pill | Hierarchy, not decoration |

## Component states

| Component | Required states |
| --- | --- |
| App shell | loading, canonical, preview, degraded, unauthenticated |
| Button | enabled, pressed, disabled, busy, success, failure |
| Text field | empty, focused, invalid, submitting, server error |
| Entity row | normal, selected, stale, trashed, conflict, unavailable |
| Assistant answer | grounded, inferred, unknown, stale evidence, provider unavailable |
| Citation | verified excerpt, missing excerpt, stale, locator unavailable |
| Sync control | idle, running, succeeded, partial, rate-limited, failed |
| Action challenge | preview, expired, confirmed, replay receipt, rejected |
| Transcript | recording, processing, editable, empty, STT error |

## Accessibility rules

- Interactive target tối thiểu 44×44 logical pixels.
- Không truyền trạng thái chỉ bằng màu; luôn có label/icon/copy.
- Focus order theo dòng đọc; desktop keyboard phải tới được composer, menu và
  confirmation.
- Contrast body text đạt WCAG AA; test ở light theme và browser zoom 200%.
- Long title/excerpt wrap được, không overflow ngang ở 390px.
