package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Bot        bot
	GRPCClient gRPCClient
	Tracing    tracing
}

type gRPCClient struct {
	ServerAddress string `env:"GRPC_SERVER_ADDR" env-required:"true"`
}

type bot struct {
	Token      string `env:"BOT_TOKEN" env-required:"true"`
	PublicHost string `env:"PUBLIC_HOST" env-default:"http://localhost:8080/"`
}

type tracing struct {
	CollectorAddr string `env:"TRACING_COLLECTOR_ADDR" env-required:"true"`
}

func NewConfig() (*Config, error) {
	var cfg Config

	// Read .env file
	// If failed to read file, will try ReadEnv
	if err := cleanenv.ReadConfig(".env", &cfg); err == nil {
		return &cfg, nil
	}

	// Read env
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
