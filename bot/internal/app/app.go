package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/misshanya/url-shortener/bot/internal/config"
	"github.com/misshanya/url-shortener/bot/internal/handler"
	"github.com/misshanya/url-shortener/bot/internal/service"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
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
)

var (
	serviceName = "bot"
)

type App struct {
	cfg            *config.Config
	l              *slog.Logger
	b              *bot.Bot
	grpcConn       *grpc.ClientConn
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
	grpcClient := pb.NewURLShortenerServiceClient(grpcConn)

	// Init service
	svc := service.New(grpcClient, cfg.Bot.PublicHost, a.l)

	// Init handler
	botHandler := handler.New(a.l, svc, tracer)

	// Configure bot
	opts := []bot.Option{
		bot.WithDefaultHandler(botHandler.Default),
	}

	b, err := bot.New(cfg.Bot.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot instance: %w", err)
	}
	a.b = b

	return a, nil
}

func (a *App) Start(ctx context.Context) {
	a.l.Info("Starting bot")
	a.b.Start(ctx)
}

func (a *App) Stop(ctx context.Context) error {
	a.l.Info("[!] Shutting down...")

	var stopErr error

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
