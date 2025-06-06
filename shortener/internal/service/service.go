package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"log/slog"
)

type postgresRepo interface {
	StoreShort(ctx context.Context, short models.Short) error
	GetShort(ctx context.Context, url string) (string, error)
	GetURL(ctx context.Context, short string) (string, error)
}
type Service struct {
	pr postgresRepo
}

func New(repo postgresRepo) *Service {
	return &Service{pr: repo}
}

func (s *Service) ShortURL(ctx context.Context, short *models.Short) error {
	// Try to get shorted by URL, and if it exists, return
	if sh, err := s.pr.GetShort(ctx, short.URL); err == nil {
		short.Short = sh
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to get short by url", "error", err)
		return errors.New("something went wrong")
	}

	slog.Info("shorting url", slog.String("url", short.URL))

	// Hash via SHA256
	hash := sha256.Sum256([]byte(short.URL))
	short.Short = fmt.Sprintf("%x", hash)[:10]

	if err := s.pr.StoreShort(ctx, *short); err != nil {
		slog.Error("failed to store short by url", "error", err)
		return errors.New("something went wrong")
	}

	return nil
}

func (s *Service) GetURL(ctx context.Context, short string) (string, error) {
	url, err := s.pr.GetURL(ctx, short)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("short not exist")
		}
		slog.Error("failed to get short by url", "error", err)
		return "", errors.New("something went wrong")
	}

	return url, nil
}
