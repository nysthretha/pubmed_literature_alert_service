-- +goose Up
-- +goose StatementBegin

-- Bootstrap guard: fail early with an actionable message if no admin exists.
-- The admin must be created via the CLI between migration 00005 (which creates
-- the users table) and 00006 (which backfills user_id on existing data).
-- See docs/DEPLOY.md.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM users WHERE is_admin = true) THEN
        RAISE EXCEPTION USING
            MESSAGE = 'No admin user exists. Run ./scheduler create-admin --username X --email Y before applying this migration. See docs/DEPLOY.md for the full bootstrap sequence.',
            ERRCODE = 'P0001';
    END IF;
END
$$;

ALTER TABLE queries ADD COLUMN user_id BIGINT;
UPDATE queries
SET user_id = (SELECT id FROM users WHERE is_admin = true ORDER BY created_at, id LIMIT 1)
WHERE user_id IS NULL;
ALTER TABLE queries
    ALTER COLUMN user_id SET NOT NULL,
    ADD CONSTRAINT queries_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX idx_queries_user_id ON queries (user_id);

ALTER TABLE digests ADD COLUMN user_id BIGINT;
UPDATE digests
SET user_id = (SELECT id FROM users WHERE is_admin = true ORDER BY created_at, id LIMIT 1)
WHERE user_id IS NULL;
ALTER TABLE digests
    ALTER COLUMN user_id SET NOT NULL,
    ADD CONSTRAINT digests_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX idx_digests_user_id ON digests (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE digests DROP CONSTRAINT IF EXISTS digests_user_id_fk;
DROP INDEX IF EXISTS idx_digests_user_id;
ALTER TABLE digests DROP COLUMN IF EXISTS user_id;

ALTER TABLE queries DROP CONSTRAINT IF EXISTS queries_user_id_fk;
DROP INDEX IF EXISTS idx_queries_user_id;
ALTER TABLE queries DROP COLUMN IF EXISTS user_id;
-- +goose StatementEnd
