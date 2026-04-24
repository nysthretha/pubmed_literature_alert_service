-- +goose Up
-- +goose StatementBegin
UPDATE queries
SET query_string = '("HEART score"[tiab] OR "HEART pathway"[tiab]) AND humans[mh]'
WHERE name = 'HEART score';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE queries
SET query_string = '("HEART score"[tiab] OR "HEART pathway"[tiab])'
WHERE name = 'HEART score';
-- +goose StatementEnd
