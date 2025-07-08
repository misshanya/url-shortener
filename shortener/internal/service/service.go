package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/misshanya/url-shortener/shortener/pkg/base62"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"time"
)

type postgresRepo interface {
	StoreURL(ctx context.Context, url string) (int64, error)
	GetID(ctx context.Context, url string) (int64, error)
	GetURL(ctx context.Context, id int64) (string, error)
}

type valkeyRepo interface {
	SetTop(ctx context.Context, top models.UnshortenedTop, ttl time.Duration) error
	GetURLByCode(ctx context.Context, code string) (string, error)
}

type Service struct {
	pr postgresRepo
	vr valkeyRepo
	l  *slog.Logger
	kw *kafka.Writer
	t  trace.Tracer
}

func New(repo postgresRepo, vr valkeyRepo, logger *slog.Logger, kafkaWriter *kafka.Writer, t trace.Tracer) *Service {
	return &Service{
		pr: repo,
		vr: vr,
		l:  logger,
		kw: kafkaWriter,
		t:  t,
	}
}

func (s *Service) ShortenURL(ctx context.Context, short *models.Short) error {
	ctx, span := s.t.Start(ctx, "ShortenURL")
	defer span.End()

	// Try to get ID by URL, and if it exists, encode and return
	ctxGet, spanGet := s.t.Start(ctx, "try-get-id-from-db")
	id, err := s.pr.GetID(ctxGet, short.URL)
	spanGet.End()
	if err == nil {
		short.Short = base62.Encode(id)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		s.l.Error("failed to get short by url", "error", err)
		return status.Error(codes.Internal, "failed to get short by url")
	}

	s.l.Info("shortening url", slog.String("url", short.URL))

	ctxStore, spanStore := s.t.Start(ctx, "store-url")
	id, err = s.pr.StoreURL(ctxStore, short.URL)
	spanStore.End()
	if err != nil {
		s.l.Error("failed to store short by url", "error", err)
		return status.Error(codes.Internal, "failed to store short")
	}

	// Encode via base62
	short.Short = base62.Encode(id)

	// Write to Kafka that we are just shortened the URL

	carrier := propagation.MapCarrier{}
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, carrier)

	msg := models.KafkaMessageShortened{
		ShortenedAt: time.Now(),
		OriginalURL: short.URL,
		ShortCode:   short.Short,
	}
	msgMarshaled, err := json.Marshal(msg)
	if err != nil {
		s.l.Error("failed to marshal KafkaMessageShortened", "error", err)
	}
	go func() {
		propagatorKafka := propagation.TraceContext{}
		ctxKafka := propagatorKafka.Extract(context.Background(), carrier)

		// Kafka message headers for the trace context
		headers := make([]kafka.Header, len(carrier.Keys()))

		for i, key := range carrier.Keys() {
			headers[i] = kafka.Header{
				Key:   key,
				Value: []byte(carrier.Get(key)),
			}
		}

		ctxKafka, spanKafka := s.t.Start(ctxKafka, "write-to-kafka")
		defer spanKafka.End()

		if err := s.kw.WriteMessages(ctxKafka,
			kafka.Message{
				Headers: headers,
				Topic:   "shortener.shortened",
				Value:   msgMarshaled,
			}); err != nil {
			s.l.Error("failed to write messages to Kafka", "error", err)
		}
	}()

	return nil
}

func (s *Service) GetURL(ctx context.Context, short string) (string, error) {
	ctx, span := s.t.Start(ctx, "GetURL")
	defer span.End()

	// Decode shorted into ID
	id := base62.Decode(short)

	ctxGetCache, spanGetCache := s.t.Start(ctx, "get-url-from-cache")
	url, err := s.vr.GetURLByCode(ctxGetCache, short)
	spanGetCache.End()
	if err != nil {
		s.l.Error("failed to get short by url from cache", "error", err)
	}

	if url == "" {
		ctxGetDB, spanGetDB := s.t.Start(ctx, "get-url-from-db")
		url, err = s.pr.GetURL(ctxGetDB, id)
		spanGetDB.End()
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", status.Error(codes.NotFound, "short not found")
			}
			s.l.Error("failed to get short by url", "error", err)
			return "", status.Error(codes.Internal, "failed to get short by url")
		}
	} else {
		s.l.Info("got from cache", "url", url)
	}

	// Write to Kafka that we are just unshortened URL

	carrier := propagation.MapCarrier{}
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, carrier)

	msg := models.KafkaMessageUnshortened{
		UnshortenedAt: time.Now(),
		OriginalURL:   url,
		ShortCode:     short,
	}
	msgMarshaled, err := json.Marshal(msg)
	if err != nil {
		s.l.Error("failed to marshal KafkaMessageUnshortened", "error", err)
	}
	go func() {
		propagatorKafka := propagation.TraceContext{}
		ctxKafka := propagatorKafka.Extract(context.Background(), carrier)

		// Kafka message headers for the trace context
		headers := make([]kafka.Header, len(carrier.Keys()))

		for i, key := range carrier.Keys() {
			headers[i] = kafka.Header{
				Key:   key,
				Value: []byte(carrier.Get(key)),
			}
		}

		ctxKafka, spanKafka := s.t.Start(ctxKafka, "write-to-kafka")
		defer spanKafka.End()

		if err := s.kw.WriteMessages(ctxKafka,
			kafka.Message{
				Headers: headers,
				Topic:   "shortener.unshortened",
				Value:   msgMarshaled,
			}); err != nil {
			s.l.Error("failed to write messages to Kafka", "error", err)
		}
	}()

	return url, nil
}

func (s *Service) SetTop(ctx context.Context, msg *models.KafkaMessageUnshortenedTop) {
	ctx, span := s.t.Start(ctx, "SetTop")
	defer span.End()

	top := models.UnshortenedTop{
		ValidUntil: msg.ValidUntil,
		Top: []struct {
			OriginalURL string
			ShortCode   string
		}(msg.Top),
	}

	ttl := top.ValidUntil.Sub(time.Now())

	ctxStore, spanStore := s.t.Start(ctx, "store top in cache")
	err := s.vr.SetTop(ctxStore, top, ttl)
	spanStore.End()
	if err != nil {
		s.l.Error("failed to set top to cache", "error", err)
		return
	}

	s.l.Info("cached top", "quantity", len(top.Top))
}
