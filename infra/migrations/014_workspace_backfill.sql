-- Backfill the deterministic workspace used by the V2 application for every
-- existing user. This is additive: legacy learning rows remain untouched and
-- no user or work data is deleted. The application derives the workspace ID
-- from the first eight bytes of SHA-256(user_id), so pgcrypto keeps the SQL
-- migration and application derivation identical.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO workspaces (id, owner_user_id, name, slug, description, metadata)
SELECT
  'workspace-' || encode(substring(digest(u.id, 'sha256') FROM 1 FOR 8), 'hex'),
  u.id,
  'DevEnglish Workspace',
  'user-' || md5(u.id),
  'Canonical workspace for the DevEnglish product reset.',
  jsonb_build_object(
    'migration', '014_workspace_backfill',
    'legacyPreserved', true,
    'legacyUserId', u.id
  )
FROM users AS u
WHERE NOT EXISTS (
  SELECT 1
  FROM workspaces AS existing
  WHERE existing.owner_user_id = u.id
    AND existing.deleted_at IS NULL
);

INSERT INTO schema_migrations(version)
VALUES ('014_workspace_backfill.sql')
ON CONFLICT (version) DO NOTHING;
