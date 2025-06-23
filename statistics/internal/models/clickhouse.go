package models

import (
	"github.com/google/uuid"
	"time"
)

type ClickHouseEventShortened struct {
	EventID     uuid.UUID
	OriginalURL string
	ShortCode   string
	ShortenedAt time.Time
}

type ClickHouseEventUnshortened struct {
	EventID       uuid.UUID
	OriginalURL   string
	ShortCode     string
	UnshortenedAt time.Time
}
