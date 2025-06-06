SHELL=/bin/bash
GO_BUILD_DIR?="./"

MOCKGEN=go run github.com/golang/mock/mockgen
ABIGEN=go run github.com/ethereum/go-ethereum/cmd/abigen

all: net/lp_rpc.pb.go net/redeemer.pb.go net/redeemer_mock.pb.go core/test_segment.go eth/contracts/chainlink/AggregatorV3Interface.go livepeer livepeer_cli livepeer_router livepeer_bench

net/lp_rpc.pb.go: net/lp_rpc.proto
	protoc -I=. --go_out=. --go-grpc_out=. $^

net/redeemer.pb.go: net/redeemer.proto
	protoc -I=. --go_out=. --go-grpc_out=. $^

net/redeemer_mock.pb.go net/redeemer_grpc_mock.pb.go: net/redeemer.pb.go net/redeemer_grpc.pb.go
	@$(MOCKGEN) -source net/redeemer.pb.go -destination net/redeemer_mock.pb.go -package net
	@$(MOCKGEN) -source net/redeemer_grpc.pb.go -destination net/redeemer_grpc_mock.pb.go -package net

core/test_segment.go:
	core/test_segment.sh core/test_segment.go

eth/contracts/chainlink/AggregatorV3Interface.go:
	solc --version | grep 0.7.6+commit.7338295f
	@set -ex; \
	for sol_file in eth/contracts/chainlink/*.sol; do \
		contract_name=$$(basename "$$sol_file" .sol); \
		solc --abi --optimize --overwrite -o $$(dirname "$$sol_file") $$sol_file; \
		$(ABIGEN) --abi=$${sol_file%.sol}.abi --pkg=chainlink --type=$$contract_name --out=$${sol_file%.sol}.go; \
	done

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
		cgo_ldflags += -L/usr/x86_64-w64-mingw32/lib -lz
	endif
endif


.PHONY: ai_worker_codegen livepeer livepeer_bench livepeer_cli livepeer_router docker swagger

# Git reference to download the OpenAPI spec from, defaults to `main` branch.
# It can also be a simple git commit hash. e.g. `make ai_worker_codegen REF=c19289d`
REF ?= refs/heads/main
ai_worker_codegen:
	go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.2.0 \
		-package worker \
		-generate types,client,chi-server,spec \
		https://raw.githubusercontent.com/livepeer/ai-worker/$(REF)/runner/openapi.yaml \
		| awk '!/WARNING/' > ai/worker/runner.gen.go

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

docker_mtx:
	docker buildx build -f docker/Dockerfile.mediamtx docker/

swagger:
	swag init --generalInfo server/ai_mediaserver.go --outputTypes yaml --output . && mv swagger.yaml liveai.openapi.yaml

# Command to run Livepeer Realtime AI Video in a Box
.PHONY: box
box: box-rebuild
	./box/box.sh

.PHONY: box-rebuild
box-rebuild:
ifeq ($(strip ${REBUILD}),false)
	@echo "Skipping rebuild of components"
else
	@$(MAKE) box-runner
ifeq ($(strip ${DOCKER}),true)
	docker build -t livepeer/go-livepeer -f docker/Dockerfile .
else
	@$(MAKE) livepeer
endif
endif

.PHONY: box-gateway
box-gateway: box-rebuild
	./box/gateway.sh

.PHONY: box-orchestrator
box-orchestrator: box-rebuild
	./box/orchestrator.sh

.PHONY: box-mediamtx
box-mediamtx:
	./box/mediamtx.sh

.PHONY: box-runner
box-runner:
	./box/build-runner.sh

.PHONY: box-stream
box-stream:
	./box/stream.sh start

.PHONY: box-playback
box-playback:
	./box/stream.sh playback

.PHONY: box-supabase
box-supabase:
	./box/supabase.sh

.PHONY: box-frontend
box-frontend:
	./box/frontend.sh
