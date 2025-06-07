package main

import (
	"github.com/misshanya/url-shortener/shortener/internal/app"
	"github.com/misshanya/url-shortener/shortener/internal/config"
	"log/slog"
	"os"
)

func main() {
	cfg := config.NewConfig()
	logger := setupLogger()
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
