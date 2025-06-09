package main

import (
	"github.com/misshanya/url-shortener/gateway/internal/app"
	"github.com/misshanya/url-shortener/gateway/internal/config"
	"log/slog"
	"os"
)

func main() {
	logger := setupLogger()
	cfg := config.NewConfig(logger)
	app.Start(cfg, logger)
}

func setupLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})

	logger := slog.New(handler)
	return logger
}
