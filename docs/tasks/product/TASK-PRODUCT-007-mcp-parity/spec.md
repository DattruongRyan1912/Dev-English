---
id: TASK-PRODUCT-007
title: Expose MCP resources and tools with REST parity
template: task@1
type: feature
module: product
status: on_hold
priority: p1
author: "@codex"
department: engineering
created_at: 2026-08-28T21:36:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: limited
client_visible: false
depends_on: [TASK-PRODUCT-006]
new_files:
  - backend/internal/mcp/
  - infra/migrations/016_mcp_tokens.sql
  - .mcp.json
modified_files:
  - backend/cmd/server/main.go
  - backend/internal/httpapi/v2.go
  - docs/project/PRODUCTION_RUNBOOK.md
---

# TASK-PRODUCT-007: Expose MCP resources and tools with REST parity

> Lifecycle hold: wait for `TASK-PRODUCT-006` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Make the same application data and safe actions available to Codex and Claude
through a bearer-authenticated Streamable HTTP MCP endpoint.

## Problem

Without a shared MCP adapter, external AI clients would need separate business
logic and could observe different semantics from the app.

## Proposed Solution

Persist digest-only MCP tokens, enforce scopes and replay protection at the
transport, and route every resource/tool call through the existing application
services used by REST.

## Alternatives Considered

- Reuse the Flutter session cookie for MCP. Rejected because client lifecycles and scopes differ.
- Implement MCP mutations beside REST handlers. Rejected because policy and idempotency would drift.

## Success Metrics

- Tokens survive restart, revoke across instances and reveal the bearer only once.
- Read resources and write tools have explicit scopes and the same service semantics as REST.
- Confirmed tools cannot execute without challenge and idempotency requirements.

## Scope

In scope: `/mcp`, token issue/revoke, persistent digest storage, scopes,
resources/tools, protocol and replay tests.

Out of scope: MCP server federation and autonomous external mutation.

## Dependencies

- TASK-PRODUCT-006 application services.
- PostgreSQL migration chain and MCP Go SDK protocol contract.
- Human authorization for external client configuration.

## 1. Description

- Use a separate bearer token namespace and scope set for MCP clients.
- Keep MCP adapters thin and delegate data/action semantics to application services.
- Require the same confirmation and receipt boundary for confirmed tools as REST.

## Acceptance criteria

- [ ] AC 1 — MCP initialize, tool/resource listing and scoped reads work over Streamable HTTP. (test: `backend/internal/mcp/mcp_test.go::TestResourceDispatchNormalizationAndHTTPTransport`)
- [ ] AC 2 — PostgreSQL token metadata survives a new TokenStore and revocation is visible across stores. (test: `backend/internal/mcp/postgres_tokens_integration_test.go::TestPostgresTokenPersistenceSurvivesRestartAndSharesRevocation`)
- [ ] AC 3 — Production smoke covers issue, initialize, revoke and rejected replay. (test: `scripts/production_runtime_smoke.sh::mcp-token-flow`)

## AI Risk Assessment

### Data Sources

MCP resources expose workspace-scoped application records and verified knowledge evidence.

### Human Oversight

Token creation, scope assignment and external mutation confirmation remain user/operator actions.

### Failure Modes

Expired/revoked tokens, missing scopes, replayed requests and persistence outages
fail closed without returning bearer secrets.

## Protected invariants

- Token digests, not bearer values, are persisted.
- REST and MCP use one application-service semantics boundary.
- MCP client access remains separately revocable and scope-limited.

## AI Authorship Disclosure

- Tools used: Codex with Go MCP, PostgreSQL integration and production smoke tests.
- Scope: re-derived and CONFIRMED: token and scope boundaries; re-derived and CORRECTED: restart/revocation behavior; measured and ADDED: persistent token and HTTP lifecycle evidence.
- Human review: the operator reviews MCP client configuration and scope assignment before release.
