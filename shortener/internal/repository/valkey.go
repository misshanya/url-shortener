package repository

import (
	"context"
	"errors"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/valkey-io/valkey-go"
	"time"
)

type ValkeyRepo struct {
	client valkey.Client
}

func NewValkeyRepo(client valkey.Client) *ValkeyRepo {
	return &ValkeyRepo{client: client}
}

func (r *ValkeyRepo) SetTop(ctx context.Context, top models.UnshortenedTop, ttl int) error {
	var errs error
	for _, v := range top.Top {
		err := r.client.Do(ctx,
			r.client.B().
				Set().
				Key(v.ShortCode).
				Value(v.OriginalURL).
				Nx().
				Ex(time.Duration(ttl)*time.Second).
				Build(),
		).Error()
		if err != nil {
			errors.Join(errs, err)
		}
	}

	return errs
}

func (r *ValkeyRepo) GetURLByCode(ctx context.Context, code string) (string, error) {
	url, err := r.client.Do(ctx, r.client.B().Get().Key(code).Build()).ToString()
	if errors.Is(err, valkey.Nil) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	return url, nil
}
