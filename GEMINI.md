# GEMINI

This repo runs **CyberOS**. Canonical agent instructions: `.cyberos/AGENT-ENTRY.md`.

Root `AGENTS.md` is the thin CyberOS spine (not the memory protocol). Memory protocol: `.cyberos/memory/AGENTS.md`.

Work is tasks; HITL is required at the two human-acceptance gates; run gates with `bash .cyberos/cuo/gates/run-gates.sh`. Never push, deploy, or merge without an explicit operator instruction.

Multi-agent runs MUST follow `docs/project/MULTI_AGENT_PROTOCOL.md`. Only the main controller may spawn or close agents; workers and reviewers must not create descendants.
