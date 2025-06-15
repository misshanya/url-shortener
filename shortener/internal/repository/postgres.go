package repository

import (
	"context"
	"github.com/misshanya/url-shortener/shortener/internal/db/sqlc/storage"
)

type PostgresRepo struct {
	queries *storage.Queries
}

func NewPostgresRepo(queries *storage.Queries) *PostgresRepo {
	return &PostgresRepo{queries: queries}
}

func (r *PostgresRepo) StoreURL(ctx context.Context, url string) (int64, error) {
	return r.queries.StoreShort(ctx, url)
}

func (r *PostgresRepo) GetID(ctx context.Context, url string) (int64, error) {
	return r.queries.GetID(ctx, url)
}

func (r *PostgresRepo) GetURL(ctx context.Context, id int64) (string, error) {
	return r.queries.GetURLByID(ctx, id)
}
