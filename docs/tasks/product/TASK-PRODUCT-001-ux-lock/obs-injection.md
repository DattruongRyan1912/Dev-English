# obs-injection@1

task_id: TASK-PRODUCT-001-ux-lock
generated_at: 2026-08-29T18:56:12+07:00
mode: evidence-only

## Observables

| observable | source | proof |
| --- | --- | --- |
| canonical shell entry | `lib/main.dart:92-219` | production shell widget test and browser smoke |
| four destinations | `test/production_shell_test.dart:92-113` | taps Today, Work, Knowledge and Learning and checks rendered labels |
| loading/empty/error/offline/degraded states | `test/workspace_state_golden_test.dart:119-250` | five deterministic mobile goldens |
| evidence rendering | `test/workspace_golden_test.dart:75-116` | assistant evidence golden and citation surface |
| separate learning data | `lib/src/features/learning/workspace_learning.dart:94-173` | source inspection plus learning-overlay tests |

## Injection decision

No application observability code was injected for this reconciliation artifact. The implementation already exposes the required state through typed controller state and visible UI banners. Adding debug logging would expand the file cone and could leak source/provider content.

## Limits

The artifact records reproducible test commands and source locations. It does not assert that the operator has completed a physical-device U0 review.
