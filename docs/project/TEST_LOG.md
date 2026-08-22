# DevEnglish test log

## 2026-08-22 — Provider-backed speaking and database recovery

Environment:

- Docker backend and PostgreSQL/pgvector running locally with the configured providers.
- A generated 16 kHz mono WAV was used for the deterministic API portion of the voice test; no secret values were logged.

Verified:

- `POST /api/v1/speaking/transcribe` reached Groq Whisper Large V3 and returned a transcript.
- The transcript was persisted as a `SpeakingSession`.
- `POST /api/v1/speaking/assess` reached Azure pronunciation/prosody and changed the session to `evaluated` with a non-null pronunciation result.
- `POST /api/v1/speaking/synthesize` returned a playable MP3 file.
- The browser reached the Speaking screen, but the in-app browser did not transition after `Start recording`; real microphone permission/device capture remains unproven.
- `./scripts/db_migrate.sh` recorded `000_schema_migrations.sql` and `001_initial.sql` in `schema_migrations`.
- `db_backup.sh` produced a PostgreSQL custom-format dump of 50,565 bytes with mode `600`.
- `db_restore.sh` restored that dump into disposable database `devenglish_restore_test`; `users|7` and `schema_migrations|2` were observed before the disposable database was removed.
- Restore without `CONFIRM_RESTORE=YES` was rejected before any database command ran.

Implementation note:

- Groq requires a supported audio extension in the multipart filename. The STT client now derives the filename extension from the uploaded MIME type, so WAV/WebM/MP3-style requests are identified correctly.

Remaining evidence:

- A real HTTPS domain, DNS, certificate and deployment target are still required for production verification.

## 2026-08-22 — P0 production-boundary hardening

Implemented and statically verified:

- Production rejects missing `DATABASE_URL`, auth secret, login secret or CORS allowlist.
- Production skips local demo seeding and disables deterministic AI fallback.
- Configured CORS rejects origins outside the explicit allowlist.
- `/api/v1/settings/test` now performs authenticated provider probes instead of reporting configuration only.
- Runtime `vietnameseFallback` fields were removed because no persisted source-of-truth existed.

Checks:

- `go test ./...` — passed (`36` tests across `10` packages).
- `go vet ./...` — passed.
- `flutter analyze` — passed.

Not yet proven in this batch:

- Production process startup with real secrets and PostgreSQL.
- Live provider probe responses with the configured DeepSeek/Groq/Azure credentials.
- HTTPS deployment.

## 2026-08-23 — P0.1 production web authentication

Implementation and verification boundary:

- Removed the Flutter `API_TOKEN` compile-time path. The client now uses the browser credentialed HTTP client and never receives `DEVENGLISH_BOOTSTRAP_KEY`.
- Added `POST /api/v1/auth/login` with a server-side `DEVENGLISH_LOGIN_SECRET`; successful login sets a `Secure`, `HttpOnly`, `SameSite=Strict` cookie without returning a bearer token.
- Added `POST /api/v1/auth/logout`, server-side session revocation, session restore through `/api/v1/auth/me`, and an expiry/401 transition back to the login screen.
- Kept `POST /api/v1/auth/session` explicitly development/API-tooling-only; production returns `404` for that bootstrap route.
- Configured CORS credentials only for an explicit origin allowlist.

Checks:

- `go test ./...` — passed after adding anonymous rejection, cookie login, reload, logout revocation and production bootstrap-route tests.
- `go vet ./...` — passed.
- `flutter analyze` — passed.
- `flutter test` — passed.
- `go test -race ./internal/httpapi` — passed.
- `flutter build web --release --dart-define=DEVENGLISH_ENV=production --dart-define=API_BASE_URL=https://app.example.com` — passed; the resulting `build/web` artifact contained no `API_TOKEN`, `DEVENGLISH_BOOTSTRAP_KEY` or login-secret marker.
- `flutter build web --release --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://localhost:8080` — passed.
- `docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build` — passed; `GET /healthz` returned `200` and unauthenticated development `GET /api/v1/home` returned `200` without a client bearer token.

Not yet proven in this batch:

- Real browser login over HTTPS with a deployed certificate and domain; `Secure` cookie behavior still belongs to the P0.5 HTTPS verification gate.
- Multi-instance/shared persistence for session revocation; the current RC1 target is one production backend instance.

## 2026-08-22 — Local runtime smoke

Environment:

- Docker backend and PostgreSQL/pgvector running locally.
- Flutter release web client served on port `8093`.

Verified:

- `GET /healthz` returned HTTP `200`.
- `GET /api/v1/auth/me` returned HTTP `200` with a development session.
- `GET /api/v1/home` returned HTTP `200`.
- `GET /api/v1/practice` returned HTTP `200`.
- `GET /api/v1/review/due` returned HTTP `200`.
- `GET /api/v1/progress` returned HTTP `200`.
- `GET /api/v1/settings/test` returned HTTP `200`.
- `flutter build web --release` completed successfully.
- A fresh browser tab rendered the Home screen with the daily mission, review section, progress score and navigation bar.

Not yet verified end-to-end:

- Live Groq transcription and Azure pronunciation/TTS flow.
- Roleplay turns, Copilot generation and GitHub import.
- Review actions changing SRS state and progress metrics.

## 2026-08-22 — Writing mission continuation fix

Flow:

- Opened the daily technical writing mission in the release web client.
- Submitted a structured answer containing observed behavior, expected behavior, impact and next step.
- Waited for the backend evaluation response and inspected the resulting focus screen.

Observed:

- Backend persisted the attempt for `user-1` with score `89.30`.
- UI rendered `89 / 100`, changed progress from `1 / 2` to `2 / 2`, displayed `Next action: Continue to the next mission.`, and changed the bottom action to `Continue to next mission`.
- A fresh reload no longer showed the false demo warning while the slow initial mission request was still completing.

Code checks:

- `flutter analyze` — passed.
- `flutter test` — passed (`1` test).
- `flutter build web --release` with local API base URL and development session token — passed.

## Test evidence format

For future entries record:

- Date and environment.
- Exact command or user flow.
- Expected result.
- Observed result.
- Follow-up issue or link, if any.

## 2026-08-23 — RC1 hardening continuation

Implementation and verification boundary:

- Added AES-GCM envelope encryption for the DeepSeek secret, PostgreSQL `provider_secrets` storage, metadata-only settings endpoints and Flutter Settings controls.
- Added safe capability probes: DeepSeek configured model list, Groq configured STT model, Azure Pronunciation assessment endpoint and Azure TTS voice list.
- Added atomic PostgreSQL/memory writing outcome persistence for attempt, evaluation, mission completion, mistakes and learning-state update.
- Applied `002_provider_secrets.sql` to the existing local PostgreSQL volume.

Checks:

- `go test ./...` — passed.
- `flutter analyze` — passed (`No issues found`).
- `flutter test` — passed (`1` test).
- `bash -n scripts/db_migrate.sh scripts/db_restore.sh scripts/db_backup.sh` — passed.
- `git diff --check` — passed.
- `./scripts/db_migrate.sh` — passed; `schema_migrations` contains `000_schema_migrations.sql`, `001_initial.sql` and `002_provider_secrets.sql`; `provider_secrets` and `user_settings.deepseek_status` exist.
- Capability probe HTTP tests — passed with safe error-code assertions and no provider response-body leakage.

## 2026-08-23 — RC1 local runtime and browser smoke

Environment:

- Docker backend and PostgreSQL/pgvector running locally.
- `.env.local` now contains an ignored 32-byte `DEVENGLISH_SECRET_ENCRYPTION_KEY`; the value was not printed or committed.
- Flutter release web client served at `http://localhost:8093`.

Verified:

- `docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build backend` — passed; the rebuilt backend started with the encryption key present.
- `GET /healthz` — `200`; `GET /api/v1/settings` — DeepSeek connected and speech configured.
- `GET /api/v1/settings/test` — DeepSeek text generation, Groq speech-to-text, Azure Pronunciation assessment and Azure Neural TTS all returned healthy capability-specific checks with safe metadata.
- DeepSeek Settings lifecycle — set/test/remove, no raw key in responses, encrypted secret survived a backend restart, and the provider-secret row was removed cleanly.
- Live PostgreSQL learning loop — generated a daily mission, submitted a writing attempt, returned score `95.5`, and persisted one completed mission, one attempt and one evaluation.
- Live roleplay and Copilot requests — completed and persisted one roleplay usage record and one Copilot usage record.
- Fresh in-app browser tab — Home rendered after the Flutter web rebuild; Practice, Review, Progress and Settings navigation rendered; Settings eventually displayed `Connected` after provider checks completed.
- Speaking screen rendered with manual transcript fallback and a microphone entry point.

Not yet proven:

- The in-app browser microphone click did not transition from `Ready`; no permission prompt was accepted. A real Chrome/device permission test is still required.
- Real HTTPS/domain/certificate deployment, production deployment and private GitHub import remain outside the local boundary.

## 2026-08-23 — RC1 provider-backed API and PostgreSQL smoke

Environment:

- Docker backend and PostgreSQL/pgvector running locally with `.env.local` provider configuration.
- Command: `DEVENGLISH_LIVE_SMOKE=YES scripts/smoke_local.sh`.
- The script created one disposable authenticated user and removed it in an exit trap.

Verified:

- Diagnostic: 12 questions, CEFR `B2`, overall score `72`.
- Daily mission → writing attempt: evaluation score `89.3`.
- Work Context import generated vocabulary; Review submission persisted successfully.
- Roleplay conversation and one provider-backed turn returned a reply.
- Copilot returned `simple`, `natural` and `professional` outputs.
- Public GitHub README import succeeded for `octocat/Spoon-Knife`; private repository provisioning was not attempted.
- PostgreSQL counts for the disposable user included learning state, diagnostic result, missions, attempt, evaluation, work context, vocabulary, review, conversation and roleplay/Copilot AI usage rows.
- Web Speaking capture now selects the `record_web`-supported PCM16 stream and wraps the raw samples in a 16 kHz mono WAV before STT/pronunciation upload.

Additional checks:

- `flutter analyze` — passed.
- `flutter test` — passed (`3` tests, including Speaking controls/manual fallback and runtime guard flags).
- `flutter test --dart-define=DEVENGLISH_ENV=production` — passed (`3` tests; production requires auth and disables demo fallback).
- `bash -n scripts/db_backup.sh scripts/db_migrate.sh scripts/db_restore.sh scripts/smoke_local.sh` — passed.
- Targeted production CORS tests — passed; HTTP origins, URL decorations, userinfo, wildcard and extra preflight headers are rejected or not advertised.

Remaining:

- Real browser microphone permission/capture and playback evidence.
- Real HTTPS/domain/certificate deployment and production Postgres/auth/secret/backup/readiness verification.
