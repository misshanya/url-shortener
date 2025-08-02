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
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/misshanya/url-shortener/statistics/internal/producer"
	"github.com/misshanya/url-shortener/statistics/internal/repository"
	"github.com/misshanya/url-shortener/statistics/internal/service"
	"github.com/misshanya/url-shortener/statistics/pkg/scheduler"
	"github.com/segmentio/kafka-go"
	"github.com/valkey-io/valkey-go"
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
	cfg                          *config.Config
	l                            *slog.Logger
	kafkaReader                  *kafka.Reader
	kafkaWriter                  *kafka.Writer
	consumer                     *consumer.Consumer
	producer                     *producer.Producer
	e                            *echo.Echo
	svc                          *service.Service
	chConn                       clickhouse.Conn
	valkeyClient                 valkey.Client
	tracerProvider               *trace.TracerProvider
	batchWriterShortenedTicker   *time.Ticker
	batchWriterUnshortenedTicker *time.Ticker
	producerScheduler            *scheduler.Scheduler
}

// New creates and initializes a new instance of App
func New(cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	if err := a.initTracing(); err != nil {
		return nil, err
	}
	tracer := a.tracerProvider.Tracer(serviceName)

	if err := a.initKafka(); err != nil {
		return nil, err
	}

	if err := a.initClickHouse(); err != nil {
		return nil, err
	}

	if err := a.initValkey(); err != nil {
		return nil, err
	}

	if err := a.initScheduler(); err != nil {
		return nil, err
	}

	m := metrics.New()

	a.initEcho()

	a.e.GET("/metrics", echoprometheus.NewHandler())

	repo := repository.NewClickHouseRepo(a.chConn)
	valkeyRepo := repository.NewValkeyRepo(a.valkeyClient)
	a.svc = service.New(
		a.l,
		make(chan models.ClickHouseEventShortened, 10),
		make(chan models.ClickHouseEventUnshortened, 10),
		m,
		repo,
		valkeyRepo,
		tracer,
		cfg.ClickHouse.BatchSize,
	)
	a.consumer = consumer.New(a.l, a.kafkaReader, a.svc, tracer)
	a.producer = producer.New(a.l, a.svc, a.kafkaWriter, cfg.TopTTL, cfg.LockTTL, cfg.TopAmount, tracer)

	return a, nil
}

// Start performs a start of all functional services
func (a *App) Start(ctx context.Context, errChan chan<- error) {
	a.l.Info("Starting...")
	go a.consumer.ReadMessages(ctx)

	a.batchWriterShortenedTicker = time.NewTicker(10 * time.Second)
	a.batchWriterUnshortenedTicker = time.NewTicker(10 * time.Second)
	a.producerScheduler.Start()

	go a.svc.ShortenedBatchWriter(ctx, a.batchWriterShortenedTicker.C)
	go a.svc.UnshortenedBatchWriter(ctx, a.batchWriterUnshortenedTicker.C)
	go a.producer.ProduceTop(ctx, a.producerScheduler.Tick)
	if err := a.e.Start(a.cfg.HttpSrv.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errChan <- err
	}
}

// Stop performs a graceful shutdown for all components
func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

	if err := a.kafkaReader.Close(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to close Kafka reader connection: %w", err))
	}

	if err := a.kafkaWriter.Close(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to close Kafka writer connection: %w", err))
	}

	if err := a.chConn.Close(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to close ClickHouse connection: %w", err))
	}

	a.valkeyClient.Close()

	a.l.Info("Stopping http server...")
	if err := a.e.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown http server: %w", err))
	}

	a.l.Info("Shutting down tracer provider...")
	if err := a.tracerProvider.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown tracer provider: %w", err))
	}

	a.l.Info("Stopping batch writer tickers")
	if a.batchWriterShortenedTicker != nil {
		a.batchWriterShortenedTicker.Stop()
	}
	if a.batchWriterUnshortenedTicker != nil {
		a.batchWriterUnshortenedTicker.Stop()
	}

	a.l.Info("Stopping producer scheduler")
	if err := a.producerScheduler.Shutdown(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown producer scheduler: %w", err))
	}

	if stopErr != nil {
		return stopErr
	}

	a.l.Info("Stopped gracefully")
	return nil
}

// initTracing sets up a new OpenTelemetry provider
func (a *App) initTracing() error {
	tracerProvider, err := newTracerProvider(context.Background(), a.cfg.Tracing.CollectorAddr)
	if err != nil {
		return fmt.Errorf("failed to create tracer provider: %w", err)
	}
	a.tracerProvider = tracerProvider
	return nil
}

// initKafka sets up both Kafka reader and writer
// It pings a Kafka server before initialize
func (a *App) initKafka() error {
	// Test a connection before creating reader and writer
	testKafkaConn, err := kafka.Dial("tcp", a.cfg.Kafka.Addr)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	testKafkaConn.Close()

	a.kafkaReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{a.cfg.Kafka.Addr},
		GroupID:     "statistics-group",
		GroupTopics: []string{"shortener.shortened", "shortener.unshortened"},
	})

	a.kafkaWriter = &kafka.Writer{
		Addr:                   kafka.TCP(a.cfg.Kafka.Addr),
		AllowAutoTopicCreation: true,
	}

	return nil
}

// initClickHouse sets up db connection and performs a migration to ensure the schema is up to date
func (a *App) initClickHouse() error {
	options := clickhouse.Options{
		Addr: []string{a.cfg.ClickHouse.Addr},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: a.cfg.ClickHouse.User,
			Password: a.cfg.ClickHouse.Password,
		},
		Debug: true,
		Debugf: func(format string, v ...interface{}) {
			a.l.Info(fmt.Sprintf(format, v))
		},
	}

	if err := db.Migrate(options); err != nil {
		return fmt.Errorf("failed to migrate ClickHouse: %w", err)
	}

	chConn, err := clickhouse.Open(&options)
	if err != nil {
		return fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}
	a.chConn = chConn

	return nil
}

// initValkey sets up a connection to cache
func (a *App) initValkey() error {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{a.cfg.Valkey.Addr},
		Password:    a.cfg.Valkey.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to init Valkey connection: %w", err)
	}
	a.valkeyClient = client
	return nil
}

// initScheduler sets up a cron for producer
func (a *App) initScheduler() error {
	producerScheduler, err := scheduler.NewScheduler(a.cfg.Scheduler.Crontab)
	if err != nil {
		return fmt.Errorf("failed to create a scheduler for producer: %w", err)
	}
	a.producerScheduler = producerScheduler
	return nil
}

// initEcho sets up a new Echo instance with logger
func (a *App) initEcho() {
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
}

// newTracerProvider creates a new OpenTelemetry provider
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
