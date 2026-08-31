---
audit_template_version: "task_rubric@1.0"
audited_file: "docs/tasks/product/TASK-PRODUCT-008-cutover-hardening/spec.md"
audited_file_sha256: "1d35926bb100a5f510d6e2e284a014badcbb75450a8a5a15432fe3719f3855c3"
audited_file_sha256_prefix: "1d35926bb100a5f510d6e"
audited_body_sha256_prefix: "d23062f7c39c97a5"
rubric_version: "audit_rubric@2.0"
skill_id: "task-audit"
skill_version: "1.0.0"
last_audit_at: "2026-08-29T13:21:59Z"
overall_status: "pass"
iterations: 1
score_pre_revision: "10/10"
score_post_expansion: "10/10"
score_post_revision: "10/10"
issue_counts:
  total: 3
  open: 0
  needs_human: 0
  fixed: 3
  wontfix: 0
trace_id: "d32672b8-3e69-4d7e-b35c-dbc8369a5e1a"
caller_persona: "codex-manual-reconciliation"
---

# TASK-PRODUCT-008 specification audit

This is a post-hoc manual **spec-correctness** audit of the cutover and
hardening task. It checks the declared evidence paths and release boundaries
against the current checkout. It does not claim that uncommitted artifacts are
accepted, that CI ran, or that a human authorized release.

## Findings (all resolved)

```text
ISSUE
id: ISS-001
rule_id: TRACE-001
status: fixed
severity: error
location: Acceptance criterion 1
evidence: "The production-shaped smoke names auth, embedding, Knowledge, MCP and provider-fail-closed checks."
description: "A local Docker smoke must fail before reporting success when migrations or readiness fail."
resolution: "The criterion maps to the smoke script and the current evidence packet records exact status results and migration count."
```

```text
ISSUE
id: ISS-002
rule_id: DATA-001
status: fixed
severity: error
location: Acceptance criterion 2 and edge cases
evidence: "Legacy upgrade, 100,000-chunk capacity and rollback are named as distinct checks."
description: "Cutover evidence must preserve existing data and stop on migration failure rather than relabeling a partial run."
resolution: "The spec retains disposable upgrade/capacity/rollback evidence and the protected WIP boundary."
```

```text
ISSUE
id: ISS-003
rule_id: QA-006
status: fixed
severity: warning
location: Acceptance criterion 3 and out-of-scope operations
evidence: "The completion matrix separates local evidence from U0, CI, lifecycle and human authorization."
description: "A truthful release packet must not turn local implementation evidence into a go-live decision."
resolution: "STATUS, ROADMAP, TEST_LOG, changelog, runbook and report retain the open gates; commit/push/merge/deploy remain out of scope."
```

## Summary

```text
SUMMARY
verdict: pass
issues_total: 3
issues_open: 0
issues_human: 0
issues_fixed: 3
iterations: 1
next_action: manual_review
```

The specification is structurally auditable. R4/R5 and human release gates
must still be resolved by the delivery workflow; this audit changes no task
status and authorizes no release action.
