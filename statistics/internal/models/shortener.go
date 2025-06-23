package models

type UnshortenedTop struct {
	Top []struct {
		OriginalURL string
		ShortCode   string
	}
}
