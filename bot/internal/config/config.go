package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
	"os"
)

type Config struct {
	Bot        bot
	GRPCClient gRPCClient
}

type gRPCClient struct {
	ServerAddress string `env:"GRPC_SERVER_ADDR" env-required:"true"`
}

type bot struct {
	Token      string `env:"BOT_TOKEN" env-required:"true"`
	PublicHost string `env:"PUBLIC_HOST" env-default:"http://localhost:8080/"`
}

func NewConfig(logger *slog.Logger) *Config {
	var cfg Config

	// Read .env file
	// If failed to read file, will try ReadEnv
	if err := cleanenv.ReadConfig(".env", &cfg); err == nil {
		return &cfg
	}

	// Read env
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		logger.Error("failed to read env vars", "error", err)
		os.Exit(1)
	}

	return &cfg
}
