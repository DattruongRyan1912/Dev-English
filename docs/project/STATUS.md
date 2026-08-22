# DevEnglish project status

Last updated: 2026-08-22

Overall state: **In progress — local V1 core is runnable**

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

## In progress

- [ ] Complete an end-to-end speaking session: record/transcribe, assess pronunciation and play TTS.
- [ ] Verify roleplay, Copilot, GitHub import and SRS state transitions with real user actions.
- [ ] Add repeatable automated browser/API smoke coverage for the core learning loop.

## Staged or not started

- [ ] Complete UX for practice modes that currently show a staged/coming-soon message.
- [ ] OAuth and private GitHub repository provisioning.
- [ ] Deeper GitHub crawling/project indexing.
- [ ] Object storage for retained audio and embedding generation/retrieval ranking.
- [ ] Production deployment, observability and release runbook.

## Current risks

- The local release web client must be rebuilt after Flutter source changes.
- Initial mission generation and writing evaluation can take 12–60 seconds with the configured provider; the client now uses longer timeouts for those endpoints.
- The current local web setup passes a development bearer token at build time; do not expose this build publicly.
- Manual AI and microphone flows still need live-provider verification beyond read-only API smoke checks.
