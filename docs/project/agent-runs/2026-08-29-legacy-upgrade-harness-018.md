# Legacy upgrade harness — migration 018

Date: 2026-08-29

## Scope

The disposable legacy-upgrade harness previously stopped its explicit
migration list at `017_workspace_scoped_challenge_identity.sql` while the
canonical schema had advanced to migration 018. This run aligns the harness
with the complete migration chain and checks the new assistant context
boundary without mutating a durable development database.

## Implementation

- `scripts/db_legacy_upgrade_test.sh` now links, applies and replays
  `018_assistant_conversation_context.sql`;
- the expected `schema_migrations` value includes migration 018;
- the harness verifies `context_type`, `context_id`,
  `assistant_conversations_context_ref_ck` and
  `assistant_conversations_context_scope_idx`;
- legacy `work_context` and `imported_sources` snapshots remain the
  preservation oracle.

## Evidence

Disposable Compose project: `devenglish-legacy-upgrade-local-20260829-r2`

- `bash -n scripts/db_legacy_upgrade_test.sh` — exit `0`;
- `rtk bash scripts/db_legacy_upgrade_test.sh` with the disposable harness
  environment — exit `0`;
- Phase 1 applied migrations `000..002`;
- Phase 2 applied migrations `003..018`;
- the second Phase 2 run reported every migration `003..018` as already
  applied;
- legacy snapshots, workspace backfill/isolation and assistant context schema
  assertions passed;
- the disposable Compose project was removed by the harness cleanup trap.

## Boundary

This is local disposable database evidence only. It does not establish
production migration approval, lifecycle acceptance or release authorization.
