-- +goose Up
-- +goose StatementBegin

-- digests_one_sent_per_day was created in 00004 before users existed as
--   UNIQUE (sent_local_date) WHERE status='sent'
-- After 00006 scoped digests to users, this constraint incorrectly blocks
-- a second user from sending a digest on a day the first user already
-- sent theirs (UniqueViolation even though the two rows belong to
-- different users). Recreate scoped by user_id.

DROP INDEX IF EXISTS digests_one_sent_per_day;
CREATE UNIQUE INDEX digests_one_sent_per_day
    ON digests (user_id, sent_local_date)
    WHERE status = 'sent';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restoring the global-per-day index could fail if multiple users now
-- share a sent_local_date with status='sent'. Down leaves the index
-- dropped rather than risk an unreversible collision.
DROP INDEX IF EXISTS digests_one_sent_per_day;

-- +goose StatementEnd
