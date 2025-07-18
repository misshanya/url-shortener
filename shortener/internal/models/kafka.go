package models

import "time"

type KafkaMessageShortened struct {
	ShortenedAt time.Time `json:"shortened_at"`
	OriginalURL string    `json:"original_url"`
	ShortCode   string    `json:"short_code"`
}

type KafkaMessageUnshortened struct {
	UnshortenedAt time.Time `json:"unshortened_at"`
	OriginalURL   string    `json:"original_url"`
	ShortCode     string    `json:"short_code"`
}

type KafkaMessageUnshortenedTop struct {
	ValidUntil time.Time `json:"valid_until"`
	Top        []struct {
		OriginalURL string `json:"original_url"`
		ShortCode   string `json:"short_code"`
	} `json:"top"`
}
