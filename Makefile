SHELL=/bin/bash

all: net/lp_rpc.pb.go net/redeemer.pb.go net/redeemer_mock.pb.go core/test_segment.go livepeer livepeer_cli livepeer_router livepeer_bench

net/lp_rpc.pb.go: net/lp_rpc.proto
	protoc -I=. --go_out=plugins=grpc:. $^

net/redeemer.pb.go: net/redeemer.proto
	protoc -I=. --go_out=plugins=grpc:. $^

net/redeemer_mock.pb.go: net/redeemer.pb.go
	mockgen -source net/redeemer.pb.go -destination net/redeemer_mock.pb.go -package net $^

core/test_segment.go:
	core/test_segment.sh core/test_segment.go

version=$(shell cat VERSION)

ldflags := -X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(shell ./print_version.sh)
cgo_cflags :=
cgo_ldflags :=

uname_s := $(shell uname -s)
ifeq ($(uname_s),Darwin)
		cgo_ldflags += -framework CoreFoundation -framework Security
	ifeq ($(GOARCH),arm64)
		cgo_cflags += --target=arm64-apple-macos11
		cgo_ldflags += --target=arm64-apple-macos11
	endif
endif

.PHONY: livepeer
livepeer:
	GO111MODULE=on CGO_ENABLED=1 CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags)" go build -tags $(BUILD_TAGS) -ldflags="$(ldflags)" cmd/livepeer/*.go

.PHONY: livepeer_cli
livepeer_cli:
	GO111MODULE=on CGO_ENABLED=1 CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags)" go build -tags $(BUILD_TAGS) -ldflags="$(ldflags)" cmd/livepeer_cli/*.go

.PHONY: livepeer_bench
livepeer_bench:
	GO111MODULE=on CGO_ENABLED=1 CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags)" go build -ldflags="$(ldflags)" cmd/livepeer_bench/*.go

.PHONY: livepeer_router
livepeer_router:
	GO111MODULE=on CGO_ENABLED=1 CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags)" go build -ldflags="$(ldflags)" cmd/livepeer_router/*.go

.PHONY: localdocker
localdocker:
	./print_version.sh > .git.describe
	# docker build -t livepeerbinary:debian -f Dockerfile.debian .
	# Manually build our context... this is hacky but docker refuses to support symlinks
	# or selectable .dockerignore files
	tar ch --exclude=.git . | docker build --build-arg BUILD_TAGS=$(BUILD_TAGS) -t livepeerbinary:debian -f docker/Dockerfile.debian -
	rm .git.describe

