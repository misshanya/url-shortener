package grpc

import (
	"context"
	"errors"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func Test_ShortenURL(t *testing.T) {
	tests := []struct {
		Name             string
		InputReq         *pb.ShortenURLRequest
		ExceptedResponse *pb.ShortenURLResponse
		ExceptedErr      error
		SetUpMocks       func(service *mockservice, short *models.Short)
	}{
		{
			Name:             "Successfully Shortened",
			InputReq:         &pb.ShortenURLRequest{Url: "https://go.dev"},
			ExceptedResponse: &pb.ShortenURLResponse{Code: "3a", OriginalUrl: "https://go.dev"},
			ExceptedErr:      nil,
			SetUpMocks: func(service *mockservice, short *models.Short) {
				service.On("ShortenURL", mock.Anything, short).
					Return(nil).Run(func(args mock.Arguments) {
					shortArg := args.Get(1).(*models.Short)
					shortArg.Short = "3a"
				}).Once()
			},
		},
		{
			Name:             "Invalid input URL",
			InputReq:         &pb.ShortenURLRequest{Url: "some invalid url"},
			ExceptedResponse: nil,
			ExceptedErr:      status.Error(codes.InvalidArgument, "bad URL"),
			SetUpMocks:       func(service *mockservice, short *models.Short) {},
		},
		{
			Name:             "Failed to shorten URL on service side",
			InputReq:         &pb.ShortenURLRequest{Url: "https://go.dev"},
			ExceptedResponse: nil,
			ExceptedErr:      status.Error(codes.Internal, "failed to get short by url"),
			SetUpMocks: func(service *mockservice, short *models.Short) {
				service.On("ShortenURL", mock.Anything, short).
					Return(status.Error(codes.Internal, "failed to get short by url")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockService := mockservice{}

			short := &models.Short{URL: tt.InputReq.Url}

			tt.SetUpMocks(&mockService, short)

			handler := Handler{service: &mockService}

			resp, err := handler.ShortenURL(context.Background(), tt.InputReq)
			assert.Equal(t, tt.ExceptedErr, err)
			assert.Equal(t, tt.ExceptedResponse, resp)

			mockService.AssertExpectations(t)
		})
	}
}

func Test_ShortenURLBatch(t *testing.T) {
	tests := []struct {
		Name             string
		InputReq         *pb.ShortenURLBatchRequest
		ExceptedResponse *pb.ShortenURLBatchResponse
		ExceptedErr      error
		SetUpMocks       func(service *mockservice, shorts []*models.Short)
	}{
		{
			Name: "Successfully Shortened 2 of 3",
			InputReq: &pb.ShortenURLBatchRequest{
				Urls: []*pb.ShortenURLRequest{
					{Url: "https://go.dev"},
					{Url: "some invalid url"},
					{Url: "https://gitlab.com"},
				},
			},
			ExceptedResponse: &pb.ShortenURLBatchResponse{
				Urls: []*pb.ShortenURLResponse{
					{OriginalUrl: "https://go.dev", Code: "3a"},
					{OriginalUrl: "some invalid url", Error: "parse \"some invalid url\": invalid URI for request"},
					{OriginalUrl: "https://gitlab.com", Code: "3b"},
				},
			},
			ExceptedErr: nil,
			SetUpMocks: func(service *mockservice, shorts []*models.Short) {
				service.On("ShortenURLBatch", mock.Anything, mock.AnythingOfType("[]*models.Short")).
					Run(func(args mock.Arguments) {
						urls := map[string]string{
							"https://go.dev":     "3a",
							"https://gitlab.com": "3b",
						}
						for _, short := range args.Get(1).([]*models.Short) {
							if short.Error == nil {
								short.Short = urls[short.URL]
							}
						}
					}).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockService := mockservice{}

			shorts := make([]*models.Short, len(tt.InputReq.Urls))

			for i, reqUrl := range tt.InputReq.Urls {
				short := models.Short{URL: reqUrl.Url}
				shorts[i] = &short
			}

			tt.SetUpMocks(&mockService, shorts)

			handler := Handler{service: &mockService}

			resp, err := handler.ShortenURLBatch(context.Background(), tt.InputReq)
			assert.Equal(t, tt.ExceptedErr, err)
			assert.Equal(t, tt.ExceptedResponse, resp)

			mockService.AssertExpectations(t)
		})
	}
}

func Test_GetURL(t *testing.T) {
	tests := []struct {
		Name             string
		InputReq         *pb.GetURLRequest
		ExceptedResponse *pb.GetURLResponse
		ExceptedErr      error
		SetUpMocks       func(service *mockservice, code string)
	}{
		{
			Name:             "Successfully Got URL",
			InputReq:         &pb.GetURLRequest{Code: "3a"},
			ExceptedResponse: &pb.GetURLResponse{Url: "https://go.dev"},
			ExceptedErr:      nil,
			SetUpMocks: func(service *mockservice, code string) {
				service.On("GetURL", mock.Anything, code).
					Return("https://go.dev", nil).Once()
			},
		},
		{
			Name:             "Service returned an error",
			InputReq:         &pb.GetURLRequest{Code: "3a"},
			ExceptedResponse: nil,
			ExceptedErr:      errors.New("some error"),
			SetUpMocks: func(service *mockservice, code string) {
				service.On("GetURL", mock.Anything, code).
					Return("", errors.New("some error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockService := mockservice{}

			tt.SetUpMocks(&mockService, tt.InputReq.Code)

			handler := Handler{service: &mockService}

			resp, err := handler.GetURL(context.Background(), tt.InputReq)
			assert.Equal(t, tt.ExceptedErr, err)
			assert.Equal(t, tt.ExceptedResponse, resp)

			mockService.AssertExpectations(t)
		})
	}
}
