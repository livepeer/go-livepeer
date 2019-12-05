module github.com/livepeer/go-livepeer

go 1.12

require (
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/aws/aws-sdk-go v1.25.48
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/ethereum/go-ethereum v1.9.9
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/livepeer/lpms v0.0.0-20191121223052-fdbf27c7cfbf
	github.com/livepeer/m3u8 v0.11.0
	github.com/mattn/go-sqlite3 v2.0.1+incompatible
	github.com/olekukonko/tablewriter v0.0.4
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.2.1
	github.com/stretchr/testify v1.4.0
	github.com/urfave/cli v1.22.2
	go.opencensus.io v0.22.2
	golang.org/x/net v0.0.0-20191206103017-1ddd1de85cb0
	google.golang.org/grpc v1.25.1
)
