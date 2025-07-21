package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"time"
)

type clickHouseRepo interface {
	WriteShortened(ctx context.Context, events []models.ClickHouseEventShortened) error
	WriteUnshortened(ctx context.Context, events []models.ClickHouseEventUnshortened) error
	GetTopUnshortened(ctx context.Context, amount, ttl int) (*models.UnshortenedTop, error)
}

type metricsProvider interface {
	Shorten()
	Unshorten()
}

type Service struct {
	l *slog.Logger
	m metricsProvider
	r clickHouseRepo

	shortenedCh   chan models.ClickHouseEventShortened
	unshortenedCh chan models.ClickHouseEventUnshortened
	DBBatchSize   int

	t trace.Tracer
}

func New(l *slog.Logger, m metricsProvider, r clickHouseRepo, t trace.Tracer, DBBatchSize int) *Service {
	return &Service{
		l: l,
		m: m,
		r: r,

		shortenedCh:   make(chan models.ClickHouseEventShortened, 10),
		unshortenedCh: make(chan models.ClickHouseEventUnshortened, 10),
		DBBatchSize:   DBBatchSize,

		t: t,
	}
}

func (s *Service) Shortened(ctx context.Context, msg *models.KafkaMessageShortened) {
	ctx, span := s.t.Start(ctx, "Shortened event in service")
	defer span.End()

	s.l.Info("Shortened URL",
		"url", msg.OriginalURL,
		"code", msg.ShortCode,
		"shortened at", msg.ShortenedAt,
	)

	// Update metrics
	_, spanMetrics := s.t.Start(ctx, "Update metrics")
	s.m.Shorten()
	spanMetrics.End()

	// Send to the ClickHouse batch channel

	carrier := propagation.MapCarrier{}
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, carrier)

	_, spanClickHouse := s.t.Start(ctx, "Send to the ClickHouse channel")
	s.shortenedCh <- models.ClickHouseEventShortened{
		Carrier:     carrier,
		EventID:     uuid.New(),
		OriginalURL: msg.OriginalURL,
		ShortCode:   msg.ShortCode,
		ShortenedAt: msg.ShortenedAt,
	}
	spanClickHouse.End()
}

func (s *Service) Unshortened(ctx context.Context, msg *models.KafkaMessageUnshortened) {
	ctx, span := s.t.Start(ctx, "Unshortened event in service")
	defer span.End()

	s.l.Info("Clicked on shortened URL",
		"url", msg.OriginalURL,
		"code", msg.ShortCode,
		"clicked at", msg.UnshortenedAt,
	)

	// Update metrics
	_, spanMetrics := s.t.Start(ctx, "Update metrics")
	s.m.Unshorten()
	spanMetrics.End()

	// Send to the ClickHouse batch channel

	carrier := propagation.MapCarrier{}
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, carrier)

	_, spanClickHouse := s.t.Start(ctx, "Send to the ClickHouse channel")
	s.unshortenedCh <- models.ClickHouseEventUnshortened{
		Carrier:       carrier,
		EventID:       uuid.New(),
		OriginalURL:   msg.OriginalURL,
		ShortCode:     msg.ShortCode,
		UnshortenedAt: msg.UnshortenedAt,
	}
	spanClickHouse.End()
}

func (s *Service) ShortenedBatchWriter(ctx context.Context) {
	var shortenedEvents []models.ClickHouseEventShortened

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case event := <-s.shortenedCh:
			propagator := propagation.TraceContext{}
			ctxEvent := propagator.Extract(context.Background(), event.Carrier)

			_, spanEvent := s.t.Start(ctxEvent, "Append event to the slice")
			shortenedEvents = append(shortenedEvents, event)
			spanEvent.End()

			if len(shortenedEvents) >= s.DBBatchSize {
				s.l.Info("Writing shortened event, len > batch size", "batch_size", s.DBBatchSize)
				ctxWrite, spanWrite := s.t.Start(ctx, "Write shortened events to the database, len > 100")
				err := s.r.WriteShortened(ctxWrite, shortenedEvents)
				spanWrite.End()
				if err != nil {
					s.l.Error("Failed to write events to DB", "error", err)
				}
				shortenedEvents = nil
			}
		case <-ticker.C:
			if len(shortenedEvents) > 0 {
				s.l.Info("Writing shortened event, ticker")
				ctxWrite, spanWrite := s.t.Start(ctx, "Write shortened events to the database, ticker")
				err := s.r.WriteShortened(ctxWrite, shortenedEvents)
				spanWrite.End()
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
			propagator := propagation.TraceContext{}
			ctxEvent := propagator.Extract(context.Background(), event.Carrier)

			_, spanEvent := s.t.Start(ctxEvent, "Append event to the slice")
			unshortenedEvents = append(unshortenedEvents, event)
			spanEvent.End()

			if len(unshortenedEvents) >= s.DBBatchSize {
				s.l.Info("Writing unshortened event, len > batch size", "batch_size", s.DBBatchSize)
				ctxWrite, spanWrite := s.t.Start(ctx, "Write shortened events to the database, len > 100")
				err := s.r.WriteUnshortened(ctxWrite, unshortenedEvents)
				spanWrite.End()
				if err != nil {
					s.l.Error("Failed to write events to DB", "error", err)
				}
				unshortenedEvents = nil
			}
		case <-ticker.C:
			if len(unshortenedEvents) > 0 {
				s.l.Info("Writing unshortened event, ticker")
				ctxWrite, spanWrite := s.t.Start(ctx, "Write shortened events to the database, ticker")
				err := s.r.WriteUnshortened(ctxWrite, unshortenedEvents)
				spanWrite.End()
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

func (s *Service) GetTopUnshortened(ctx context.Context, amount, ttl int) (*models.UnshortenedTop, error) {
	ctx, span := s.t.Start(ctx, "GetTopUnshortened")
	defer span.End()

	return s.r.GetTopUnshortened(ctx, amount, ttl)
}
