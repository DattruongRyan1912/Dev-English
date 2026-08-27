-- DevEnglish V1 platform foundation. This migration is additive and depends on
-- the users table created by 001_initial.sql.

CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY,
  owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT workspaces_id_owner_uidx UNIQUE (id, owner_user_id)
);
CREATE INDEX IF NOT EXISTS workspaces_owner_user_idx
  ON workspaces (owner_user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_active_slug_uidx
  ON workspaces (owner_user_id, lower(slug))
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS module_states (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  module_id TEXT NOT NULL,
  module_version TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'enabled',
  state JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS module_states_workspace_idx
  ON module_states (workspace_id, updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS module_states_active_workspace_module_uidx
  ON module_states (workspace_id, module_id)
  WHERE deleted_at IS NULL;

-- Audit events are append-only. They intentionally have no soft-delete column.
CREATE TABLE IF NOT EXISTS audit_events (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
  actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  entity_type TEXT NOT NULL DEFAULT '',
  entity_id TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_workspace_occurred_idx
  ON audit_events (workspace_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_entity_idx
  ON audit_events (workspace_id, entity_type, entity_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS background_jobs (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key TEXT,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts > 0),
  run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_at TIMESTAMPTZ,
  locked_by TEXT,
  completed_at TIMESTAMPTZ,
  last_error TEXT,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS background_jobs_queue_idx
  ON background_jobs (status, run_at, created_at);
CREATE INDEX IF NOT EXISTS background_jobs_workspace_status_idx
  ON background_jobs (workspace_id, status, updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS background_jobs_active_idempotency_uidx
  ON background_jobs (workspace_id, idempotency_key)
  WHERE deleted_at IS NULL AND idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS action_challenges (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  action_hash TEXT NOT NULL,
  prompt TEXT NOT NULL,
  parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'pending',
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '5 minutes'),
  consumed_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT action_challenges_workspace_user_fk
    FOREIGN KEY (workspace_id, user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE,
  CONSTRAINT action_challenges_scope_uidx UNIQUE (id, workspace_id, user_id)
);
CREATE INDEX IF NOT EXISTS action_challenges_user_status_idx
  ON action_challenges (user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS action_challenges_workspace_expiry_idx
  ON action_challenges (workspace_id, expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS action_challenges_pending_hash_uidx
  ON action_challenges (workspace_id, user_id, action_hash)
  WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS action_receipts (
  id TEXT PRIMARY KEY,
  challenge_id TEXT NOT NULL REFERENCES action_challenges(id) ON DELETE RESTRICT,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL,
  message TEXT,
  output JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT action_receipts_challenge_scope_fk
    FOREIGN KEY (challenge_id, workspace_id, user_id)
    REFERENCES action_challenges (id, workspace_id, user_id)
    ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS action_receipts_workspace_created_idx
  ON action_receipts (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS action_receipts_user_created_idx
  ON action_receipts (user_id, created_at DESC);
-- Keep the durable reservation identity aligned with SafeWriteReceiptStore:
-- workspace, user, operation (the SQL action column), and idempotency key.
CREATE UNIQUE INDEX IF NOT EXISTS action_receipts_active_idempotency_uidx
  ON action_receipts (workspace_id, user_id, action, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

INSERT INTO schema_migrations(version)
VALUES ('003_platform_foundation.sql')
ON CONFLICT (version) DO NOTHING;
