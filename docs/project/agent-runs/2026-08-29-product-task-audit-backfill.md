# Product task audit backfill — 2026-08-29

## Purpose

Backfill the missing hash-bound, manual spec-correctness audits for
`TASK-PRODUCT-002` through `TASK-PRODUCT-008`. This closes the reconcile
`R1` evidence gap without changing task lifecycle state or treating local
implementation evidence as human acceptance.

## Scope

Audited task specs:

- `TASK-PRODUCT-002-walking-skeleton`
- `TASK-PRODUCT-003-work-depth`
- `TASK-PRODUCT-004-knowledge-sync`
- `TASK-PRODUCT-005-grounded-assistant`
- `TASK-PRODUCT-006-learning-voice`
- `TASK-PRODUCT-007-mcp-parity`
- `TASK-PRODUCT-008-cutover-hardening`

Each task now has a co-located `audit.md` with:

- normative-body SHA-256 binding that excludes lifecycle-mutable fields;
- explicit acceptance/evidence traceability findings;
- data, safety, UX or security boundary findings where applicable;
- an explicit manual-review next action and no release authority.

## Verification

- `rtk node .cyberos/docs-tools/task-lint.mjs docs/tasks/product docs/tasks/improvement`
  — exit `0`; only the documented informational `TRACE-001` messages remain;
- `rtk node .cyberos/docs-tools/task-reconcile.mjs TASK-PRODUCT-002-walking-skeleton --run-tests --json`
  through `TASK-PRODUCT-008-cutover-hardening` — each report records `R1: pass`
  and `R2: pass`;
- the same reports retain the real non-terminal boundaries: `R3: absent`,
  `R4: red` for deliverables not present in `HEAD`, and `R5: red` only where
  the cited production smoke suite is not tracked at `HEAD` (P007/P008);
- `rtk git diff --check` — exit `0`.

## Boundary

This is a post-hoc spec audit, not an independent Sol review, implementation
approval or task lifecycle transition. The working tree remains dirty and
operator-owned. Commit, push, merge and deployment remain separately
unauthorized.
