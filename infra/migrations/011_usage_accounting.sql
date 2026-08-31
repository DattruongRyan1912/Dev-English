ALTER TABLE ai_usage
  ADD COLUMN IF NOT EXISTS usage_available BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS ai_usage_user_created_idx
  ON ai_usage (user_id, created_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('011_usage_accounting.sql')
ON CONFLICT (version) DO NOTHING;
