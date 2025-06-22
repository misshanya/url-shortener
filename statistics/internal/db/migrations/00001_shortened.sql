-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS default.shortened
(
    EventID UUID,
    OriginalURL String,
    ShortCode String,
    ShortenedAt DateTime
)
ENGINE = MergeTree
ORDER BY ShortenedAt;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS default.shortened;
-- +goose StatementEnd
