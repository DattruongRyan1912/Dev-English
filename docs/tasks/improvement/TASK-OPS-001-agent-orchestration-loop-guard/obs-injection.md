# obs-injection@1

task_id: TASK-OPS-001-agent-orchestration-loop-guard
generated_at: 2026-08-29T18:56:12+07:00
mode: evidence-only

## Observables

| observable | source | proof |
| --- | --- | --- |
| single spawn/close authority | `docs/project/MULTI_AGENT_PROTOCOL.md:31-35` | protocol verifier entrypoint and manifest checks |
| no descendants | `scripts/verify_multi_agent_protocol.sh:79-82` | manifests check rejects active descendants |
| one active task assignment | `scripts/verify_multi_agent_protocol.sh:82-85` | duplicate task IDs fail |
| one active reviewer | `scripts/verify_multi_agent_protocol.sh:94-95` | reviewer concurrency fails above one |
| timeout is non-terminal | `docs/project/MULTI_AGENT_PROTOCOL.md:44-50` | protocol text and historical Wave 2 incident record |
| paused/attention stops new work | `scripts/verify_multi_agent_protocol.sh:101-114` | paused-state and attention-agent checks |
| operator-only release boundary | `docs/project/agent-runs/WAVE-2.json:298-305` | human authorization fields remain false |

## Injection decision

No runtime code or additional telemetry was injected. The protocol already has a deterministic verifier and durable manifest. Adding an agent or mutating active lifecycle state would invalidate the evidence and repeat the historical loop.

## Limits

The verifier proves the static manifest/protocol invariants. It does not prove a future controller will obey them unless every spawn/close operation is routed through the canonical controller and the verifier is run at the documented boundaries.
