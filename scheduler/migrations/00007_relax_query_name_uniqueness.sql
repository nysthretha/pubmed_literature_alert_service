-- +goose Up
-- +goose StatementBegin

-- Queries had a global UNIQUE(name) from 00001 (before users existed).
-- After 00006 scoped queries to users, this constraint incorrectly prevents
-- two different users from having queries with the same name.
-- Replace with UNIQUE(user_id, name).

ALTER TABLE queries DROP CONSTRAINT IF EXISTS queries_name_key;
ALTER TABLE queries ADD CONSTRAINT queries_user_id_name_key UNIQUE (user_id, name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Can't cleanly restore the global UNIQUE(name) because different users may
-- by now have queries sharing a name. Down migration leaves the relaxed
-- constraint in place.
ALTER TABLE queries DROP CONSTRAINT IF EXISTS queries_user_id_name_key;

-- +goose StatementEnd
