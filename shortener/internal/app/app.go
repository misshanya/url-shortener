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

// New creates and initializes a new instance of App
func New(ctx context.Context, cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	if err := a.initTracing(); err != nil {
		return nil, err
	}
	tracer := a.tracerProvider.Tracer(serviceName)

	if err := a.initListener(); err != nil {
		return nil, err
	}

	if err := a.initDB(ctx); err != nil {
		return nil, err
	}

	if err := a.initValkey(); err != nil {
		return nil, err
	}

	if err := a.migrateDB(); err != nil {
		return nil, err
	}

	queries := storage.New(a.dbPool)

	a.initGRPCServer()

	if err := a.initKafka(); err != nil {
		return nil, err
	}

	repo := repository.NewPostgresRepo(queries)
	valkeyRepo := repository.NewValkeyRepo(a.valkeyClient)
	svc := service.New(repo, valkeyRepo, a.l, a.kafkaWriter, tracer, cfg.MaxBatchWorkers)

	a.consumer = consumer.New(a.l, a.kafkaReader, svc)

	handler.NewHandler(a.grpcSrv, svc)

	return a, nil
}

// Start performs a start of all functional services
func (a *App) Start(ctx context.Context, errChan chan<- error) {
	a.l.Info("starting server", slog.String("addr", a.cfg.Server.Addr))
	go a.consumer.ReadMessages(ctx)
	if err := a.grpcSrv.Serve(*a.lis); err != nil {
		errChan <- err
	}
}

// Stop performs a graceful shutdown for all components
func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

	a.l.Info("Stopping gRPC server...")
	a.grpcSrv.GracefulStop()

	a.l.Info("Closing database pool...")
	a.dbPool.Close()

	a.l.Info("Closing Valkey connection...")
	a.valkeyClient.Close()

	if err := a.kafkaWriter.Close(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to close Kafka connection: %w", err))
	}

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

// initDB initializes a new pool for PostgreSQL db
func initDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}

	pool.Config().MaxConns = 100

	return pool, nil
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

// initListener sets up a tcp listener ready for gRPC
func (a *App) initListener() error {
	lis, err := net.Listen("tcp", a.cfg.Server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	a.lis = &lis
	return nil
}

// initDB sets up PostgreSQL db
func (a *App) initDB(ctx context.Context) error {
	dbPool, err := initDB(ctx, a.cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("failed to init db connection: %w", err)
	}
	a.dbPool = dbPool
	return nil
}

// migrateDB performs a migration to ensure the schema is up to date
func (a *App) migrateDB() error {
	return db.Migrate(sql.OpenDB(stdlib.GetConnector(*a.dbPool.Config().ConnConfig)))
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

// initGRPCServer sets up a gRPC server with interceptor logger
func (a *App) initGRPCServer() {
	opts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}

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
		GroupID:     "shortener-group",
		GroupTopics: []string{"shortener.top_unshortened"},
	})

	a.kafkaWriter = &kafka.Writer{
		Addr:                   kafka.TCP(a.cfg.Kafka.Addr),
		AllowAutoTopicCreation: true,
	}

	return nil
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
