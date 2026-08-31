-- Persist the canonical action hash so a durable idempotency replay can be
-- compared against the original target without storing provider credentials or
-- an executable/raw provider payload.
ALTER TABLE action_receipts
  ADD COLUMN IF NOT EXISTS action_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS action_receipts_lookup_idx
  ON action_receipts (user_id, action, idempotency_key, created_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('012_action_receipt_hash.sql')
ON CONFLICT (version) DO NOTHING;
