package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/misshanya/url-shortener/statistics/internal/config"
	"github.com/misshanya/url-shortener/statistics/internal/consumer"
	"github.com/misshanya/url-shortener/statistics/internal/db"
	"github.com/misshanya/url-shortener/statistics/internal/metrics"
	"github.com/misshanya/url-shortener/statistics/internal/repository"
	"github.com/misshanya/url-shortener/statistics/internal/service"
	"github.com/segmentio/kafka-go"

	"log/slog"
	"net/http"
	"time"
)

type App struct {
	cfg         *config.Config
	l           *slog.Logger
	kafkaReader *kafka.Reader
	consumer    *consumer.Consumer
	e           *echo.Echo
	svc         *service.Service
	chConn      clickhouse.Conn
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

	// Configure options for ClickHouse
	chOptions := clickhouse.Options{
		Addr: []string{cfg.ClickHouse.Addr},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: cfg.ClickHouse.User,
			Password: cfg.ClickHouse.Password,
		},
		Debug: true,
		Debugf: func(format string, v ...interface{}) {
			l.Info(fmt.Sprintf(format, v))
		},
	}

	// Migrate ClickHouse
	if err := db.Migrate(chOptions); err != nil {
		return nil, err
	}

	// Create a ClickHouse connection
	chConn, err := clickhouse.Open(&chOptions)
	if err != nil {
		return nil, err
	}
	a.chConn = chConn

	// Test connection with ClickHouse
	if err := a.chConn.Ping(context.Background()); err != nil {
		return nil, err
	}

	// Init metrics
	m := metrics.New()

	// Create http server to gather metrics from
	a.e = echo.New()
	a.e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				a.l.LogAttrs(context.Background(), slog.LevelInfo, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("ip", v.RemoteIP),
					slog.String("latency", time.Now().Sub(v.StartTime).String()),
				)
			} else {
				a.l.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("ip", v.RemoteIP),
					slog.String("latency", time.Now().Sub(v.StartTime).String()),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))
	a.e.GET("/metrics", echoprometheus.NewHandler())

	// Create a repository
	repo := repository.NewClickHouseRepo(a.chConn)

	a.svc = service.New(a.l, m, repo)
	a.consumer = consumer.New(a.l, a.kafkaReader, a.svc)

	return a, nil
}

func (a *App) Start(ctx context.Context, errChan chan<- error) {
	a.l.Info("starting consumer, batch writers and http server for prometheus")
	go a.consumer.ReadMessages(ctx)
	go a.svc.ShortenedBatchWriter(ctx)
	go a.svc.UnshortenedBatchWriter(ctx)
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

	// Close ClickHouse connection
	if err := a.chConn.Close(); err != nil {
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
