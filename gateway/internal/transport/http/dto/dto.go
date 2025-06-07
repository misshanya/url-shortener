package dto

type ShortURLRequest struct {
	URL string `json:"url"`
}

type ShortURLResponse struct {
	URL string `json:"url"`
}
