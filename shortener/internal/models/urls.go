package models

import "time"

type Short struct {
	URL   string
	Short string
}

type UnshortenedTop struct {
	ValidUntil time.Time
	Top        []struct {
		OriginalURL string
		ShortCode   string
	}
}
