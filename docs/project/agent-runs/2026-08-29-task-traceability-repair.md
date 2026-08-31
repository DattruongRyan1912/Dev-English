# Task traceability repair — 2026-08-29

## Finding

The product task specs used `tests:` on several acceptance criteria, while the
CyberOS task linter recognizes the canonical `test:` or `verify:` trace fields.
As a result, the product/improvement task lint exited `2` with eight
`TRACE-002` errors even though the referenced test evidence existed.

## Change

Normalized the affected acceptance criteria in:

- `docs/tasks/product/TASK-PRODUCT-001-ux-lock/spec.md`
- `docs/tasks/product/TASK-PRODUCT-003-work-depth/spec.md`
- `docs/tasks/product/TASK-PRODUCT-004-knowledge-sync/spec.md`
- `docs/tasks/product/TASK-PRODUCT-006-learning-voice/spec.md`

The referenced suites and test names were not changed; only the traceability
field was corrected from `tests:` to `test:`.

## Verification

```text
rtk node .cyberos/docs-tools/task-lint.mjs docs/tasks/product docs/tasks/improvement
exit 0 — no TRACE-002 errors; remaining TRACE-001 entries are informational

rtk bash scripts/verify_multi_agent_protocol.sh
PASS protocol
PASS entrypoints
PASS manifests

rtk git diff --check
exit 0
```

This repairs the governance gate only. It does not advance any CyberOS task
lifecycle or replace the required human review/acceptance gates.
