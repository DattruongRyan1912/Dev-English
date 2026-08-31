#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backup_root="${CYBEROS_BACKUP_ROOT:-$repo_root/.cyberos-backup}"
hooks_root="$(git -C "$repo_root" rev-parse --git-path hooks)"
if [[ "$hooks_root" != /* ]]; then
  hooks_root="$repo_root/$hooks_root"
fi

python3 - "$repo_root" "$backup_root" "$hooks_root" <<'PY'
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import sys
import tomllib

root = Path(sys.argv[1])
backup = Path(sys.argv[2])
hooks = Path(sys.argv[3])


def fail(case: str, message: str) -> None:
    raise SystemExit(f"FAIL {case}: {message}")


def passed(case: str) -> None:
    print(f"PASS {case}")


required_pointer = ".cyberos/AGENT-ENTRY.md"
legacy_tokens = (
    "Dev-English delivery governance",
    ".agents/README.md",
    ".agents/manifest.yaml",
    "gitnexus",
    "luna-max",
    "sol-ultra",
    "workflows/change-delivery.md",
)
for relative in ("AGENTS.md", "CLAUDE.md", "GEMINI.md"):
    path = root / relative
    if not path.is_file():
        fail("entrypoints", f"missing {relative}")
    text = path.read_text()
    if required_pointer not in text:
        fail("entrypoints", f"{relative} does not point to {required_pointer}")
    lowered = text.lower()
    for token in legacy_tokens:
        if token.lower() in lowered:
            fail("entrypoints", f"{relative} still contains legacy token {token!r}")
if "cyberos-agent-spine (managed by cyberos" in (root / "AGENTS.md").read_text():
    fail("entrypoints", "AGENTS.md contains the v1.13.0 ownership marker")
passed("entrypoints")

expected_agent_top = {"mcp_config.json", "rules", "skills"}
actual_agent_top = {path.name for path in (root / ".agents").iterdir()}
if actual_agent_top != expected_agent_top:
    fail("agents_surface", f"unexpected .agents entries: {sorted(actual_agent_top ^ expected_agent_top)}")
if {path.name for path in (root / ".agents/rules").iterdir()} != {"cyberos.md"}:
    fail("agents_surface", ".agents/rules is not CyberOS-only")
expected_shared_skills = {
    ".cyberos-owned",
    "ship-tasks",
    "task-author",
    "task-audit",
    "inspection-report-author",
    "inspection-report-audit",
    "harden-record-author",
    "harden-record-audit",
}
shared_skills = root / ".agents/skills"
actual_shared_skills = {path.name for path in shared_skills.iterdir()}
if actual_shared_skills != expected_shared_skills:
    fail("agents_surface", f"unexpected shared skill entries: {sorted(actual_shared_skills ^ expected_shared_skills)}")
for name in expected_shared_skills - {".cyberos-owned"}:
    path = shared_skills / name
    if not path.is_symlink() or not path.exists():
        fail("agents_surface", f"invalid shared CyberOS skill link: {name}")
passed("agents_surface")

claude_root = root / ".claude"
if {path.name for path in claude_root.iterdir()} != {"skills"}:
    fail("claude_surface", ".claude contains a non-CyberOS top-level entry")
expected_claude_skills = expected_shared_skills - {".cyberos-owned"}
actual_claude_skills = {path.name for path in (claude_root / "skills").iterdir()}
if actual_claude_skills != expected_claude_skills:
    fail("claude_surface", f"unexpected Claude skill entries: {sorted(actual_claude_skills ^ expected_claude_skills)}")
for name in expected_claude_skills:
    path = claude_root / "skills" / name
    if not path.is_symlink() or not path.exists():
        fail("claude_surface", f"invalid Claude CyberOS skill link: {name}")
passed("claude_surface")

codex_root = root / ".codex"
if {path.name for path in codex_root.iterdir()} != {"config.toml", "skills"}:
    fail("static_runtime", ".codex contains a non-CyberOS entry")
if {path.name for path in (codex_root / "skills").iterdir()} != {"ship-tasks"}:
    fail("static_runtime", ".codex/skills is not CyberOS-only")
codex_skill = codex_root / "skills/ship-tasks"
if not codex_skill.is_symlink() or not codex_skill.exists():
    fail("static_runtime", "Codex ship-tasks skill link is invalid")

with (root / ".mcp.json").open() as handle:
    root_mcp = json.load(handle)
with (root / ".agents/mcp_config.json").open() as handle:
    agents_mcp = json.load(handle)
with (root / ".codex/config.toml").open("rb") as handle:
    codex_config = tomllib.load(handle)
expected_args = [".cyberos/mcp/cyberos-mcp.mjs"]
for name, config in (("root", root_mcp), ("agents", agents_mcp)):
    server = config.get("mcpServers", {}).get("cyberos", {})
    if server.get("command") != "node" or server.get("args") != expected_args:
        fail("static_runtime", f"{name} MCP adapter does not match the CyberOS stdio server")
server = codex_config.get("mcp_servers", {}).get("cyberos", {})
if server.get("command") != "node" or server.get("args") != expected_args:
    fail("static_runtime", "Codex MCP adapter does not match the CyberOS stdio server")
cyberos_root = root / ".cyberos"
if cyberos_root.is_symlink():
    fail("static_runtime", ".cyberos must be self-contained, not an absolute symlink")
if (cyberos_root / "VERSION").read_text().strip() != "1.13.0":
    fail("static_runtime", "unexpected installed CyberOS version")
if "gates:" not in (root / ".cyberos/config.yaml").read_text():
    fail("static_runtime", "project-aware gate override is absent")
for hook in ("pre-commit", "commit-msg"):
    if not (hooks / hook).is_file():
        fail("static_runtime", f"missing CyberOS {hook} hook")
passed("static_runtime")

receipt = backup / "README.md"
manifest = backup / "application-wip.sha256"
if not receipt.is_file() or not manifest.is_file():
    fail("backup_and_scope", "backup receipt or application-WIP manifest is missing")
preservation_path = root / "docs/project/agent-runs/20260830-final-v1-preservation.json"
allowed_mutations = {}
if preservation_path.is_file():
    try:
        preservation = json.loads(preservation_path.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        fail("backup_and_scope", f"invalid preservation manifest: {exc}")
    if preservation.get("schema") != "devenglish-wip-preservation@1":
        fail("backup_and_scope", "unsupported preservation manifest schema")
    for entry in preservation.get("entries", []):
        relative = entry.get("path")
        expected = entry.get("expectedSha256")
        actual = entry.get("actualSha256")
        reason = entry.get("reason")
        if not isinstance(relative, str) or not relative or Path(relative).is_absolute() or ".." in Path(relative).parts:
            fail("backup_and_scope", f"invalid preservation path: {relative!r}")
        if not isinstance(expected, str) or len(expected) != 64 or not all(char in "0123456789abcdef" for char in expected):
            fail("backup_and_scope", f"invalid expected hash for {relative!r}")
        if not isinstance(actual, str) or len(actual) != 64 or not all(char in "0123456789abcdef" for char in actual):
            fail("backup_and_scope", f"invalid actual hash for {relative!r}")
        if not isinstance(reason, str) or not reason.strip():
            fail("backup_and_scope", f"missing mutation reason for {relative!r}")
        if relative in allowed_mutations:
            fail("backup_and_scope", f"duplicate preservation entry: {relative}")
        allowed_mutations[relative] = (expected, actual)
backup_paths = set()
for line in manifest.read_text().splitlines():
    expected, relative = line.split(" ", 1)
    backup_paths.add(relative)
    path = root / relative
    if not path.is_file():
        fail("backup_and_scope", f"pre-existing WIP path disappeared: {relative}")
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    if actual != expected:
        allowed = allowed_mutations.get(relative)
        if allowed != (expected, actual):
            fail("backup_and_scope", f"pre-existing WIP path changed without an approved mutation record: {relative}")
for relative in allowed_mutations:
    if relative not in backup_paths:
        fail("backup_and_scope", f"preservation manifest names a non-WIP path: {relative}")
passed("backup_and_scope")
PY
