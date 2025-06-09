package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
	"os"
)

type Config struct {
	Server     server
	GRPCClient gRPCClient
}

type server struct {
	Addr       string `env:"SERVER_ADDR" env-default:":8080"`
	PublicHost string `env:"PUBLIC_HOST" env-default:"localhost:8080"`
}

type gRPCClient struct {
	ServerAddress string `env:"GRPC_SERVER_ADDR" env-required:"true"`
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
