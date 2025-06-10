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
	"os"
	"os/signal"
)

func Start(cfg *config.Config, logger *slog.Logger) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Init gRPC connection to the shortener service
	grpcConn, err := grpc.NewClient(cfg.GRPCClient.ServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to grpc server", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create gRPC client for the shortener service
	grpcClient := pb.NewURLShortenerServiceClient(grpcConn)

	// Init service
	svc := service.New(grpcClient, cfg.Bot.PublicHost, logger)

	// Init handler
	botHandler := handler.New(logger, svc)

	// Configure bot
	opts := []bot.Option{
		bot.WithDefaultHandler(botHandler.Default),
	}

	b, err := bot.New(cfg.Bot.Token, opts...)
	if err != nil {
		panic(err)
	}

	// Start bot
	logger.Info("starting bot")
	b.Start(ctx)
}
