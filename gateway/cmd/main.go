package main

import (
	"github.com/misshanya/url-shortener/gateway/internal/app"
	"github.com/misshanya/url-shortener/gateway/internal/config"
	"log/slog"
	"os"
)

func main() {
	logger := setupLogger()

	cfg, err := config.NewConfig()
	if err != nil {
		logger.Error("failed to read config", slog.Any("error", err))
		os.Exit(1)
	}

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
