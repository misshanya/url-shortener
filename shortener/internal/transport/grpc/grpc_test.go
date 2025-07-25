package grpc

import (
	"context"
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
