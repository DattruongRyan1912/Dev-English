-- A user can work in more than one private workspace. Pending safe-write
-- challenges must therefore be unique within the workspace boundary; the
-- previous index keyed only by user and action hash could make an unrelated
-- workspace reject the same canonical action.
DROP INDEX IF EXISTS action_challenges_pending_hash_uidx;

CREATE UNIQUE INDEX IF NOT EXISTS action_challenges_pending_workspace_hash_uidx
  ON action_challenges (workspace_id, user_id, action_hash)
  WHERE status = 'pending';

INSERT INTO schema_migrations(version)
VALUES ('017_workspace_scoped_challenge_identity.sql')
ON CONFLICT (version) DO NOTHING;
