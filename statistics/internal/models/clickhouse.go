package models

import (
	"context"
	"github.com/google/uuid"
	"time"
)

type ClickHouseEventShortened struct {
	Ctx         context.Context
	EventID     uuid.UUID
	OriginalURL string
	ShortCode   string
	ShortenedAt time.Time
}

type ClickHouseEventUnshortened struct {
	Ctx           context.Context
	EventID       uuid.UUID
	OriginalURL   string
	ShortCode     string
	UnshortenedAt time.Time
}
