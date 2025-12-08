FROM --platform=$BUILDPLATFORM livepeerci/cuda:12.0.0-cudnn8-devel-ubuntu20.04 as build

ARG	TARGETARCH
ARG	BUILDARCH

ENV	GOARCH="$TARGETARCH" \
	PATH="/usr/local/go/bin:/go/bin:${PATH}" \
	PKG_CONFIG_PATH="/root/compiled/lib/pkgconfig" \
	CPATH="/usr/local/cuda_${TARGETARCH}/include" \
	LIBRARY_PATH="/usr/local/cuda_${TARGETARCH}/lib64" \
	DEBIAN_FRONTEND="noninteractive" \
	CGO_LDFLAGS="-L/usr/local/cuda_${TARGETARCH}/lib64"

RUN	apt update \
	&& apt install -yqq software-properties-common curl apt-transport-https lsb-release nasm \
	&& curl -fsSL https://dl.google.com/go/go1.21.5.linux-${BUILDARCH}.tar.gz | tar -C /usr/local -xz \
	&& curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add - \
	&& add-apt-repository "deb [arch=${BUILDARCH}] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
	&& curl -fsSl https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - \
	&& add-apt-repository "deb [arch=${BUILDARCH}] https://apt.llvm.org/$(lsb_release -cs)/ llvm-toolchain-$(lsb_release -cs)-14 main" \
	&& apt update \
	&& apt -yqq install clang-14 clang-tools-14 lld-14 build-essential pkg-config autoconf git python docker-ce-cli pciutils gcc-multilib libgcc-8-dev-arm64-cross gcc-mingw-w64-x86-64 zlib1g zlib1g-dev libx264-dev libfdk-aac-dev

RUN	update-alternatives --install /usr/bin/clang++ clang++ /usr/bin/clang++-14 30 \
	&& update-alternatives --install /usr/bin/clang clang /usr/bin/clang-14 30 \
	&& update-alternatives --install /usr/bin/ld ld /usr/bin/lld-14 30

RUN	GRPC_HEALTH_PROBE_VERSION=v0.3.6 \
	&& curl -fsSL https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-${TARGETARCH} -o /usr/bin/grpc_health_probe \
	&& chmod +x /usr/bin/grpc_health_probe \
	&& ldconfig /usr/local/lib

RUN FFMPEG_SHA=894da5ca7d742e4429ffb2af534fcda0103ef593 git clone --depth 1 https://git.ffmpeg.org/ffmpeg.git /ffmpeg \
	&& cd /ffmpeg && git fetch --depth 1 origin ${FFMPEG_SHA} \
	&& git checkout ${FFMPEG_SHA} \
	&& ./configure --enable-gpl --enable-libx264 --enable-libfdk-aac --enable-nonfree --prefix=build && make -j"$(nproc)" && make install

ENV	GOPATH=/go \
	GO_BUILD_DIR=/build/ \
	GOFLAGS="-mod=readonly"

WORKDIR	/src

RUN	mkdir -p /go \
	&& curl -fsSLO https://github.com/livepeer/livepeer-ml/releases/download/v0.3/tasmodel.pb

COPY	./install_ffmpeg.sh	./install_ffmpeg.sh

ARG	BUILD_TAGS
ENV	BUILD_TAGS=${BUILD_TAGS}

COPY	go.mod	go.sum	./
RUN	go mod download

RUN	./install_ffmpeg.sh \
	&& GO111MODULE=on go get -v github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.2 \
	&& go get -v github.com/jstemmer/go-junit-report

COPY	.	.

RUN	make livepeer livepeer_cli livepeer_bench livepeer_router

FROM --platform=$TARGETPLATFORM	nvidia/cuda:12.0.0-cudnn8-runtime-ubuntu20.04 AS livepeer-amd64-base

FROM --platform=$TARGETPLATFORM	nvidia/cuda:12.0.0-cudnn8-runtime-ubuntu20.04 AS livepeer-arm64-base

FROM	livepeer-${TARGETARCH}-base

ENV	NVIDIA_DRIVER_CAPABILITIES=all

RUN	apt update && apt install -y libx264-155 libfdk-aac1

COPY --from=build	/build/	/usr/local/bin/
COPY --from=build	/usr/bin/grpc_health_probe	/usr/local/bin/grpc_health_probe
COPY --from=build	/src/tasmodel.pb	/tasmodel.pb
COPY --from=build	/usr/share/misc/pci.ids	/usr/share/misc/pci.ids
COPY --from=build /ffmpeg/build/ /usr/local
RUN ldconfig /usr/local/lib

ENTRYPOINT	["/usr/local/bin/livepeer"]
