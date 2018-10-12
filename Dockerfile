FROM golang:1.10.3-stretch

ENV PKG_CONFIG_PATH /root/compiled/lib/pkgconfig
WORKDIR /go/src/github.com/livepeer/go-livepeer
COPY . .

RUN curl -sL https://deb.nodesource.com/setup_8.x | bash -
RUN apt-get update && apt-get -y install build-essential pkg-config autoconf nodejs gnutls-dev
RUN ./install_ffmpeg.sh
RUN go get github.com/golang/glog
RUN go get github.com/ericxtang/m3u8
RUN go get -u google.golang.org/grpc

CMD ["/bin/bash", "/go/src/github.com/livepeer/go-livepeer/test.sh"]
