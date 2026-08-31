# DevEnglish S1 — Walking Skeleton Delivery Report

**Date:** 2026-08-28  
**Status:** Implemented and runtime-verified in local Docker/PostgreSQL  
**Scope:** `docs/project/REBUILD_PLAN.md`, S1 only

## Outcome

S1 now has one durable vertical slice:

`canonical workspace bootstrap → Today/Work/Knowledge shell → create Project → create Task → import manual Knowledge source → lexical search → grounded assistant response → citation/unknown response → PostgreSQL restart persistence`

The product reset is not complete. This is the first working walking skeleton; S2–S7 remain planned work.

## Implemented

### Backend

- Added application composition in `backend/internal/application/` for memory and PostgreSQL repositories.
- Added workspace-scoped REST v2 endpoints in `backend/internal/httpapi/v2.go`:
  - `GET /api/v2/bootstrap`
  - `POST /api/v2/projects`
  - `POST /api/v2/projects/{projectID}/tasks`
  - `POST /api/v2/knowledge/sources/manual`
  - `GET /api/v2/knowledge/search`
  - `POST /api/v2/assistant/conversations`
  - `POST /api/v2/assistant/conversations/{conversationID}/messages`
- Added durable assistant conversation/message storage, including the canonical response JSON.
- Added PostgreSQL repositories for S1 Project/Task reads and creates, Knowledge import/search, and assistant persistence.
- Kept workspace and user scope on every canonical query.
- Added idempotency replay for Project/Task creates and optimistic version fields in the persisted model.
- Added migrations `006_work.sql` through `009_knowledge_evidence_trigger_repair.sql`.
  - `008` repairs missing knowledge root workspace foreign keys on existing databases.
  - `009` repairs the knowledge evidence trigger/function without dropping revision history.

### Flutter

- Added `WorkspaceApi`, canonical workspace models, and `WorkspaceController` production data path.
- Wired the authenticated production shell to load canonical bootstrap data instead of silently substituting demo data.
- Wired Today, Work, Knowledge, and assistant citation/unknown states to the v2 API.
- Preserved the explicit demo path for isolated legacy/widget tests.
- Added API/model/controller coverage in `test/workspace_api_test.dart`.

### Correctness repair found during runtime smoke

The first real PostgreSQL POST exposed two persistence defects that unit tests did not cover:

1. The advisory-lock key passed a NUL-containing string to PostgreSQL `hashtext`, producing `SQLSTATE 22021`.
2. Custom Go enum values were passed to pgx without explicit string conversion.

The implementation now hashes the scoped idempotency key locally and sends enum values as strings. The new opt-in PostgreSQL repository regression test covers create, replay, history, and the corrected lock path.

## Verification evidence

All commands below were run from the repository root and returned exit code `0` unless stated otherwise.

| Gate | Result |
|---|---|
| `go build ./...` | Passed |
| `go vet ./...` | Passed |
| `go test ./...` | Passed |
| `go test -race ./backend/...` | Passed |
| `dart format --output=none --set-exit-if-changed lib test` | Passed |
| `flutter analyze` | Passed |
| `flutter test` | `19 passed, 2 skipped` |
| `flutter build web --dart-define=API_BASE_URL=http://localhost:8080` | Passed |
| `flutter build apk --debug` | Passed |
| CyberOS `run-gates.sh` | `GATES: GREEN` |

The CyberOS coverage command also passed. It reports package-level coverage separately; this report does not claim that every new package has reached the eventual S1/S2 coverage target.

## Docker/PostgreSQL runtime smoke

Verified against the local services from `infra/docker-compose.yml`:

- `GET /healthz` returned healthy.
- `GET /readyz` returned ready.
- `GET /api/v2/bootstrap` returned the canonical workspace payload.
- Project create returned `201`; replaying the same `Idempotency-Key` returned the same entity/version.
- Task create returned `201` and remained linked to the project.
- Manual source import created an immutable revision and chunk.
- Knowledge search returned the source-backed excerpt.
- Assistant returned a grounded response with evidence and, for an unsupported question, `grounding: "unknown"` with no evidence.
- After `docker compose restart backend`, bootstrap, search, project/task data, source/revision/chunk, and conversation data remained readable.

The smoke created deterministic local fixture rows in the default development workspace so the result can be inspected while developing. They are not production seed data.

## Deliberate S1 limits

These are not claimed as complete yet:

- Project/Task update, trash, restore, and purge routes.
- Google Drive/GitHub sync and external write confirmation.
- Embedding sidecar, hybrid/RRF retrieval, and 100k-chunk load evidence.
- Production provider routing, usage guard, and quota failure handling.
- Live MCP HTTP transport and REST/MCP parity tests.
- Push-to-talk, TTS, pronunciation, and adaptive Learning overlay.
- Full U0 product UI replacement, accessibility audit, golden screenshots, and device QA.
- Production authentication/cutover, deployment, commit, and push.

## Next safe step

Proceed to S2 only after reviewing this evidence and accepting the S1 boundary. The next implementation slice should complete Work mutations with `expectedVersion`/`idempotencyKey`, then add the corresponding REST/UI actions and conflict tests. No external connector or autonomous action should be added in that slice.

**Git state:** changes remain uncommitted and unpushed by design.
