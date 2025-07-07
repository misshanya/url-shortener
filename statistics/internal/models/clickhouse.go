package models

import (
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/propagation"
	"time"
)

type ClickHouseEventShortened struct {
	Carrier     propagation.MapCarrier
	EventID     uuid.UUID
	OriginalURL string
	ShortCode   string
	ShortenedAt time.Time
}

type ClickHouseEventUnshortened struct {
	Carrier       propagation.MapCarrier
	EventID       uuid.UUID
	OriginalURL   string
	ShortCode     string
	UnshortenedAt time.Time
}
