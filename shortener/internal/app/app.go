package app

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/misshanya/url-shortener/shortener/internal/config"
	"github.com/misshanya/url-shortener/shortener/internal/db"
	"github.com/misshanya/url-shortener/shortener/internal/db/sqlc/storage"
	"github.com/misshanya/url-shortener/shortener/internal/repository"
	"github.com/misshanya/url-shortener/shortener/internal/service"
	handler "github.com/misshanya/url-shortener/shortener/internal/transport/grpc"
	"google.golang.org/grpc"
	"log"
	"log/slog"
	"net"
	"os"
)

func Start(cfg *config.Config) {
	ctx := context.Background()

	// Add a listener address
	lis, err := net.Listen("tcp", cfg.Server.Addr)
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	// Init db connection
	conn, err := initDB(ctx, cfg.Postgres.URL)
	if err != nil {
		slog.Error("failed to connect to database")
		os.Exit(1)
	}

	// Migrate db
	if err := db.Migrate(sql.OpenDB(stdlib.GetConnector(*conn.Config().ConnConfig))); err != nil {
		slog.Error("failed to migrate database", slog.Any("err", err))
		os.Exit(1)
	}

	// Init SQL queries
	queries := storage.New(conn)

	// Start the grpc server
	grpcServer := grpc.NewServer()

	repo := repository.NewPostgresRepo(queries)
	svc := service.New(repo)

	handler.NewHandler(grpcServer, svc)

	slog.Info("starting server")
	log.Fatal(grpcServer.Serve(lis))
}

func initDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}

	pool.Config().MaxConns = 100 // Max 100 connections

	return pool, nil
}
