package app

import (
	"context"
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
	"time"
)

func Start(cfg *config.Config, logger *slog.Logger) {
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
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				logger.LogAttrs(context.Background(), slog.LevelInfo, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("latency", time.Now().Sub(v.StartTime).String()),
				)
			} else {
				logger.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("latency", time.Now().Sub(v.StartTime).String()),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))

	// Recoverer
	e.Use(middleware.Recover())

	// Connect handlers to the routes
	e.POST("/shorten", shortenerHandler.ShortURL)
	e.GET("/:hash", shortenerHandler.UnshortURL)

	logger.Info("starting server", slog.String("addr", cfg.Server.Addr))

	// Start the server
	e.Logger.Fatal(e.Start(cfg.Server.Addr))
}
