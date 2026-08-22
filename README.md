# DevEnglish

DevEnglish is a focus-first technical English learning system for developers. It turns real work into a repeatable loop:

`Real Work → AI Simulation → Feedback → Mistake Memory → Next Mission`

The implementation is based on the six project documents in the Google Drive folder `DevEnglish - IT English Learning System`:

- Product & Learning System
- Technical Architecture & Data
- AI Content Factory & Prompt System
- AI Providers, Voice & Cost Plan
- MVP Scope & Roadmap
- UI/UX Design System & Screen Rules

Project tracking is maintained in [`docs/project/`](docs/project/README.md). Start with the [current status](docs/project/STATUS.md), then update the [roadmap](docs/project/ROADMAP.md) and [test log](docs/project/TEST_LOG.md) as work progresses.

## What is implemented

### Backend

- Go modular monolith with deterministic fallback behavior only in development; production fails closed when the primary provider is unavailable.
- Adaptive daily mission, writing evaluation, retry feedback, mistake extraction and deterministic skill updates.
- SRS review schedule: 1d → 3d → 7d → 14d → 30d.
- Diagnostic questions with CEFR/skill result and recommended plan.
- Learn From My Work with secret-pattern rejection, domain/term extraction and persisted mission/vocabulary.
- Roleplay scenarios, conversation persistence and one-focused-follow-up turns, including stand-up, code review, system design and technical interview modes.
- GitHub repository/README, Issue and Pull Request import with changed-file summaries, URL allowlisting and optional backend token support.
- English Copilot with simple/natural/professional rewrites.
- Groq Whisper Large V3 STT, Azure Pronunciation Assessment/Prosody and Azure Neural TTS provider boundaries.
- Speaking-session metadata with raw-audio expiry metadata; audio bytes are never written to the database.
- Usage/cost records, configurable monthly budget and a hard 300,000 VND ceiling.
- Personal vocabulary graph, seven-day learning analytics and weekly speaking assessment recommendations.
- Provider connection status endpoint for DeepSeek, Groq Whisper and Azure speech capabilities with authenticated health probes, without exposing secrets.
- HMAC token verification, HttpOnly web sessions, per-user repository scoping and authenticated data export/deletion boundary.
- PostgreSQL/pgvector migration with tables for users, learning state, missions, attempts, evaluations, mistakes, vocabulary, conversations, speaking sessions, imported work and AI usage.

### Flutter client

- Mobile-first light design system with focus mode and four bottom destinations: Home, Practice, Review and Progress.
- Onboarding diagnostic, daily writing mission, feedback/retry, work/GitHub import, speaking recorder/transcript fallback, roleplay simulators, Copilot, vocabulary graph, SRS review, analytics and provider settings views.
- `record` microphone streaming and `audioplayers` TTS playback are wired behind the backend API.
- Provider API keys and authentication secrets are never stored in the client. Production web access uses a server-issued HttpOnly session cookie.

### Content factory

- Deterministic catalog validator and report command.
- 39 exercise types, 24 roleplay roles, 4 rubrics, 24 grammar/communication concepts, 10 prompts and 5 JSON schemas.
- 116 derived skill units from the skill/CEFR/task matrix.
- Generated deterministic evaluation fixture with 200 cases.

## Run locally

### Memory mode

```bash
go run ./backend/cmd/server
```

The API starts at `http://localhost:8080`. Without `DATABASE_URL`, it uses a seeded in-memory repository.
Memory mode is intentionally single-user (`user-1`) for local UI work; use PostgreSQL mode for authenticated multi-user isolation.

### PostgreSQL/pgvector mode

```bash
docker compose -f infra/docker-compose.yml up -d
export DATABASE_URL='postgres://devenglish:devenglish-local-only@localhost:5433/devenglish?sslmode=disable'
go run ./backend/cmd/server
```

The migration is mounted into the database container. The production boundary expects the `pgvector/pgvector:pg16` image; do not use the local password outside development.

### Provider configuration

```bash
export DEVENGLISH_ENV="development"
export DEVENGLISH_ALLOWED_ORIGINS="http://localhost:8093"
export DEEPSEEK_API_KEY="..."
export DEEPSEEK_FAST_MODEL="deepseek-v4-flash"
export DEEPSEEK_SMART_MODEL="deepseek-v4-pro"
export GROQ_API_KEY="..."
export AZURE_SPEECH_KEY="..."
export AZURE_SPEECH_REGION="..."
# Optional: leave blank to derive the official regional STT/TTS endpoints.
export AZURE_SPEECH_STT_BASE_URL=""
export AZURE_SPEECH_TTS_BASE_URL=""
```

Copy `.env.example` for the complete list. AI output is schema-validated with one bounded retry. The backend owns final writing/speaking scores, SRS transitions, mastery, due selection, quotas and usage records.

### Authenticated mode

Set a random secret of at least 32 characters:

```bash
export DEVENGLISH_AUTH_SECRET="a-long-random-secret-at-least-32-characters"
export DEVENGLISH_BOOTSTRAP_KEY="local-bootstrap-key" # development/API tooling only
export DEVENGLISH_LOGIN_SECRET="a-random-production-login-secret"
```

For development/API tooling, create a bearer session with `POST /api/v1/auth/session` and send `Authorization: Bearer <token>`. The Flutter client does not embed that token or the bootstrap key; development mode allows the local single-user flow without a client secret.

For a production process, set `DEVENGLISH_ENV=production`, provide `DATABASE_URL`, a 32+ character `DEVENGLISH_AUTH_SECRET`, a 16+ character `DEVENGLISH_LOGIN_SECRET`, and a comma-separated `DEVENGLISH_ALLOWED_ORIGINS` allowlist. The browser signs in through `POST /api/v1/auth/login`; the backend returns a `Secure`, `HttpOnly`, `SameSite=Strict` cookie and never returns a bearer token to the browser. `DEVENGLISH_BOOTSTRAP_KEY` is not a production browser credential and must never be compiled into Flutter. Production does not seed demo data, does not use deterministic AI fallback, and rejects startup when this boundary is incomplete.

### Local Docker development

The recommended local development boundary is Flutter on the host, with the Go backend and PostgreSQL running in Docker. Put provider secrets in the ignored `.env.local` file (copied from `.env.example`), then run:

```bash
docker compose --env-file .env.local -f infra/docker-compose.yml up -d --build
docker compose --env-file .env.local -f infra/docker-compose.yml logs -f backend
```

The backend is available at `http://localhost:8080`. Inside Compose, it connects to PostgreSQL through `postgres:5432`; do not use `localhost` for `DATABASE_URL` inside the container. Run Flutter on the host with:

```bash
flutter run -d chrome --dart-define=DEVENGLISH_ENV=development --dart-define=API_BASE_URL=http://localhost:8080
```

Stop the local stack with `docker compose --env-file .env.local -f infra/docker-compose.yml down`. This stops and removes the containers but keeps the named PostgreSQL volume.

### Migrations and database recovery

The Compose init directory is only applied automatically when PostgreSQL initializes a new volume. For an existing local volume, apply the tracked migrations explicitly:

```bash
make db-migrate
```

Create a private custom-format backup outside the repository when testing recovery:

```bash
BACKUP_DIR=/tmp/devenglish-backups make db-backup
```

Restore is destructive and requires an explicit confirmation. Always verify the target database before running it:

```bash
BACKUP_FILE=/tmp/devenglish-backups/devenglish-<timestamp>.dump \
CONFIRM_RESTORE=YES make db-restore
```

Production restore additionally requires `ALLOW_PRODUCTION_RESTORE=YES`. Backup artifacts belong outside Git; the repository ignores `backups/` for accidental local output.

### HTTPS deployment boundary

`infra/Caddyfile.example` is the reverse-proxy template for a real domain. Set `DEVENGLISH_DOMAIN` to that domain, proxy to the backend, and configure `DEVENGLISH_ALLOWED_ORIGINS` with the matching `https://...` origin. HTTPS is not considered verified until DNS, the certificate and a real deployment target have been exercised.

## Flutter

```bash
flutter pub get
flutter run -d chrome --dart-define=API_BASE_URL=http://localhost:8080
```

In development, an unavailable backend enables an inspectable demo state for UI work. A production build must pass `--dart-define=DEVENGLISH_ENV=production`; it fails closed with a retry screen instead of showing demo scores, analytics or provider health.

## API surface

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/healthz` | Service/provider health |
| POST | `/api/v1/auth/session` | Create a development/API-tooling bearer session |
| POST | `/api/v1/auth/login` | Create the production HttpOnly browser session |
| POST | `/api/v1/auth/logout` | Revoke and clear the production browser session |
| GET | `/api/v1/auth/me` | Current authenticated user |
| GET | `/api/v1/home` | Mission, learning state and due count |
| GET | `/api/v1/practice` | Practice launcher |
| GET/POST | `/api/v1/diagnostic...` | Diagnostic questions/result |
| POST | `/api/v1/missions/daily` | Generate and persist an adaptive mission |
| POST | `/api/v1/missions/{id}/attempts` | Submit writing and receive feedback |
| POST | `/api/v1/work-context` | Analyze safe work context |
| POST | `/api/v1/integrations/github/import` | Import a GitHub repository, README, Issue or Pull Request |
| GET/POST | `/api/v1/review...` | Due/library review and SRS update |
| GET | `/api/v1/vocabulary` | Personal technical vocabulary |
| GET | `/api/v1/vocabulary/graph` | Related vocabulary nodes and edges |
| GET/POST | `/api/v1/roleplay...` | Scenarios, conversations and turns |
| POST | `/api/v1/copilot` | Three-level English rewrite |
| POST | `/api/v1/speaking/transcribe` | Groq STT multipart endpoint |
| POST | `/api/v1/speaking/transcript` | Manual/local transcript fallback |
| POST | `/api/v1/speaking/assess` | Azure pronunciation/prosody assessment |
| POST | `/api/v1/speaking/synthesize` | Azure Neural TTS audio |
| GET | `/api/v1/speaking/weekly` | Weekly speaking/pronunciation assessment |
| GET | `/api/v1/analytics` | Seven-day learning analytics |
| GET | `/api/v1/settings/test` | Provider configuration status |
| GET | `/api/v1/usage` | Monthly usage/cost summary |
| GET | `/api/v1/privacy/export` | User-scoped data export |
| DELETE | `/api/v1/privacy/data` | Authenticated user-scoped deletion |

## Content factory

```bash
go run ./backend/cmd/contentfactory --root .
go run ./backend/cmd/contentfactory --root . --write-evaluation-cases tests/evaluation_cases.generated.json --target 200
```

The factory validates duplicates, required catalogs and structured evaluation fixtures before content is promoted to runtime seed data.

## Checks

```bash
gofmt -w backend
go test ./...
go vet ./...
dart format lib test
flutter analyze
flutter test
flutter build web --release
```

## Deliberate scope boundary

The current repository implements the core V1, V1.5 and V2 learning loop in a modular monolith. The remaining production boundary is additive: OAuth/private-repository provisioning, deeper GitHub crawling and project indexing, object-storage upload/download for retained audio, and embedding generation/retrieval ranking. The database already has pgvector and imported-source boundaries so those additions do not require a rewrite of the learning core.
