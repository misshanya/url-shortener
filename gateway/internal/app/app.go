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

	// Init gRPC connection to the shortener service
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
		return nil, fmt.Errorf("failed to init gRPC connection to the shortener service: %w", err)
	}
	a.grpcConn = grpcConn

	// Create gRPC client for the shortener service
	grpcClient := pb.NewURLShortenerServiceClient(a.grpcConn)

	// Init service
	svc := service.NewService(grpcClient, a.cfg.Server.PublicHost)

	// Init handler
	shortenerHandler := handler.NewHandler(svc)

	// Init Echo
	a.e = echo.New()

	// CORS
	a.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{cfg.Server.CORSOrigin},
	}))

	// Tracer
	a.e.Use(otelecho.Middleware(serviceName, otelecho.WithTracerProvider(a.tracerProvider)))

	// Logger
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

	// Recoverer
	a.e.Use(middleware.Recover())

	// Connect handlers to the routes
	a.e.POST("/shorten/batch", shortenerHandler.ShortenURLBatch)
	a.e.POST("/shorten", shortenerHandler.ShortenURL)
	a.e.GET("/:code", shortenerHandler.UnshortenURL)

	return a, nil
}

func (a *App) Start(errChan chan<- error) {
	a.l.Info("starting server", slog.String("addr", a.cfg.Server.Addr))
	if err := a.e.Start(a.cfg.Server.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errChan <- err
	}
}

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
