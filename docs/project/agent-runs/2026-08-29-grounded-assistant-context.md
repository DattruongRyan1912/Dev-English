# Grounded assistant context continuation — 2026-08-29

## Scope

This bounded continuation adds a canonical context reference to the assistant
flow. A conversation may be started from one Project, Task, Decision or
Knowledge Source, and that reference is retained for subsequent turns. The
assistant resolves the reference inside the authenticated workspace before
retrieval; it does not trust a presentation label or accept a context switch
inside an existing conversation.

## Implementation

- `backend/internal/assistant/types.go` defines and validates `ContextRef`.
- `backend/internal/application/application.go` resolves canonical context,
  persists it, reuses it on continuation and rejects context changes with a
  conflict. Pinned evidence is first and the final retrieval set is capped at
  20 items.
- `backend/internal/application/conversations_memory.go` and
  `backend/internal/application/conversations_postgres.go` retain context
  across memory and PostgreSQL conversation reads.
- `infra/migrations/018_assistant_conversation_context.sql` adds the bounded
  context columns and check constraint.
- `backend/internal/httpapi/v2.go` accepts optional `contextType` and
  `contextId` fields for start/send assistant requests.
- Flutter Work and Knowledge actions send canonical IDs, and the assistant
  surface displays the pinned context without changing canonical Work or
  Knowledge data.

## Verification evidence

- Go: `543` tests passed across 23 packages; `543` race tests passed; vet and
  build passed.
- Flutter: analyze and web release build passed. The latest full/isolated test
  rerun stayed at the frontend compiler loading phase and was stopped with exit
  `130`; it is inconclusive, not a pass.
- PostgreSQL: migration `018` applied on the disposable coverage database;
  DB-enabled coverage total is `68.6%` with application `82.0%`, assistant
  `84.1%`, Knowledge `82.1%`, MCP `82.0%`, Work `84.5%`, connectors `72.2%`
  and HTTP API `62.8%`.
- Production-shaped Docker smoke passed with `19` migrations and the existing
  Work/MCP/Knowledge/auth/CORS/provider-failure checks.
- Fresh browser visual smoke rendered `DevEnglish` Today from `build/web` at
  `http://localhost:8094/`; the page was non-empty. Flutter's DOM snapshot
  exposed only its accessibility placeholder, so no DOM-semantic claim is
  made.

## Release boundary

This is implementation evidence, not a release approval. The 90% coverage
threshold, real-device/operator U0 verdict, independent review of this latest
source change, lifecycle acceptance and human authorization for Git/release
operations remain open. No commit, push, merge or deployment was performed.
