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

- Production rejects missing `DATABASE_URL`, auth secret, bootstrap key or CORS allowlist.
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
