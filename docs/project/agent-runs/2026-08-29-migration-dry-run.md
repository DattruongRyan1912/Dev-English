# Migration dry-run and rollback evidence — 2026-08-29

## Scope

Validate the production-shaped migration workflow without touching a
production database. The check covers the non-mutating preview, ordered
application, idempotent re-application and rollback of a failing migration.

## Implementation

- `scripts/db_migrate.sh` accepts `DRY_RUN=YES` and validates all migration
  filenames before reporting status;
- dry-run reports `pending` or `already applied` versions and never creates or
  writes `schema_migrations`;
- `scripts/db_migrate_test.sh` exercises the same runner against a disposable
  PostgreSQL service and verifies failure rollback.

## Verified commands and observed results

| Check | Result |
| --- | --- |
| `rtk bash -n scripts/db_migrate.sh` | exit `0` |
| `rtk bash -n scripts/db_migrate_test.sh` | exit `0` |
| Invalid `DRY_RUN` value | exit `2` with the expected validation message before Docker/database access |
| Disposable `scripts/db_migrate_test.sh` harness | exit `0`; dry-run reported `19` pending migrations and left `schema_migrations` absent; the real run applied `000..018`; the second run reported all migrations already applied; the injected failing migration rolled back successfully |
| `rtk git diff --check` | exit `0` |

The disposable Compose project and volume were removed after verification.

## Boundary

This is local disposable-database evidence. It does not apply migrations to a
production database, replace backup/recovery review or grant authorization for
release operations.
