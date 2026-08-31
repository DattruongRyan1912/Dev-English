-- Repair migration for databases that recorded 005_knowledge.sql before the
-- deferred canonical-evidence guard was introduced. Recreate the functions
-- and constraint triggers without touching knowledge rows.

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
      PERFORM validate_knowledge_claim_evidence_for(OLD.workspace_id, OLD.id, FALSE);
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
      PERFORM validate_knowledge_claim_evidence_for(NEW.workspace_id, NEW.id, TRUE);
    END IF;
  ELSIF TG_TABLE_NAME = 'claim_evidence' THEN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
      PERFORM validate_knowledge_claim_evidence_for(OLD.workspace_id, OLD.claim_id, TRUE);
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
      PERFORM validate_knowledge_claim_evidence_for(NEW.workspace_id, NEW.claim_id, TRUE);
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
VALUES ('009_knowledge_evidence_trigger_repair.sql')
ON CONFLICT (version) DO NOTHING;
