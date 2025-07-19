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
	ShortenURL(ctx context.Context, short *models.Short) error
	ShortenURLBatch(ctx context.Context, shorts []*models.Short)
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

func (h *Handler) ShortenURL(ctx context.Context, req *pb.ShortenURLRequest) (*pb.ShortenURLResponse, error) {
	short := models.Short{URL: req.Url}

	// Validate URL
	if _, err := url.ParseRequestURI(short.URL); err != nil {
		return nil, status.Error(codes.InvalidArgument, "bad URL")
	}

	if err := h.service.ShortenURL(ctx, &short); err != nil {
		return nil, err
	}

	return &pb.ShortenURLResponse{Code: short.Short, OriginalUrl: short.URL}, nil
}

func (h *Handler) ShortenURLBatch(ctx context.Context, req *pb.ShortenURLBatchRequest) (*pb.ShortenURLBatchResponse, error) {
	shorts := make([]*models.Short, len(req.Urls))

	// Validate and map URLs into models
	for i, reqUrl := range req.Urls {
		short := models.Short{URL: reqUrl.Url}
		shorts[i] = &short

		if _, err := url.ParseRequestURI(reqUrl.Url); err != nil {
			short.Error = err
		}
	}

	h.service.ShortenURLBatch(ctx, shorts)

	// Prepare response struct with URLs slice
	response := pb.ShortenURLBatchResponse{Urls: make([]*pb.ShortenURLResponse, len(req.Urls))}
	for i, short := range shorts {
		resp := pb.ShortenURLResponse{OriginalUrl: short.URL}
		response.Urls[i] = &resp

		if short.Error != nil {
			resp.Error = short.Error.Error()
			continue
		}
		resp.Code = short.Short
	}

	return &response, nil
}

func (h *Handler) GetURL(ctx context.Context, req *pb.GetURLRequest) (*pb.GetURLResponse, error) {
	code := req.Code

	originalURL, err := h.service.GetURL(ctx, code)
	if err != nil {
		return nil, err
	}

	return &pb.GetURLResponse{Url: originalURL}, nil
}
