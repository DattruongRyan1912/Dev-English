-- MCP bearer credentials must survive backend restarts and share revocation
-- state across instances. Only a one-way digest is stored; the clear token is
-- returned once by the application and is never written to PostgreSQL.
CREATE TABLE IF NOT EXISTS mcp_tokens (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  digest BYTEA NOT NULL CHECK (octet_length(digest) = 32),
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(scopes) = 'array'),
  workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT mcp_tokens_identity_pair_ck CHECK ((workspace_id IS NULL) = (user_id IS NULL))
);

CREATE INDEX IF NOT EXISTS mcp_tokens_active_expiry_idx
  ON mcp_tokens (expires_at)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS mcp_tokens_identity_idx
  ON mcp_tokens (workspace_id, user_id, created_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('016_mcp_tokens.sql')
ON CONFLICT (version) DO NOTHING;
