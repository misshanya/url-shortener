package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/misshanya/url-shortener/shortener/pkg/base62"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
)

type postgresRepo interface {
	StoreURL(ctx context.Context, url string) (int64, error)
	GetID(ctx context.Context, url string) (int64, error)
	GetURL(ctx context.Context, id int64) (string, error)
}
type Service struct {
	pr postgresRepo
	l  *slog.Logger
}

func New(repo postgresRepo, logger *slog.Logger) *Service {
	return &Service{pr: repo, l: logger}
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

	return nil
}

func (s *Service) GetURL(ctx context.Context, short string) (string, error) {
	// Decode shorted into ID
	id := base62.Decode(short)

	url, err := s.pr.GetURL(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", status.Error(codes.NotFound, "short not found")
		}
		s.l.Error("failed to get short by url", "error", err)
		return "", status.Error(codes.Internal, "failed to get short by url")
	}

	return url, nil
}
