package server

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoverUnary(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	resp, err := recoverUnary(context.Background(), nil, info, func(context.Context, any) (any, error) {
		panic("boom")
	})
	require.Nil(t, resp)
	require.Equal(t, codes.Internal, status.Code(err))

	wantErr := errors.New("handler error")
	resp, err = recoverUnary(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return "ok", wantErr
	})
	require.Equal(t, "ok", resp)
	require.Equal(t, wantErr, err)
}

func TestRecoverStream(t *testing.T) {
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}

	err := recoverStream(nil, nil, info, func(any, grpc.ServerStream) error {
		panic("boom")
	})
	require.Equal(t, codes.Internal, status.Code(err))

	wantErr := errors.New("handler error")
	err = recoverStream(nil, nil, info, func(any, grpc.ServerStream) error {
		return wantErr
	})
	require.Equal(t, wantErr, err)
}
