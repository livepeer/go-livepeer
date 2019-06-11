FROM nvidia/cuda:10.1-base

ENTRYPOINT ["/usr/bin/livepeer"]

COPY --from=livepeerci/build:latest /go/src/github.com/livepeer/go-livepeer/livepeer /usr/bin/livepeer
COPY --from=livepeerci/build:latest /go/src/github.com/livepeer/go-livepeer/livepeer_cli /usr/bin/livepeer_cli
