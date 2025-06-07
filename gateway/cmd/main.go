package main

import (
	"github.com/misshanya/url-shortener/gateway/internal/app"
	"github.com/misshanya/url-shortener/gateway/internal/config"
)

func main() {
	cfg := config.NewConfig()
	app.Start(cfg)
}
