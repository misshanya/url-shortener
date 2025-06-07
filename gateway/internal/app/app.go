package app

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/misshanya/url-shortener/gateway/internal/config"
	"github.com/misshanya/url-shortener/gateway/internal/service"
	handler "github.com/misshanya/url-shortener/gateway/internal/transport/http"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"os"
)

func Start(cfg *config.Config) {
	// Init gRPC connection to the shortener service
	grpcConn, err := grpc.NewClient(cfg.GRPCClient.ServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to grpc server", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create gRPC client for the shortener service
	grpcClient := pb.NewURLShortenerServiceClient(grpcConn)

	// Init service
	svc := service.NewService(grpcClient, cfg.Server.PublicHost)

	// Init handler
	shortenerHandler := handler.NewHandler(svc)

	// Init Echo
	e := echo.New()

	// CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"}, // Change in production !!!
	}))

	// Logger
	e.Use(middleware.Logger())

	// Recoverer
	e.Use(middleware.Recover())

	// Connect handlers to the routes
	e.POST("/shorten", shortenerHandler.ShortURL)
	e.GET("/:hash", shortenerHandler.UnshortURL)

	slog.Info("starting server", slog.String("addr", cfg.Server.Addr))

	// Start the server
	e.Logger.Fatal(e.Start(cfg.Server.Addr))
}
