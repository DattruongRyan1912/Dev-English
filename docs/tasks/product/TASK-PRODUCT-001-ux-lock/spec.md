---
id: TASK-PRODUCT-001
title: Lock the product-reset UX and information architecture
template: task@1
type: improvement
module: product
status: ready_to_review
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-28T21:30:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: minimal
client_visible: false
depends_on: []
new_files:
  - docs/project/ux/HIGH_FIDELITY_SPEC.md
  - test/production_shell_test.dart
  - test/workspace_golden_test.dart
  - docs/project/reports/PRODUCT-RESET-IMPLEMENTATION-REPORT.html
modified_files:
  - lib/main.dart
  - lib/src/design_system/
---

# TASK-PRODUCT-001: Lock the product-reset UX and information architecture

## Summary

Define and implement the Today, Work, Knowledge and Learning shell so later
slices share one product hierarchy and one mobile-first visual language.

## Problem

The legacy learning shell and the generated workspace preview expose two
different products. Without a locked information architecture, backend slices
can be completed without producing a coherent user workflow.

## Proposed Solution

Use the high-fidelity specification as the UI contract, keep the assistant
inside workflow surfaces, and cover loading, empty, error, offline and stale
states with production-shell and golden tests.

## Alternatives Considered

- Keep extending the legacy Home/Practice/Review/Progress navigation. Rejected because it hides the work-assistant product.
- Treat the existing card wall as final UI. Rejected because it does not establish hierarchy or state behavior.

## Success Metrics

- The production shell starts at Today and exposes the four product modules.
- The locked states render deterministically at mobile and desktop breakpoints.
- A human U0 review can trace the primary workflow without relying on demo-only copy.

## Scope

In scope: information architecture, design tokens, shell routing, state
surfaces, responsive goldens and the operator U0 review packet.

Out of scope: new provider behavior, external writes and final visual approval
without human review.

## Dependencies

- Existing Flutter feature seams and workspace controller.
- `docs/project/ux/HIGH_FIDELITY_SPEC.md`.
- Operator review on a real mobile viewport.

## 1. Description

- Replace the generated card-wall presentation with the locked command-center shell.
- Keep legacy learning logic available behind compatibility routing while the new shell is exercised by production mode.
- Record visual evidence and unresolved U0 findings in the implementation report.

## Acceptance criteria

- [ ] AC 1 — Production mode opens Today and exposes Today, Work, Knowledge and Learning navigation. (traces_to: shell; test: `test/production_shell_test.dart::successful authenticated production bootstrap opens the new canonical shell`)
- [ ] AC 2 — Loading, empty, error, offline and stale states have deterministic mobile coverage. (traces_to: states; test: `test/workspace_state_golden_test.dart::{loading state golden; empty state golden; stale degraded retrieval state golden; provider error state golden; offline state golden}`)
- [ ] AC 3 — The high-fidelity specification and implementation report identify the remaining human U0 decision. (verify: `docs/project/ux/HIGH_FIDELITY_SPEC.md` and `docs/project/reports/PRODUCT-RESET-IMPLEMENTATION-REPORT.html`)

## Edge cases

- A missing bootstrap response must not silently select demo data in production mode.
- Narrow mobile widths must preserve readable evidence and action controls.
- Legacy routes remain reachable only through explicit compatibility mode.

## Protected invariants

- Work and knowledge data remain separate from learning observations.
- No credential, provider output or source content is embedded in golden artifacts.
- Human U0 review remains open until the operator accepts the mobile flow.

## AI Authorship Disclosure

- Tools used: Codex with local Flutter tests, golden rendering and repository inspection.
- Scope: re-derived and CONFIRMED: the product shell and state surfaces; re-derived and CORRECTED: the legacy-versus-workspace route boundary; measured and ADDED: production-shell and golden evidence references.
- Human review: the operator reviews the high-fidelity mobile flow and records U0 acceptance before release.
