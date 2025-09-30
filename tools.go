//go:build tools
// +build tools

package tools

import (
	_ "github.com/ethereum/go-ethereum/cmd/abigen"
	_ "github.com/golang/mock/mockgen"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
