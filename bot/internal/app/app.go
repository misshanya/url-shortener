package app

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/misshanya/url-shortener/bot/internal/config"
	"github.com/misshanya/url-shortener/bot/internal/handler"
	"github.com/misshanya/url-shortener/bot/internal/service"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
)

type App struct {
	cfg      *config.Config
	l        *slog.Logger
	b        *bot.Bot
	grpcConn *grpc.ClientConn
}

func New(cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	// Init gRPC connection to the shortener service
	grpcConn, err := grpc.NewClient(cfg.GRPCClient.ServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	a.grpcConn = grpcConn

	// Create gRPC client for the shortener service
	grpcClient := pb.NewURLShortenerServiceClient(grpcConn)

	// Init service
	svc := service.New(grpcClient, cfg.Bot.PublicHost, a.l)

	// Init handler
	botHandler := handler.New(a.l, svc)

	// Configure bot
	opts := []bot.Option{
		bot.WithDefaultHandler(botHandler.Default),
	}

	b, err := bot.New(cfg.Bot.Token, opts...)
	if err != nil {
		return nil, err
	}
	a.b = b

	return a, nil
}

func (a *App) Start(ctx context.Context) {
	a.l.Info("Starting bot")
	a.b.Start(ctx)
}

func (a *App) Stop() error {
	a.l.Info("[!] Shutting down...")

	a.l.Info("Closing gRPC connection...")
	if err := a.grpcConn.Close(); err != nil {
		return err
	}

	a.l.Info("Stopped gracefully")
	return nil
}
