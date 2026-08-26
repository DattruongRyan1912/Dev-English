-- Tighten invariants introduced by 003_platform_foundation.sql. The tables
-- are new in V1; fail closed if an environment already contains an invalid
-- receipt rather than silently fabricating a challenge or replay key.

ALTER TABLE audit_events
  DROP CONSTRAINT IF EXISTS audit_events_workspace_id_fkey;
ALTER TABLE audit_events
  ADD CONSTRAINT audit_events_workspace_id_fkey
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE RESTRICT;

ALTER TABLE action_receipts
  ALTER COLUMN challenge_id SET NOT NULL,
  ALTER COLUMN idempotency_key SET NOT NULL;

INSERT INTO schema_migrations(version)
VALUES ('004_platform_safety.sql')
ON CONFLICT (version) DO NOTHING;
