SHELL=/bin/bash
GO_BUILD_DIR?="./"

all: net/lp_rpc.pb.go net/redeemer.pb.go net/redeemer_mock.pb.go core/test_segment.go livepeer livepeer_cli livepeer_router livepeer_bench

net/lp_rpc.pb.go: net/lp_rpc.proto
	protoc -I=. --go_out=. --go-grpc_out=. $^

net/redeemer.pb.go: net/redeemer.proto
	protoc -I=. --go_out=. --go-grpc_out=. $^

net/redeemer_mock.pb.go net/redeemer_grpc_mock.pb.go: net/redeemer.pb.go net/redeemer_grpc.pb.go
	@mockgen -source net/redeemer.pb.go -destination net/redeemer_mock.pb.go -package net
	@mockgen -source net/redeemer_grpc.pb.go -destination net/redeemer_grpc_mock.pb.go -package net

core/test_segment.go:
	core/test_segment.sh core/test_segment.go

version=$(shell cat VERSION)

ldflags := -X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(shell ./print_version.sh)
cgo_cflags :=
cgo_ldflags :=
cc :=

# Build platform flags
BUILDOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
BUILDARCH ?= $(shell uname -m | tr '[:upper:]' '[:lower:]')
ifeq ($(BUILDARCH),aarch64)
		BUILDARCH=arm64
endif
ifeq ($(BUILDARCH),x86_64)
		BUILDARCH=amd64
endif

# Override these (not BUILDOS/BUILDARCH) for cross-compilation
export GOOS ?= $(BUILDOS)
export GOARCH ?= $(BUILDARCH)

# Cross-compilation section. Currently supported:
# darwin amd64 --> darwin arm64
# darwin arm64 --> linux arm64
# linux amd64 --> linux arm64
# linux amd64 --> windows amd64

ifeq ($(BUILDOS),darwin)
	ifeq ($(GOOS),darwin)
		cgo_ldflags += -framework CoreFoundation -framework Security
		ifeq ($(BUILDARCH),amd64)
			ifeq ($(GOARCH),arm64)
				cgo_cflags += --target=arm64-apple-macos11
				cgo_ldflags += --target=arm64-apple-macos11
			endif
		endif
	endif

	ifeq ($(GOOS),linux)
		ifeq ($(GOARCH),arm64)
			LLVM_PATH ?= /opt/homebrew/opt/llvm/bin
			SYSROOT ?= /tmp/sysroot-aarch64-linux-gnu
			cgo_cflags += --target=aarch64-linux-gnu -Wno-error=unused-command-line-argument -fuse-ld=$(LLVM_PATH)/ld.lld
			cgo_ldflags += --target=aarch64-linux-gnu
			cc = $(LLVM_PATH)/clang -fuse-ld=$(LLVM_PATH)/ld.lld --sysroot=$(SYSROOT)
		endif
	endif
endif

ifeq ($(BUILDOS),linux)
	ifeq ($(BUILDARCH),amd64)
		ifeq ($(GOARCH),arm64)
			ifeq ($(GOOS),linux)
				cgo_cflags += --target=aarch64-linux-gnu
				cgo_ldflags += --target=aarch64-linux-gnu
				cc = clang
			endif
		endif
	endif

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
