# Dev-English multi-agent protocol

This protocol is the project-specific orchestration addendum to CyberOS. It
governs every delegated implementation or review run. CyberOS remains the
canonical lifecycle and human-acceptance authority.

## Roles

### Main controller

The main chat is the only orchestration authority. It owns task decomposition,
agent creation, waiting, shutdown, integration, verification and reporting. It
must keep the run manifest current before issuing another agent action.

### Luna Max worker

A worker implements exactly one task in an isolated worktree and an explicit
file cone. It writes tests and evidence, but cannot review or approve its own
work. A worker MUST NOT spawn, create, resume, close or delegate to another
agent.

### Sol Ultra reviewer

The reviewer performs one independent, read-only review of frozen worker
outputs. It may run non-mutating inspection and test commands, but MUST NOT
edit, integrate, commit, push, merge or deploy. It MUST NOT spawn or delegate
to another agent. Exactly one reviewer may be active for a review cycle.

## Hard invariants

- **ORCH-001 — Single spawn authority.** Only the main controller may call an
  agent/thread creation, resume, interrupt or close operation.
- **ORCH-002 — No descendants.** Every worker and reviewer prompt must include
  `Do not call spawn_agent, create_thread, resume_agent, or delegate work`.
- **ORCH-003 — One task, one active agent.** A task ID may appear on at most one
  agent whose status is `pending_init` or `running`.
- **ORCH-004 — One reviewer.** At most one reviewer may be `pending_init` or
  `running` for a wave. Reviewers never fan out review work.
- **ORCH-005 — Four-worker ceiling.** At most four implementation workers may
  be active, and their write cones must be disjoint.
- **ORCH-006 — Minimal context.** Spawn agents with `fork_context: false` and a
  self-contained prompt. Broad conversation history is not an authorization
  surface.
- **ORCH-007 — Timeout is not failure.** A wait timeout means only that no final
  status arrived in that observation window. It MUST NOT trigger interruption,
  replacement or a second reviewer.
- **ORCH-008 — Terminal-before-replacement.** A replacement is allowed only
  after the previous agent is confirmed terminal and the manifest records the
  stop reason. A budget overrun moves the run to `attention`, not to a duplicate
  spawn.
- **ORCH-009 — Frozen review input.** Review starts only after all workers are
  terminal and their paths, file lists and test evidence are recorded.
- **ORCH-010 — One accepted verdict.** A review cycle records exactly one
  reviewer agent ID. Results from interrupted, replaced or descendant reviewers
  are evidence only and cannot authorize integration.
- **ORCH-011 — Evidence before orchestration.** The main controller MUST read
  the authoritative worker/reviewer message or a saved evidence artifact before
  interpreting progress, selecting a next task, interrupting, replacing or
  reporting an agent result. A `timed_out`, `pending_init` or `running` status is
  lifecycle evidence only; it does not prove that the agent produced no plan or
  result. If content cannot be read, or lifecycle and content disagree, record
  the state as `unknown`/`attention` and HOLD: issue no new spawn, replacement,
  interruption, integration or task transition until the state is reconciled.
  Record the message source, observation time and exact evidence path/reference
  in the manifest.
- **ORCH-012 — Active-thread wait or wake-up.** The main controller MUST NOT end
  its turn while a worker or reviewer is still `pending_init` or `running`. It
  must wait on the exact Codex thread with a bounded `wait_threads` observation,
  then read the authoritative message after wake-up. If the controller must
  yield across a longer interval, it must create or update a thread heartbeat
  that only re-checks the same thread and manifest; it must not spawn, replace,
  interrupt or integrate work. A wait timeout leaves the same assignment active
  and is never a result.

## Run state machine

```text
planned
  -> workers_running
  -> outputs_frozen
  -> reviewing
  -> approved | changes_requested | blocked
  -> integrating
  -> verified
  -> CyberOS human gate
```

`paused` is entered on an operator stop signal. `attention` is entered when an
agent exceeds its observation budget or its status cannot be reconciled. Both
states forbid new agent creation until the main controller records a recovery
decision. An `attention` run may retain the exact running agent that exceeded
the observation budget; the controller must keep its ID, record it as
`attentionAgentId`, and never replace it merely because it is slow.

## Required run manifest

Before the first spawn, create or update one JSON file under
`docs/project/agent-runs/`. It must record:

- wave ID, state, integration target and human authorization boundary;
- maximum worker and reviewer concurrency;
- each task ID, agent ID, role, parent ID, model, status and file cone;
- spawn time, latest observed status, terminal reason and evidence paths;
- the single accepted reviewer ID, if a verdict has been accepted;
- incidents and recovery decisions.

Run `rtk bash scripts/verify_multi_agent_protocol.sh` before every spawn wave,
before review, and before integration. A failure is a protocol blocker.

## Spawn procedure

1. Confirm the task graph and disjoint file cones locally.
2. Write intended agents to the manifest with status `planned`.
3. Run the protocol verifier.
4. Spawn only the eligible workers, using `fork_context: false`.
5. Immediately record returned IDs and status; do not rely on nicknames.
6. Wait on the existing IDs. Progress updates do not create replacement work.
7. Freeze worker outputs only after each worker is terminal.
8. Run the verifier again, then spawn exactly one reviewer.

Every prompt must state the task ID, allowed paths, forbidden paths, required
tests, no-live-provider boundary, no-secret boundary and the descendant ban.

## Waiting and circuit breaker

- Use one bounded wait observation at a time. A timeout updates only
  `last_observed_at`.
- When the target is a Codex thread, prefer `wait_threads` over a lifecycle-only
  agent wait because it can wake on the actual completed turn or attention
  event. Do not finish the controller turn while the target remains active;
  either continue bounded waiting or leave a heartbeat wake-up tied to the
  exact thread ID and manifest path.
- Treat lifecycle status and agent content as two separate signals. After each
  meaningful observation, read the latest authoritative message or evidence
  artifact before making an orchestration decision; never infer "no output" from
  a wait timeout.
- Do not send an interrupt merely to obtain a faster report.
- When an observation budget is exceeded, set the run to `attention`, report
  the agent as still running and keep the same ID.
- Only an operator stop signal, a confirmed terminal error or an explicit
  recovery decision permits shutdown/replacement.
- If an unexpected descendant or duplicate active assignment appears, set the
  run to `paused`, issue no new spawn, record an incident and reconcile all
  statuses before continuing.

### Evidence-first status reconciliation

The controller must reconcile these signals in order:

1. Read the actual latest worker/reviewer message from the authoritative agent
   channel, or read the saved report/evidence path recorded by the run.
2. Record what the message establishes: plan, implementation progress, verdict,
   limitation or request for intervention. Do not paraphrase a missing message
   as a negative result.
3. Compare that content with the live lifecycle status and the durable manifest.
4. Only then choose `continue`, `HOLD`, `attention`, repair, replacement or the
   next eligible task.

If the controller cannot access the authoritative content, the safe result is
`UNKNOWN/HOLD`, not a guessed plan. A timeout remains non-terminal until a
terminal status is confirmed and the content/status reconciliation is recorded.

## Review and repair

Sol reviews frozen output and returns `approved`, `changes_requested` or
`blocked` per task. `changes_requested` creates bounded repair assignments for
Luna; Sol never edits the code. Re-review uses the same reviewer ID when the
agent remains available. If it is terminal, one replacement may be created
only after the manifest records the reason and clears the former active slot.

Only output approved by the manifest's accepted reviewer may be integrated.
Commit, push, merge, deploy and CyberOS acceptance gates remain operator-only.

## Prompt footer

Append this footer to every delegated prompt:

```text
Agent boundary: do not call spawn_agent, create_thread, resume_agent, close_agent,
or delegate this task. Work only in the assigned worktree and file cone. Do not
commit, push, merge, deploy, expose secrets, or call live providers. Return file
paths, exact test commands with exit codes, limitations and evidence.
```
