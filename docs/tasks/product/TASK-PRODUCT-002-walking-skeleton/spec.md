---
id: TASK-PRODUCT-002
title: Deliver the PostgreSQL-backed product walking skeleton
template: task@1
type: feature
module: product
status: on_hold
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-28T21:31:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: minimal
client_visible: false
depends_on: [TASK-PRODUCT-001]
new_files:
  - backend/internal/application/
  - backend/internal/platform/
  - infra/migrations/003_platform_foundation.sql
  - test/workspace_api_test.dart
modified_files:
  - backend/cmd/server/main.go
  - backend/internal/httpapi/server.go
  - lib/src/workspace_api.dart
  - lib/src/workspace_controller.dart
---

# TASK-PRODUCT-002: Deliver the PostgreSQL-backed product walking skeleton

> Lifecycle hold: wait for `TASK-PRODUCT-001` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Connect the new shell to a composed application boundary, workspace bootstrap
and PostgreSQL-backed platform services.

## Problem

The preview shell previously depended on demo services, while the backend
packages were not composed into one authenticated runtime path.

## Proposed Solution

Compose platform and application services at server startup, expose the v2
bootstrap contract, and let the Flutter controller distinguish real production
data from explicit demo mode.

## Alternatives Considered

- Let each screen call repositories directly. Rejected because REST, Flutter and MCP would diverge.
- Keep the demo gateway as a silent fallback. Rejected because fake work data can be mistaken for canonical data.

## Success Metrics

- Authenticated bootstrap returns one workspace and capability state.
- A disposable PostgreSQL runtime can reload the same workspace after restart.
- Production mode has no implicit demo fallback.

## Scope

In scope: composition root, platform migration wiring, bootstrap contract,
workspace isolation and Flutter production data loading.

Out of scope: full multi-tenant RBAC and external connector synchronization.

## Dependencies

- TASK-PRODUCT-001 UX shell contract.
- Existing platform migrations and auth/session boundary.
- PostgreSQL and optional embedding sidecar in local Docker.

## 1. Description

- Join platform, application and HTTP adapters at one server composition root.
- Keep workspace identity explicit across persistence and client bootstrap.
- Preserve a separately selected demo mode for tests and local previews.

## Acceptance criteria

- [ ] AC 1 — Authenticated bootstrap returns workspace and capability data from the composed server. (test: `backend/internal/httpapi/server_test.go::TestV2WalkingSkeletonPersistsProjectTaskKnowledgeAndConversation`)
- [ ] AC 2 — PostgreSQL migration and repository tests preserve workspace isolation across reloads. (test: `backend/internal/store/postgres_integration_test.go::TestPostgresRepositoryIntegration`)
- [ ] AC 3 — Production Flutter mode does not silently use demo gateway data. (test: `test/production_shell_test.dart::new shell does not require legacy learning endpoints`)

## Edge cases

- Missing database configuration in development may use the explicit seeded path but production stays fail-closed.
- A deleted workspace cannot be returned by the normal bootstrap query.
- User and workspace identifiers from one account cannot resolve another account's records.

## Protected invariants

- Existing auth, secret redaction and session-cookie boundaries remain intact.
- Migration order remains append-only and reversible at the deployment boundary.
- The operator retains the release decision.

## AI Authorship Disclosure

- Tools used: Codex with Go, PostgreSQL integration and Flutter runtime tests.
- Scope: re-derived and CONFIRMED: the composition and bootstrap path; re-derived and CORRECTED: production/demo separation; measured and ADDED: workspace isolation and reload evidence.
- Human review: the operator reviews the authenticated walking skeleton on the target mobile device.
