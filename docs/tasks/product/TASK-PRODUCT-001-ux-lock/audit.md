---
audit_template_version: "task_rubric@1.0"
audited_file: "docs/tasks/product/TASK-PRODUCT-001-ux-lock/spec.md"
audited_file_sha256: "da21d1cef405b441aecb5d2b27a2c4338ee5b3ac7707cbe0172d5b4353f5733b"
audited_file_sha256_prefix: "da21d1cef405b441"
audited_body_sha256_prefix: "584f44f26d9e8742"
rubric_version: "audit_rubric@2.0"
skill_id: "task-audit"
skill_version: "1.0.0"
last_audit_at: "2026-08-29T11:56:12Z"
overall_status: "pass"
iterations: 1
score_pre_revision: "10/10"
score_post_expansion: "10/10"
score_post_revision: "10/10"
issue_counts:
  total: 6
  open: 0
  needs_human: 0
  fixed: 6
  wontfix: 0
trace_id: "b5a2f1b5-3f9d-4d5e-8a87-0e0f77b9f6c1"
caller_persona: "codex-manual-reconciliation"
---

# TASK-PRODUCT-001 specification audit

This is a post-hoc manual **spec-correctness** audit because this repository does not ship an executable `task-audit` runner. It reads the spec, the cited source/test files and the current test evidence. It does not claim independent Sol review, U0 acceptance or implementation/release approval.

## Findings (all resolved)

```text
ISSUE
id: ISS-001
rule_id: TRACE-001
status: fixed
severity: error
location: Acceptance criteria 1-2
evidence: "The shell and state clauses need explicit downstream evidence."
description: "The normative shell/state promises were checked against their named widget and golden tests rather than accepted from prose alone."
suggestion: "Keep one named test reference for each observable acceptance condition."
auto_fix_applied: false
resolution: "AC 1 maps to production_shell_test.dart; AC 2 maps to workspace_state_golden_test.dart and the evidence pack records the exact test names."
resolved_at: "2026-08-29T11:56:12Z"
opened_at: "2026-08-29T11:56:12Z"
updated_at: "2026-08-29T11:56:12Z"
```

```text
ISSUE
id: ISS-002
rule_id: TRACE-002
status: fixed
severity: error
location: Acceptance criteria
evidence: "The human U0 criterion is not an automated widget assertion."
description: "The spec distinguishes automated shell/state checks from the operator visual review."
suggestion: "Keep U0 as an explicit human gate instead of implying a golden test is final visual approval."
auto_fix_applied: false
resolution: "Scope, protected invariants and the evidence pack retain U0 as an open human decision."
resolved_at: "2026-08-29T11:56:12Z"
opened_at: "2026-08-29T11:56:12Z"
updated_at: "2026-08-29T11:56:12Z"
```

```text
ISSUE
id: ISS-003
rule_id: TRACE-003
status: fixed
severity: error
location: Frontmatter new_files and cited tests
evidence: "The state golden test is cited and exists on disk."
description: "The citation was resolved against the actual repository path, including the previously omitted state-golden surface."
suggestion: "Keep the actual path in the task evidence and do not use the stale lib/src/screens path."
auto_fix_applied: false
resolution: "context-map.md and impl-plan.md record test/workspace_state_golden_test.dart; the file exists and the Flutter suite passed."
resolved_at: "2026-08-29T11:56:12Z"
opened_at: "2026-08-29T11:56:12Z"
updated_at: "2026-08-29T11:56:12Z"
```

```text
ISSUE
id: ISS-004
rule_id: TRACE-006
status: fixed
severity: error
location: Description and acceptance criteria
evidence: "'Opens', 'exposes' and 'renders' require visible UI assertions, not source-string presence."
description: "The cited production-shell and golden tests assert rendered labels, taps and golden output."
suggestion: "Document the verb-to-observable mapping so a passing payload-only test cannot be mistaken for rendered evidence."
auto_fix_applied: false
resolution: "obs-injection.md records the rendered-surface observables and their exact source/test locations."
resolved_at: "2026-08-29T11:56:12Z"
opened_at: "2026-08-29T11:56:12Z"
updated_at: "2026-08-29T11:56:12Z"
```

```text
ISSUE
id: ISS-005
rule_id: QA-006
status: fixed
severity: warning
location: Scope
evidence: "The shell task could accidentally absorb provider, connector or release work."
description: "The spec has explicit in-scope/out-of-scope boundaries and names external writes and final visual approval as out of scope."
suggestion: "Preserve the boundary while reconciling the implementation."
auto_fix_applied: false
resolution: "The file cone and impl-plan keep provider behavior, external writes and release actions outside this task."
resolved_at: "2026-08-29T11:56:12Z"
opened_at: "2026-08-29T11:56:12Z"
updated_at: "2026-08-29T11:56:12Z"
```

```text
ISSUE
id: ISS-006
rule_id: TRACE-007
status: fixed
severity: error
location: Success metrics and protected invariants
evidence: "Claims about four destinations, deterministic states and separation from legacy data need reproducible derivations."
description: "The audit re-ran source/test searches and the Flutter/browser commands instead of trusting the task summary."
suggestion: "Keep command output and source locations in a separate evidence artifact."
auto_fix_applied: false
resolution: "impl-plan.md and obs-injection.md record the exact commands, source locations and current results; no secrets or source content are copied into goldens."
resolved_at: "2026-08-29T11:56:12Z"
opened_at: "2026-08-29T11:56:12Z"
updated_at: "2026-08-29T11:56:12Z"
```

## Summary

```text
SUMMARY
verdict: pass
issues_total: 6
issues_open: 0
issues_human: 0
issues_fixed: 6
iterations: 1
next_action: manual_review
```

The spec is structurally and semantically auditable. The next action is still human U0/reviewer confirmation; this audit does not authorize Git integration or release.
