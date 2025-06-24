package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/misshanya/url-shortener/shortener/pkg/base62"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"strconv"
	"time"
)

type postgresRepo interface {
	StoreURL(ctx context.Context, url string) (int64, error)
	GetID(ctx context.Context, url string) (int64, error)
	GetURL(ctx context.Context, id int64) (string, error)
}

type valkeyRepo interface {
	SetTop(ctx context.Context, top models.UnshortenedTop, ttl int) error
	GetURLByCode(ctx context.Context, code string) (string, error)
}

type Service struct {
	pr postgresRepo
	vr valkeyRepo
	l  *slog.Logger
	kw *kafka.Writer

	topTTL int
}

func New(repo postgresRepo, vr valkeyRepo, logger *slog.Logger, kafkaWriter *kafka.Writer, topTTL int) *Service {
	return &Service{
		pr: repo,
		vr: vr,
		l:  logger,
		kw: kafkaWriter,

		topTTL: topTTL,
	}
}

func (s *Service) ShortenURL(ctx context.Context, short *models.Short) error {
	// Try to get ID by URL, and if it exists, encode and return
	if id, err := s.pr.GetID(ctx, short.URL); err == nil {
		short.Short = base62.Encode(id)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		s.l.Error("failed to get short by url", "error", err)
		return status.Error(codes.Internal, "failed to get short by url")
	}

	s.l.Info("shortening url", slog.String("url", short.URL))

	id, err := s.pr.StoreURL(ctx, short.URL)
	if err != nil {
		s.l.Error("failed to store short by url", "error", err)
		return status.Error(codes.Internal, "failed to store short")
	}

	// Encode via base62
	short.Short = base62.Encode(id)

	// Write to Kafka that we are just shortened the URL
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
		if err := s.kw.WriteMessages(context.Background(),
			kafka.Message{
				Topic: "shortener.shortened",
				Value: msgMarshaled,
			}); err != nil {
			s.l.Error("failed to write messages to Kafka", "error", err)
		}
	}()

	return nil
}

func (s *Service) GetURL(ctx context.Context, short string) (string, error) {
	// Decode shorted into ID
	id := base62.Decode(short)

	url, err := s.vr.GetURLByCode(ctx, strconv.Itoa(int(id)))
	if err != nil {
		s.l.Error("failed to get short by url from cache", "error", err)
	}

	if url == "" {
		url, err = s.pr.GetURL(ctx, id)
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
	msg := models.KafkaMessageUnshortened{
		UnshortenedAt: time.Now(),
		OriginalURL:   url,
		ShortCode:     short,
	}
	msgMarshaled, err := json.Marshal(msg)
	if err != nil {
		s.l.Error("failed to marshal KafkaMessageShortened", "error", err)
	}
	go func() {
		if err := s.kw.WriteMessages(context.Background(),
			kafka.Message{
				Topic: "shortener.unshortened",
				Value: msgMarshaled,
			}); err != nil {
			s.l.Error("failed to write messages to Kafka", "error", err)
		}
	}()

	return url, nil
}

func (s *Service) SetTop(ctx context.Context, msg *models.KafkaMessageUnshortenedTop) {
	top := models.UnshortenedTop{
		Top: []struct {
			OriginalURL string
			ShortCode   string
		}(msg.Top),
	}

	err := s.vr.SetTop(ctx, top, s.topTTL)
	if err != nil {
		s.l.Error("failed to set top to cache", "error", err)
		return
	}

	s.l.Info("cached top", "quantity", len(top.Top))
}
