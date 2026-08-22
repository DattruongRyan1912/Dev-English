# DevEnglish test log

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
