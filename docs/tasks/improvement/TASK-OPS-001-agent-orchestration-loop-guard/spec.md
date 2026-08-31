---
id: TASK-OPS-001
title: Add multi-agent orchestration loop guard
template: task@1
type: improvement
module: improvement
status: ready_to_review
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-26T12:25:34+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: not_ai
client_visible: false
depends_on: []
routed_back_count: 0
awh: N/A
service: .
new_files:
  - docs/project/MULTI_AGENT_PROTOCOL.md
  - docs/project/agent-runs/WAVE-2.json
  - scripts/verify_multi_agent_protocol.sh
  - docs/tasks/improvement/TASK-OPS-001-agent-orchestration-loop-guard/spec.md
  - docs/tasks/improvement/TASK-OPS-001-agent-orchestration-loop-guard/implementation.md
modified_files:
  - AGENTS.md
  - CLAUDE.md
  - GEMINI.md
  - docs/tasks/BACKLOG.md
  - changelog.md
---

# TASK-OPS-001: Add multi-agent orchestration loop guard

## Summary

Prevent duplicate reviewers, descendant review fan-out and timeout-triggered
agent replacement before Wave 2 resumes.

## Problem

The Wave 2 review run replaced a still-running reviewer after observation
timeouts. The replacement then created a descendant with the same review
assignment. This produced duplicate work, wasted model usage and required an
operator stop.

## Proposed Solution

Record one durable manifest for the active run, make the main controller the
only spawn/close authority, and require bounded observation waits that never
replace a still-running reviewer.

## Alternatives Considered

- Replace a timed-out reviewer automatically. Rejected because a timeout is not terminal evidence.
- Let reviewers spawn repair descendants. Rejected because it creates duplicate ownership and an unbounded tree.

## Success Metrics

- The verifier rejects duplicate task assignments, active descendants and excess reviewer concurrency.
- A run manifest makes every active reviewer and worker assignment auditable.
- Human merge, push and deploy authority remains outside the agent loop.

## Scope

In scope: agent entrypoints, protocol, run manifests, deterministic verifier and
task evidence. Out of scope: application behavior, source integration and
external release mutations.

## Dependencies

- CyberOS lifecycle and human-gate rules.
- Existing `.agents` MCP adapter and project-aware gate configuration.
- Operator-owned worktrees and the current Wave 2 review state.

## Acceptance criteria

- [ ] AC 1 — A tracked protocol makes the main controller the only spawn/close authority. (test: `scripts/verify_multi_agent_protocol.sh::protocol`)
- [ ] AC 2 — Workers and reviewers are explicitly forbidden from creating descendants. (verify: `docs/project/MULTI_AGENT_PROTOCOL.md`)
- [ ] AC 3 — One task has at most one active agent and one wave has at most one reviewer. (test: `scripts/verify_multi_agent_protocol.sh::manifests`)
- [ ] AC 4 — Wait timeout is defined as an observation result, not an agent failure. (verify: `docs/project/MULTI_AGENT_PROTOCOL.md`)
- [ ] AC 5 — A tracked run manifest records the interrupted Wave 2 review chain. (verify: `docs/project/agent-runs/WAVE-2.json`)
- [ ] AC 6 — A deterministic verifier rejects duplicate active assignments, active descendants and excess reviewer concurrency. (test: `scripts/verify_multi_agent_protocol.sh::main`)
- [ ] AC 7 — All agent entrypoints load the protocol without weakening CyberOS-only governance. (test: `scripts/verify_cyberos_only.sh::entrypoints`)

## Protected invariants

- CyberOS remains the canonical lifecycle and human-gate authority.
- `.agents` remains CyberOS-owned; legacy role/workflow files are not restored.
- Application WIP and Wave 2 worktrees are not modified or integrated by this task.
- Commit, push, merge and deploy remain operator-only.

## AI Authorship Disclosure

- Tools used: Codex with CyberOS task lint, manifest inspection and deterministic shell verifiers.
- Scope: re-derived and CONFIRMED: the single-controller and bounded-wait rules; re-derived and CORRECTED: timeout handling and reviewer ownership; measured and ADDED: manifest/verifier acceptance evidence.
- Human review: the operator reviews the orchestration protocol and accepts lifecycle transitions through the CyberOS gates.
