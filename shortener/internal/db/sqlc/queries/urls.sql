-- name: StoreShort :one
INSERT INTO urls (url) VALUES ($1)
RETURNING id;

-- name: GetID :one
SELECT id FROM urls WHERE url = $1;

-- name: GetURLByID :one
SELECT url FROM urls WHERE id = $1;
