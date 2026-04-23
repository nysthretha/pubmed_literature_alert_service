-- +goose Up
-- +goose StatementBegin
CREATE TABLE queries (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    query_string    TEXT NOT NULL,
    last_polled_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE articles (
    pmid               TEXT PRIMARY KEY,
    title              TEXT NOT NULL,
    abstract           TEXT,
    journal            TEXT,
    publication_date   DATE,
    authors            JSONB NOT NULL DEFAULT '[]'::jsonb,
    publication_types  TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    fetched_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_xml            TEXT
);

CREATE TABLE query_matches (
    query_id    BIGINT      NOT NULL REFERENCES queries(id) ON DELETE CASCADE,
    pmid        TEXT        NOT NULL REFERENCES articles(pmid) ON DELETE CASCADE,
    matched_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (query_id, pmid)
);

CREATE INDEX idx_articles_pub_date ON articles (publication_date DESC);
CREATE INDEX idx_query_matches_matched_at ON query_matches (matched_at DESC);

INSERT INTO queries (name, query_string) VALUES
    ('HEART score', '("HEART score"[tiab] OR "HEART pathway"[tiab])');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS query_matches;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS queries;
-- +goose StatementEnd
