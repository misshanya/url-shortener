package service

import (
	"context"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"os"
	"testing"
)

func Test_ShortenURL(t *testing.T) {
	tests := []struct {
		Name           string
		PublicHost     string
		InputURL       string
		ExceptedResult string
		ExceptedErr    error
		SetUpMocks     func(client *mockgrpcClient)
	}{
		{
			Name:           "Successfully Shortened",
			PublicHost:     "https://sh.some/",
			InputURL:       "https://go.dev",
			ExceptedResult: "https://sh.some/3a",
			ExceptedErr:    nil,
			SetUpMocks: func(client *mockgrpcClient) {
				client.On("ShortenURL", mock.Anything, &pb.ShortenURLRequest{Url: "https://go.dev"}).
					Return(
						&pb.ShortenURLResponse{
							Code: "3a",
						},
						status.New(codes.OK, "").Err(),
					).Once()
			},
		},
		{
			Name:           "gRPC server answered with internal error",
			PublicHost:     "https://sh.some/",
			InputURL:       "https://go.dev",
			ExceptedResult: "",
			ExceptedErr:    status.New(codes.Internal, "Internal Server Error").Err(),
			SetUpMocks: func(client *mockgrpcClient) {
				client.On("ShortenURL", mock.Anything, &pb.ShortenURLRequest{Url: "https://go.dev"}).
					Return(
						nil,
						status.New(codes.Internal, "Internal Server Error").Err(),
					).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockClient := mockgrpcClient{}

			tt.SetUpMocks(&mockClient)

			service := New(
				&mockClient,
				tt.PublicHost,
				slog.New(
					slog.NewTextHandler(
						os.Stdout,
						&slog.HandlerOptions{},
					),
				),
			)

			result, err := service.ShortenURL(context.Background(), tt.InputURL)
			assert.Equal(t, tt.ExceptedErr, err)
			assert.Equal(t, tt.ExceptedResult, result)

			mockClient.AssertExpectations(t)
		})
	}
}
