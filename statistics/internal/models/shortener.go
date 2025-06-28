package models

import "time"

type UnshortenedTop struct {
	ValidUntil time.Time
	Top        []struct {
		OriginalURL string
		ShortCode   string
	}
}
