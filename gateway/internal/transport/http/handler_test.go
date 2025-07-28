package http

import (
	"github.com/labstack/echo/v4"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_ShortenURL(t *testing.T) {
	tests := []struct {
		Name           string
		RequestBody    string
		ExceptedStatus int
		ExceptedBody   string
		SetUpMocks     func(service *mockservice)
	}{
		{
			Name:           "Successfully Shortened",
			RequestBody:    `{ "url": "https://go.dev" }`,
			ExceptedStatus: http.StatusCreated,
			ExceptedBody:   `{ "short_url": "https://sh.some/3a", "original_url": "https://go.dev" }`,
			SetUpMocks: func(service *mockservice) {
				service.On("ShortenURL", mock.Anything, "https://go.dev").
					Return("https://sh.some/3a", nil).Once()
			},
		},
		{
			Name:           "URL in request body is not a string",
			RequestBody:    `{ "url": 1 }`,
			ExceptedStatus: http.StatusBadRequest,
			ExceptedBody:   `{ "message": "code=400, message=Unmarshal type error: expected=string, got=number, field=url, offset=10, internal=json: cannot unmarshal number into Go struct field ShortenURLRequest.url of type string" }`,
			SetUpMocks:     func(service *mockservice) {},
		},
		{
			Name:           "Service returned an internal server error",
			RequestBody:    `{ "url": "https://go.dev" }`,
			ExceptedStatus: http.StatusInternalServerError,
			ExceptedBody:   `{ "message": "Unknown error :)" }`,
			SetUpMocks: func(service *mockservice) {
				service.On("ShortenURL", mock.Anything, "https://go.dev").
					Return(
						"",
						&models.HTTPError{
							Code:    http.StatusInternalServerError,
							Message: "Unknown error :)",
						},
					).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockService := mockservice{}

			tt.SetUpMocks(&mockService)

			e := echo.New()

			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(tt.RequestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)

			handler := NewHandler(&mockService)

			err := handler.ShortenURL(c)
			if err != nil {
				e.HTTPErrorHandler(err, c)
			}

			assert.Equal(t, tt.ExceptedStatus, rec.Code)
			assert.JSONEq(t, tt.ExceptedBody, rec.Body.String())

			mockService.AssertExpectations(t)
		})
	}
}
