---
id: TASK-CYB-001
title: Standardize Dev-English on the CyberOS-only workflow
template: task@1
type: improvement
module: cyberos
status: testing
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-25T22:15:27+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: not_ai
client_visible: false
depends_on: []
routed_back_count: 0
awh: N/A
service: .
new_files:
  - docs/tasks/cyberos/TASK-CYB-001-cyberos-only-workflow/spec.md
  - docs/tasks/cyberos/TASK-CYB-001-cyberos-only-workflow/audit.md
  - docs/tasks/cyberos/TASK-CYB-001-cyberos-only-workflow/implementation.md
  - docs/tasks/cyberos/TASK-CYB-001-cyberos-only-workflow/review.md
  - docs/tasks/cyberos/TASK-CYB-001-cyberos-only-workflow/review-findings.json
  - docs/tasks/cyberos/TASK-CYB-001-cyberos-only-workflow/review.audit.md
  - scripts/verify_cyberos_only.sh
modified_files:
  - AGENTS.md
  - CLAUDE.md
  - docs/tasks/BACKLOG.md
  - .agents/README.md
  - .agents/manifest.yaml
  - .agents/context/project.md
  - .agents/policies/change-control.md
  - .agents/policies/quality-gates.md
  - .agents/roles/luna-max-implementer.md
  - .agents/roles/sol-ultra-reviewer.md
  - .agents/workflows/change-delivery.md
  - .agents/workflows/hotfix.md
  - .claude/skills/gitnexus
---

# TASK-CYB-001: Standardize Dev-English on the CyberOS-only workflow

## Summary

Make CyberOS the sole active agent workflow for Dev-English while retaining the installed MCP adapters, project-aware machine gates, task corpus, BRAIN, and application WIP.

## Problem

The root agent entrypoint currently loads both the legacy Dev-English governance system and CyberOS. The legacy manifest still selects `workflows/change-delivery.md` as its default, so an agent can choose two different lifecycles and any effectiveness benchmark is confounded.

## Proposed Solution

Retire the legacy `.agents` governance files and project-local GitNexus skill bundle from the active checkout after preserving an exact external backup. Reduce root agent entrypoints to CyberOS pointers, retain only CyberOS-owned `.agents` adapters/rules/skills, and add an executable verifier that fails if a competing workflow reappears.

## Alternatives Considered

- Keep both systems and document precedence. Rejected because two lifecycle/state models remain active and benchmark attribution stays ambiguous.
- Build a bridge workflow between the two systems. Rejected because the operator explicitly requested a CyberOS-only system rather than composition.
- Remove all `.agents` and `.claude` content. Rejected because CyberOS itself uses those directories for MCP compatibility, rules, and generated skills.

## Success Metrics

- Baseline: the repository advertises both the legacy manifest workflow and CyberOS; target: every active root/project agent entrypoint resolves only to CyberOS by the human review gate.
- Baseline: no project-owned CyberOS-only invariant check exists; target: `rtk bash scripts/verify_cyberos_only.sh` exits zero by the human review gate.
- Baseline: the task backlog contains no real task; target: this migration is indexed and reaches the CyberOS review acceptance gate with reproducible evidence in this task folder.

## Scope

The migration is limited to agent orchestration, its task evidence, and a deterministic verifier.

### In scope

- Root `AGENTS.md` and `CLAUDE.md` agent entrypoints.
- Active `.agents` and `.claude/skills` project surfaces.
- Backup, deterministic verification, reinstall stability, MCP discovery, and project-aware gates.
- CyberOS task, audit, review, status, and benchmark evidence for this migration.

### Out of scope

- Application source, product behavior, release configuration, deployment, commit, push, merge, or production mutation.
- Changes to global user skills or global MCP registrations.
- Upstream CyberOS source fixes.

## Dependencies

- Installed CyberOS `v1.13.0` at `.cyberos/`.
- Existing project MCP adapters and project-aware `.cyberos/config.yaml` gate overrides.
- Recoverable pre-migration backup at `/Users/ryantruong/.agents/backups/20260825T151527Z-dev-english-cyberos-only/`.
- No external service, credential, quota, or team dependency.

## AI Authorship Disclosure

- Tools used: Codex with local filesystem and shell verification.
- Scope: re-derived and CONFIRMED: active entrypoints, workflow references, installed CyberOS paths, and current repository state; re-derived and CORRECTED: the earlier assumption that CyberOS was already the default workflow; measured and ADDED: the legacy-surface inventory, invariant verifier, reinstall hashes, MCP smoke, and gate evidence. The audit's own figures are treated as claims to verify.
- Human review: the operator must review the exact migration diff at `reviewing -> ready_to_test` and later record final acceptance at `testing -> done`.

## 1. Description

- 1.1 The active root and project agent entrypoints MUST direct agents to `.cyberos/AGENT-ENTRY.md` without requiring the retired Dev-English manifest, roles, policies, templates, reviews, or workflows.
- 1.2 The active `.agents` directory MUST contain only the CyberOS MCP adapter, CyberOS rule pointer, generated CyberOS skills, and CyberOS ownership marker.
- 1.3 The active project `.claude/skills` directory MUST contain only CyberOS-generated skills; the project-local GitNexus bundle MUST NOT remain active.
- 1.4 The migration MUST preserve an exact recoverable backup and MUST NOT alter application WIP outside the declared agent/task/verification cone.
- 1.5 Reinstalling the pinned CyberOS payload MUST preserve the CyberOS-only entrypoints, adapters, local gate override, and hooks without reintroducing legacy files.
- 1.6 Static verification, MCP initialization/discovery, and the configured project-aware machine gates MUST pass, while coverage output MUST NOT be represented as proof that the upstream 90 percent threshold is enforced.

## 2. Acceptance criteria

- [ ] AC 1 — Root and project entrypoints contain no legacy workflow reference and resolve to CyberOS. (traces_to: §1 #1; test: `scripts/verify_cyberos_only.sh::entrypoints`)
- [ ] AC 2 — `.agents` has only the allowed CyberOS surface. (traces_to: §1 #2; test: `scripts/verify_cyberos_only.sh::agents_surface`)
- [ ] AC 3 — `.claude/skills` has only valid CyberOS skill links and no GitNexus bundle. (traces_to: §1 #3; test: `scripts/verify_cyberos_only.sh::claude_surface`)
- [ ] AC 4 — Backup receipt exists and application-WIP path inventory matches the pre-migration baseline. (traces_to: §1 #4; test: `scripts/verify_cyberos_only.sh::backup_and_scope`)
- [ ] AC 5 — Two final reinstall snapshots have identical hashes for protected config and hooks. (traces_to: §1 #5; verify: compare the recorded hash snapshots in `review.md`)
- [ ] AC 6 — JSON/TOML, symlink, MCP, and project-aware gates pass with the coverage limitation recorded. (traces_to: §1 #6; test: `scripts/verify_cyberos_only.sh::static_runtime`; verify: inspect `review.md` runtime evidence)

## 3. Edge cases

- Installer regeneration must not restore a managed marker that makes `AGENTS.md` ownership ambiguous.
- Ignored generated skill links must remain valid even though they do not appear as trackable candidates.
- Case-insensitive `changelog.md`/`CHANGELOG.md` handling must remain untouched.
- Existing application WIP and untracked Android/deployment/test files must not be removed, formatted, staged, or rewritten.
- The verifier must fail closed on unexpected active files instead of silently accepting a second workflow.
- Absence of AGY project-MCP discovery and unenforced coverage must remain explicit limitations, not green claims.

## 4. Protected invariants this task must not weaken

- The operator remains the only authority for commit, push, merge, deploy, destructive actions, and both CyberOS human acceptance gates.
- Existing application WIP remains user-owned and outside the migration cone.
- Project-aware Go, Flutter, and content gates remain configured in `.cyberos/config.yaml`.
- CyberOS BRAIN and task history remain intact.
- No secret or credential value may enter logs, task artifacts, backups, or status pages.
