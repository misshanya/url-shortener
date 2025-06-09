package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
	"os"
)

type Config struct {
	Server   server
	Postgres postgres
}

type server struct {
	Addr string `env:"SERVER_ADDR" env-default:":8080"`
}

type postgres struct {
	URL string `env:"POSTGRES_URL" env-required:"true"`
}

func NewConfig() *Config {
	var cfg Config

	// Read .env file
	// If failed to read file, will try ReadEnv
	if err := cleanenv.ReadConfig(".env", &cfg); err == nil {
		return &cfg
	}

	// Read env
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("failed to read env vars", "error", err)
		os.Exit(1)
	}

	return &cfg
}
