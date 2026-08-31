-- Learning observations are an adaptive overlay only. They may reference
-- canonical work items, but no learning write can mutate Work or Knowledge.
CREATE TABLE IF NOT EXISTS learning_observations (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL DEFAULT '',
  source_id TEXT NOT NULL DEFAULT '',
  skill TEXT NOT NULL,
  prompt TEXT NOT NULL,
  response TEXT NOT NULL,
  feedback TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (length(skill) <= 120),
  CHECK (length(prompt) <= 20000),
  CHECK (length(response) <= 20000),
  CHECK (length(feedback) <= 20000)
);

CREATE INDEX IF NOT EXISTS learning_observations_scope_created_idx
  ON learning_observations (workspace_id, user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS learning_observations_source_idx
  ON learning_observations (workspace_id, source_type, source_id, created_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('013_learning_overlay.sql')
ON CONFLICT (version) DO NOTHING;
