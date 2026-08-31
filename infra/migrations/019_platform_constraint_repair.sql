-- Repair the scoped action invariants for databases that applied an earlier
-- weakened version of 003_platform_foundation.sql. The checks fail closed so
-- operators can resolve ambiguous rows instead of silently changing ownership.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM action_challenges AS challenge
    LEFT JOIN workspaces AS workspace
      ON workspace.id = challenge.workspace_id
     AND workspace.owner_user_id = challenge.user_id
    WHERE workspace.id IS NULL
  ) THEN
    RAISE EXCEPTION 'action_challenges contains rows outside the workspace owner scope';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM action_receipts AS receipt
    LEFT JOIN action_challenges AS challenge
      ON challenge.id = receipt.challenge_id
     AND challenge.workspace_id = receipt.workspace_id
     AND challenge.user_id = receipt.user_id
    WHERE challenge.id IS NULL
  ) THEN
    RAISE EXCEPTION 'action_receipts contains rows outside the challenge scope';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM action_challenges
    WHERE status = 'pending'
    GROUP BY workspace_id, user_id, action_hash
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'action_challenges contains duplicate pending scoped hashes';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM action_receipts
    GROUP BY workspace_id, user_id, action, idempotency_key
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'action_receipts contains duplicate scoped idempotency keys';
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS workspaces_id_owner_uidx
  ON workspaces (id, owner_user_id);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'action_challenges'::regclass
      AND conname = 'action_challenges_workspace_user_fk'
  ) THEN
    ALTER TABLE action_challenges
      ADD CONSTRAINT action_challenges_workspace_user_fk
      FOREIGN KEY (workspace_id, user_id)
      REFERENCES workspaces (id, owner_user_id)
      ON DELETE CASCADE;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'action_challenges'::regclass
      AND conname = 'action_challenges_scope_uidx'
  ) THEN
    ALTER TABLE action_challenges
      ADD CONSTRAINT action_challenges_scope_uidx
      UNIQUE (id, workspace_id, user_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'action_receipts'::regclass
      AND conname = 'action_receipts_challenge_scope_fk'
  ) THEN
    ALTER TABLE action_receipts
      ADD CONSTRAINT action_receipts_challenge_scope_fk
      FOREIGN KEY (challenge_id, workspace_id, user_id)
      REFERENCES action_challenges (id, workspace_id, user_id)
      ON DELETE RESTRICT;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS knowledge_import_idempotency (
  workspace_id TEXT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  operation TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  response JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id, operation, idempotency_key),
  FOREIGN KEY (workspace_id, user_id)
    REFERENCES workspaces (id, owner_user_id)
    ON DELETE CASCADE
);

-- 003 created the legacy name and 017 introduced the workspace-explicit
-- canonical name. Drop the legacy name and keep exactly one pending-challenge
-- invariant so upgrades cannot leave two equivalent indexes behind.
DROP INDEX IF EXISTS action_challenges_pending_hash_uidx;
CREATE UNIQUE INDEX IF NOT EXISTS action_challenges_pending_workspace_hash_uidx
  ON action_challenges (workspace_id, user_id, action_hash)
  WHERE status = 'pending';

DROP INDEX IF EXISTS action_receipts_active_idempotency_uidx;
CREATE UNIQUE INDEX action_receipts_active_idempotency_uidx
  ON action_receipts (workspace_id, user_id, action, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

INSERT INTO schema_migrations(version)
VALUES ('019_platform_constraint_repair.sql')
ON CONFLICT (version) DO NOTHING;
