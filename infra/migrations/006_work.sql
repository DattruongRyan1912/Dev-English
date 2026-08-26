-- DevEnglish work records. This migration depends on 001_initial.sql for users
-- and 003_platform_foundation.sql for workspaces.
--
-- Work IDs are scoped to both workspace and owner. This matches the domain
-- Scope type and permits an explicit ID to be reused by another tenant after
-- the original row has been purged without mixing records or history.

CREATE TABLE IF NOT EXISTS projects (
  id TEXT NOT NULL,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'on_hold', 'completed', 'archived')),
  origin TEXT NOT NULL DEFAULT 'canonical'
    CHECK (origin = 'canonical'),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  PRIMARY KEY (workspace_id, owner_user_id, id),
  CONSTRAINT projects_workspace_owner_fk
    FOREIGN KEY (workspace_id, owner_user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE,
  CHECK (id <> '' AND btrim(id) = id),
  CHECK (octet_length(name) BETWEEN 1 AND 200),
  CHECK (octet_length(description) <= 20000)
);

CREATE INDEX IF NOT EXISTS projects_scope_created_idx
  ON projects (workspace_id, owner_user_id, created_at, id);
CREATE INDEX IF NOT EXISTS projects_scope_deleted_idx
  ON projects (workspace_id, owner_user_id, deleted_at, updated_at DESC);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT NOT NULL,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'todo'
    CHECK (status IN ('backlog', 'todo', 'in_progress', 'blocked', 'done', 'cancelled')),
  priority TEXT NOT NULL DEFAULT 'normal'
    CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
  due_at TIMESTAMPTZ,
  origin TEXT NOT NULL DEFAULT 'canonical'
    CHECK (origin = 'canonical'),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  PRIMARY KEY (workspace_id, owner_user_id, id),
  CONSTRAINT tasks_workspace_owner_fk
    FOREIGN KEY (workspace_id, owner_user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, owner_user_id, project_id)
    REFERENCES projects (workspace_id, owner_user_id, id)
    ON DELETE RESTRICT,
  CHECK (id <> '' AND btrim(id) = id),
  CHECK (octet_length(title) BETWEEN 1 AND 500),
  CHECK (octet_length(description) <= 20000)
);

CREATE INDEX IF NOT EXISTS tasks_scope_created_idx
  ON tasks (workspace_id, owner_user_id, created_at, id);
CREATE INDEX IF NOT EXISTS tasks_scope_project_idx
  ON tasks (workspace_id, owner_user_id, project_id, created_at, id);
CREATE INDEX IF NOT EXISTS tasks_scope_deleted_idx
  ON tasks (workspace_id, owner_user_id, deleted_at, updated_at DESC);

CREATE TABLE IF NOT EXISTS decisions (
  id TEXT NOT NULL,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT,
  title TEXT NOT NULL,
  context TEXT NOT NULL DEFAULT '',
  outcome TEXT NOT NULL,
  rationale TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'proposed'
    CHECK (status IN ('proposed', 'accepted', 'rejected', 'superseded')),
  origin TEXT NOT NULL DEFAULT 'canonical'
    CHECK (origin = 'canonical'),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  PRIMARY KEY (workspace_id, owner_user_id, id),
  CONSTRAINT decisions_workspace_owner_fk
    FOREIGN KEY (workspace_id, owner_user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, owner_user_id, project_id)
    REFERENCES projects (workspace_id, owner_user_id, id)
    ON DELETE RESTRICT,
  CHECK (id <> '' AND btrim(id) = id),
  CHECK (octet_length(title) BETWEEN 1 AND 500),
  CHECK (octet_length(context) <= 20000),
  CHECK (octet_length(outcome) BETWEEN 1 AND 20000),
  CHECK (octet_length(rationale) <= 20000)
);

CREATE INDEX IF NOT EXISTS decisions_scope_created_idx
  ON decisions (workspace_id, owner_user_id, created_at, id);
CREATE INDEX IF NOT EXISTS decisions_scope_project_idx
  ON decisions (workspace_id, owner_user_id, project_id, created_at, id);
CREATE INDEX IF NOT EXISTS decisions_scope_deleted_idx
  ON decisions (workspace_id, owner_user_id, deleted_at, updated_at DESC);

-- History is append-only and deliberately has no foreign key to a work row:
-- purge removes the current row but must retain its audit trail.
CREATE TABLE IF NOT EXISTS work_history (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
  actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  entity_type TEXT NOT NULL
    CHECK (entity_type IN ('project', 'task', 'decision')),
  entity_id TEXT NOT NULL,
  action TEXT NOT NULL
    CHECK (action IN ('created', 'updated', 'trashed', 'restored', 'purged')),
  from_version BIGINT NOT NULL CHECK (from_version >= 0),
  to_version BIGINT NOT NULL CHECK (to_version > from_version),
  idempotency_key TEXT NOT NULL,
  before_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  after_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (entity_id <> '' AND btrim(entity_id) = entity_id),
  CHECK (idempotency_key <> '' AND octet_length(idempotency_key) <= 128)
);

CREATE INDEX IF NOT EXISTS work_history_scope_entity_idx
  ON work_history (workspace_id, actor_user_id, entity_type, entity_id, created_at, id);
CREATE INDEX IF NOT EXISTS work_history_scope_created_idx
  ON work_history (workspace_id, actor_user_id, created_at, id);

CREATE OR REPLACE FUNCTION prevent_work_history_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'work_history is append-only';
END;
$$;

DROP TRIGGER IF EXISTS work_history_append_only_guard ON work_history;
CREATE TRIGGER work_history_append_only_guard
  BEFORE UPDATE OR DELETE ON work_history
  FOR EACH ROW EXECUTE FUNCTION prevent_work_history_mutation();

-- One receipt per scoped idempotency key. The application compares operation
-- and request_hash on replay and returns a typed conflict on mismatch.
CREATE TABLE IF NOT EXISTS work_idempotency (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idempotency_key TEXT NOT NULL,
  operation TEXT NOT NULL
    CHECK (operation IN (
      'project.create', 'project.update', 'project.trash',
      'project.restore', 'project.purge',
      'task.create', 'task.update', 'task.trash',
      'task.restore', 'task.purge',
      'decision.create', 'decision.update', 'decision.trash',
      'decision.restore', 'decision.purge'
    )),
  request_hash TEXT NOT NULL
    CHECK (request_hash ~ '^[0-9A-Fa-f]{64}$'),
  entity_type TEXT NOT NULL
    CHECK (entity_type IN ('project', 'task', 'decision')),
  entity_id TEXT NOT NULL,
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_idempotency_workspace_user_fk
    FOREIGN KEY (workspace_id, user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE,
  PRIMARY KEY (workspace_id, user_id, idempotency_key),
  CHECK (idempotency_key = btrim(idempotency_key)),
  CHECK (octet_length(idempotency_key) BETWEEN 1 AND 128),
  CHECK (entity_id <> '' AND btrim(entity_id) = entity_id)
);

CREATE INDEX IF NOT EXISTS work_idempotency_scope_created_idx
  ON work_idempotency (workspace_id, user_id, created_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('006_work.sql')
ON CONFLICT (version) DO NOTHING;
