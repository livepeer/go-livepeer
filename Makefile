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

.PHONY: localdocker
localdocker:
	git describe --always --long --dirty > .git.describe
	# docker build -t livepeerbinary:debian -f Dockerfile.debian .
	# Manually build our context... this is hacky but docker refuses to support symlinks
	# or selectable .dockerignore files
	-tar ch --exclude=.git . | docker build -t livepeerbinary:debian -f docker/Dockerfile.debian -
	rm .git.describe
