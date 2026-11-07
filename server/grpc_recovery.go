package server

import (
	"context"
	"runtime/debug"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newGRPCServer builds a gRPC server that recovers from handler panics.
// gRPC runs each handler in its own goroutine, outside net/http's
// recovery, so an unrecovered panic would crash the process.
func newGRPCServer(opts ...grpc.ServerOption) *grpc.Server {
	opts = append(opts,
		grpc.ChainUnaryInterceptor(recoverUnary),
		grpc.ChainStreamInterceptor(recoverStream),
	)
	return grpc.NewServer(opts...)
}

func recoverUnary(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered from panic in gRPC handler method=%s err=%v\n%s", info.FullMethod, r, debug.Stack())
			err = status.Error(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}

func recoverStream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered from panic in gRPC handler method=%s err=%v\n%s", info.FullMethod, r, debug.Stack())
			err = status.Error(codes.Internal, "internal error")
		}
	}()
	return handler(srv, ss)
}
