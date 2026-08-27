-- DevEnglish V1 Knowledge bounded context.
--
-- This migration is ordered after 003_platform_foundation.sql, which owns the
-- canonical workspaces table. Every row carries an explicit workspace_id;
-- independent knowledge roots anchor that value to workspaces, while composite
-- foreign keys prevent cross-workspace references inside this bounded context.
--
-- The migration is additive and intentionally has no DROP TABLE/DROP COLUMN
-- rollback. To roll back safely, stop applying this version and use a reviewed,
-- data-preserving forward migration; never delete knowledge history implicitly.
-- scripts/db_migrate.sh applies each migration with psql --single-transaction;
-- this file therefore does not open a nested transaction.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS knowledge_sources (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  kind TEXT NOT NULL CHECK (btrim(kind) <> ''),
  name TEXT NOT NULL CHECK (btrim(name) <> ''),
  uri TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (workspace_id, id),
  CONSTRAINT knowledge_sources_workspace_fk
    FOREIGN KEY (workspace_id)
    REFERENCES workspaces(id)
    ON DELETE RESTRICT
);

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
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS knowledge_sources_active_uri_idx
  ON knowledge_sources (workspace_id, uri)
  WHERE deleted_at IS NULL AND btrim(uri) <> '';

CREATE TABLE IF NOT EXISTS source_items (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  source_id TEXT NOT NULL,
  external_id TEXT NOT NULL CHECK (btrim(external_id) <> '' AND external_id = btrim(external_id)),
  title TEXT NOT NULL DEFAULT '',
  uri TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT '',
  current_revision_id TEXT,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (workspace_id, id),
  FOREIGN KEY (workspace_id, source_id)
    REFERENCES knowledge_sources (workspace_id, id)
    ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS source_items_active_external_idx
  ON source_items (workspace_id, source_id, external_id)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS source_revisions (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  source_item_id TEXT NOT NULL,
  revision_key TEXT NOT NULL CHECK (btrim(revision_key) <> '' AND revision_key = btrim(revision_key)),
  content_hash TEXT NOT NULL CHECK (btrim(content_hash) <> ''),
  content_type TEXT NOT NULL DEFAULT '',
  source_uri TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL CHECK (btrim(content) <> ''),
  modified_at TIMESTAMPTZ,
  ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, id),
  UNIQUE (workspace_id, source_item_id, revision_key),
  UNIQUE (workspace_id, id, source_item_id),
  FOREIGN KEY (workspace_id, source_item_id)
    REFERENCES source_items (workspace_id, id)
    ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS source_revisions_item_ingested_idx
  ON source_revisions (workspace_id, source_item_id, ingested_at DESC);

CREATE OR REPLACE FUNCTION prevent_knowledge_revision_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'source revisions are immutable' USING ERRCODE = '55000';
END;
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgrelid = 'source_revisions'::regclass
      AND tgname = 'source_revisions_immutable'
  ) THEN
    CREATE TRIGGER source_revisions_immutable
      BEFORE UPDATE OR DELETE ON source_revisions
      FOR EACH ROW EXECUTE FUNCTION prevent_knowledge_revision_mutation();
  END IF;
END;
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'source_items'::regclass
      AND conname = 'source_items_current_revision_fk'
  ) THEN
    ALTER TABLE source_items
      ADD CONSTRAINT source_items_current_revision_fk
      FOREIGN KEY (workspace_id, current_revision_id, id)
      REFERENCES source_revisions (workspace_id, id, source_item_id)
      ON DELETE RESTRICT;
  END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS knowledge_chunks (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  revision_id TEXT NOT NULL,
  ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
  chunk_text TEXT NOT NULL CHECK (btrim(chunk_text) <> ''),
  token_count INTEGER NOT NULL DEFAULT 0 CHECK (token_count >= 0),
  embedding vector(384),
  search_vector TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple', chunk_text)) STORED,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, id),
  UNIQUE (workspace_id, revision_id, ordinal),
  UNIQUE (workspace_id, id, revision_id),
  FOREIGN KEY (workspace_id, revision_id)
    REFERENCES source_revisions (workspace_id, id)
    ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS knowledge_chunks_search_idx
  ON knowledge_chunks USING GIN (search_vector);

CREATE INDEX IF NOT EXISTS knowledge_chunks_embedding_idx
  ON knowledge_chunks USING hnsw (embedding vector_cosine_ops)
  WHERE embedding IS NOT NULL;

CREATE TABLE IF NOT EXISTS topics (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  name TEXT NOT NULL CHECK (btrim(name) <> ''),
  description TEXT NOT NULL DEFAULT '',
  parent_id TEXT,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (workspace_id, id),
  CONSTRAINT topics_workspace_fk
    FOREIGN KEY (workspace_id)
    REFERENCES workspaces(id)
    ON DELETE RESTRICT
);

DO $$
BEGIN
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
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS topics_active_name_idx
  ON topics (workspace_id, lower(name))
  WHERE deleted_at IS NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'topics'::regclass
      AND conname = 'topics_parent_fk'
  ) THEN
    ALTER TABLE topics
      ADD CONSTRAINT topics_parent_fk
      FOREIGN KEY (workspace_id, parent_id)
      REFERENCES topics (workspace_id, id)
      ON DELETE RESTRICT;
  END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS knowledge_claims (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  topic_id TEXT,
  statement TEXT NOT NULL CHECK (btrim(statement) <> ''),
  certainty TEXT NOT NULL CHECK (certainty IN ('canonical', 'inferred', 'unknown')),
  freshness TEXT NOT NULL CHECK (freshness IN ('current', 'stale', 'unknown')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (workspace_id, id),
  CONSTRAINT knowledge_claims_workspace_fk
    FOREIGN KEY (workspace_id)
    REFERENCES workspaces(id)
    ON DELETE RESTRICT,
  FOREIGN KEY (workspace_id, topic_id)
    REFERENCES topics (workspace_id, id)
    ON DELETE RESTRICT
);

DO $$
BEGIN
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

CREATE INDEX IF NOT EXISTS knowledge_claims_workspace_freshness_idx
  ON knowledge_claims (workspace_id, freshness, updated_at DESC)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS claim_evidence (
  id TEXT PRIMARY KEY CHECK (btrim(id) <> '' AND id = btrim(id)),
  workspace_id TEXT NOT NULL CHECK (btrim(workspace_id) <> '' AND workspace_id = btrim(workspace_id)),
  claim_id TEXT NOT NULL,
  source_revision_id TEXT NOT NULL,
  chunk_id TEXT,
  locator TEXT NOT NULL DEFAULT '',
  quote TEXT NOT NULL CHECK (btrim(quote) <> ''),
  freshness TEXT NOT NULL CHECK (freshness IN ('current', 'stale', 'unknown')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, id),
  FOREIGN KEY (workspace_id, claim_id)
    REFERENCES knowledge_claims (workspace_id, id)
    ON DELETE RESTRICT,
  FOREIGN KEY (workspace_id, source_revision_id)
    REFERENCES source_revisions (workspace_id, id)
    ON DELETE RESTRICT,
  FOREIGN KEY (workspace_id, chunk_id, source_revision_id)
    REFERENCES knowledge_chunks (workspace_id, id, revision_id)
    ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS claim_evidence_claim_idx
  ON claim_evidence (workspace_id, claim_id, created_at);

CREATE OR REPLACE FUNCTION validate_knowledge_claim_evidence_for(
  p_workspace_id TEXT,
  p_claim_id TEXT,
  p_require_claim BOOLEAN
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  claim_certainty TEXT;
  claim_freshness TEXT;
  claim_deleted_at TIMESTAMPTZ;
  evidence_count BIGINT;
  current_evidence_count BIGINT;
  stale_evidence_count BIGINT;
BEGIN
  IF p_workspace_id IS NULL OR p_claim_id IS NULL THEN
    RAISE EXCEPTION 'knowledge claim identity cannot be null'
      USING ERRCODE = '23514';
  END IF;

  SELECT certainty, freshness, deleted_at
    INTO claim_certainty, claim_freshness, claim_deleted_at
    FROM knowledge_claims
   WHERE workspace_id = p_workspace_id
     AND id = p_claim_id;

  IF NOT FOUND THEN
    IF p_require_claim THEN
      RAISE EXCEPTION 'knowledge evidence references a missing claim'
        USING ERRCODE = '23503';
    END IF;
    RETURN;
  END IF;

  -- Soft-deleted and non-canonical claims do not have an active grounding
  -- requirement. Active canonical claims must satisfy the Go domain rules
  -- against the final deferred state, not merely the row that fired a trigger.
  IF claim_deleted_at IS NOT NULL OR claim_certainty <> 'canonical' THEN
    RETURN;
  END IF;

  SELECT count(*),
         count(*) FILTER (WHERE freshness = 'current'),
         count(*) FILTER (WHERE freshness IN ('stale', 'unknown'))
    INTO evidence_count, current_evidence_count, stale_evidence_count
    FROM claim_evidence
   WHERE workspace_id = p_workspace_id
     AND claim_id = p_claim_id;

  IF claim_freshness = 'current' AND stale_evidence_count > 0 THEN
    RAISE EXCEPTION 'current knowledge claims cannot use stale or unknown evidence'
      USING ERRCODE = '23514';
  END IF;
  IF evidence_count = 0 THEN
    RAISE EXCEPTION 'canonical knowledge claims require evidence'
      USING ERRCODE = '23514';
  END IF;
  IF current_evidence_count = 0 THEN
    RAISE EXCEPTION 'canonical knowledge claims require current evidence'
      USING ERRCODE = '23514';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION enforce_knowledge_claim_evidence()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_TABLE_NAME = 'knowledge_claims' THEN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
      PERFORM validate_knowledge_claim_evidence_for(
        OLD.workspace_id,
        OLD.id,
        FALSE
      );
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
      PERFORM validate_knowledge_claim_evidence_for(
        NEW.workspace_id,
        NEW.id,
        TRUE
      );
    END IF;
  ELSIF TG_TABLE_NAME = 'claim_evidence' THEN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
      PERFORM validate_knowledge_claim_evidence_for(
        OLD.workspace_id,
        OLD.claim_id,
        TRUE
      );
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
      PERFORM validate_knowledge_claim_evidence_for(
        NEW.workspace_id,
        NEW.claim_id,
        TRUE
      );
    END IF;
  ELSE
    RAISE EXCEPTION 'knowledge evidence trigger attached to an unsupported table'
      USING ERRCODE = '23514';
  END IF;

  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS knowledge_claims_evidence_required ON knowledge_claims;
CREATE CONSTRAINT TRIGGER knowledge_claims_evidence_required
  AFTER INSERT OR UPDATE OR DELETE ON knowledge_claims
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION enforce_knowledge_claim_evidence();

DROP TRIGGER IF EXISTS claim_evidence_preserves_canonical ON claim_evidence;
CREATE CONSTRAINT TRIGGER claim_evidence_preserves_canonical
  AFTER INSERT OR UPDATE OR DELETE ON claim_evidence
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION enforce_knowledge_claim_evidence();

INSERT INTO schema_migrations(version)
VALUES ('005_knowledge.sql')
ON CONFLICT (version) DO NOTHING;
