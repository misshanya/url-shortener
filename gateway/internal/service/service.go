package service

import (
	"context"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

type grpcClient interface {
	ShortenURL(ctx context.Context, in *pb.ShortenURLRequest, opts ...grpc.CallOption) (*pb.ShortenURLResponse, error)
	ShortenURLBatch(ctx context.Context, in *pb.ShortenURLBatchRequest, opts ...grpc.CallOption) (*pb.ShortenURLBatchResponse, error)
	GetURL(ctx context.Context, in *pb.GetURLRequest, opts ...grpc.CallOption) (*pb.GetURLResponse, error)
}

type Service struct {
	client     grpcClient
	publicHost string
}

func NewService(client grpcClient, publicHost string) *Service {
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
	case codes.OK:
		return nil
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
	default:
		return &models.HTTPError{
			Code:    http.StatusInternalServerError,
			Message: "Internal Server Error",
		}
	}
}

func (s *Service) ShortenURL(ctx context.Context, url string) (string, *models.HTTPError) {
	resp, err := s.client.ShortenURL(ctx, &pb.ShortenURLRequest{Url: url})
	if httpErr := mapGRPCError(err); httpErr != nil {
		return "", &models.HTTPError{
			Code:    httpErr.Code,
			Message: httpErr.Message,
		}
	}

	short := s.publicHost + resp.Code
	return short, nil
}

func (s *Service) ShortenURLBatch(ctx context.Context, urls []*models.Short) *models.HTTPError {
	urlsForReq := make([]*pb.ShortenURLRequest, len(urls))
	for i, url := range urls {
		urlsForReq[i] = &pb.ShortenURLRequest{Url: url.OriginalURL}
	}
	resp, err := s.client.ShortenURLBatch(ctx, &pb.ShortenURLBatchRequest{Urls: urlsForReq})
	if httpErr := mapGRPCError(err); httpErr != nil {
		return &models.HTTPError{
			Code:    httpErr.Code,
			Message: httpErr.Message,
		}
	}

	if len(resp.Urls) != len(urls) {
		return &models.HTTPError{
			Code:    http.StatusInternalServerError,
			Message: "Internal Server Error",
		}
	}

	for i, url := range resp.Urls {
		if url.Error != "" {
			urls[i].Error = url.Error
			continue
		}
		urls[i].ShortURL = s.publicHost + url.Code
	}

	return nil
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
