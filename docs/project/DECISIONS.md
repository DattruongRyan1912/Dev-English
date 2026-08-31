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

## D-005 — Production runtime boundary is explicit

Date: 2026-08-22

`DEVENGLISH_ENV=production` is a separate runtime boundary. It requires PostgreSQL, a 32+ character signing secret, a 16+ character login secret and an explicit CORS origin allowlist. The browser uses a server-issued `Secure`, `HttpOnly`, `SameSite=Strict` session cookie; bootstrap credentials remain development/API-tooling only and are never compiled into Flutter. It does not seed demo data and never silently replaces a failed primary AI request with deterministic output. Development keeps the deterministic fallback for local UI work.

## D-006 — Metrics need persisted evidence

Date: 2026-08-22

The Vietnamese fallback percentage was removed from API/domain/UI models because the repository had no event-level source of truth. A metric is not exposed until the underlying behavior is persisted and can be recomputed for the requested time window.

## D-007 — Migrations and recovery are explicit local operations

Date: 2026-08-22

Local PostgreSQL init scripts cover a new development volume. Production Compose has no init-directory mount; `scripts/db_migrate.sh` is the sole production migration path and applies the ordered SQL files with one transaction per migration while verifying the version row. `scripts/db_backup.sh` uses a private custom-format dump; `scripts/db_restore.sh` requires `CONFIRM_RESTORE=YES`, an exact `CONFIRM_RESTORE_TARGET` database name and a live `current_database()` match before restoring. Production additionally requires an explicit allow flag.

## D-008 — HTTPS requires a real deployment target

Date: 2026-08-22

The repository includes a Caddy reverse-proxy template, but HTTPS is not marked verified without a real domain, DNS, certificate and deployed backend. The production CORS allowlist must contain the final HTTPS origin.

## D-009 — Provider status is capability-specific

Date: 2026-08-23

Provider checks report only safe metadata: configured, reachable, healthy, capability, model, latency and an error code. Groq validates the configured STT model, Azure Pronunciation probes the pronunciation assessment endpoint separately from Azure TTS voice validation, and provider response bodies are never returned by the settings API.
