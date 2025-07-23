package service

import (
	"context"
	"errors"
	"github.com/misshanya/url-shortener/gateway/internal/models"
	pb "github.com/misshanya/url-shortener/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"testing"
)

func Test_mapGRPCError(t *testing.T) {
	tests := []struct {
		Name        string
		InputErr    error
		ExceptedErr *models.HTTPError
	}{
		{
			Name:        "OK",
			InputErr:    status.New(codes.OK, "").Err(),
			ExceptedErr: nil,
		},
		{
			Name:     "Internal",
			InputErr: status.New(codes.Internal, "Internal").Err(),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Internal",
			},
		},
		{
			Name:     "Not Found",
			InputErr: status.New(codes.NotFound, "Not Found").Err(),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusNotFound,
				Message: "Not Found",
			},
		},
		{
			Name:     "Invalid Argument",
			InputErr: status.New(codes.InvalidArgument, "Invalid Argument").Err(),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusBadRequest,
				Message: "Invalid Argument",
			},
		},
		{
			Name:     "Non-gRPC error",
			InputErr: errors.New("some unmappable error"),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
			},
		},
		{
			Name:     "Default case (unknown)",
			InputErr: status.New(codes.Unknown, "Unknown Error").Err(),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			httpErr := mapGRPCError(tt.InputErr)
			assert.Equal(t, tt.ExceptedErr, httpErr)
		})
	}
}

func Test_ShortenURL(t *testing.T) {
	tests := []struct {
		Name           string
		PublicHost     string
		InputURL       string
		ExceptedResult string
		ExceptedErr    *models.HTTPError
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
							Code:        "3a",
							OriginalUrl: "https://go.dev",
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
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
			},
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

			service := NewService(&mockClient, tt.PublicHost)

			result, err := service.ShortenURL(context.Background(), tt.InputURL)
			assert.Equal(t, tt.ExceptedErr, err)
			assert.Equal(t, tt.ExceptedResult, result)

			mockClient.AssertExpectations(t)
		})
	}
}

func Test_ShortenURLBatch(t *testing.T) {
	type inputModel struct {
		OriginalURL string
		Code        string
		Error       string
	}

	tests := []struct {
		Name        string
		PublicHost  string
		InputURLs   []inputModel
		ExceptedErr *models.HTTPError
		SetUpMocks  func(client *mockgrpcClient, inputs []inputModel)
	}{
		{
			Name:       "Successfully Shortened 2 of 3",
			PublicHost: "https://sh.some/",
			InputURLs: []inputModel{
				{
					OriginalURL: "https://go.dev",
					Code:        "3a",
				},
				{
					OriginalURL: "https://github.com",
					Error:       "some error",
				},
				{
					OriginalURL: "https://gitlab.com",
					Code:        "3c",
				},
			},
			ExceptedErr: nil,
			SetUpMocks: func(client *mockgrpcClient, inputs []inputModel) {
				urlsForReq := make([]*pb.ShortenURLRequest, len(inputs))
				for i, input := range inputs {
					urlsForReq[i] = &pb.ShortenURLRequest{Url: input.OriginalURL}
				}

				urlsForResp := make([]*pb.ShortenURLResponse, len(inputs))
				for i, input := range inputs {
					urlsForResp[i] = &pb.ShortenURLResponse{
						Code:        input.Code,
						OriginalUrl: input.OriginalURL,
						Error:       input.Error,
					}
				}

				client.On("ShortenURLBatch", mock.Anything, &pb.ShortenURLBatchRequest{Urls: urlsForReq}).
					Return(
						&pb.ShortenURLBatchResponse{
							Urls: urlsForResp,
						},
						nil,
					).Once()
			},
		},
		{
			Name:      "gRPC server returned slice not the same length",
			InputURLs: make([]inputModel, 5),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
			},
			SetUpMocks: func(client *mockgrpcClient, inputs []inputModel) {
				client.On("ShortenURLBatch", mock.Anything, mock.Anything).
					Return(
						&pb.ShortenURLBatchResponse{
							Urls: make([]*pb.ShortenURLResponse, len(inputs)-1),
						},
						nil,
					).Once()
			},
		},
		{
			Name:      "gRPC server answered with error",
			InputURLs: make([]inputModel, 2),
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Something went wrong",
			},
			SetUpMocks: func(client *mockgrpcClient, inputs []inputModel) {
				client.On("ShortenURLBatch", mock.Anything, mock.Anything).
					Return(
						nil,
						status.New(codes.Internal, "Something went wrong").Err(),
					).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockClient := mockgrpcClient{}

			tt.SetUpMocks(&mockClient, tt.InputURLs)

			service := NewService(&mockClient, tt.PublicHost)

			shorts := make([]*models.Short, len(tt.InputURLs))
			for i, input := range tt.InputURLs {
				shorts[i] = &models.Short{
					OriginalURL: input.OriginalURL,
				}
			}
			err := service.ShortenURLBatch(context.Background(), shorts)
			assert.Equal(t, tt.ExceptedErr, err)

			for i := range shorts {
				assert.Equal(t, tt.InputURLs[i].OriginalURL, shorts[i].OriginalURL)
				assert.Equal(t, tt.InputURLs[i].Error, shorts[i].Error)
				if tt.InputURLs[i].Error == "" {
					assert.Equal(t, tt.PublicHost+tt.InputURLs[i].Code, shorts[i].ShortURL)
				}
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func Test_UnshortenURL(t *testing.T) {
	tests := []struct {
		Name           string
		InputCode      string
		ExceptedResult string
		ExceptedErr    *models.HTTPError
		SetUpMocks     func(client *mockgrpcClient)
	}{
		{
			Name:           "Successfully Unshortened",
			InputCode:      "3a",
			ExceptedResult: "https://go.dev",
			ExceptedErr:    nil,
			SetUpMocks: func(client *mockgrpcClient) {
				client.On("GetURL", mock.Anything, &pb.GetURLRequest{Code: "3a"}).
					Return(&pb.GetURLResponse{Url: "https://go.dev"}, nil).Once()
			},
		}, {
			Name:           "gRPC server answered with internal error",
			InputCode:      "3a",
			ExceptedResult: "",
			ExceptedErr: &models.HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "Internal Server Error",
			},
			SetUpMocks: func(client *mockgrpcClient) {
				client.On("GetURL", mock.Anything, &pb.GetURLRequest{Code: "3a"}).
					Return(
						nil,
						status.New(
							codes.Internal,
							"Internal Server Error",
						).Err(),
					).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockClient := mockgrpcClient{}

			tt.SetUpMocks(&mockClient)

			service := NewService(&mockClient, "")

			result, err := service.UnshortenURL(context.Background(), tt.InputCode)
			assert.Equal(t, tt.ExceptedErr, err)
			assert.Equal(t, tt.ExceptedResult, result)

			mockClient.AssertExpectations(t)
		})
	}
}
