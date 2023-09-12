SHELL=/bin/bash
GO_BUILD_DIR?="./"

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
cc :=

uname_s := $(shell uname -s)
ifeq ($(uname_s),Darwin)
		cgo_ldflags += -framework CoreFoundation -framework Security
	ifeq ($(GOARCH),arm64)
		cgo_cflags += --target=arm64-apple-macos11
		cgo_ldflags += --target=arm64-apple-macos11
	endif
endif

ifeq ($(uname_s),Linux)
	ifeq ($(GOARCH),arm64)
		cgo_cflags += --target=aarch64-linux-gnu
		cgo_ldflags += --target=aarch64-linux-gnu
		cc = clang --sysroot=/usr/aarch64-linux-gnu
	endif
endif

pkg_config_libdir :=
ifeq ($(uname_s),Linux)
	ifeq ($(GOOS),windows)
		cc = x86_64-w64-mingw32-gcc
	endif
endif

.PHONY: livepeer livepeer_bench livepeer_cli livepeer_router docker

livepeer:
	GO111MODULE=on CGO_ENABLED=1 CC="$(cc)" CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags) ${CGO_LDFLAGS}" go build -o $(GO_BUILD_DIR) -tags "$(BUILD_TAGS)" -ldflags="$(ldflags)" cmd/livepeer/*.go

livepeer_cli:
	GO111MODULE=on CGO_ENABLED=1 CC="$(cc)" CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags) ${CGO_LDFLAGS}" go build -o $(GO_BUILD_DIR) -tags "$(BUILD_TAGS)" -ldflags="$(ldflags)" cmd/livepeer_cli/*.go

livepeer_bench:
	GO111MODULE=on CGO_ENABLED=1 CC="$(cc)" CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags) ${CGO_LDFLAGS}" go build -o $(GO_BUILD_DIR) -ldflags="$(ldflags)" cmd/livepeer_bench/*.go

livepeer_router:
	GO111MODULE=on CGO_ENABLED=1 CC="$(cc)" CGO_CFLAGS="$(cgo_cflags)" CGO_LDFLAGS="$(cgo_ldflags) ${CGO_LDFLAGS}" go build -o $(GO_BUILD_DIR) -ldflags="$(ldflags)" cmd/livepeer_router/*.go

docker:
	docker buildx build --build-arg='BUILD_TAGS=mainnet,experimental' -f docker/Dockerfile .
