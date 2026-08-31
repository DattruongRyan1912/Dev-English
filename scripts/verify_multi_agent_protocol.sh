#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

python3 - "$repo_root" <<'PY'
from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys

root = Path(sys.argv[1])
protocol = root / "docs/project/MULTI_AGENT_PROTOCOL.md"
runs = root / "docs/project/agent-runs"


def fail(case: str, message: str) -> None:
    raise SystemExit(f"FAIL {case}: {message}")


def passed(case: str) -> None:
    print(f"PASS {case}")


if not protocol.is_file():
    fail("protocol", "missing docs/project/MULTI_AGENT_PROTOCOL.md")
text = protocol.read_text()
for number in range(1, 11):
    marker = f"ORCH-{number:03d}"
    if marker not in text:
        fail("protocol", f"missing invariant {marker}")
for phrase in (
    "Only the main controller may call",
    "fork_context: false",
    "Timeout is not failure",
    "Exactly one reviewer may be active",
    "Do not call spawn_agent",
):
    if phrase not in text:
        fail("protocol", f"missing required rule: {phrase}")
passed("protocol")

for entrypoint in ("AGENTS.md", "CLAUDE.md", "GEMINI.md"):
    path = root / entrypoint
    if "docs/project/MULTI_AGENT_PROTOCOL.md" not in path.read_text():
        fail("entrypoints", f"{entrypoint} does not load the protocol")
passed("entrypoints")

allowed_statuses = {
    "planned", "pending_init", "running", "completed", "errored",
    "interrupted", "shutdown", "blocked", "not_found",
}
active_statuses = {"pending_init", "running"}
run_files = sorted(runs.glob("*.json"))
if not run_files:
    fail("manifests", "no agent run manifest found")

for path in run_files:
    data = json.loads(path.read_text())
    schema = data.get("schema")
    if schema == "devenglish-file-cone@1":
        base = data.get("base")
        if not isinstance(base, str) or not base.strip():
            fail("file_cone", f"{path.name}: missing base commit")
        try:
            actual_diff = subprocess.run(
                ["git", "-C", str(root), "diff", "--name-only", base],
                check=True,
                text=True,
                capture_output=True,
            ).stdout.splitlines()
            actual_untracked = subprocess.run(
                ["git", "-C", str(root), "ls-files", "--others", "--exclude-standard"],
                check=True,
                text=True,
                capture_output=True,
            ).stdout.splitlines()
        except subprocess.CalledProcessError as exc:
            fail("file_cone", f"{path.name}: cannot enumerate candidate paths: {exc.stderr.strip()}")
        actual = sorted(set(actual_diff + actual_untracked))
        entries = data.get("paths")
        if not isinstance(entries, list) or not entries:
            fail("file_cone", f"{path.name}: paths must be a non-empty list")
        declared: list[str] = []
        for entry in entries:
            if not isinstance(entry, dict):
                fail("file_cone", f"{path.name}: each path entry must be an object")
            candidate = entry.get("path")
            owners = entry.get("owners")
            if not isinstance(candidate, str) or not candidate or Path(candidate).is_absolute() or ".." in Path(candidate).parts:
                fail("file_cone", f"{path.name}: invalid relative path {candidate!r}")
            if not isinstance(owners, list) or not owners or any(not isinstance(owner, str) or not owner.strip() for owner in owners):
                fail("file_cone", f"{path.name}: {candidate!r} must name at least one owner")
            if candidate in declared:
                fail("file_cone", f"{path.name}: duplicate path {candidate!r}")
            declared.append(candidate)
        if sorted(declared) != actual:
            missing = sorted(set(actual) - set(declared))
            extra = sorted(set(declared) - set(actual))
            fail("file_cone", f"{path.name}: path set mismatch; missing={missing[:5]!r} extra={extra[:5]!r}")
        passed("file_cone")
        continue
    if schema == "devenglish-wip-preservation@1":
        passed("preservation_manifest")
        continue
    if schema != "devenglish-agent-run@1":
        fail("manifests", f"{path.name}: unsupported schema")
    agents = data.get("agents", [])
    ids: set[str] = set()
    active_tasks: set[str] = set()
    active_workers = 0
    active_reviewers = 0
    for agent in agents:
        agent_id = agent.get("agentId")
        task_id = agent.get("taskId")
        role = agent.get("role")
        status = agent.get("status")
        if not agent_id or agent_id in ids:
            fail("manifests", f"{path.name}: missing or duplicate agentId {agent_id!r}")
        ids.add(agent_id)
        if status not in allowed_statuses:
            fail("manifests", f"{path.name}: invalid status {status!r}")
        if agent.get("canSpawn") is not False:
            fail("manifests", f"{path.name}: {agent_id} may spawn descendants")
        if status in active_statuses:
            if agent.get("parentAgentId") is not None:
                fail("manifests", f"{path.name}: active descendant {agent_id}")
            if task_id in active_tasks:
                fail("manifests", f"{path.name}: duplicate active task {task_id}")
            active_tasks.add(task_id)
            if role == "worker":
                active_workers += 1
            elif role == "reviewer":
                active_reviewers += 1
            else:
                fail("manifests", f"{path.name}: unknown role {role!r}")
    if active_workers > data.get("maxWorkers", 4):
        fail("manifests", f"{path.name}: worker concurrency exceeded")
    if active_reviewers > data.get("maxReviewers", 1) or active_reviewers > 1:
        fail("manifests", f"{path.name}: reviewer concurrency exceeded")
    accepted = data.get("acceptedReviewerAgentId")
    if accepted is not None:
        matching = [a for a in agents if a.get("agentId") == accepted and a.get("role") == "reviewer"]
        if len(matching) != 1 or matching[0].get("status") != "completed":
            fail("manifests", f"{path.name}: accepted reviewer is not completed")
    if data.get("state") == "paused" and (active_workers or active_reviewers):
        fail("manifests", f"{path.name}: paused/attention run still has active agents")
    if data.get("state") == "attention":
        if data.get("newSpawnAllowed") is not False:
            fail("manifests", f"{path.name}: attention run may spawn new agents")
        attention_id = data.get("attentionAgentId")
        attention_matches = [
            a for a in agents
            if a.get("agentId") == attention_id and a.get("status") in active_statuses
        ]
        if len(attention_matches) != 1:
            fail("manifests", f"{path.name}: attention agent is not the same running agent")

passed("manifests")
PY
