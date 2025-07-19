package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/misshanya/url-shortener/shortener/internal/config"
	"github.com/misshanya/url-shortener/shortener/internal/consumer"
	"github.com/misshanya/url-shortener/shortener/internal/db"
	"github.com/misshanya/url-shortener/shortener/internal/db/sqlc/storage"
	"github.com/misshanya/url-shortener/shortener/internal/repository"
	"github.com/misshanya/url-shortener/shortener/internal/service"
	handler "github.com/misshanya/url-shortener/shortener/internal/transport/grpc"
	"github.com/segmentio/kafka-go"
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.34.0"
	"google.golang.org/grpc"
	"log/slog"
	"net"
)

var (
	serviceName = "shortener"
)

type App struct {
	cfg            *config.Config
	l              *slog.Logger
	lis            *net.Listener
	dbPool         *pgxpool.Pool
	grpcSrv        *grpc.Server
	kafkaWriter    *kafka.Writer
	kafkaReader    *kafka.Reader
	valkeyClient   valkey.Client
	consumer       *consumer.Consumer
	tracerProvider *trace.TracerProvider
}

// InterceptorLogger adapts slog logger to interceptor logger.
func InterceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func New(ctx context.Context, cfg *config.Config, l *slog.Logger) (*App, error) {
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

	// Add a listener address
	lis, err := net.Listen("tcp", cfg.Server.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}
	a.lis = &lis

	// Init db connection
	a.dbPool, err = initDB(ctx, cfg.Postgres.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to init db connection: %w", err)
	}

	// Init Valkey connection
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.Valkey.Addr},
		Password:    cfg.Valkey.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init Valkey connection: %w", err)
	}
	a.valkeyClient = client

	// Migrate db
	if err := db.Migrate(sql.OpenDB(stdlib.GetConnector(*a.dbPool.Config().ConnConfig))); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Init SQL queries
	queries := storage.New(a.dbPool)

	// Configure interceptor logger
	opts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}

	// Create a gRPC server
	a.grpcSrv = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(InterceptorLogger(a.l), opts...),
		),
		grpc.StatsHandler(
			otelgrpc.NewServerHandler(
				otelgrpc.WithTracerProvider(a.tracerProvider),
			),
		),
	)

	// Test connection with Kafka
	testKafkaConn, err := kafka.Dial("tcp", cfg.Kafka.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	testKafkaConn.Close()

	// Create a Kafka reader
	a.kafkaReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{cfg.Kafka.Addr},
		GroupID:     "shortener-group",
		GroupTopics: []string{"shortener.top_unshortened"},
	})

	// Create a Kafka writer
	a.kafkaWriter = &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Addr),
		AllowAutoTopicCreation: true,
	}

	repo := repository.NewPostgresRepo(queries)
	valkeyRepo := repository.NewValkeyRepo(a.valkeyClient)
	svc := service.New(repo, valkeyRepo, a.l, a.kafkaWriter, tracer, cfg.MaxBatchWorkers)

	a.consumer = consumer.New(a.l, a.kafkaReader, svc)

	handler.NewHandler(a.grpcSrv, svc)

	return a, nil
}

func (a *App) Start(ctx context.Context, errChan chan<- error) {
	a.l.Info("starting server", slog.String("addr", a.cfg.Server.Addr))
	go a.consumer.ReadMessages(ctx)
	if err := a.grpcSrv.Serve(*a.lis); err != nil {
		errChan <- err
	}
}

func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

	// Stop server
	a.l.Info("Stopping gRPC server...")
	a.grpcSrv.GracefulStop()

	// Close DB pool
	a.l.Info("Closing database pool...")
	a.dbPool.Close()

	// Close Valkey connection
	a.l.Info("Closing Valkey connection...")
	a.valkeyClient.Close()

	// Close Kafka connection
	if err := a.kafkaWriter.Close(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to close Kafka connection: %w", err))
	}

	// Shut down tracer provider
	a.l.Info("Shutting down tracer provider...")
	if err := a.tracerProvider.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to shutdown tracer provider: %w", err))
	}

	if stopErr != nil {
		return stopErr
	}

	a.l.Info("Stopped gracefully")
	return nil
}

func initDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}

	pool.Config().MaxConns = 100 // Max 100 connections

	return pool, nil
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
