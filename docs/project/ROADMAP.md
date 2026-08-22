# DevEnglish roadmap

## Phase 0 — Personal Go-Live RC1 hardening

Status: **In progress — local P0 complete; deployment gates partial**

- Remove fake runtime metrics from production responses.
- Probe configured providers through real authenticated health requests.
- Make production auth/database/login/CORS requirements fail closed.
- Disable deterministic AI fallback and demo seeding in production.
- Done locally: live provider probes, encrypted Settings lifecycle, capability-specific probe tests, API-level voice chain, atomic writing persistence, migration tracking and guarded backup/restore rehearsal.
- Remaining: browser microphone evidence, real HTTPS deployment and production deployment evidence.

## Phase 1 — Local foundation

Status: **Done**

- Dockerised backend and PostgreSQL/pgvector.
- Environment-based provider configuration.
- Flutter web client running against the local backend.
- Release web serving and basic health/API smoke checks.

## Phase 2 — Core learning loop

Status: **In progress — text loop and live AI features verified**

- Run and verify: diagnostic → daily mission → writing feedback → retry.
- Persist mistake and vocabulary extraction.
- Verify review scheduling and progress updates after a completed mission.
- Verify live provider behavior for text, speech-to-text, pronunciation and TTS.
- Remaining: real browser microphone capture and an explicit SRS review-state transition check.

## Phase 3 — Product hardening

Status: **In progress — boundary controls added**

- Add automated API integration tests for the core learning loop.
- Keep the writing outcome and PostgreSQL learning-state update atomic.
- Add browser smoke tests for the four primary destinations. A manual smoke pass is complete; repeatable automation remains.
- Improve loading, empty, provider-error and microphone-permission states.
- Add usage/cost alerts and a repeatable local reset/seed workflow.

## Phase 4 — Production boundary

Status: **Not started**

- OAuth/private GitHub access and deeper project indexing.
- Object storage and retention jobs for audio.
- Embedding generation and retrieval ranking.
- Production secrets, observability, deployment and rollback procedures.
