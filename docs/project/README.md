# DevEnglish project documentation

This folder is the working record for project scope, delivery status, decisions and verification evidence.

## Documents

- [`REQUIREMENTS.md`](REQUIREMENTS.md) — product goals and agreed V1 requirements.
- [`STATUS.md`](STATUS.md) — current implementation state, priorities and blockers.
- [`ROADMAP.md`](ROADMAP.md) — delivery phases and remaining work.
- [`REBUILD_PLAN.md`](REBUILD_PLAN.md) — source-verified product reset, end-to-end delivery and full UI rebuild plan.
- [`MCP_CLIENT_SETUP.md`](MCP_CLIENT_SETUP.md) — product MCP handoff for Codex/Claude, token scopes and operator acceptance.
- [`DECISIONS.md`](DECISIONS.md) — dated technical and product decisions.
- [`TEST_LOG.md`](TEST_LOG.md) — commands, smoke checks and manual-test results.

## Update rules

1. Update `STATUS.md` whenever a feature changes state.
2. Add a dated entry to `TEST_LOG.md` for every meaningful verification.
3. Record decisions in `DECISIONS.md` instead of burying them in chat history.
4. Keep secrets, API keys, bearer tokens and private source data out of this folder.
5. Use these states consistently: `Done`, `In progress`, `Staged`, `Blocked` and `Not started`.

## Current local development boundary

- Flutter web client: `http://localhost:8093`
- Go backend: `http://localhost:8080`
- PostgreSQL/pgvector: `localhost:5433`
- Provider configuration: ignored `.env.local`
