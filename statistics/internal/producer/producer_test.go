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
	"sync"
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

func Test_ProduceTop(t *testing.T) {
	tests := []struct {
		Name                  string
		AmountOfTicks         int
		AmountOfServiceCalled int
		AmountOfSendTopCalled int
		TopAmount             int
		TopTTL                int
		Top                   *models.UnshortenedTop
		SetUpMocks            func(
			kw *mockkafkaWriter,
			svc *mockservice,
			top *models.UnshortenedTop,
			topAmount, topTTL int,
			kwTimes, svcTimes int,
			kafkaWg *sync.WaitGroup, svcWg *sync.WaitGroup,
		)
	}{
		{
			Name:                  "Successfully produced 3 times",
			AmountOfTicks:         3,
			AmountOfServiceCalled: 3,
			AmountOfSendTopCalled: 3,
			TopAmount:             2,
			TopTTL:                60,
			Top: &models.UnshortenedTop{
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
			SetUpMocks: func(
				kw *mockkafkaWriter,
				svc *mockservice,
				top *models.UnshortenedTop,
				topAmount, topTTL int,
				kwTimes, svcTimes int,
				kafkaWg *sync.WaitGroup, svcWg *sync.WaitGroup,
			) {
				kw.On("WriteMessages", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						kafkaWg.Done()
					}).Return(nil).Times(kwTimes)
				svc.On("GetTopUnshortened", mock.Anything, topAmount, topTTL).
					Run(func(args mock.Arguments) {
						svcWg.Done()
					}).Return(top, nil).Times(svcTimes)
			},
		},
		{
			Name:                  "Service returned an empty top",
			AmountOfTicks:         1,
			AmountOfServiceCalled: 1,
			AmountOfSendTopCalled: 0,
			TopAmount:             2,
			TopTTL:                60,
			Top: &models.UnshortenedTop{
				ValidUntil: time.Now().Add(5 * time.Minute),
				Top: []struct {
					OriginalURL string
					ShortCode   string
				}{},
			},
			SetUpMocks: func(
				kw *mockkafkaWriter,
				svc *mockservice,
				top *models.UnshortenedTop,
				topAmount, topTTL int,
				kwTimes, svcTimes int,
				kafkaWg *sync.WaitGroup, svcWg *sync.WaitGroup,
			) {
				svc.On("GetTopUnshortened", mock.Anything, topAmount, topTTL).
					Run(func(args mock.Arguments) {
						svcWg.Done()
					}).Return(top, nil).Times(svcTimes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			kw := mockkafkaWriter{}
			svc := mockservice{}

			var kafkaWg, svcWg sync.WaitGroup
			kafkaWg.Add(tt.AmountOfSendTopCalled)
			svcWg.Add(tt.AmountOfServiceCalled)

			tt.SetUpMocks(
				&kw,
				&svc,
				tt.Top,
				tt.TopAmount, tt.TopTTL,
				tt.AmountOfServiceCalled, tt.AmountOfSendTopCalled,
				&kafkaWg, &svcWg,
			)

			tracerProvider := noop.NewTracerProvider()
			tracer := tracerProvider.Tracer("")

			producer := New(
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{},
					),
				),
				&svc,
				&kw,
				tt.TopTTL,
				tt.TopAmount,
				tracer,
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tick := make(chan time.Time)

			go producer.ProduceTop(ctx, tick)

			for range tt.AmountOfTicks {
				tick <- time.Time{}
			}

			svcWg.Wait()
			kafkaWg.Wait()

			kw.AssertExpectations(t)
			svc.AssertExpectations(t)
		})
	}
}
