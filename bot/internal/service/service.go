package service

import (
	"context"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"google.golang.org/grpc"
	"log/slog"
)

type grpcClient interface {
	ShortenURL(ctx context.Context, in *pb.ShortenURLRequest, opts ...grpc.CallOption) (*pb.ShortenURLResponse, error)
}

type Service struct {
	client     grpcClient
	publicHost string
	l          *slog.Logger
}

func New(client grpcClient, publicHost string, logger *slog.Logger) *Service {
	return &Service{client: client, publicHost: publicHost, l: logger}
}

func (s *Service) ShortenURL(ctx context.Context, url string) (string, error) {
	resp, err := s.client.ShortenURL(ctx, &pb.ShortenURLRequest{Url: url})
	if err != nil {
		s.l.Error("failed to shorten url", slog.Any("error", err))
		return "", err
	}

	short := s.publicHost + resp.Code
	return short, nil
}
