# repo-context-map@1

task_id: TASK-OPS-001-agent-orchestration-loop-guard
generated_at: 2026-08-29T18:56:12+07:00
task_module: improvement
repo_root: /Users/ryantruong/Project/Orther/Dev-English

## Existing patterns

- kind: error_type; value: verifier functions fail closed with a named check and exit non-zero; pinned_in: `scripts/verify_multi_agent_protocol.sh:44`
- kind: state_management; value: durable JSON run manifest is the source for active/terminal/attention states; pinned_in: `docs/project/agent-runs/WAVE-2.json:1`
- kind: logging; value: deterministic `PASS`/`FAIL` lines identify the check and reason; pinned_in: `scripts/verify_multi_agent_protocol.sh:29`
- kind: test_framework; value: shell verifier plus JSON parsing and CyberOS-only gate scripts; pinned_in: `scripts/verify_multi_agent_protocol.sh:1`

## Schema and protocol surface

- table_or_type: `run manifest`; defined_in: `docs/project/agent-runs/WAVE-2.json:1`; consumed_by: protocol verifier and controller handoff
- table_or_type: `agent.status`; defined_in: `scripts/verify_multi_agent_protocol.sh:54`; consumed_by: active-task/reviewer/descendant checks
- table_or_type: `humanAuthorization`; defined_in: `docs/project/agent-runs/WAVE-2.json:298`; consumed_by: commit/push/merge/deploy boundary
- table_or_type: `MULTI_AGENT_PROTOCOL.md`; defined_in: `docs/project/MULTI_AGENT_PROTOCOL.md:1`; consumed_by: agent entrypoints and controller operations

## Sampled surface and outside-domain files

The scan sampled the protocol, the Wave 2 manifest, the deterministic verifier, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, the CyberOS-only verifier and the current acceptance report. It did not inspect or modify application source, provider credentials or worktree contents.

- path: `AGENTS.md`; reason: root entrypoint must load the protocol and preserve human release authority; risk: high
- path: `CLAUDE.md`; reason: Claude entrypoint must share the same protocol boundary; risk: high
- path: `GEMINI.md`; reason: Gemini entrypoint must not create descendants; risk: high
- path: `scripts/verify_cyberos_only.sh`; reason: verifies no competing agent surface was introduced; risk: medium

## Blast radius

```yaml
files_in_immediate_domain: 3
files_outside_immediate_domain: 4
modules_touched: 1
cross_module_edges: 2
score: 35
```

module_placement_warning: null

## Integrity notes

- Timeout is treated as non-terminal observation; the manifest must retain the same assignment until a terminal status is confirmed.
- The current CyberOS-only verifier still reports the pre-existing `.github/workflows/ci.yml` WIP mismatch. That WIP is protected and is not a reason to weaken the verifier.
