package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/misshanya/url-shortener/gateway/internal/config"
	"github.com/misshanya/url-shortener/gateway/internal/service"
	handler "github.com/misshanya/url-shortener/gateway/internal/transport/http"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.34.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"net/http"
	"time"
)

var (
	serviceName = "gateway"
)

type App struct {
	e              *echo.Echo
	grpcConn       *grpc.ClientConn
	cfg            *config.Config
	l              *slog.Logger
	tracerProvider *trace.TracerProvider
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

	if err := a.initGRPCClient(); err != nil {
		return nil, err
	}
	grpcClient := pb.NewURLShortenerServiceClient(a.grpcConn)

	svc := service.NewService(grpcClient, a.cfg.Server.PublicHost)
	shortenerHandler := handler.NewHandler(svc)

	a.initEcho()

	a.e.POST("/shorten/batch", shortenerHandler.ShortenURLBatch)
	a.e.POST("/shorten", shortenerHandler.ShortenURL)
	a.e.GET("/:code", shortenerHandler.UnshortenURL)

	return a, nil
}

// Start performs a start of all functional services
func (a *App) Start(errChan chan<- error) {
	a.l.Info("starting server", slog.String("addr", a.cfg.Server.Addr))
	if err := a.e.Start(a.cfg.Server.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errChan <- err
	}
}

// Stop performs a graceful shutdown for all components
func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

	a.l.Info("Stopping http server...")
	if err := a.e.Shutdown(ctx); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to stop http server: %w", err))
	}

	a.l.Info("Closing gRPC connection...")
	if err := a.grpcConn.Close(); err != nil {
		stopErr = errors.Join(stopErr, fmt.Errorf("failed to close gRPC connection: %w", err))
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

// initTracing sets up a new OpenTelemetry provider
func (a *App) initTracing() error {
	tracerProvider, err := newTracerProvider(context.Background(), a.cfg.Tracing.CollectorAddr)
	if err != nil {
		return fmt.Errorf("failed to create tracer provider: %w", err)
	}
	a.tracerProvider = tracerProvider
	return nil
}

// initGRPCClient sets up a new gRPC connection to shortener service
func (a *App) initGRPCClient() error {
	grpcConn, err := grpc.NewClient(a.cfg.GRPCClient.ServerAddress,
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		grpc.WithStatsHandler(
			otelgrpc.NewClientHandler(
				otelgrpc.WithTracerProvider(a.tracerProvider),
			),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to init gRPC connection to the shortener service: %w", err)
	}
	a.grpcConn = grpcConn
	return nil
}

// initEcho sets up a new Echo instance with CORS, tracer, logger and recoverer
func (a *App) initEcho() {
	a.e = echo.New()

	a.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{a.cfg.Server.CORSOrigin},
	}))

	a.e.Use(otelecho.Middleware(serviceName, otelecho.WithTracerProvider(a.tracerProvider)))

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

	a.e.Use(middleware.Recover())
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
