SHELL=/bin/bash

all: net/lp_rpc.pb.go core/test_segment.go livepeer livepeer_cli

net/lp_rpc.pb.go: net/lp_rpc.proto
	protoc -I=. --go_out=plugins=grpc:. $^

core/test_segment.go:
	core/test_segment.sh core/test_segment.go

version=$(shell cat VERSION)

ldflags := -X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(shell ./print_version.sh)
cgo_ldflags :=

uname_s := $(shell uname -s)
ifeq ($(uname_s),Darwin)
		cgo_ldflags += -framework CoreFoundation -framework Security
endif

.PHONY: livepeer
livepeer:
	GO111MODULE=on CGO_LDFLAGS="$(cgo_ldflags)" go build -tags "$(HIGHEST_CHAIN_TAG)" -ldflags="$(ldflags)" cmd/livepeer/*.go

.PHONY: livepeer_cli
livepeer_cli:
	GO111MODULE=on CGO_LDFLAGS="$(cgo_ldflags)" go build -tags "$(HIGHEST_CHAIN_TAG)" -ldflags="$(ldflags)" cmd/livepeer_cli/*.go

.PHONY: localdocker
localdocker:
	./print_version.sh > .git.describe
	# docker build -t livepeerbinary:debian -f Dockerfile.debian .
	# Manually build our context... this is hacky but docker refuses to support symlinks
	# or selectable .dockerignore files
	tar ch --exclude=.git . | docker build --build-arg HIGHEST_CHAIN_TAG=${HIGHEST_CHAIN_TAG} -t livepeerbinary:debian -f docker/Dockerfile.debian -
	rm .git.describe

