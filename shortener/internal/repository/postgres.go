package repository

import (
	"context"
	"github.com/misshanya/url-shortener/shortener/internal/db/sqlc/storage"
	"github.com/misshanya/url-shortener/shortener/internal/models"
)

type PostgresRepo struct {
	queries *storage.Queries
}

func NewPostgresRepo(queries *storage.Queries) *PostgresRepo {
	return &PostgresRepo{queries: queries}
}

func (r *PostgresRepo) StoreShort(ctx context.Context, short models.Short) error {
	return r.queries.StoreShort(ctx, storage.StoreShortParams{
		Url:   short.URL,
		Short: short.Short,
	})
}

func (r *PostgresRepo) GetShort(ctx context.Context, url string) (string, error) {
	return r.queries.GetShortByURL(ctx, url)
}

func (r *PostgresRepo) GetURL(ctx context.Context, short string) (string, error) {
	return r.queries.GetURLByShort(ctx, short)
}
