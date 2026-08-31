-- Connector checkpoints and run evidence for incremental, read-only imports.
-- Provider cursors are opaque and are only advanced after the corresponding
-- page has been committed to the immutable knowledge revision store.

CREATE TABLE IF NOT EXISTS connector_sync_cursors (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
  provider TEXT NOT NULL CHECK (provider IN ('google_drive', 'github')),
  target TEXT NOT NULL DEFAULT '',
  cursor TEXT NOT NULL DEFAULT '',
  has_more BOOLEAN NOT NULL DEFAULT FALSE,
  last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, provider, target)
);

CREATE INDEX IF NOT EXISTS connector_sync_cursors_workspace_idx
  ON connector_sync_cursors (workspace_id, provider, updated_at DESC);

CREATE TABLE IF NOT EXISTS connector_sync_runs (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
  provider TEXT NOT NULL CHECK (provider IN ('google_drive', 'github')),
  target TEXT NOT NULL DEFAULT '',
  cursor_before TEXT NOT NULL DEFAULT '',
  cursor_after TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
  seen_count INTEGER NOT NULL DEFAULT 0 CHECK (seen_count >= 0),
  upserted_count INTEGER NOT NULL DEFAULT 0 CHECK (upserted_count >= 0),
  skipped_count INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
  error_code TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS connector_sync_runs_scope_started_idx
  ON connector_sync_runs (workspace_id, provider, target, started_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('010_connector_sync.sql')
ON CONFLICT (version) DO NOTHING;
