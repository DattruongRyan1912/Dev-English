# DevEnglish project status

Last updated: 2026-08-23

Overall state: **In progress — local V1 core is runnable; P0.1 web authentication is implemented, RC1 is not ready**

## Current runtime

- Flutter release web build is served at `http://localhost:8093`.
- Go backend runs in Docker at `http://localhost:8080`.
- PostgreSQL/pgvector runs in Docker and is exposed locally on port `5433`.
- The local provider configuration is loaded from ignored `.env.local`.

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

## In progress

- [ ] Complete the browser microphone leg of speaking: the Speaking screen renders, but microphone capture/permission did not transition under the in-app browser.
- [ ] Complete P0.2 DeepSeek secret-management hardening before moving to provider or database follow-up work.
- [ ] Exercise the new login/logout flow against a real HTTPS deployment; local HTTP cannot validate a `Secure` cookie.
- [ ] Verify roleplay, Copilot, GitHub import and SRS state transitions with real user actions.
- [ ] Add repeatable automated browser/API smoke coverage for the core learning loop.
- [ ] Complete the remaining P0 production checks: real HTTPS deployment and browser microphone evidence.

## Staged or not started

- [ ] Complete UX for practice modes that currently show a staged/coming-soon message.
- [ ] OAuth and private GitHub repository provisioning.
- [ ] Deeper GitHub crawling/project indexing.
- [ ] Object storage for retained audio and embedding generation/retrieval ranking.
- [ ] Production deployment, observability and release runbook.

## Current risks

- The local release web client must be rebuilt after Flutter source changes; production builds no longer accept `API_TOKEN`.
- Initial mission generation and writing evaluation can take 12–60 seconds with the configured provider; the client now uses longer timeouts for those endpoints.
- Production sessions are single-instance in-memory revocable sessions; a future multi-instance deployment needs a shared session store.
- The browser microphone path still needs a real browser/device permission test; the API-level voice chain is verified locally.
- AI usage cost values remain service-side estimates and are not a substitute for provider billing data.
- No production DNS, certificate or hosting target has been provided, so HTTPS deployment remains unverified.
