package config

import (
	"github.com/ilyakaznacheev/cleanenv"
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
