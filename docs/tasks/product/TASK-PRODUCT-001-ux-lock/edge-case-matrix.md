# edge-case-matrix@1

task_id: TASK-PRODUCT-001-ux-lock
generated_at: 2026-08-29T18:56:12+07:00
total_rows: 10

| id | category | trigger | expected | severity | planned_test |
| --- | --- | --- | --- | --- | --- |
| ECM-001 | NULL_INPUT | Authenticated bootstrap returns no workspace payload. | Production shell shows an explicit unavailable state and does not silently select demo data. | critical | `test/production_shell_test.dart::new shell does not require legacy learning endpoints` |
| ECM-002 | BOUNDARY | View width is the 390x844 mobile golden viewport. | Navigation, evidence text and action controls remain visible without overflow. | high | `test/workspace_state_golden_test.dart::loading state golden` |
| ECM-003 | BOUNDARY | View is rendered at a wider desktop breakpoint. | The same four canonical surfaces remain reachable and content hierarchy stays stable. | medium | `test/workspace_golden_test.dart::workspace surfaces match ${entry.key} goldens` |
| ECM-004 | MALFORMED | Workspace API returns a structured provider error. | Error banner is visible, raw provider details are not exposed, and retry remains explicit. | high | `test/workspace_state_golden_test.dart::provider error state golden` |
| ECM-005 | MALFORMED | Bootstrap response contains no projects, tasks or decisions. | Work presents a deterministic empty state rather than placeholder facts. | medium | `test/workspace_state_golden_test.dart::empty state golden` |
| ECM-006 | DEGRADATION | Embedding retrieval is unavailable while lexical search still works. | Knowledge marks the result stale/degraded and renders the fallback result with its status. | high | `test/workspace_state_golden_test.dart::stale degraded retrieval state golden` |
| ECM-007 | DEGRADATION | Network/API call fails before a workspace snapshot is available. | Today shows an offline state with a retry action and preserves canonical navigation. | high | `test/workspace_state_golden_test.dart::offline state golden` |
| ECM-008 | SECURITY | Production mode is started with the legacy learning route unavailable. | Canonical shell still renders and does not depend on the legacy endpoint. | high | `test/production_shell_test.dart::new shell does not require legacy learning endpoints` |
| ECM-009 | CONCURRENT | A load/refresh notification arrives while the controller is already loading. | Controller avoids duplicate visible loading transitions and settles on one snapshot/error state. | medium | `test/workspace_state_golden_test.dart::loading state golden` |
| ECM-010 | SECURITY | A learning observation is recorded from the Learning surface. | Observation is stored separately and does not mutate Work or Knowledge data. | high | `backend/internal/httpapi/server_test.go::TestV2LearningOverlayRecordsWithoutMutatingWork` |

## Closure

Rows are mapped to existing widget/golden coverage. This matrix does not claim that a human has accepted the visual result; the operator U0 verdict remains open in `docs/project/ux/OPERATOR_VERDICT.md`.
