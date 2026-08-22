# DevEnglish roadmap

## Phase 0 — Personal Go-Live RC1 hardening

Status: **In progress — local P0 hardening, production-mode smoke and deployment artifacts complete; real target gate remains**

- Remove fake runtime metrics from production responses.
- Probe configured providers through real authenticated health requests.
- Make production auth/database/login/CORS requirements fail closed.
- Disable deterministic AI fallback and demo seeding in production.
- Done locally: live provider probes, encrypted Settings lifecycle, capability-specific probe tests, API-level voice chain, atomic writing persistence, migration tracking, guarded backup/restore rehearsal, production-mode container auth/readiness and production web/Caddy packaging.
- Remaining: real HTTPS deployment and production deployment evidence.

## Phase 1 — Local foundation

Status: **Done**

- Dockerised backend and PostgreSQL/pgvector.
- Environment-based provider configuration.
- Flutter web client running against the local backend.
- Release web serving and basic health/API smoke checks.

## Phase 2 — Core learning loop

Status: **Done locally — text, SRS and browser voice loops verified against PostgreSQL**

- Run and verify: diagnostic → daily mission → writing feedback → retry.
- Persist mistake and vocabulary extraction.
- Verify review scheduling and progress updates after a completed mission.
- Verify live provider behavior for text, speech-to-text, pronunciation and TTS.
- Real Chrome microphone capture, speaking assessment/TTS playback and an SRS mastery/next-review transition are verified.

## Phase 3 — Product hardening

Status: **In progress — boundary controls, API smoke and Chrome smoke added**

- Add automated API integration tests for the core learning loop.
- Keep the writing outcome and PostgreSQL learning-state update atomic.
- Add browser smoke tests for the four primary destinations. A repeatable Chrome smoke and API/database smoke are complete.
- Improve loading, empty, provider-error and microphone-permission states. Speaking state labels and retry paths are now explicit; real permission/device evidence remains.
- Add usage/cost alerts and a repeatable local reset/seed workflow.

## Phase 4 — Production boundary

Status: **Deployment artifacts ready — real production target is still required**

- OAuth/private GitHub access and deeper project indexing.
- Object storage and retention jobs for audio.
- Embedding generation and retrieval ranking.
- Production secrets, observability, deployment and rollback procedures are documented in `docs/project/PRODUCTION_RUNBOOK.md`; execution still requires the real target.
