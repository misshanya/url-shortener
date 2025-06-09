package grpc

import (
	"context"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/url"
)

type service interface {
	ShortURL(ctx context.Context, short *models.Short) error
	GetURL(ctx context.Context, short string) (string, error)
}

type Handler struct {
	service service
	pb.UnimplementedURLShortenerServiceServer
}

func NewHandler(grpcServer *grpc.Server, service service) {
	shortenerGrpc := &Handler{service: service}
	pb.RegisterURLShortenerServiceServer(grpcServer, shortenerGrpc)
}

func (h *Handler) ShortURL(ctx context.Context, req *pb.ShortURLRequest) (*pb.ShortURLResponse, error) {
	short := models.Short{URL: req.Url}

	// Validate URL
	if _, err := url.ParseRequestURI(short.URL); err != nil {
		return nil, status.Error(codes.InvalidArgument, "bad URL")
	}

	if err := h.service.ShortURL(ctx, &short); err != nil {
		return nil, err
	}

	return &pb.ShortURLResponse{Url: short.Short}, nil
}

func (h *Handler) GetURL(ctx context.Context, req *pb.GetURLRequest) (*pb.GetURLResponse, error) {
	hash := req.Hash

	originalURL, err := h.service.GetURL(ctx, hash)
	if err != nil {
		return nil, err
	}

	return &pb.GetURLResponse{Url: originalURL}, nil
}
