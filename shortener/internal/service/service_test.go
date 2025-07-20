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
