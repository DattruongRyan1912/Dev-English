# DevEnglish project status

Last updated: 2026-08-23

Overall state: **In progress — P0–P5 artifacts and local production-mode gates are verified; P5 external deployment is blocked on a real target/domain**

## Current runtime

- Flutter release web build is served at `http://localhost:8093`.
- Go backend runs in Docker at `http://localhost:8080`.
- PostgreSQL/pgvector runs in Docker and is exposed locally on port `5433`.
- The local provider configuration is loaded from ignored `.env.local`.
- The ignored local environment contains `DEVENGLISH_SECRET_ENCRYPTION_KEY` for durable server-side secret storage.

## Done

- [x] Flutter shell with Home, Practice, Review and Progress destinations.
- [x] Daily technical writing mission and focus screen.
- [x] Diagnostic flow and result summary.
- [x] English Copilot with Simple, Natural and Professional outputs.
- [x] AI roleplay flows for general roleplay, system design and technical interview.
- [x] Speaking recording/transcript fallback and provider boundaries for STT, pronunciation and TTS.
- [x] Learn From My Work and GitHub source import boundary.
- [x] Vocabulary list/graph and spaced-repetition review UI.
- [x] Progress and weekly speaking analytics views.
- [x] Provider connection status and pronunciation setting.
- [x] Docker development topology and local release-web serving.
- [x] Writing submission state transition: feedback renders, progress reaches `2 / 2`, and a passing result continues to the next mission.
- [x] Removed the hard-coded Vietnamese fallback metric from the runtime API and Progress UI.
- [x] Added real authenticated provider probes for DeepSeek, Groq Whisper and Azure Speech.
- [x] Added production startup gates for database, auth/login secrets, CORS allowlist and no-demo-seed/no-fallback behavior.
- [x] Added tracked migration versions plus guarded local database backup/restore scripts.
- [x] Verified the provider-backed speaking API chain: Groq transcription, persisted session, Azure pronunciation/prosody assessment and Azure TTS output.
- [x] Replaced production build-time bearer authentication with a server-issued `Secure`, `HttpOnly`, `SameSite=Strict` browser session.
- [x] Added production login, session restoration, expiry handling, logout revocation and a fail-closed Flutter login state.
- [x] Configured the Flutter web HTTP client to send session credentials cross-origin without embedding provider or auth secrets.
- [x] Added encrypted server-side DeepSeek secret lifecycle with safe metadata-only settings responses and hot reload.
- [x] Added capability-specific provider probes with safe errors, latency, configured model and Azure Pronunciation/TTS separation.
- [x] Added an atomic PostgreSQL writing-outcome transaction and an atomic memory-store equivalent for local tests.
- [x] Rebuilt the local Docker backend with the RC1 hardening batch and verified the encrypted DeepSeek lifecycle against PostgreSQL.
- [x] Verified live capability probes for DeepSeek, Groq Whisper, Azure Pronunciation and Azure Neural TTS.
- [x] Verified the live PostgreSQL learning loop, one roleplay turn and one Copilot request.
- [x] Manually smoke-tested browser rendering and navigation for Home, Practice, Review, Progress and Settings.
- [x] Added a repeatable provider-backed API/PostgreSQL smoke script covering diagnostic, mission/writing, work import, review, roleplay, Copilot and public GitHub import; the disposable user is cleaned up automatically.
- [x] Added explicit Speaking permission, recording, upload, transcription, assessment, TTS playback, success and retry state labels plus a Flutter widget regression test; web capture uses PCM16 streaming wrapped as WAV for Groq/Azure.
- [x] Hardened production CORS to HTTPS-only explicit origins, credentialed cookies and a minimal preflight header allowlist; targeted Go tests pass.
- [x] Verified the real Chrome speaking path: microphone permission, recording, Groq STT, PostgreSQL session persistence, Azure pronunciation/prosody assessment and Azure TTS playback.
- [x] Verified the browser/API/PostgreSQL learning loops: diagnostic state, work-context/GitHub mission and vocabulary, writing evaluation/mistake memory/skill update, SRS review, roleplay and Copilot.
- [x] Increased the Work Context client timeout to 90 seconds for provider-backed analysis and added an API-client regression test.
- [x] Added a repeatable Chrome widget smoke for Home, Practice, Review and Progress and wired it into CI.
- [x] P4 delivery controls are active: feature branch `chore/rc1-timeout-browser-smoke`, draft PR #1, protected `main`, backend-aware Chrome smoke and a real migration-runner regression gate.
- [x] Added PostgreSQL-backed `/readyz`, a production Flutter/Caddy image, hardened production Compose topology and a guarded production runbook.
- [x] Ran a local production-mode container smoke: `/healthz`, `/readyz`, login, HttpOnly session lookup/logout and HTTPS-origin CORS passed.
- [x] Added an opt-in PostgreSQL repository integration test and a disposable PostgreSQL CI job covering per-user isolation and the atomic writing outcome.

## In progress

- [x] Complete the browser microphone leg of speaking with a real Chrome permission/capture test.
- [ ] Exercise the new login/logout flow against a real HTTPS deployment; local HTTP cannot validate a `Secure` cookie.
- [x] Add repeatable automated API/PostgreSQL smoke coverage for the core learning loop.
- [x] Add repeatable automated Chrome smoke coverage for the four primary destinations.
- [ ] Verify the private GitHub import path after a GitHub token and repository access are deliberately provisioned.
- [ ] Complete the remaining P0 production check: real HTTPS deployment.

## Staged or not started

- [ ] Complete UX for practice modes that currently show a staged/coming-soon message.
- [ ] OAuth and private GitHub repository provisioning.
- [ ] Deeper GitHub crawling/project indexing.
- [ ] Object storage for retained audio and embedding generation/retrieval ranking.
- [ ] Execute production deployment, external observability and release/rollback checks on the real target; the runbook is now tracked.

## Current risks

- The local release web client must be rebuilt after Flutter source changes; production builds no longer accept `API_TOKEN`.
- Provider-backed mission, writing and work-context generation can take 12–60 seconds; the client now uses longer timeouts for those endpoints.
- Production sessions are single-instance in-memory revocable sessions; a future multi-instance deployment needs a shared session store.
- The in-app browser did not transition its microphone state; the real Chrome path is verified, so the remaining browser coverage is a second device/browser when practical.
- AI usage cost values remain service-side estimates and are not a substitute for provider billing data.
- No production DNS, certificate or hosting target has been provided, so HTTPS deployment remains unverified.
- The local production-mode container smoke validates the runtime boundary but cannot prove public DNS, certificate issuance, external backup retention or off-host observability.
