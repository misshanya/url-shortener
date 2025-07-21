package dto

type ShortenURLRequest struct {
	URL string `json:"url"`
}

type ShortenURLResponse struct {
	ShortURL    string `json:"short_url,omitempty"`
	OriginalURL string `json:"original_url"`
	Error       string `json:"error,omitempty"`
}

type ShortenURLBatchRequest struct {
	URLs []ShortenURLRequest `json:"urls"`
}

type ShortenURLBatchResponse struct {
	URLs []ShortenURLResponse `json:"urls"`
}
