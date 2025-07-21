package service

import (
	"context"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace/noop"
	"log/slog"
	"os"
	"sync"
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

func TestService_ShortenedBatchWriter(t *testing.T) {
	tests := []struct {
		Name                    string
		AmountOfShortenedEvents int
		Tick                    bool
		BatchSize               int
		SetUpMocks              func(clickhouse *mockclickHouseRepo, wg *sync.WaitGroup)
	}{
		{
			Name:                    "Write 5 events once on ticker",
			AmountOfShortenedEvents: 5,
			Tick:                    true,
			BatchSize:               10,
			SetUpMocks: func(clickhouse *mockclickHouseRepo, wg *sync.WaitGroup) {
				clickhouse.On("WriteShortened", mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) { wg.Done() })
			},
		},
		{
			Name:                    "Write 10 events once on batch size limit",
			AmountOfShortenedEvents: 10,
			Tick:                    false,
			BatchSize:               10,
			SetUpMocks: func(clickhouse *mockclickHouseRepo, wg *sync.WaitGroup) {
				clickhouse.On("WriteShortened", mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) { wg.Done() })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockClickHouse := mockclickHouseRepo{}

			var wg sync.WaitGroup

			tt.SetUpMocks(&mockClickHouse, &wg)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			shortenedCh := make(chan models.ClickHouseEventShortened, tt.AmountOfShortenedEvents)
			unshortenedCh := make(chan models.ClickHouseEventUnshortened)

			ticks := make(chan time.Time)

			service := New(
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{}),
				),
				shortenedCh,
				unshortenedCh,
				nil,
				&mockClickHouse,
				tracer,
				tt.BatchSize,
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go service.ShortenedBatchWriter(ctx, ticks)

			for range tt.AmountOfShortenedEvents {
				shortenedCh <- models.ClickHouseEventShortened{
					OriginalURL: "https://go.dev",
					ShortCode:   "3a",
					ShortenedAt: time.Now(),
				}
			}

			var exceptedClickHouseWrites int

			exceptedClickHouseWrites = tt.AmountOfShortenedEvents / tt.BatchSize
			remainingEvents := tt.AmountOfShortenedEvents % tt.BatchSize
			if remainingEvents > 0 && tt.Tick {
				exceptedClickHouseWrites++
			}

			wg.Add(exceptedClickHouseWrites)

			// Tick if needed
			if tt.Tick {
				ticks <- time.Time{}
			}

			// Wait for the repo calls
			wg.Wait()

			mockClickHouse.AssertExpectations(t)
		})
	}
}

func TestService_UnshortenedBatchWriter(t *testing.T) {
	tests := []struct {
		Name                      string
		AmountOfUnshortenedEvents int
		Tick                      bool
		BatchSize                 int
		SetUpMocks                func(clickhouse *mockclickHouseRepo, wg *sync.WaitGroup)
	}{
		{
			Name:                      "Write 5 events once on ticker",
			AmountOfUnshortenedEvents: 5,
			Tick:                      true,
			BatchSize:                 10,
			SetUpMocks: func(clickhouse *mockclickHouseRepo, wg *sync.WaitGroup) {
				clickhouse.On("WriteUnshortened", mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) { wg.Done() })
			},
		},
		{
			Name:                      "Write 10 events once on batch size limit",
			AmountOfUnshortenedEvents: 10,
			Tick:                      false,
			BatchSize:                 10,
			SetUpMocks: func(clickhouse *mockclickHouseRepo, wg *sync.WaitGroup) {
				clickhouse.On("WriteUnshortened", mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) { wg.Done() })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockClickHouse := mockclickHouseRepo{}

			var wg sync.WaitGroup

			tt.SetUpMocks(&mockClickHouse, &wg)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			shortenedCh := make(chan models.ClickHouseEventShortened)
			unshortenedCh := make(chan models.ClickHouseEventUnshortened, tt.AmountOfUnshortenedEvents)

			ticks := make(chan time.Time)

			service := New(
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{}),
				),
				shortenedCh,
				unshortenedCh,
				nil,
				&mockClickHouse,
				tracer,
				tt.BatchSize,
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go service.UnshortenedBatchWriter(ctx, ticks)

			for range tt.AmountOfUnshortenedEvents {
				unshortenedCh <- models.ClickHouseEventUnshortened{
					OriginalURL:   "https://go.dev",
					ShortCode:     "3a",
					UnshortenedAt: time.Now(),
				}
			}

			var exceptedClickHouseWrites int

			exceptedClickHouseWrites = tt.AmountOfUnshortenedEvents / tt.BatchSize
			remainingEvents := tt.AmountOfUnshortenedEvents % tt.BatchSize
			if remainingEvents > 0 && tt.Tick {
				exceptedClickHouseWrites++
			}

			wg.Add(exceptedClickHouseWrites)

			// Tick if needed
			if tt.Tick {
				ticks <- time.Time{}
			}

			// Wait for the repo calls
			wg.Wait()

			mockClickHouse.AssertExpectations(t)
		})
	}
}
