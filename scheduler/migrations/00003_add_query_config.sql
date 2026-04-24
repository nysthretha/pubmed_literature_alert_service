-- +goose Up
-- +goose StatementBegin
ALTER TABLE queries
    ADD COLUMN poll_interval_seconds      INTEGER NOT NULL DEFAULT 21600,
    ADD COLUMN is_active                  BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN min_abstract_length        INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN publication_type_allowlist TEXT[],
    ADD COLUMN publication_type_blocklist TEXT[] NOT NULL DEFAULT ARRAY['Comment', 'Retraction of Publication', 'Published Erratum']::TEXT[],
    ADD COLUMN notes                      TEXT;

UPDATE queries
SET min_abstract_length = 200,
    notes = 'Clinical decision tool for stratifying ED chest pain patients by MACE risk. Low scores (0-3) support safe early discharge; high scores warrant admission and workup.'
WHERE name = 'HEART score';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE queries
    DROP COLUMN IF EXISTS notes,
    DROP COLUMN IF EXISTS publication_type_blocklist,
    DROP COLUMN IF EXISTS publication_type_allowlist,
    DROP COLUMN IF EXISTS min_abstract_length,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS poll_interval_seconds;
-- +goose StatementEnd
