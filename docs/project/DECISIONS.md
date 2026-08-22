# DevEnglish decisions

## D-001 — Local Docker boundary

Date: 2026-08-22

During development, PostgreSQL/pgvector and the Go backend run in Docker while Flutter runs on the host. This keeps the database and backend environment reproducible while preserving fast client iteration.

- Flutter: `localhost:8093`
- Backend: `localhost:8080`
- PostgreSQL: `localhost:5433` from the host, `postgres:5432` inside Compose

## D-002 — Provider responsibilities

Date: 2026-08-22

The backend keeps provider-specific code behind service boundaries:

- DeepSeek for text generation/evaluation.
- Groq Whisper for speech-to-text.
- Azure Speech for pronunciation/prosody assessment and neural TTS.

The backend remains responsible for final scores, persistence, quotas and usage records.

## D-003 — Secrets stay outside the repository

Date: 2026-08-22

Provider keys and local bootstrap/auth values belong in ignored `.env.local`. Documentation may describe variable names and setup, but must never contain secret values or bearer tokens.

## D-004 — Verification evidence is separate from implementation status

Date: 2026-08-22

`STATUS.md` describes what is delivered; `TEST_LOG.md` records what was actually run and observed. A configured provider or an implemented endpoint is not treated as an end-to-end feature pass until the relevant user flow is exercised.

