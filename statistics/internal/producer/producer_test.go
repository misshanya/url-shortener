package producer

import (
	"context"
	"errors"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace/noop"
	"log/slog"
	"os"
	"testing"
	"time"
)

func Test_sendTopToKafka(t *testing.T) {
	tests := []struct {
		Name       string
		InputTop   models.UnshortenedTop
		WantErr    bool
		SetUpMocks func(kw *mockkafkaWriter)
	}{
		{
			Name: "Successfully Sent",
			InputTop: models.UnshortenedTop{
				ValidUntil: time.Now().Add(5 * time.Minute),
				Top: []struct {
					OriginalURL string
					ShortCode   string
				}{
					{
						OriginalURL: "https://go.dev",
						ShortCode:   "3a",
					},
					{
						OriginalURL: "https://github.com",
						ShortCode:   "3b",
					},
				},
			},
			WantErr: false,
			SetUpMocks: func(kw *mockkafkaWriter) {
				kw.On("WriteMessages", mock.Anything, mock.Anything).
					Return(nil).Once()
			},
		},
		{
			Name: "Failed to WriteMessages",
			InputTop: models.UnshortenedTop{
				ValidUntil: time.Now().Add(5 * time.Minute),
				Top: []struct {
					OriginalURL string
					ShortCode   string
				}{
					{
						OriginalURL: "https://go.dev",
						ShortCode:   "3a",
					},
				},
			},
			WantErr: true,
			SetUpMocks: func(kw *mockkafkaWriter) {
				kw.On("WriteMessages", mock.Anything, mock.Anything).
					Return(errors.New("something went wrong")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			kw := mockkafkaWriter{}

			tt.SetUpMocks(&kw)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			producer := New(
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{},
					),
				),
				nil,
				&kw,
				300,
				5,
				tracer,
			)

			err := producer.sendTopToKafka(context.Background(), tt.InputTop)
			if tt.WantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			kw.AssertExpectations(t)
		})
	}
}
