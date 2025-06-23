package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/misshanya/url-shortener/statistics/internal/metrics"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"log/slog"
	"time"
)

type clickHouseRepo interface {
	WriteShortened(ctx context.Context, events []models.ClickHouseEventShortened) error
	WriteUnshortened(ctx context.Context, events []models.ClickHouseEventUnshortened) error
}

type Service struct {
	l *slog.Logger
	m *metrics.Metrics
	r clickHouseRepo

	shortenedCh   chan models.ClickHouseEventShortened
	unshortenedCh chan models.ClickHouseEventUnshortened
}

func New(l *slog.Logger, m *metrics.Metrics, r clickHouseRepo) *Service {
	return &Service{
		l: l,
		m: m,
		r: r,

		shortenedCh:   make(chan models.ClickHouseEventShortened, 10),
		unshortenedCh: make(chan models.ClickHouseEventUnshortened, 10),
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

	// Send to the ClickHouse batch channel
	s.shortenedCh <- models.ClickHouseEventShortened{
		EventID:     uuid.New(),
		OriginalURL: msg.OriginalURL,
		ShortCode:   msg.ShortCode,
		ShortenedAt: msg.ShortenedAt,
	}
}

func (s *Service) Unshortened(msg *models.KafkaMessageUnshortened) {
	s.l.Info("Clicked on shortened URL",
		"url", msg.OriginalURL,
		"code", msg.ShortCode,
		"clicked at", msg.UnshortenedAt,
	)

	// Update metrics
	s.m.Unshorten()

	// Send to the ClickHouse batch channel
	s.unshortenedCh <- models.ClickHouseEventUnshortened{
		EventID:       uuid.New(),
		OriginalURL:   msg.OriginalURL,
		ShortCode:     msg.ShortCode,
		UnshortenedAt: msg.UnshortenedAt,
	}
}

func (s *Service) ShortenedBatchWriter(ctx context.Context) {
	var shortenedEvents []models.ClickHouseEventShortened

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case event := <-s.shortenedCh:
			shortenedEvents = append(shortenedEvents, event)
			if len(shortenedEvents) >= 100 {
				s.l.Info("Writing shortened event, len > 100")
				err := s.r.WriteShortened(ctx, shortenedEvents)
				if err != nil {
					s.l.Error("Failed to write events to DB", "error", err)
				}
				shortenedEvents = nil
			}
		case <-ticker.C:
			if len(shortenedEvents) > 0 {
				s.l.Info("Writing shortened event, ticker")
				err := s.r.WriteShortened(ctx, shortenedEvents)
				if err != nil {
					s.l.Error("Failed to write events to DB", "error", err)
				}
				shortenedEvents = nil
			}
		case <-ctx.Done():
			break
		}
	}
}

func (s *Service) UnshortenedBatchWriter(ctx context.Context) {
	var unshortenedEvents []models.ClickHouseEventUnshortened

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case event := <-s.unshortenedCh:
			unshortenedEvents = append(unshortenedEvents, event)
			if len(unshortenedEvents) >= 100 {
				s.l.Info("Writing unshortened event, len > 100")
				err := s.r.WriteUnshortened(ctx, unshortenedEvents)
				if err != nil {
					s.l.Error("Failed to write events to DB", "error", err)
				}
				unshortenedEvents = nil
			}
		case <-ticker.C:
			if len(unshortenedEvents) > 0 {
				s.l.Info("Writing unshortened event, ticker")
				err := s.r.WriteUnshortened(ctx, unshortenedEvents)
				if err != nil {
					s.l.Error("Failed to write events to DB", "error", err)
				}
				unshortenedEvents = nil
			}
		case <-ctx.Done():
			break
		}
	}
}
