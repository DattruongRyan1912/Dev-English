-- Atomic short-lived reservations prevent concurrent AI calls from passing
-- the same monthly quota check before either call records usage.
CREATE TABLE IF NOT EXISTS ai_usage_reservations (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  month_start TIMESTAMPTZ NOT NULL,
  feature TEXT NOT NULL DEFAULT '',
  metric TEXT NOT NULL CHECK (metric IN ('tokens', 'cost')),
  amount DOUBLE PRECISION NOT NULL CHECK (amount > 0),
  limit_value DOUBLE PRECISION NOT NULL CHECK (limit_value > 0),
  expires_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'committed', 'released')),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ai_usage_reservations_scope_idx
  ON ai_usage_reservations (user_id, month_start, feature, metric, status, expires_at);

INSERT INTO schema_migrations(version)
VALUES ('015_usage_reservations.sql')
ON CONFLICT (version) DO NOTHING;
