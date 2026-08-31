# DevEnglish

DevEnglish is an AI work assistant for developers with an adaptive English
learning overlay. It keeps work data canonical while turning real workflow
context into small, useful English practice:

`Work capture → sourced knowledge → grounded assistant → safe action → English overlay`

The implementation is based on the six project documents in the Google Drive folder `DevEnglish - IT English Learning System`:

- Product & Learning System
- Technical Architecture & Data
- AI Content Factory & Prompt System
- AI Providers, Voice & Cost Plan
- MVP Scope & Roadmap
- UI/UX Design System & Screen Rules

Project tracking is maintained in [`docs/project/`](docs/project/README.md). Start with the [current status](docs/project/STATUS.md), then update the [roadmap](docs/project/ROADMAP.md) and [test log](docs/project/TEST_LOG.md) as work progresses.

The current product target is the `Today | Work | Knowledge | Learning` shell
described in [`docs/project/REBUILD_PLAN.md`](docs/project/REBUILD_PLAN.md).
The old Home/Practice/Review/Progress experience remains only as a reversible
compatibility surface for retained learning logic.

## What is implemented in the product reset

### Canonical V1 workflow

- Today briefing with priorities, next action, work-derived mission and the
  text-first assistant;
- workspace-scoped Project, Task and Decision records with versions, history,
  trash/restore and guarded purge;
- sourced Knowledge with immutable revisions, chunks, claims, evidence, exact
  search and optional `multilingual-e5-small` hybrid retrieval;
- read-only incremental Drive/GitHub sync with cursors, tombstones and
  workspace isolation;
- grounded/inferred/unknown/stale assistant responses with citations, usage
  accounting and bounded context;
- confirmed GitHub issue/comment/label actions with challenge, idempotency and
  receipt boundaries;
- workflow-derived Learning overlay with Vietnamese help, editable voice
  transcript and on-demand TTS;
- Streamable HTTP MCP at `/mcp` with separate bearer tokens, scopes, resources,
  tools and REST-parity application services.

### Retained learning and provider capabilities

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

- Mobile-first light design system with the canonical Today, Work, Knowledge
  and Learning destinations, responsive desktop navigation and explicit
  loading/empty/error/offline/conflict states.
- The retained diagnostic, daily writing mission, feedback/retry, work/GitHub
  import, speaking recorder/transcript fallback, roleplay, Copilot, vocabulary
  graph, SRS, analytics and provider settings views remain available through
  the Learning compatibility surface.
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

The local migration is mounted into the database container for a new development volume. Production does not use that init mount; follow the [production runbook](docs/project/PRODUCTION_RUNBOOK.md) and run `scripts/db_migrate.sh` explicitly. The production boundary expects the `pgvector/pgvector:pg16` image; do not use the local password outside development.

### Provider configuration

```bash
export DEVENGLISH_ENV="development"
export DEVENGLISH_MODULES="all" # or: platform,work,knowledge
export DEVENGLISH_ALLOWED_ORIGINS="http://localhost:8093"
export DEEPSEEK_API_KEY="..."
export DEEPSEEK_FAST_MODEL="deepseek-v4-flash"
export DEEPSEEK_SMART_MODEL="deepseek-v4-pro"
# Server-side envelope-encryption key for the Settings UI; keep it only in .env.local.
export DEVENGLISH_SECRET_ENCRYPTION_KEY="a-random-secret-at-least-32-characters"
export GROQ_API_KEY="..."
export AZURE_SPEECH_KEY="..."
export AZURE_SPEECH_REGION="..."
# Optional: leave blank to derive the official regional STT/TTS endpoints.
export AZURE_SPEECH_STT_BASE_URL=""
export AZURE_SPEECH_TTS_BASE_URL=""
```

Google Drive sync is read-only. For a short local smoke, set
`GOOGLE_DRIVE_ACCESS_TOKEN`. For a long-running backend, configure
`GOOGLE_DRIVE_CLIENT_ID`, `GOOGLE_DRIVE_CLIENT_SECRET` and
`GOOGLE_DRIVE_REFRESH_TOKEN` from an OAuth consent flow whose scope is
`https://www.googleapis.com/auth/drive.readonly`; the backend refreshes the
access token in memory and never stores or logs it. `GOOGLE_DRIVE_API_BASE_URL`
defaults to `https://www.googleapis.com/drive/v3`.

Copy `.env.example` for the complete list. AI output is schema-validated with one bounded retry. The backend owns final writing/speaking scores, SRS transitions, mastery, due selection, quotas and usage records.

The Settings screen can store a DeepSeek key without returning it to the client. Set, replace, test or remove it through the UI; the backend stores only authenticated ciphertext and exposes status/model metadata. Generate a local encryption key with `openssl rand -hex 32`, put the result in the ignored `.env.local`, and rebuild the backend. Production requires this variable explicitly and does not derive it from another secret.

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

To test the Android app from a phone that is not on the same Wi-Fi, keep
Tailscale running on the Mac, install Tailscale on the phone and sign in to
the same tailnet. The APK helper prefers an explicit `API_BASE_URL`, then a
Tailscale IPv4 address, and finally the local Wi-Fi address. It checks the
backend before building so the APK is not produced with an unreachable
endpoint:

```bash
TAILSCALE_IP="$(tailscale ip -4 | head -n 1)" \
  DEVENGLISH_APK_OUTPUT="$HOME/Desktop/DevEnglish-tailscale.apk" \
  scripts/android_local_apk.sh
```

The resulting APK calls `http://<tailscale-ip>:8080`; this is a private
tailnet development path, not public internet exposure. The phone must keep
Tailscale connected, and the Mac, Docker backend and embedding sidecar must
remain running. For a public or production phone build, use the HTTPS Caddy
origin instead of exposing port `8080`.

To repeat the local Android emulator smoke without sending a provider request,
start a booted emulator and run:

```bash
ANDROID_APK_PATH="$HOME/Desktop/DevEnglish-tailscale.apk" \
  scripts/android_emulator_smoke.sh
```

The smoke installs the APK, opens Today, walks the four-destination shell and
assistant composer in the accessibility tree, captures `today.png`,
`work.png`, `knowledge.png`, `learning.png`, `today-assistant.png`,
`window.xml` and app-process `logcat.txt` in a temporary evidence directory,
and fails on an Android/Dart crash signature. Set `ANDROID_DEVICE_ID` to
choose a specific ADB device or `ANDROID_SMOKE_OUTPUT` to choose the evidence
directory. This does not replace physical-device, microphone or cross-network
acceptance.

Run the repeatable local RC1 API/database smoke only when provider usage is intended. It creates a disposable authenticated user, exercises the core learning loop, roleplay, Copilot and public GitHub import, checks PostgreSQL rows, then removes that user:

```bash
DEVENGLISH_LIVE_SMOKE=YES scripts/smoke_local.sh
```

Run the repeatable Chrome UI smoke for the four primary destinations. The backend and PostgreSQL must be running first, PostgreSQL must be healthy and the tracked migrations must be applied (`make db-migrate`). The explicit flags below enable the backend-aware smoke; without them the test is intentionally skipped.

```bash
flutter test test/browser_smoke_test.dart -d chrome \
  --dart-define=DEVENGLISH_BROWSER_SMOKE=true \
  --dart-define=INTEGRATION_TEST_SHOULD_REPORT_RESULTS_TO_NATIVE=false \
  --dart-define=DEVENGLISH_ENV=development \
  --dart-define=API_BASE_URL=http://127.0.0.1:8080
```

`All tests skipped.` is not a successful smoke result. A valid run must execute the browser test against the backend-backed PostgreSQL state.

Stop the local stack with `docker compose --env-file .env.local -f infra/docker-compose.yml down`. This stops and removes the containers but keeps the named PostgreSQL volume.

### Migrations and database recovery

The local Compose init directory is only applied automatically when PostgreSQL initializes a new volume. Production has no init-directory mount and always uses the authoritative runner. For an existing local volume, apply the tracked migrations explicitly:

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
CONFIRM_RESTORE=YES CONFIRM_RESTORE_TARGET=devenglish make db-restore
```

The restore script verifies `current_database()` against `CONFIRM_RESTORE_TARGET` before invoking `pg_restore`. Production restore additionally requires `ALLOW_PRODUCTION_RESTORE=YES`. Backup artifacts belong outside Git; the repository ignores `backups/` for accidental local output.

### HTTPS deployment boundary

`infra/Caddyfile.example` is the local reverse-proxy template. The production topology is in `infra/docker-compose.production.yml`, with `Dockerfile.web`, `infra/Caddyfile.production` and `infra/production.env.example`. Follow [the production runbook](docs/project/PRODUCTION_RUNBOOK.md) on a real host. `/healthz` is liveness; `/readyz` checks PostgreSQL readiness. HTTPS is not considered verified until DNS, the certificate and a real deployment target have been exercised.

## Flutter

```bash
flutter pub get
flutter run -d chrome --dart-define=API_BASE_URL=http://localhost:8080
```

In development, an unavailable backend enables an inspectable demo state for UI work. A production build must pass `--dart-define=DEVENGLISH_ENV=production`; it fails closed with a retry screen instead of showing demo scores, analytics or provider health.

## API surface

### Product-reset API and MCP

The canonical shell uses the authenticated `/api/v2/*` surface for workspace,
Work, Knowledge, connector sync, assistant, learning observations, actions and
MCP-token lifecycle. The complete route registration is in
`backend/internal/httpapi/v2.go`.

| Surface | Purpose |
| --- | --- |
| `GET /api/v2/bootstrap` | Load the workspace-scoped Today/Work/Knowledge/Learning snapshot |
| `/api/v2/projects`, `/api/v2/tasks`, `/api/v2/decisions` | Versioned Work CRUD, history and trash/restore/purge |
| `/api/v2/knowledge/*` | Manual sources, source detail, revisions, claims and search |
| `/api/v2/connectors/*/sync` | Read-only Drive/GitHub incremental synchronization |
| `/api/v2/assistant/conversations/*` | Persistent grounded text conversation |
| `/api/v2/actions/*` | Challenge, confirmation and receipt flow |
| `POST /api/v2/mcp/tokens` | Issue an application-bound MCP bearer token once |
| `DELETE /api/v2/mcp/tokens/{tokenID}` | Revoke an MCP token for the authenticated workspace |
| `POST /mcp` | Streamable HTTP MCP resources/tools with bearer scope checks |

For Codex/Claude product-client setup, token issuance, scope minimization,
network boundaries and the operator checklist, see
[MCP client setup](docs/project/MCP_CLIENT_SETUP.md). The tracked
`.mcp.json` and `.codex/config.toml` remain CyberOS governance
configuration; add the product MCP as a separate client entry.

The `/api/v1/*` table below is retained for compatibility and for the legacy
learning surfaces.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/healthz` | Service/provider health |
| GET | `/readyz` | PostgreSQL-backed readiness check |
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
| GET | `/api/v1/settings` | Safe settings metadata and DeepSeek status |
| PUT | `/api/v1/settings` | Validate and save model/budget settings |
| PUT | `/api/v1/settings/deepseek` | Encrypt, save, hot-load and test a DeepSeek key; never returns the key |
| POST | `/api/v1/settings/deepseek/test` | Run a capability/model probe without changing the key |
| DELETE | `/api/v1/settings/deepseek` | Remove the stored DeepSeek key |
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
go test -race ./...
go vet ./...
dart format lib test
flutter analyze
flutter test
flutter test --dart-define=DEVENGLISH_ENV=production
flutter build web --release
docker compose --env-file infra/production.env.example -f infra/docker-compose.production.yml config --quiet
```

## Deliberate scope boundary

The local repository contains implementation evidence through S7. Remaining
release work is acceptance and environment-specific: operator U0/device
walkthrough, CyberOS human lifecycle decisions, one fresh independent review,
GitHub CI on the delivery commit, live provider/billing reconciliation and a
real production target with DNS, HTTPS, backups and secrets. These are not
claimed from local tests.

The following remain intentionally out of scope for V1: Drive writes, GitHub
push/merge/code mutation, realtime full-duplex voice, OCR/scanned-document
ingestion, autonomous actions without confirmation and a dynamic third-party
plugin marketplace.
