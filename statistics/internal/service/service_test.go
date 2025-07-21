package service

import (
	"context"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace/noop"
	"log/slog"
	"os"
	"testing"
	"time"
)

func Test_Shortened(t *testing.T) {
	tests := []struct {
		Name         string
		InputMessage *models.KafkaMessageShortened
		SetUpMocks   func(metrics *mockmetricsProvider)
	}{
		{
			Name: "Successfully Shortened",
			InputMessage: &models.KafkaMessageShortened{
				ShortenedAt: time.Now(),
				OriginalURL: "https://go.dev",
				ShortCode:   "3a",
			},
			SetUpMocks: func(metrics *mockmetricsProvider) {
				metrics.On("Shorten").Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockMetrics := mockmetricsProvider{}

			tt.SetUpMocks(&mockMetrics)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			shortenedCh := make(chan models.ClickHouseEventShortened, 10)
			unshortenedCh := make(chan models.ClickHouseEventUnshortened, 10)

			service := New(
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{}),
				),
				shortenedCh,
				unshortenedCh,
				&mockMetrics,
				nil,
				tracer,
				10,
			)

			service.Shortened(context.Background(), tt.InputMessage)

			mockMetrics.AssertExpectations(t)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			select {
			case event := <-shortenedCh:
				assert.Equal(t, tt.InputMessage.ShortCode, event.ShortCode)
				assert.Equal(t, tt.InputMessage.OriginalURL, event.OriginalURL)
				assert.Equal(t, tt.InputMessage.ShortenedAt, event.ShortenedAt)
			case <-ctx.Done():
				t.Fatal("didn't get event in the channel")
			}

			// Check that we didn't get the unshortened event
			ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			select {
			case <-unshortenedCh:
				t.Fatal("got event in the unshortened channel")
			case <-ctx.Done():
			}
		})
	}
}

func Test_Unshortened(t *testing.T) {
	tests := []struct {
		Name         string
		InputMessage *models.KafkaMessageUnshortened
		SetUpMocks   func(metrics *mockmetricsProvider)
	}{
		{
			Name: "Successfully Unshortened",
			InputMessage: &models.KafkaMessageUnshortened{
				UnshortenedAt: time.Now(),
				OriginalURL:   "https://go.dev",
				ShortCode:     "3a",
			},
			SetUpMocks: func(metrics *mockmetricsProvider) {
				metrics.On("Unshorten").Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockMetrics := mockmetricsProvider{}

			tt.SetUpMocks(&mockMetrics)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			shortenedCh := make(chan models.ClickHouseEventShortened, 10)
			unshortenedCh := make(chan models.ClickHouseEventUnshortened, 10)

			service := New(
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{}),
				),
				shortenedCh,
				unshortenedCh,
				&mockMetrics,
				nil,
				tracer,
				10,
			)

			service.Unshortened(context.Background(), tt.InputMessage)

			mockMetrics.AssertExpectations(t)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			select {
			case event := <-unshortenedCh:
				assert.Equal(t, tt.InputMessage.ShortCode, event.ShortCode)
				assert.Equal(t, tt.InputMessage.OriginalURL, event.OriginalURL)
				assert.Equal(t, tt.InputMessage.UnshortenedAt, event.UnshortenedAt)
			case <-ctx.Done():
				t.Fatal("didn't get event in the channel")
			}

			// Check that we didn't get the shortened event
			ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			select {
			case <-shortenedCh:
				t.Fatal("got event in the shortened channel")
			case <-ctx.Done():
			}
		})
	}
}
