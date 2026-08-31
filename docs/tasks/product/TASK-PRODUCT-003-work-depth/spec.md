---
id: TASK-PRODUCT-003
title: Complete versioned Work entities and safe local actions
template: task@1
type: feature
module: product
status: on_hold
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-28T21:32:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: minimal
client_visible: false
depends_on: [TASK-PRODUCT-002]
new_files:
  - backend/internal/work/
  - infra/migrations/006_work.sql
  - lib/src/features/work/workspace_work.dart
  - test/workspace_ui_test.dart
modified_files:
  - backend/internal/application/
  - backend/internal/httpapi/v2.go
---

# TASK-PRODUCT-003: Complete versioned Work entities and safe local actions

> Lifecycle hold: wait for `TASK-PRODUCT-002` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Make Project, Task and Decision useful as the canonical work record for each
workspace, with history, trash and optimistic concurrency.

## Problem

The product reset describes Work but the preview does not yet provide the
durable CRUD and conflict behavior needed for real daily work.

## Proposed Solution

Expose versioned application services and REST adapters for project, task and
decision flows, then connect the Work screen to those services with explicit
conflict and restore states.

## Alternatives Considered

- Let the assistant write work records directly. Rejected because model output is not canonical data.
- Resolve conflicts by last-write-wins. Rejected because it can destroy user edits.

## Success Metrics

- Create, update, list, trash and restore work records within a workspace.
- A stale expected version returns a conflict without overwriting the current record.
- Work history is visible enough to explain the last change.

## Scope

In scope: Project, Task, Decision CRUD, versioning, history, trash/restore and
Work UI states.

Out of scope: GitHub writes and autonomous assistant actions.

## Dependencies

- TASK-PRODUCT-002 walking skeleton.
- Platform workspace and audit services.
- Existing action challenge/receipt boundary for future external writes.

## 1. Description

- Use PostgreSQL and memory implementations behind the same work repository interface.
- Require expected version and idempotency metadata on mutable work requests.
- Show current-versus-expected versions when a user edit conflicts.

## Acceptance criteria

- [ ] AC 1 — Project, Task and Decision CRUD is workspace-scoped and versioned. (test: `backend/internal/work/work_test.go::{TestProjectLifecycleHistoryAndRetention; TestTaskAndDecisionLifecycleAndProjectDependencies; TestConcurrentUpdatesProduceOneSuccessAndTypedConflicts; TestScopeIsolationSameIDReuseAndHistoryValidation}`)
- [ ] AC 2 — Trash and restore preserve history and do not expose deleted records in normal lists. (test: `backend/internal/work/work_test.go::TestProjectLifecycleHistoryAndRetention`; `backend/internal/work/postgres_integrity_test.go::TestPostgresWorkWorkspaceOwnerIntegrity`)
- [ ] AC 3 — Work UI shows conflict, empty and loaded states without silently overwriting. (test: `test/workspace_state_golden_test.dart::work conflict state golden`; `test/workspace_ui_test.dart::work conflict remains an explicit reload action`)

## Edge cases

- Replaying the same idempotency key returns the original result.
- A restore of an already active record is idempotent.
- A record from another workspace is treated as not found.

## Protected invariants

- Project, Task and Decision remain canonical in the app database.
- Assistant suggestions require an explicit user action before mutation.
- Soft-delete remains recoverable during the retention window.

## AI Authorship Disclosure

- Tools used: Codex with Go unit/integration tests and Flutter UI tests.
- Scope: re-derived and CONFIRMED: work ownership and conflict rules; re-derived and CORRECTED: canonical versus suggested data boundaries; measured and ADDED: version, trash and workspace-isolation evidence.
- Human review: the operator reviews the conflict and restore flows before task acceptance.
