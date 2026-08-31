-- Repair migration for knowledge databases created before the root workspace
-- foreign keys were enforced. It is idempotent and intentionally additive.

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'knowledge_sources'::regclass
      AND conname = 'knowledge_sources_workspace_fk'
  ) THEN
    ALTER TABLE knowledge_sources
      ADD CONSTRAINT knowledge_sources_workspace_fk
      FOREIGN KEY (workspace_id)
      REFERENCES workspaces(id)
      ON DELETE RESTRICT;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'topics'::regclass
      AND conname = 'topics_workspace_fk'
  ) THEN
    ALTER TABLE topics
      ADD CONSTRAINT topics_workspace_fk
      FOREIGN KEY (workspace_id)
      REFERENCES workspaces(id)
      ON DELETE RESTRICT;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'knowledge_claims'::regclass
      AND conname = 'knowledge_claims_workspace_fk'
  ) THEN
    ALTER TABLE knowledge_claims
      ADD CONSTRAINT knowledge_claims_workspace_fk
      FOREIGN KEY (workspace_id)
      REFERENCES workspaces(id)
      ON DELETE RESTRICT;
  END IF;
END;
$$;

INSERT INTO schema_migrations(version)
VALUES ('008_knowledge_root_fks.sql')
ON CONFLICT (version) DO NOTHING;
