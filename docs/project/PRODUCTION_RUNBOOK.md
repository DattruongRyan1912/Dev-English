# DevEnglish production runbook

This runbook is the deployment boundary for the RC1 Docker topology. It is intentionally separate from the local development stack. The repository does not contain production credentials, a domain or a hosting account; those are supplied by the operator.

## Target topology

- Caddy serves the production Flutter web bundle, terminates HTTPS and proxies `/api/*`, `/healthz` and `/readyz` to the backend.
- The Go backend runs with `DEVENGLISH_ENV=production`, PostgreSQL/pgvector and no deterministic fallback or demo state.
- PostgreSQL is private to the Compose network and is migrated explicitly before the backend is exposed.
- Caddy and the backend write structured logs to container stdout; the host must forward and retain them according to its operations policy.

## Required operator inputs

Before deployment, provide all of the following:

1. A DNS name whose A/AAAA records point to the Docker host, with inbound TCP 80 and 443 allowed.
2. A real PostgreSQL/pgvector persistence target, or the production PostgreSQL service in `infra/docker-compose.production.yml` with durable storage and off-host backups.
3. The provider credentials for DeepSeek, Groq Whisper and Azure Speech, plus the production authentication/encryption secrets.
4. An explicit `DEVENGLISH_ALLOWED_ORIGINS` value that exactly matches the HTTPS web origin.

## First deployment

From the repository root on the production host:

```bash
cp infra/production.env.example .env.production
# Edit .env.production. URL-encode the PostgreSQL password inside DATABASE_URL.
chmod 600 .env.production

docker compose --env-file .env.production \
  -f infra/docker-compose.production.yml config --quiet

docker compose --env-file .env.production \
  -f infra/docker-compose.production.yml up -d postgres

# Replace the values below if POSTGRES_USER/POSTGRES_DB were customized.
COMPOSE_FILE=infra/docker-compose.production.yml \
ENV_FILE=.env.production \
POSTGRES_SERVICE=postgres \
POSTGRES_USER=devenglish \
POSTGRES_DB=devenglish \
./scripts/db_migrate.sh

docker compose --env-file .env.production \
  -f infra/docker-compose.production.yml up -d --build backend web
```

The first startup of Caddy may take a short time while it obtains the certificate. Do not call the deployment production-ready until DNS and certificate issuance have been verified from outside the host.

## Readiness and smoke checks

```bash
curl --fail --silent --show-error https://english.example.com/healthz
curl --fail --silent --show-error https://english.example.com/readyz
curl --fail --silent --show-error --head https://english.example.com/
```

`/healthz` is a liveness check. `/readyz` performs a PostgreSQL readiness check and returns `503` without exposing database details when the database is unavailable. Then verify the browser production build: login, Home, one Writing submission, Review, Speaking and logout. The production bundle is compiled with `DEVENGLISH_ENV=production` and never receives `DEVENGLISH_BOOTSTRAP_KEY` or a bearer token.

## Backup and restore

Run backups before every migration and release, retain them outside the repository, and copy at least one verified backup off-host:

```bash
COMPOSE_FILE=infra/docker-compose.production.yml \
ENV_FILE=.env.production \
POSTGRES_SERVICE=postgres \
POSTGRES_USER=devenglish \
POSTGRES_DB=devenglish \
BACKUP_DIR=/var/backups/devenglish \
./scripts/db_backup.sh
```

Restore is destructive and requires both an exact database target and an explicit production allow flag:

```bash
ALLOW_PRODUCTION_RESTORE=YES \
CONFIRM_RESTORE=YES \
CONFIRM_RESTORE_TARGET=devenglish \
COMPOSE_FILE=infra/docker-compose.production.yml \
ENV_FILE=.env.production \
POSTGRES_SERVICE=postgres \
POSTGRES_USER=devenglish \
POSTGRES_DB=devenglish \
BACKUP_FILE=/var/backups/devenglish/<verified-backup>.dump \
./scripts/db_restore.sh
```

After a restore, rerun migrations, `/readyz`, login and the browser smoke flow. Record the backup filename, checksum, restore target and verification timestamp in the release log.

## Release, rollback and observability

- Record the current backend/web image IDs before `--build`; retain them until the new smoke checks pass.
- Deploy the new images, then verify `/healthz`, `/readyz`, login and one representative learning loop.
- Roll back by retagging the recorded image IDs and running `docker compose ... up -d --no-build backend web`; never delete the only known-good images before verification.
- Inspect `docker compose ... logs --since=15m backend web`; backend and Caddy logs are structured and must be shipped to durable monitoring in the host environment.
- Alert on non-2xx `/readyz`, repeated provider failures, container restarts, certificate renewal errors, backup age and backup restore failures.

## RC1 completion gate

P5 is complete only after the exact external domain, DNS, certificate, production PostgreSQL, secrets, CORS, backup/restore, readiness, logging and production browser checks above have been executed and recorded. A valid Compose configuration or a Caddyfile alone is not production evidence.
