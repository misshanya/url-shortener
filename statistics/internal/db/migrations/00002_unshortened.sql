-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS default.unshortened
(
    EventID UUID,
    OriginalURL String,
    ShortCode String,
    UnshortenedAt DateTime
)
ENGINE = MergeTree
ORDER BY UnshortenedAt;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS default.unshortened;
-- +goose StatementEnd
