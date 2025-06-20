package service

import (
	"github.com/misshanya/url-shortener/statistics/internal/metrics"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"log/slog"
)

type Service struct {
	l *slog.Logger
	m *metrics.Metrics
}

func New(l *slog.Logger, m *metrics.Metrics) *Service {
	return &Service{
		l: l,
		m: m,
	}
}

func (s *Service) Shortened(msg *models.KafkaMessageShortened) {
	s.l.Info("Shortened URL",
		"url", msg.OriginalURL,
		"code", msg.ShortCode,
		"shortened at", msg.ShortenedAt,
	)

	// Update metrics
	s.m.Shorten()
}

func (s *Service) Unshortened(msg *models.KafkaMessageUnshortened) {
	s.l.Info("Clicked on shortened URL",
		"url", msg.OriginalURL,
		"code", msg.ShortCode,
		"clicked at", msg.UnshortenedAt,
	)

	// Update metrics
	s.m.Unshorten()
}
