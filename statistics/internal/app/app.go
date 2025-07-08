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
	"github.com/misshanya/url-shortener/statistics/internal/producer"
	"github.com/misshanya/url-shortener/statistics/internal/repository"
	"github.com/misshanya/url-shortener/statistics/internal/service"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.34.0"

	"log/slog"
	"net/http"
	"time"
)

var (
	serviceName = "statistics"
)

type App struct {
	cfg            *config.Config
	l              *slog.Logger
	kafkaReader    *kafka.Reader
	kafkaWriter    *kafka.Writer
	consumer       *consumer.Consumer
	producer       *producer.Producer
	e              *echo.Echo
	svc            *service.Service
	chConn         clickhouse.Conn
	tracerProvider *trace.TracerProvider
}

func New(cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	// Init tracing
	tracerProvider, err := newTracerProvider(context.Background(), cfg.Tracing.CollectorAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer provider: %w", err)
	}
	a.tracerProvider = tracerProvider
	tracer := a.tracerProvider.Tracer(serviceName)

	// Test connection with Kafka
	testKafkaConn, err := kafka.Dial("tcp", cfg.Kafka.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	testKafkaConn.Close()

	// Create a Kafka reader
	a.kafkaReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{cfg.Kafka.Addr},
		GroupID:     "statistics-group",
		GroupTopics: []string{"shortener.shortened", "shortener.unshortened"},
	})

	// Create a Kafka writer
	a.kafkaWriter = &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Addr),
		AllowAutoTopicCreation: true,
	}

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

	a.svc = service.New(a.l, m, repo, tracer)
	a.consumer = consumer.New(a.l, a.kafkaReader, a.svc, tracer)
	a.producer = producer.New(a.l, a.svc, a.kafkaWriter, cfg.TopTTL, cfg.TopAmount)

	return a, nil
}

func (a *App) Start(ctx context.Context, errChan chan<- error) {
	a.l.Info("starting consumer, batch writers, producer and http server for prometheus")
	go a.consumer.ReadMessages(ctx)
	go a.svc.ShortenedBatchWriter(ctx)
	go a.svc.UnshortenedBatchWriter(ctx)
	go a.producer.ProduceTop(ctx)
	if err := a.e.Start(a.cfg.HttpSrv.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errChan <- err
	}
}

func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

	// Close Kafka reader connection
	if err := a.kafkaReader.Close(); err != nil {
		stopErr = errors.Join(stopErr, err)
	}

	// Close Kafka writer connection
	if err := a.kafkaWriter.Close(); err != nil {
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

	// Shut down tracer provider
	a.l.Info("Shutting down tracer provider...")
	if err := a.tracerProvider.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, err)
	}

	if stopErr != nil {
		return stopErr
	}

	a.l.Info("Stopped gracefully")
	return nil
}

func newTracerProvider(ctx context.Context, collectorAddr string) (*trace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(collectorAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to create an exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a resource: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tracerProvider, err
}
