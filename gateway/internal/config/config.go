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

func NewConfig() *Config {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	return &cfg
}
