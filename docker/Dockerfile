FROM golang:1-stretch

ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /go/src/github.com/livepeer/go-livepeer

RUN curl -sL https://deb.nodesource.com/setup_8.x | bash -
RUN apt-get update && apt-get -y install build-essential pkg-config autoconf nodejs gnutls-dev
RUN go get github.com/golang/glog
RUN go get -u -v github.com/ericxtang/m3u8
RUN go get github.com/aws/aws-sdk-go/aws
RUN go get -u google.golang.org/grpc
RUN go get github.com/pkg/errors
RUN go get github.com/stretchr/testify/mock
RUN go get -u -v go.opencensus.io/stats
RUN go get -u -v go.opencensus.io/tag
RUN go get -u -v go.opencensus.io/exporter/prometheus

COPY install_ffmpeg.sh install_ffmpeg.sh
RUN ./install_ffmpeg.sh

COPY . .

CMD ["/bin/bash", "/go/src/github.com/livepeer/go-livepeer/test.sh"]
