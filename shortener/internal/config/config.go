package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Server   server
	Postgres postgres
	Kafka    kafka
}

type server struct {
	Addr string `env:"SERVER_ADDR" env-default:":8080"`
}

type postgres struct {
	URL string `env:"POSTGRES_URL" env-required:"true"`
}

type kafka struct {
	Addr string `env:"KAFKA_ADDR" env-required:"true"`
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
