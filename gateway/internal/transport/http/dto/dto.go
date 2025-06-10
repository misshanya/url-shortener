package dto

type ShortenURLRequest struct {
	URL string `json:"url"`
}

type ShortenURLResponse struct {
	URL string `json:"url"`
}
