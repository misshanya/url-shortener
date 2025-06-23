package repository

import (
	"context"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/misshanya/url-shortener/statistics/internal/models"
)

type ClickHouseRepo struct {
	conn clickhouse.Conn
}

func NewClickHouseRepo(conn clickhouse.Conn) *ClickHouseRepo {
	return &ClickHouseRepo{conn: conn}
}

func (r *ClickHouseRepo) WriteShortened(ctx context.Context, events []models.ClickHouseEventShortened) error {
	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO shortened")
	if err != nil {
		return err
	}

	for _, event := range events {
		err := batch.AppendStruct(&event)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}

func (r *ClickHouseRepo) WriteUnshortened(ctx context.Context, events []models.ClickHouseEventUnshortened) error {
	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO unshortened")
	if err != nil {
		return err
	}

	for _, event := range events {
		err := batch.AppendStruct(&event)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}
