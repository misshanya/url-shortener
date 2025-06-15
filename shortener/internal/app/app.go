package app

import (
	"context"
	"database/sql"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/misshanya/url-shortener/shortener/internal/config"
	"github.com/misshanya/url-shortener/shortener/internal/db"
	"github.com/misshanya/url-shortener/shortener/internal/db/sqlc/storage"
	"github.com/misshanya/url-shortener/shortener/internal/repository"
	"github.com/misshanya/url-shortener/shortener/internal/service"
	handler "github.com/misshanya/url-shortener/shortener/internal/transport/grpc"
	"google.golang.org/grpc"
	"log/slog"
	"net"
)

type App struct {
	cfg     *config.Config
	l       *slog.Logger
	lis     *net.Listener
	dbPool  *pgxpool.Pool
	grpcSrv *grpc.Server
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

	// Add a listener address
	lis, err := net.Listen("tcp", cfg.Server.Addr)
	if err != nil {
		return nil, err
	}
	a.lis = &lis

	// Init db connection
	a.dbPool, err = initDB(ctx, cfg.Postgres.URL)
	if err != nil {
		return nil, err
	}

	// Migrate db
	if err := db.Migrate(sql.OpenDB(stdlib.GetConnector(*a.dbPool.Config().ConnConfig))); err != nil {
		return nil, err
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
	)

	repo := repository.NewPostgresRepo(queries)
	svc := service.New(repo, a.l)

	handler.NewHandler(a.grpcSrv, svc)

	return a, nil
}

func (a *App) Start(errChan chan<- error) {
	a.l.Info("starting server", slog.String("addr", a.cfg.Server.Addr))
	if err := a.grpcSrv.Serve(*a.lis); err != nil {
		errChan <- err
	}
}

func (a *App) Stop() {
	a.l.Info("[!] Shutting down...")

	// Stop server
	a.l.Info("Stopping gRPC server...")
	a.grpcSrv.GracefulStop()

	// Close DB pool
	a.l.Info("Closing database pool...")
	a.dbPool.Close()

	a.l.Info("Stopped gracefully")
}

func initDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}

	pool.Config().MaxConns = 100 // Max 100 connections

	return pool, nil
}
