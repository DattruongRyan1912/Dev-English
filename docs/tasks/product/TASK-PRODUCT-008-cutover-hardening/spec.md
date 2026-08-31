---
id: TASK-PRODUCT-008
title: Cut over the product reset and harden the local release path
template: task@1
type: improvement
module: product
status: on_hold
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-28T21:37:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: minimal
client_visible: false
depends_on: [TASK-PRODUCT-007]
new_files:
  - infra/docker-compose.production.smoke.yml
  - scripts/production_runtime_smoke.sh
  - scripts/db_legacy_upgrade_test.sh
  - scripts/knowledge_capacity_test.sh
  - docs/project/agent-runs/2026-08-29-product-ac-completion-matrix.md
modified_files:
  - infra/docker-compose.production.yml
  - docs/project/STATUS.md
  - docs/project/ROADMAP.md
  - docs/project/TEST_LOG.md
  - changelog.md
---

# TASK-PRODUCT-008: Cut over the product reset and harden the local release path

> Lifecycle hold: wait for `TASK-PRODUCT-007` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Prove the complete local Docker path, backfill compatibility data, record
release evidence and define the remaining human acceptance gates.

## Problem

The product reset has implemented slices, but old data, migration upgrades,
runtime startup, coverage and device acceptance still need one auditable gate.

## Proposed Solution

Run disposable PostgreSQL, backend and embedding-sidecar checks, exercise the
authenticated product flow, verify capacity and legacy upgrades, and publish a
truthful status/report packet without claiming human acceptance.

## Alternatives Considered

- Treat unit tests as release proof. Rejected because migration and runtime wiring need real services.
- Remove legacy endpoints immediately. Rejected until compatibility and rollback evidence exist.

## Success Metrics

- Docker smoke starts the complete local stack and exercises auth, knowledge, MCP, CORS and provider fail-closed behavior.
- Legacy upgrade and 100,000-chunk capacity checks produce reproducible evidence.
- Coverage, U0 review, Sol review and operator authorization remain visible as open or accepted states.

## Scope

In scope: migration/backfill compatibility, Docker runtime, capacity,
security/concurrency/provider gates, status/changelog/report and release runbook.

Out of scope: commit, push, merge, deploy and final human release approval.

## Dependencies

- TASK-PRODUCT-007 MCP parity.
- Disposable Docker/PostgreSQL environment.
- Operator U0 and human acceptance gates.

## 1. Description

- Run the product stack from migration zero through the current migration and verify restart behavior.
- Keep compatibility endpoints until data parity and rollback evidence are recorded.
- Publish exact command exits, coverage and known limitations in the evidence packet.

## Acceptance criteria

- [ ] AC 1 — Disposable production-shaped Docker smoke passes auth, embedding, knowledge, MCP and provider-fail-closed checks. (test: `scripts/production_runtime_smoke.sh::main`)
- [ ] AC 2 — Legacy upgrade and 100,000-chunk capacity checks complete without data-integrity errors. (test: `scripts/db_legacy_upgrade_test.sh::main`; test: `scripts/knowledge_capacity_test.sh::main`)
- [ ] AC 3 — Status, changelog, runbook and implementation report distinguish verified evidence from open U0, coverage and human-authorization gates. (verify: `docs/project/agent-runs/2026-08-29-product-ac-completion-matrix.md`)

## Edge cases

- A protected pre-existing WIP change must not be reverted to make a verifier green.
- Provider quota or network failure must not activate production fallback behavior.
- A migration failure stops the smoke before reporting release success.

## Protected invariants

- No release mutation occurs without explicit operator authorization.
- Existing user WIP remains recoverable and outside the implementation cone.
- Coverage below the plan threshold is reported as open rather than relabeled.

## AI Authorship Disclosure

- Tools used: Codex with Docker, PostgreSQL, Go, Flutter and CyberOS gates.
- Scope: re-derived and CONFIRMED: the runtime and evidence gates; re-derived and CORRECTED: migration count and MCP smoke coverage; measured and ADDED: exact exits, coverage and protected-WIP caveat.
- Human review: the operator reviews the release packet, U0 result and final authorization before go-live.
