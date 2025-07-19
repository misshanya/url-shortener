package http

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	"github.com/misshanya/url-shortener/gateway/internal/transport/http/dto"
	"net/http"
)

type service interface {
	ShortenURL(ctx context.Context, url string) (string, *models.HTTPError)
	ShortenURLBatch(ctx context.Context, urls []*models.Short) *models.HTTPError
	UnshortenURL(ctx context.Context, code string) (string, *models.HTTPError)
}

type Handler struct {
	service service
}

func NewHandler(service service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ShortenURL(c echo.Context) error {
	ctx := c.Request().Context()

	var req dto.ShortenURLRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	url, httpErr := h.service.ShortenURL(ctx, req.URL)
	if httpErr != nil {
		return echo.NewHTTPError(httpErr.Code, httpErr.Message)
	}

	resp := &dto.ShortenURLResponse{ShortURL: url, OriginalURL: req.URL}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ShortenURLBatch(c echo.Context) error {
	ctx := c.Request().Context()

	var req dto.ShortenURLBatchRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	urls := make([]*models.Short, len(req.URLs))
	for i, url := range req.URLs {
		urls[i] = &models.Short{
			OriginalURL: url.URL,
		}
	}

	if httpErr := h.service.ShortenURLBatch(ctx, urls); httpErr != nil {
		return echo.NewHTTPError(httpErr.Code, httpErr.Message)
	}

	urlsForResp := make([]dto.ShortenURLResponse, len(urls))
	for i, url := range urls {
		urlsForResp[i].OriginalURL = url.OriginalURL
		urlsForResp[i].ShortURL = url.ShortURL
		urlsForResp[i].Error = url.Error
	}
	resp := &dto.ShortenURLBatchResponse{URLs: urlsForResp}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UnshortenURL(c echo.Context) error {
	ctx := c.Request().Context()

	code := c.Param("code")

	url, httpErr := h.service.UnshortenURL(ctx, code)
	if httpErr != nil {
		return echo.NewHTTPError(httpErr.Code, httpErr.Message)
	}

	return c.Redirect(http.StatusFound, url)
}
