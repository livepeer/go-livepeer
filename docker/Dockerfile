FROM --platform=$BUILDPLATFORM ubuntu:20.04 as build

ARG TARGETARCH
ENV GOARCH=$TARGETARCH

ENV PATH "/usr/local/go/bin:/go/bin:${PATH}"
ENV PKG_CONFIG_PATH "/root/compiled/lib/pkgconfig"
ENV CPATH /usr/local/cuda/include
ENV LIBRARY_PATH /usr/local/cuda/lib64

RUN apt-get update \
  && apt-get install -y software-properties-common curl apt-transport-https \
  && curl https://dl.google.com/go/go1.17.6.linux-amd64.tar.gz | tar -C /usr/local -xz \
  && curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add - \
  && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs)  stable" \
  && apt-get update \
  && apt-get -y install clang-10 clang-tools-10 lld-10 build-essential pkg-config autoconf sudo git python docker-ce-cli pciutils gcc-multilib libgcc-8-dev-arm64-cross

RUN update-alternatives --install /usr/bin/clang++ clang++ /usr/bin/clang++-10 30 \
  && update-alternatives --install /usr/bin/clang clang /usr/bin/clang-10 30 \
  && update-alternatives --install /usr/bin/ld ld /usr/bin/lld-10 30

RUN GRPC_HEALTH_PROBE_VERSION=v0.3.6 && \
    curl -L https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 -o /usr/bin/grpc_health_probe && \
    chmod +x /usr/bin/grpc_health_probe

ENV GOPATH /go
RUN mkdir -p /go
WORKDIR /build

RUN curl -LO https://github.com/livepeer/livepeer-ml/releases/download/v0.3/tasmodel.pb

ARG BUILD_TAGS
ENV BUILD_TAGS ${BUILD_TAGS}

COPY ./install_ffmpeg.sh ./install_ffmpeg.sh
RUN ./install_ffmpeg.sh

RUN GO111MODULE=on go get -v github.com/golangci/golangci-lint/cmd/golangci-lint@v1.25.0
RUN go get -v github.com/jstemmer/go-junit-report

ENV GOFLAGS "-mod=readonly"

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .
RUN make livepeer livepeer_cli livepeer_bench livepeer_router

FROM --platform=$TARGETPLATFORM nvidia/cuda:11.6.2-cudnn8-runtime-ubuntu20.04

ENTRYPOINT ["/usr/bin/livepeer"]

# this is needed to access GPU inside Docker Swarm
ENV NVIDIA_DRIVER_CAPABILITIES=all

COPY --from=build /build/livepeer /usr/bin/livepeer
COPY --from=build /build/livepeer_cli /usr/bin/livepeer_cli
COPY --from=build /build/livepeer_bench /usr/bin/livepeer_bench
COPY --from=build /build/livepeer_router /usr/bin/livepeer_router
COPY --from=build /usr/bin/grpc_health_probe /usr/bin/grpc_health_probe

COPY --from=build /build/tasmodel.pb /tasmodel.pb

COPY --from=build /usr/share/misc/pci.ids /usr/share/misc/pci.ids

# Configure linker
RUN ldconfig
