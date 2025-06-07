package http

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	"github.com/misshanya/url-shortener/gateway/internal/transport/http/dto"
	"net/http"
)

type service interface {
	ShortURL(ctx context.Context, url string) (string, *models.HTTPError)
	UnshortURL(ctx context.Context, hash string) (string, *models.HTTPError)
}

type Handler struct {
	service service
}

func NewHandler(service service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ShortURL(c echo.Context) error {
	ctx := c.Request().Context()

	var req dto.ShortURLRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	url, httpErr := h.service.ShortURL(ctx, req.URL)
	if httpErr != nil {
		return echo.NewHTTPError(httpErr.Code, httpErr.Message)
	}

	resp := &dto.ShortURLResponse{URL: url}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UnshortURL(c echo.Context) error {
	ctx := c.Request().Context()

	hash := c.Param("hash")

	url, httpErr := h.service.UnshortURL(ctx, hash)
	if httpErr != nil {
		return echo.NewHTTPError(httpErr.Code, httpErr.Message)
	}

	return c.Redirect(http.StatusFound, url)
}
