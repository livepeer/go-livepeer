#!/bin/bash

set -e

go get -u -v contrib.go.opencensus.io/exporter/prometheus
go get -u -v github.com/ericxtang/m3u8
go get -u -v go.opencensus.io/stats
go get -u -v go.opencensus.io/tag
go get -u google.golang.org/grpc
go get github.com/alecthomas/gometalinter && gometalinter --install
go get github.com/aws/aws-sdk-go/aws
go get github.com/golang/glog
go get github.com/jstemmer/go-junit-report
go get github.com/pkg/errors
go get github.com/stretchr/testify
go get github.com/stretchr/testify/mock
