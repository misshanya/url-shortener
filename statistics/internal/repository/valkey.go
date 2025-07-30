package repository

import (
	"context"
	"fmt"
	"github.com/misshanya/url-shortener/statistics/internal/errorz"
	"github.com/valkey-io/valkey-go"
	"time"
)

type ValkeyRepo struct {
	client valkey.Client
}

func NewValkeyRepo(client valkey.Client) *ValkeyRepo {
	return &ValkeyRepo{client: client}
}

// Lock tries to write to Valkey lock record with specified TTL
// If record already exists, returns errorz.ErrTopLocked
func (r *ValkeyRepo) Lock(ctx context.Context, ttl time.Duration) error {
	res := r.client.Do(ctx, r.client.B().
		Set().
		Key("top_lock").
		Value("locked").
		Nx().
		Ex(ttl).
		Build(),
	)
	if msg, _ := res.ToMessage(); msg.IsNil() {
		return errorz.ErrTopLocked
	}
	if res.Error() != nil {
		return fmt.Errorf("failed to query valkey: %w", res.Error())
	}
	return nil
}
