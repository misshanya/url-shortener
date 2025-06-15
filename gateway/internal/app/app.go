package app

import (
	"context"
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/misshanya/url-shortener/gateway/internal/config"
	"github.com/misshanya/url-shortener/gateway/internal/service"
	handler "github.com/misshanya/url-shortener/gateway/internal/transport/http"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"net/http"
	"time"
)

type App struct {
	e        *echo.Echo
	grpcConn *grpc.ClientConn
	cfg      *config.Config
	l        *slog.Logger
}

func New(cfg *config.Config, l *slog.Logger) (*App, error) {
	a := &App{
		cfg: cfg,
		l:   l,
	}

	// Init gRPC connection to the shortener service
	grpcConn, err := grpc.NewClient(a.cfg.GRPCClient.ServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
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
		AllowOrigins: []string{"*"}, // Change in production !!!
	}))

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
	a.e.POST("/shorten", shortenerHandler.ShortenURL)
	a.e.GET("/:hash", shortenerHandler.UnshortenURL)

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
		stopErr = errors.Join(stopErr, err)
	}

	a.l.Info("Closing gRPC connection...")
	if err := a.grpcConn.Close(); err != nil {
		stopErr = errors.Join(stopErr, err)
	}

	if stopErr != nil {
		return stopErr
	}

	a.l.Info("Stopped gracefully")
	return nil
}
