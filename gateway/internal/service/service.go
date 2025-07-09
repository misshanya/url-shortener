package service

import (
	"context"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

type Service struct {
	client     pb.URLShortenerServiceClient
	publicHost string
}

func NewService(client pb.URLShortenerServiceClient, publicHost string) *Service {
	return &Service{client: client, publicHost: publicHost}
}

func mapGRPCError(err error) *models.HTTPError {
	s, ok := status.FromError(err)
	if !ok {
		return &models.HTTPError{
			Code:    http.StatusInternalServerError,
			Message: "Internal Server Error",
		}
	}

	switch s.Code() {
	case codes.Internal:
		return &models.HTTPError{
			Code:    http.StatusInternalServerError,
			Message: s.Message(),
		}
	case codes.NotFound:
		return &models.HTTPError{
			Code:    http.StatusNotFound,
			Message: s.Message(),
		}
	case codes.InvalidArgument:
		return &models.HTTPError{
			Code:    http.StatusBadRequest,
			Message: s.Message(),
		}
	}

	return nil
}

func (s *Service) ShortenURL(ctx context.Context, url string) (string, *models.HTTPError) {
	resp, err := s.client.ShortenURL(ctx, &pb.ShortenURLRequest{Url: url})
	if httpErr := mapGRPCError(err); httpErr != nil {
		return "", &models.HTTPError{
			Code:    httpErr.Code,
			Message: httpErr.Message,
		}
	}

	short := s.publicHost + resp.Url
	return short, nil
}

func (s *Service) UnshortenURL(ctx context.Context, code string) (string, *models.HTTPError) {
	resp, err := s.client.GetURL(ctx, &pb.GetURLRequest{Code: code})
	if httpErr := mapGRPCError(err); httpErr != nil {
		return "", &models.HTTPError{
			Code:    httpErr.Code,
			Message: httpErr.Message,
		}
	}

	return resp.Url, nil
}
