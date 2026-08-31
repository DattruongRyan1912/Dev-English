# edge-case-matrix@1

task_id: TASK-OPS-001-agent-orchestration-loop-guard
generated_at: 2026-08-29T18:56:12+07:00
total_rows: 10

| id | category | trigger | expected | severity | planned_test |
| --- | --- | --- | --- | --- | --- |
| ECM-001 | NULL_INPUT | No run manifest exists in the configured agent-run directory. | Verifier fails the manifests check and the controller does not infer active state. | high | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-002 | MALFORMED | Manifest has unsupported schema or a missing agent identifier. | Verifier fails closed with the manifest filename and does not continue to spawn. | high | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-003 | BOUNDARY | Four workers are active, which equals the configured maximum. | Verifier accepts the boundary; a fifth worker is rejected. | medium | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-004 | CONCURRENT | Two active agents claim the same task ID. | Verifier rejects the duplicate assignment. | critical | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-005 | CONCURRENT | A worker or reviewer is marked active and also has a descendant flag. | Verifier rejects the descendant relationship. | critical | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-006 | BOUNDARY | Two reviewers are active for one wave. | Verifier rejects reviewer concurrency above one. | critical | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-007 | DEGRADATION | A bounded wait returns a timeout while the target thread is still running. | Timeout is recorded as observation only; assignment remains active and no replacement is spawned. | critical | `docs/project/MULTI_AGENT_PROTOCOL.md` plus `docs/project/agent-runs/WAVE-2.json` |
| ECM-008 | SECURITY | A paused/attention run still contains an active worker or reviewer. | Verifier rejects the inconsistent state and blocks additional spawn. | critical | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-009 | MALFORMED | Manifest contains an unknown role or invalid lifecycle status. | Verifier rejects the entry rather than silently treating it as a worker. | high | `scripts/verify_multi_agent_protocol.sh::manifests` |
| ECM-010 | SECURITY | An accepted reviewer is not marked completed, or human release authorization is true unexpectedly. | Verifier rejects the release state; commit/push/merge/deploy remain operator-only. | critical | `scripts/verify_multi_agent_protocol.sh::manifests` and `docs/project/MULTI_AGENT_PROTOCOL.md` |

## Closure

The rows are protocol-level boundary cases. They do not authorize creation of a new agent, replacement of a timed-out thread or Git/release mutation.
