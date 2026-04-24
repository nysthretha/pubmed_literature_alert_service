-- +goose Up
-- +goose StatementBegin
CREATE TABLE digests (
    id               BIGSERIAL PRIMARY KEY,
    sent_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_local_date  DATE,
    articles_included INTEGER NOT NULL DEFAULT 0,
    status           TEXT NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
    error_message    TEXT,
    manual           BOOLEAN NOT NULL DEFAULT false
);

-- Enforces "at most one sent digest per local date" on the scheduled path.
-- Pending and failed rows are excluded; manual sends share the slot with scheduled.
CREATE UNIQUE INDEX digests_one_sent_per_day
    ON digests (sent_local_date)
    WHERE status = 'sent';

CREATE INDEX idx_digests_sent_at ON digests (sent_at DESC);

CREATE TABLE digest_articles (
    digest_id BIGINT NOT NULL REFERENCES digests(id) ON DELETE CASCADE,
    pmid      TEXT   NOT NULL REFERENCES articles(pmid) ON DELETE CASCADE,
    PRIMARY KEY (digest_id, pmid)
);

CREATE INDEX idx_digest_articles_pmid ON digest_articles (pmid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS digest_articles;
DROP TABLE IF EXISTS digests;
-- +goose StatementEnd
