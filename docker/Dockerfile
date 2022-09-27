FROM --platform=$BUILDPLATFORM	ubuntu:18.04	as	build

ARG	TARGETARCH
ARG GOOS

ENV	GOARCH="$TARGETARCH" \
	GOOS="$GOOS" \
	PATH="/usr/local/go/bin:/go/bin:${PATH}" \
	PKG_CONFIG_PATH="/root/compiled/lib/pkgconfig" \
	CPATH="/usr/local/cuda/include" \
	LIBRARY_PATH="/usr/local/cuda/lib64"

RUN	apt update \
	&& apt install -yqq software-properties-common curl apt-transport-https \
	&& curl https://dl.google.com/go/go1.17.6.linux-amd64.tar.gz | tar -C /usr/local -xz \
	&& curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add - \
	&& add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
	&& curl -fsSl https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - \
	&& add-apt-repository "deb [arch=amd64] https://apt.llvm.org/xenial/ llvm-toolchain-xenial-12 main" \
	&& apt update \
	&& apt -yqq install clang-12 clang-tools-12 lld-12 build-essential pkg-config autoconf git python docker-ce-cli pciutils gcc-multilib libgcc-8-dev-arm64-cross gcc-mingw-w64-x86-64

RUN	update-alternatives --install /usr/bin/clang++ clang++ /usr/bin/clang++-12 30 \
	&& update-alternatives --install /usr/bin/clang clang /usr/bin/clang-12 30 \
	&& update-alternatives --install /usr/bin/ld ld /usr/bin/lld-12 30

RUN	GRPC_HEALTH_PROBE_VERSION=v0.3.6 \
	&& curl -L https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 -o /usr/bin/grpc_health_probe \
	&& chmod +x /usr/bin/grpc_health_probe \
	&& curl -fsSL https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-gpu-linux-x86_64-2.8.0.tar.gz | tar -C /usr/local -xzf - \
	&& ldconfig /usr/local/lib

ENV	GOPATH=/go \
	GO_BUILD_DIR=/build/ \
	GOFLAGS="-mod=readonly"

WORKDIR	/src

RUN	mkdir -p /go \
	&& curl -LO https://github.com/livepeer/livepeer-ml/releases/download/v0.3/tasmodel.pb

COPY	./install_ffmpeg.sh	./install_ffmpeg.sh

RUN	./install_ffmpeg.sh \
	&& GO111MODULE=on go get -v github.com/golangci/golangci-lint/cmd/golangci-lint@v1.25.0 \
	&& go get -v github.com/jstemmer/go-junit-report

COPY	go.mod	go.sum	./
RUN	go mod download
COPY	.	.

ARG	BUILD_TAGS
ENV	BUILD_TAGS=${BUILD_TAGS}

RUN	make livepeer livepeer_cli livepeer_bench livepeer_router

FROM --platform=$TARGETPLATFORM	nvidia/cuda:10.1-cudnn7-runtime AS livepeer-amd64-base

FROM --platform=$TARGETPLATFORM nvidia/cuda:11.6.2-cudnn8-runtime-ubuntu20.04 AS livepeer-arm64-base

FROM livepeer-${TARGETARCH}-base

ENV	NVIDIA_DRIVER_CAPABILITIES=all

COPY --from=build	/build/	/usr/local/bin/
COPY --from=build	/usr/bin/grpc_health_probe	/usr/local/bin/grpc_health_probe
COPY --from=build	/src/tasmodel.pb	/tasmodel.pb
COPY --from=build	/usr/share/misc/pci.ids	/usr/share/misc/pci.ids

RUN	ldconfig

ENTRYPOINT	["/usr/bin/livepeer"]
