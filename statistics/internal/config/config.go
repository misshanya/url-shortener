package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Kafka      kafka
	HttpSrv    httpServer
	ClickHouse clickHouse
	Valkey     valkey
	Tracing    tracing
	Scheduler  scheduler

	TopTTL    int `env:"TOP_TTL" env-default:"3600"`
	TopAmount int `env:"TOP_AMOUNT" env-default:"100"`
	LockTTL   int `env:"PRODUCER_LOCK_INTERVAL_SECONDS" env-default:"30"`
}

type kafka struct {
	Addr string `env:"KAFKA_ADDR" env-required:"true"`
}

type httpServer struct {
	Addr string `env:"SERVER_ADDR" env-required:"true"`
}

type clickHouse struct {
	Addr      string `env:"CLICKHOUSE_ADDR" env-required:"true"`
	User      string `env:"CLICKHOUSE_USER" env-required:"true"`
	Password  string `env:"CLICKHOUSE_PASSWORD" env-required:"true"`
	BatchSize int    `env:"CLICKHOUSE_BATCH_SIZE" env-required:"true"`
}

type valkey struct {
	Addr     string `env:"VALKEY_ADDR" env-required:"true"`
	Password string `env:"VALKEY_PASSWORD" env-required:"true"`
}

type tracing struct {
	CollectorAddr string `env:"TRACING_COLLECTOR_ADDR" env-required:"true"`
}

type scheduler struct {
	Crontab string `env:"PRODUCER_SCHEDULER_CRONTAB" env-default:"0 * * * *"`
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
