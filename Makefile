all: net/lp_rpc.pb.go livepeer livepeer_cli

net/lp_rpc.pb.go: net/lp_rpc.proto
	protoc -I=. --go_out=plugins=grpc:. $^

version=$(shell cat VERSION)

.PHONY: livepeer
livepeer:
	go build -ldflags="-X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(version)-$(shell git describe --always --long --dirty)" cmd/livepeer/*.go

.PHONY: livepeer_cli
livepeer_cli:
	go build -ldflags="-X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(version)-$(shell git describe --always --long --dirty)" cmd/livepeer_cli/*.go
