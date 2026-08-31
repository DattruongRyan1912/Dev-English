# TASK-CYB-001 implementation evidence

## Batch selection

`batch-selection@1` selected only `TASK-CYB-001`; `swarm_required` was `false` and no eligible task was excluded.

## Repository context map

| Surface | Before migration | CyberOS-only target |
| --- | --- | --- |
| Root entry | Legacy GitNexus and Dev-English governance plus CyberOS pointer | Thin CyberOS pointer |
| Claude entry | Project-specific legacy instructions | Thin CyberOS pointer |
| `.agents` | Manifest, context, policies, reviews, roles, templates, workflows plus CyberOS adapters | MCP adapter, CyberOS rule, generated skills only |
| `.claude/skills` | GitNexus bundle plus CyberOS skills | CyberOS skills only |
| Runtime | CyberOS v1.13.0, local gate override, hooks, MCP | Preserved |
| Product WIP | Existing modified/untracked application work | Byte-preserved outside the migration cone |

The change spans more than three agent-configuration domains, so the architecture decision is explicit below rather than implicit in file deletion.

## Architecture decision

Dev-English uses CyberOS as the sole active AI-delivery workflow. Legacy governance remains recoverable outside the repository at `/Users/ryantruong/.agents/backups/20260825T151527Z-dev-english-cyberos-only/`, but no active entrypoint references it.

Consequences:

- CyberOS task frontmatter and `docs/tasks/BACKLOG.md` become the only lifecycle state.
- CyberOS HITL gates replace the retired Luna/Sol state machine.
- Project-specific gate commands remain valid through `.cyberos/config.yaml`.
- Project-local GitNexus skills are retired; global tools remain outside this repository's active workflow.
- Reintroducing a second agent workflow causes `scripts/verify_cyberos_only.sh` to fail.

## Edge-case matrix

| Case | Risk | Control | Result before review |
| --- | --- | --- | --- |
| Installer sees a managed ownership marker | Rewrites the root file on reinstall | Canonical pointer intentionally omits the marker | Static verifier passes |
| Legacy file survives under an unexpected `.agents` path | Second workflow remains active | Complete top-level and child allowlists | Static verifier passes |
| GitNexus survives as a Claude skill | Claude can route outside CyberOS | Exact Claude skill allowlist | Static verifier passes |
| Generated symlink is ignored but broken | Agent skill silently unavailable | Require symlink and resolved target | Static verifier passes |
| Migration alters product WIP | User work is damaged or benchmark is invalid | SHA-256 manifest for every pre-existing modified application path | Static verifier passes |
| Backup is missing | Retired configuration cannot be recovered | Require backup receipt and WIP manifest | Static verifier passes |
| MCP JSON/TOML drifts | Agent starts without CyberOS tools | Parse and assert exact stdio adapter | Static verifier passes |
| Gate override disappears | Root autodetection returns false-green coverage | Require live `gates:` override | Static verifier passes |
| Coverage remains below 90 percent | CyberOS reports a false-green threshold | Report coverage separately; never equate gate exit with threshold proof | Open upstream limitation, not hidden |
| AGY ignores project MCP | AGY does not expose CyberOS tools | Keep adapter but qualify AGY as unverified; Codex/Claude are runtime targets | Open host limitation, not hidden |

## Implementation plan and execution

1. Back up all active agent surfaces, hooks, task/status artifacts, runtime configuration, BRAIN, and tracked WIP.
2. Create and audit `TASK-CYB-001`, then use CyberOS state tools for lifecycle transitions.
3. Replace `AGENTS.md` and `CLAUDE.md` with canonical CyberOS pointers.
4. Retire the exact legacy `.agents` governance paths and `.claude/skills/gitnexus` via recoverable Trash operations.
5. Add `scripts/verify_cyberos_only.sh` with fail-closed active-surface and WIP-preservation checks.
6. Reinstall the pinned payload twice, compare protected hashes, smoke MCP, run configured gates, and prepare the human review packet.

## Audit of the plan

- Scope is confined to agent orchestration, task evidence, and verification.
- The backup precedes every retirement action.
- No commit, push, merge, deploy, secret, product source edit, or destructive hard delete is included.
- Every normative clause in the task has a corresponding acceptance check.
- The verifier checks complete directory sets rather than selected legacy filenames.

## Observability disposition

No application runtime path changes, so log/metric/trace injection is not applicable. The migration's observable state is the task lifecycle, BRAIN audit chain, deterministic verifier output, reinstall hashes, MCP handshake, gate output, and generated status page.
