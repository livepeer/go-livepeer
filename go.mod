module github.com/livepeer/go-livepeer

go 1.13

require (
	cloud.google.com/go/storage v1.6.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/aws/aws-sdk-go v1.33.11
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cespare/cp v1.1.1 // indirect
	github.com/deckarep/golang-set v1.7.1 // indirect
	github.com/ethereum/go-ethereum v1.10.8
	github.com/fatih/color v1.12.0 // indirect
	github.com/golang/glog v0.0.0-20210429001901-424d2337a529
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.5.2
	github.com/jaypipes/ghw v0.7.0
	github.com/livepeer/livepeer-data v0.2.0
	github.com/livepeer/lpms v0.0.0-20211208221036-644122186d24
	github.com/livepeer/m3u8 v0.11.1
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/peterbourgon/ff/v3 v3.1.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/rjeczalik/notify v0.9.2 // indirect
	github.com/status-im/keycard-go v0.0.0-20190424133014-d95853db0f48 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/tyler-smith/go-bip39 v1.0.2 // indirect
	github.com/urfave/cli v1.20.0
	go.opencensus.io v0.22.3
	go.uber.org/goleak v1.0.0
	golang.org/x/net v0.0.0-20210805182204-aaa1db679c0d
	google.golang.org/api v0.29.0
	google.golang.org/grpc v1.28.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	pgregory.net/rapid v0.4.0
)

replace github.com/ethereum/go-ethereum => github.com/livepeer/go-ethereum v1.10.9-0.20210930013949-80e3965c7b18
