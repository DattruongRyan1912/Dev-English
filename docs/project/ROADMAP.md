# DevEnglish roadmap

## Phase 1 — Local foundation

Status: **Done**

- Dockerised backend and PostgreSQL/pgvector.
- Environment-based provider configuration.
- Flutter web client running against the local backend.
- Release web serving and basic health/API smoke checks.

## Phase 2 — Core learning loop

Status: **In progress**

- Run and verify: diagnostic → daily mission → writing feedback → retry.
- Persist mistake and vocabulary extraction.
- Verify review scheduling and progress updates after a completed mission.
- Verify live provider behavior for text, speech-to-text, pronunciation and TTS.

## Phase 3 — Product hardening

Status: **Not started**

- Add automated API integration tests for the core learning loop.
- Add browser smoke tests for the four primary destinations.
- Improve loading, empty, provider-error and microphone-permission states.
- Add usage/cost alerts and a repeatable local reset/seed workflow.

## Phase 4 — Production boundary

Status: **Not started**

- OAuth/private GitHub access and deeper project indexing.
- Object storage and retention jobs for audio.
- Embedding generation and retrieval ranking.
- Production secrets, observability, deployment and rollback procedures.

