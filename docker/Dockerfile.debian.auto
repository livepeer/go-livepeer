# This file used on Docker Hub to automatically create offical images
FROM livepeer/ffmpeg-base:latest as builder

FROM golang:1-stretch as builder2
ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /root
RUN apt update \
    && apt install -y \
    git gcc g++ gnutls-dev 
COPY --from=builder /root/compiled /root/compiled/

ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /go/src/github.com/livepeer/go-livepeer

RUN go get github.com/golang/glog
RUN go get -u -v github.com/livepeer/m3u8
RUN go get github.com/aws/aws-sdk-go/aws
RUN go get -u google.golang.org/grpc
RUN go get github.com/pkg/errors
RUN go get github.com/stretchr/testify/mock
RUN go get -u -v go.opencensus.io/stats
RUN go get -u -v go.opencensus.io/tag
RUN go get -u -v contrib.go.opencensus.io/exporter/prometheus

COPY . .
RUN git describe --always --long --dirty > .git.describe

RUN go build -ldflags="-X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(cat VERSION)-$(cat .git.describe)" -v cmd/livepeer/livepeer.go
RUN go build -ldflags="-X github.com/livepeer/go-livepeer/core.LivepeerVersion=$(cat VERSION)-$(cat .git.describe)" -v cmd/livepeer_cli/*

FROM debian:stretch-slim

WORKDIR /root
RUN apt update && apt install -y  ca-certificates jq libgnutls30 && apt clean
RUN mkdir -p /root/.lpData/mainnet/keystore && \
  mkdir -p /root/.lpData/rinkeby/keystore && \
  mkdir -p /root/.lpData/devenv/keystore && mkdir -p /root/.lpData/offchain/keystore
COPY --from=builder2 /go/src/github.com/livepeer/go-livepeer/livepeer /usr/bin/livepeer
COPY --from=builder2 /go/src/github.com/livepeer/go-livepeer/livepeer_cli /usr/bin/livepeer_cli

COPY docker/start.sh .
RUN chmod +x start.sh

EXPOSE 7935/tcp
EXPOSE 8935/tcp
EXPOSE 1935/tcp

ENTRYPOINT ["/root/start.sh"]
CMD ["--help"]

# Build Docker image: docker build -t livepeerbinary:edge -f docker/Dockerfile.debian.auto .
