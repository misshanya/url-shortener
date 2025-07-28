package http

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func Test_ShortenURLBatch(t *testing.T) {
	tests := []struct {
		Name           string
		RequestBody    string
		ExceptedStatus int
		ExceptedBody   string
		SetUpMocks     func(service *mockservice)
	}{
		{
			Name: "Successfully Shortened 2 of 3",
			RequestBody: `
{
  "urls": [
    { "url": "https://go.dev" },
    { "url": "not a url" },
    { "url": "https://gitlab.com" }
  ]
}
`,
			ExceptedStatus: http.StatusCreated,
			ExceptedBody: `
{
  "urls": [
    {
      "short_url": "https://sh.some/3a",
      "original_url": "https://go.dev"
    },
    {
      "original_url": "not a url",
      "error": "parse \"not a url\": invalid URI for request"
    },
    {
      "short_url": "https://sh.some/3b",
      "original_url": "https://gitlab.com"
    }
  ]
}
`,
			SetUpMocks: func(service *mockservice) {
				service.On("ShortenURLBatch", mock.Anything, mock.AnythingOfType("[]*models.Short")).
					Run(func(args mock.Arguments) {
						urls := map[string]string{
							"https://go.dev":     "https://sh.some/3a",
							"https://gitlab.com": "https://sh.some/3b",
						}
						for _, short := range args.Get(1).([]*models.Short) {
							_, err := url.ParseRequestURI(short.OriginalURL)
							if err == nil {
								short.ShortURL = urls[short.OriginalURL]
							} else {
								short.Error = err.Error()
							}
						}
					}).Return(nil).Once()
			},
		},
		{
			Name: "Service returned an error",
			RequestBody: `
{
  "urls": [
    { "url": "https://go.dev" }
  ]
}
`,
			ExceptedStatus: http.StatusInternalServerError,
			ExceptedBody:   `{ "message": "some error :)" }`,
			SetUpMocks: func(service *mockservice) {
				service.On("ShortenURLBatch", mock.Anything, mock.AnythingOfType("[]*models.Short")).
					Return(&models.HTTPError{
						Code:    http.StatusInternalServerError,
						Message: "some error :)",
					}).Once()
			},
		},
		{
			Name: "Invalid request body",
			RequestBody: `
{
  "urls": [
    "https://github.com",
    "https://gitlab.com"
  ]
}
`,
			ExceptedStatus: http.StatusBadRequest,
			ExceptedBody:   `{ "message": "code=400, message=Unmarshal type error: expected=dto.ShortenURLRequest, got=string, field=urls, offset=39, internal=json: cannot unmarshal string into Go struct field ShortenURLBatchRequest.urls of type dto.ShortenURLRequest" }`,
			SetUpMocks:     func(service *mockservice) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockService := mockservice{}

			tt.SetUpMocks(&mockService)

			e := echo.New()

			req := httptest.NewRequest(http.MethodPost, "/shorten/batch", strings.NewReader(tt.RequestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)

			handler := NewHandler(&mockService)

			err := handler.ShortenURLBatch(c)
			if err != nil {
				e.HTTPErrorHandler(err, c)
			}

			assert.Equal(t, tt.ExceptedStatus, rec.Code)
			assert.JSONEq(t, tt.ExceptedBody, rec.Body.String())

			mockService.AssertExpectations(t)
		})
	}
}

func Test_UnshortenURL(t *testing.T) {
	tests := []struct {
		Name           string
		InputCode      string
		ExceptedStatus int
		ExceptedURL    string
		ExceptedBody   string
		SetUpMocks     func(service *mockservice)
	}{
		{
			Name:           "Successfully Unshortened",
			InputCode:      "3a",
			ExceptedStatus: http.StatusFound,
			ExceptedURL:    "https://go.dev",
			SetUpMocks: func(service *mockservice) {
				service.On("UnshortenURL", mock.Anything, "3a").
					Return("https://go.dev", nil).Once()
			},
		},
		{
			Name:           "Service returned an error",
			InputCode:      "3a",
			ExceptedStatus: http.StatusInternalServerError,
			ExceptedBody:   `{ "message": "some error :)" }`,
			SetUpMocks: func(service *mockservice) {
				service.On("UnshortenURL", mock.Anything, "3a").
					Return("", &models.HTTPError{
						Code:    http.StatusInternalServerError,
						Message: "some error :)",
					}).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockService := mockservice{}

			tt.SetUpMocks(&mockService)

			e := echo.New()

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s", tt.InputCode), nil)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)

			c.SetParamNames("code")
			c.SetParamValues(tt.InputCode)

			handler := NewHandler(&mockService)

			err := handler.UnshortenURL(c)
			if err != nil {
				e.HTTPErrorHandler(err, c)
			}

			assert.Equal(t, tt.ExceptedStatus, rec.Code)

			assert.Equal(t, tt.ExceptedURL, rec.Header().Get("Location"))

			if tt.ExceptedBody != "" {
				assert.JSONEq(t, tt.ExceptedBody, rec.Body.String())
			}

			mockService.AssertExpectations(t)
		})
	}
}
