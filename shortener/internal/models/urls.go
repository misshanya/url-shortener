package models

type Short struct {
	URL   string
	Short string
}

type UnshortenedTop struct {
	Top []struct {
		OriginalURL string
		ShortCode   string
	}
}
