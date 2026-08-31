# Dev-English task backlog

Source of truth for task state = each task's frontmatter `status`. This file indexes them. ONE backlog for ALL work: net-new features (`class: product`, the default) and hardening/refactor/audit-remediation (`class: improvement`) live here together — improvement is not a separate track and never gets a second backlog file. Tag improvement rows with `(improvement)`; untagged rows are product.

Task files live under `docs/tasks/`: flat (`TASK-001-slug.md`) for small repos, or grouped in subfolders by module for larger ones. `improvement/` is a normal subfolder there for cross-cutting hardening tasks.

The `ship-tasks` workflow reads this file, picks the first eligible task (`ready_to_implement` with all `depends_on` done), and drives it through the lifecycle. HITL is required: the agent halts at review acceptance and final acceptance for a recorded human verdict, and never sets `done` itself.

Lifecycle: draft -> ready_to_implement -> implementing -> ready_to_review -> reviewing -> ready_to_test -> testing -> done. Off-ramps: on_hold, closed, cannot_reproduce (`type: bug` only), duplicate (requires `duplicate_of:`). See `.cyberos/cuo/STATUS-REFERENCE.md`.

## ready_to_implement

- (none yet - add rows as `- [ready_to_implement] TASK-001-slug - title`; append `(improvement)` for hardening tasks)

## in flight

- (implementing / reviewing / testing tasks appear here)
- [ready_to_review] TASK-PRODUCT-001-ux-lock - Lock the product-reset UX and information architecture (improvement)
- [ready_to_review] TASK-OPS-001-agent-orchestration-loop-guard - Add multi-agent orchestration loop guard (improvement)
- [testing] TASK-CYB-001-cyberos-only-workflow - Standardize Dev-English on the CyberOS-only workflow (improvement)

## done

- (shipped tasks, for the audit trail)

## on_hold / closed

- [on_hold] TASK-PRODUCT-002-walking-skeleton - Waiting for TASK-PRODUCT-001 human review gate
- [on_hold] TASK-PRODUCT-003-work-depth - Waiting for TASK-PRODUCT-002 human review gate
- [on_hold] TASK-PRODUCT-004-knowledge-sync - Waiting for TASK-PRODUCT-003 human review gate
- [on_hold] TASK-PRODUCT-005-grounded-assistant - Waiting for TASK-PRODUCT-004 human review gate
- [on_hold] TASK-PRODUCT-006-learning-voice - Waiting for TASK-PRODUCT-005 human review gate
- [on_hold] TASK-PRODUCT-007-mcp-parity - Waiting for TASK-PRODUCT-006 human review gate
- [on_hold] TASK-PRODUCT-008-cutover-hardening - Waiting for TASK-PRODUCT-007 human review gate
