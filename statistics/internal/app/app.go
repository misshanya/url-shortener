package app

import (
	"context"
	"errors"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/misshanya/url-shortener/statistics/internal/config"
	"github.com/misshanya/url-shortener/statistics/internal/consumer"
	"github.com/misshanya/url-shortener/statistics/internal/metrics"
	"github.com/misshanya/url-shortener/statistics/internal/service"
	"github.com/segmentio/kafka-go"
	"log/slog"
	"net/http"
)

type App struct {
	cfg         *config.Config
	l           *slog.Logger
	kafkaReader *kafka.Reader
	consumer    *consumer.Consumer
	e           *echo.Echo
}

func New(cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	// Create a Kafka reader
	a.kafkaReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{cfg.Kafka.Addr},
		GroupID:     "statistics-group",
		GroupTopics: []string{"shortener.shortened", "shortener.unshortened"},
	})

	// Init metrics
	m := metrics.New()

	// Create http server to gather metrics from
	a.e = echo.New()
	a.e.Use(middleware.Logger())
	a.e.GET("/metrics", echoprometheus.NewHandler())

	svc := service.New(a.l, m)
	a.consumer = consumer.New(a.l, a.kafkaReader, svc)

	return a, nil
}

func (a *App) Start(ctx context.Context, errChan chan<- error) {
	a.l.Info("starting consumer and http server for prometheus")
	go a.consumer.ReadMessages(ctx)
	if err := a.e.Start(a.cfg.HttpSrv.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errChan <- err
	}
}

func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

	// Close Kafka connection
	if err := a.kafkaReader.Close(); err != nil {
		stopErr = errors.Join(stopErr, err)
	}

	// Stop metrics server
	a.l.Info("Stopping http server...")
	if err := a.e.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, err)
	}

	if stopErr != nil {
		return stopErr
	}

	a.l.Info("Stopped gracefully")
	return nil
}
