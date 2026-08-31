-- Persist the canonical entity a conversation is focused on. The reference
-- stays separate from source content so it can be re-resolved inside the
-- authenticated workspace on every assistant request.

ALTER TABLE assistant_conversations
  ADD COLUMN IF NOT EXISTS context_type TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS context_id TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'assistant_conversations_context_ref_ck'
      AND conrelid = 'assistant_conversations'::regclass
  ) THEN
    ALTER TABLE assistant_conversations
      ADD CONSTRAINT assistant_conversations_context_ref_ck
      CHECK (
        (context_type = '' AND context_id = '')
        OR (
          context_type IN ('project', 'task', 'decision', 'source')
          AND context_id <> ''
        )
      );
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS assistant_conversations_context_scope_idx
  ON assistant_conversations (workspace_id, user_id, context_type, context_id, updated_at DESC, id);

INSERT INTO schema_migrations(version)
VALUES ('018_assistant_conversation_context.sql')
ON CONFLICT (version) DO NOTHING;
