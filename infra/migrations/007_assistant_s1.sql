-- S1 assistant conversation persistence. Messages keep the canonical
-- assistant response JSON so citations and unknowns survive a restart.

CREATE TABLE IF NOT EXISTS assistant_conversations (
  id TEXT PRIMARY KEY CHECK (id <> '' AND btrim(id) = id),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, user_id, id),
  UNIQUE (id, workspace_id, user_id),
  CONSTRAINT assistant_conversations_workspace_user_fk
    FOREIGN KEY (workspace_id, user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS assistant_conversations_scope_updated_idx
  ON assistant_conversations (workspace_id, user_id, updated_at DESC, id);

CREATE TABLE IF NOT EXISTS assistant_messages (
  id TEXT PRIMARY KEY CHECK (id <> '' AND btrim(id) = id),
  conversation_id TEXT NOT NULL REFERENCES assistant_conversations(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL CHECK (btrim(content) <> ''),
  response JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (conversation_id, workspace_id, user_id)
    REFERENCES assistant_conversations (id, workspace_id, user_id)
    ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS assistant_messages_conversation_created_idx
  ON assistant_messages (conversation_id, created_at, id);
CREATE INDEX IF NOT EXISTS assistant_messages_scope_created_idx
  ON assistant_messages (workspace_id, user_id, created_at DESC, id);

INSERT INTO schema_migrations(version)
VALUES ('007_assistant_s1.sql')
ON CONFLICT (version) DO NOTHING;
