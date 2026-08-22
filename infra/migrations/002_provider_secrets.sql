ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS deepseek_status TEXT NOT NULL DEFAULT 'not_configured'
    CHECK (deepseek_status IN ('not_configured', 'connected', 'invalid'));

CREATE TABLE IF NOT EXISTS provider_secrets (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  ciphertext BYTEA NOT NULL,
  nonce BYTEA NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, provider)
);

INSERT INTO schema_migrations(version)
VALUES ('002_provider_secrets.sql')
ON CONFLICT (version) DO NOTHING;
