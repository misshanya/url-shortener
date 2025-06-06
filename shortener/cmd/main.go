package main

import (
	"github.com/misshanya/url-shortener/shortener/internal/app"
	"github.com/misshanya/url-shortener/shortener/internal/config"
)

func main() {
	cfg := config.NewConfig()
	app.Start(cfg)
}
