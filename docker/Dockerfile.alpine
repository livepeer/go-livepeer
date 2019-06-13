# Docker file for building livepeer binary on alpine linux
# this creates image of 63M in size
FROM alpine:latest as builder

ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /root

RUN apk add --no-cache \
        libstdc++ \
    && apk add --no-cache \
        binutils-gold curl g++ gcc gnupg \
        libgcc linux-headers make git python bash autoconf gnutls-dev coreutils
COPY install_ffmpeg.sh install_ffmpeg.sh
RUN ./install_ffmpeg.sh


FROM golang:alpine as builder2
ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /root
RUN apk add --no-cache \
    git gcc g++ gnutls-dev linux-headers
COPY --from=builder /root/compiled /root/compiled/

ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /go/src/github.com/livepeer/go-livepeer

RUN go get github.com/golang/glog
RUN go get github.com/livepeer/m3u8
RUN go get github.com/aws/aws-sdk-go/aws
RUN go get -u google.golang.org/grpc
RUN go get github.com/pkg/errors
RUN go get github.com/stretchr/testify/mock

COPY . .
RUN go build cmd/livepeer/livepeer.go

FROM alpine:latest

WORKDIR /root
RUN apk add --no-cache gnutls ca-certificates
RUN mkdir -p /root/lpDev/mainnet
RUN mkdir -p /root/.lpData/mainnet/keystore && \
  mkdir -p /root/.lpData/rinkeby/keystore && \
  mkdir -p /root/.lpData/devenv/keystore && mkdir -p /root/.lpData/offchain/keystore
COPY --from=builder2 /go/src/github.com/livepeer/go-livepeer/livepeer /usr/bin/livepeer

# Build Docker image: docker build -t livepeerbinary:alpine -f Dockerfile.alpine .
