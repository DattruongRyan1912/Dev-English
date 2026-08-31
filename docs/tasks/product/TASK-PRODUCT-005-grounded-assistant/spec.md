---
id: TASK-PRODUCT-005
title: Deliver grounded assistant routing and safe usage accounting
template: task@1
type: feature
module: product
status: on_hold
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-28T21:34:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: limited
client_visible: false
depends_on: [TASK-PRODUCT-004]
new_files:
  - backend/internal/assistant/
  - backend/internal/actions/
  - backend/internal/usageguard/
  - lib/src/features/assistant/workspace_assistant.dart
  - infra/migrations/007_assistant_s1.sql
  - infra/migrations/011_usage_accounting.sql
modified_files:
  - backend/internal/ai/provider.go
  - backend/internal/httpapi/v2.go
---

# TASK-PRODUCT-005: Deliver grounded assistant routing and safe usage accounting

> Lifecycle hold: wait for `TASK-PRODUCT-004` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Provide the text-first 1:1 assistant with verified evidence, provider routing,
conversation persistence, usage budgets and preview-confirm-receipt actions.

## Problem

An LLM can produce plausible but unsupported work facts, while provider quotas
and external writes need deterministic limits and receipts.

## Proposed Solution

Retrieve verified evidence before generation, return explicit grounded/inferred/
unknown states, route capabilities to configured providers, and place every
mutation behind challenge, idempotency and durable receipt services.

## Alternatives Considered

- Return the raw model answer when retrieval is empty. Rejected because it creates fabricated facts.
- Charge a fixed estimate for every request. Rejected because provider-reported usage is available for some calls.

## Success Metrics

- Golden corpus answers never cite evidence that was not retrieved.
- Unknown and stale-source states are visible in API and UI responses.
- Provider failures fail closed in production and usage accounting is replay-safe.

## Scope

In scope: grounded conversations, DeepSeek routing, usage guard, challenge and
receipt integration, action preview and assistant UI.

Out of scope: autonomous mutation and production provider credentials in tests.

## Dependencies

- TASK-PRODUCT-004 evidence-backed retrieval.
- Configured DeepSeek provider and usage persistence.
- Existing safe-write challenge and receipt services.

## 1. Description

- Require verified retrieved evidence before labeling an answer grounded.
- Keep model output separate from canonical Work and Knowledge records.
- Use provider usage when reported and fail closed when production credentials or provider calls are unavailable.

## Acceptance criteria

- [ ] AC 1 — Assistant responses carry answer, grounding, evidence, unknowns and stale-source fields. (test: `backend/internal/assistant/grounding_test.go::TestNormalizeResponseReturnsGroundedContractForCurrentEvidence`)
- [ ] AC 2 — Provider failure does not activate the deterministic fallback in production. (test: `backend/internal/ai/provider_http_test.go::TestFallbackProviderDoesNotHidePrimaryFailureWhenDisabled`; UI evidence: `test/workspace_ui_test.dart::provider failure stays visible without a fallback answer`)
- [ ] AC 3 — External mutations require challenge confirmation and return a stable receipt on replay. (test: `backend/internal/httpapi/actions_test.go::TestActionHTTPFlowRequiresChallengeAndReturnsStableReplayReceipt`)

## Edge cases

- Empty retrieval returns unknown rather than an uncited model assertion.
- A stale revision is disclosed even when it is the best available evidence.
- A failed receipt completion is treated as uncertain and cannot replay the provider blindly.

## AI Risk Assessment

### Data Sources

Assistant input is user text plus retrieved app/external source evidence.

### Human Oversight

Users confirm every external mutation; the operator reviews provider and quota gates.

### Failure Modes

Unsupported claims, stale citations, provider outages and quota exhaustion are
returned as explicit states rather than hidden fallbacks.

## Protected invariants

- Retrieved documents cannot override system policy or invoke tools by themselves.
- Receipt creation is server-owned and durable before external success is accepted.
- Credentials and source content remain redacted from logs.

## AI Authorship Disclosure

- Tools used: Codex with Go assistant, provider, action and HTTP tests.
- Scope: re-derived and CONFIRMED: grounding and action boundaries; re-derived and CORRECTED: production fail-closed behavior; measured and ADDED: receipt uncertainty and usage evidence.
- Human review: the operator reviews grounded answers and confirms the production safety boundary.
