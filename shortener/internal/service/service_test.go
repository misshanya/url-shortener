package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace/noop"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func Test_ShortenURL(t *testing.T) {
	tests := []struct {
		Name         string
		OriginalURL  string
		ExpectedCode string
		WantErr      bool
		SetUpMocks   func(db *mockpostgresRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup)
		WaitForKafka bool
	}{
		{
			Name:         "New URL",
			OriginalURL:  "https://google.com",
			ExpectedCode: "1",
			WantErr:      false,
			SetUpMocks: func(db *mockpostgresRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				db.On("GetID", mock.Anything, "https://google.com").
					Return(int64(0), sql.ErrNoRows).Once()
				db.On("StoreURL", mock.Anything, "https://google.com").
					Return(int64(1), nil).Once()
				kafkaWriter.On("WriteMessages", mock.Anything, mock.Anything).
					Return(nil).Once().Run(func(args mock.Arguments) { wg.Done() })
			},
			WaitForKafka: true,
		},
		{
			Name:        "New URL, failed to store",
			OriginalURL: "https://google.com",
			WantErr:     true,
			SetUpMocks: func(db *mockpostgresRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				db.On("GetID", mock.Anything, "https://google.com").
					Return(int64(0), sql.ErrNoRows).Once()
				db.On("StoreURL", mock.Anything, "https://google.com").
					Return(int64(0), errors.New("some unknown error")).Once()
			},
		},
		{
			Name:         "Existing URL",
			OriginalURL:  "https://google.com",
			ExpectedCode: "1",
			WantErr:      false,
			SetUpMocks: func(db *mockpostgresRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				db.On("GetID", mock.Anything, "https://google.com").
					Return(int64(1), nil).Once()
			},
		},
		{
			Name:        "Failed to get from DB",
			OriginalURL: "https://google.com",
			WantErr:     true,
			SetUpMocks: func(db *mockpostgresRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				db.On("GetID", mock.Anything, "https://google.com").
					Return(int64(0), errors.New("some unknown error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockPostgres := mockpostgresRepo{}
			mockKafka := mockkafkaWriter{}

			var wg sync.WaitGroup

			if tt.WaitForKafka {
				wg.Add(1)
			}

			tt.SetUpMocks(&mockPostgres, &mockKafka, &wg)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			service := New(
				&mockPostgres,
				nil,
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{},
					),
				),
				&mockKafka,
				tracer,
				10,
			)

			short := &models.Short{URL: tt.OriginalURL}

			err := service.ShortenURL(context.Background(), short)
			if tt.WantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.ExpectedCode, short.Short)
			}

			if tt.WaitForKafka {
				wg.Wait()
			}

			mockPostgres.AssertExpectations(t)
			mockKafka.AssertExpectations(t)
		})
	}
}

func Test_GetURL(t *testing.T) {
	tests := []struct {
		Name         string
		ShortCode    string
		ExceptedURL  string
		WantErr      bool
		SetUpMocks   func(db *mockpostgresRepo, valkey *mockvalkeyRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup)
		WaitForKafka bool
	}{
		{
			Name:        "Existing URL",
			ShortCode:   "3a",
			ExceptedURL: "https://google.com",
			WantErr:     false,
			SetUpMocks: func(db *mockpostgresRepo, valkey *mockvalkeyRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				valkey.On("GetURLByCode", mock.Anything, "3a").
					Return("", nil).Once()
				db.On("GetURL", mock.Anything, int64(222)).
					Return("https://google.com", nil).Once()
				kafkaWriter.On("WriteMessages", mock.Anything, mock.Anything).
					Return(nil).Once().Run(func(args mock.Arguments) { wg.Done() })
			},
			WaitForKafka: true,
		},
		{
			Name:        "Existing URL in cache",
			ShortCode:   "3a",
			ExceptedURL: "https://google.com",
			WantErr:     false,
			SetUpMocks: func(db *mockpostgresRepo, valkey *mockvalkeyRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				valkey.On("GetURLByCode", mock.Anything, "3a").
					Return("https://google.com", nil).Once()
				kafkaWriter.On("WriteMessages", mock.Anything, mock.Anything).
					Return(nil).Once().Run(func(args mock.Arguments) { wg.Done() })
			},
			WaitForKafka: true,
		},
		{
			Name:      "Non-existing URL",
			ShortCode: "3a",
			WantErr:   true,
			SetUpMocks: func(db *mockpostgresRepo, valkey *mockvalkeyRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				valkey.On("GetURLByCode", mock.Anything, "3a").
					Return("", nil).Once()
				db.On("GetURL", mock.Anything, int64(222)).
					Return("", sql.ErrNoRows).Once()
			},
		},
		{
			Name:      "Failed to get from DB",
			ShortCode: "3a",
			WantErr:   true,
			SetUpMocks: func(db *mockpostgresRepo, valkey *mockvalkeyRepo, kafkaWriter *mockkafkaWriter, wg *sync.WaitGroup) {
				valkey.On("GetURLByCode", mock.Anything, "3a").
					Return("", nil).Once()
				db.On("GetURL", mock.Anything, int64(222)).
					Return("", errors.New("some unknown error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockPostgres := mockpostgresRepo{}
			mockKafka := mockkafkaWriter{}
			mockValkey := mockvalkeyRepo{}

			var wg sync.WaitGroup

			if tt.WaitForKafka {
				wg.Add(1)
			}

			tt.SetUpMocks(&mockPostgres, &mockValkey, &mockKafka, &wg)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			service := New(
				&mockPostgres,
				&mockValkey,
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{},
					),
				),
				&mockKafka,
				tracer,
				10,
			)

			url, err := service.GetURL(context.Background(), tt.ShortCode)
			if tt.WantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.ExceptedURL, url)
			}

			if tt.WaitForKafka {
				wg.Wait()
			}

			mockPostgres.AssertExpectations(t)
			mockValkey.AssertExpectations(t)
			mockKafka.AssertExpectations(t)
		})
	}
}

func Test_SetTop(t *testing.T) {
	tests := []struct {
		Name         string
		InputMessage *models.KafkaMessageUnshortenedTop
		SetUpMocks   func(valkey *mockvalkeyRepo, exceptedTop models.UnshortenedTop)
	}{
		{
			Name: "Successfully cached top",
			InputMessage: &models.KafkaMessageUnshortenedTop{
				ValidUntil: time.Now().Add(time.Hour),
				Top: []struct {
					OriginalURL string `json:"original_url"`
					ShortCode   string `json:"short_code"`
				}{
					{
						OriginalURL: "https://go.dev",
						ShortCode:   "3a",
					},
					{
						OriginalURL: "https://github.com",
						ShortCode:   "1",
					},
				},
			},
			SetUpMocks: func(valkey *mockvalkeyRepo, exceptedTop models.UnshortenedTop) {
				valkey.On("SetTop", mock.Anything, exceptedTop, mock.Anything).
					Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockValkey := mockvalkeyRepo{}

			exceptedTop := models.UnshortenedTop{
				ValidUntil: tt.InputMessage.ValidUntil,
				Top: []struct {
					OriginalURL string
					ShortCode   string
				}(tt.InputMessage.Top),
			}

			tt.SetUpMocks(&mockValkey, exceptedTop)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			service := New(
				nil,
				&mockValkey,
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{},
					),
				),
				nil,
				tracer,
				10,
			)

			service.SetTop(context.Background(), tt.InputMessage)

			mockValkey.AssertExpectations(t)
		})
	}
}
