-- name: StoreShort :exec
INSERT INTO urls (url, short) VALUES ($1, $2);

-- name: GetURLByShort :one
SELECT url FROM urls WHERE short = $1;

-- name: GetShortByURL :one
SELECT short FROM urls WHERE url = $1;
