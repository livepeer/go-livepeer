module github.com/livepeer/go-livepeer

go 1.13

require (
	cloud.google.com/go v0.38.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/BurntSushi/toml v0.3.1
	github.com/OneOfOne/xxhash v1.2.2
	github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc
	github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf
	github.com/allegro/bigcache v1.2.1
	github.com/apilayer/freegeoip v3.5.0+incompatible
	github.com/aristanetworks/goarista v0.0.0-20190909155222-05df9ecbb0dc
	github.com/aws/aws-sdk-go v1.23.19
	github.com/btcsuite/btcd v0.0.0-20190824003749-130ea5bddde3 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cespare/cp v1.1.1
	github.com/cespare/xxhash v1.1.0
	github.com/chzyer/logex v1.1.10
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/chzyer/test v0.0.0-20180213035817-a1ea475d72b1
	github.com/client9/misspell v0.3.4
	github.com/deckarep/golang-set v1.7.1
	github.com/dgryski/go-sip13 v0.0.0-20181026042036-e10d5fee7954
	github.com/docker/docker v1.13.1 // indirect
	github.com/edsrzf/mmap-go v1.0.0
	github.com/elastic/gosigar v0.10.5
	github.com/ethereum/go-ethereum v1.9.3
	github.com/fatih/color v1.7.0
	// replace example.com/some/dependency => example.com/some/dependency-fork v1.2.3
	github.com/fjl/memsize v0.0.0-20190710130421-bcb5799ab5e5
	// github.com/gballet/go-libpcsclite v0.0.0-20190607065134-2772fd86a8ff // indirect
	github.com/gballet/go-libpcsclite v0.0.0-20190403181518-312b5175032f
	github.com/go-logfmt/logfmt v0.4.0
	github.com/go-stack/stack v1.8.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6
	github.com/golang/protobuf v1.3.2
	github.com/golang/snappy v0.0.1
	github.com/google/btree v1.0.0
	github.com/google/gofuzz v1.0.0
	github.com/google/martian v2.1.0+incompatible
	github.com/google/uuid v1.0.0
	github.com/googleapis/gax-go v2.0.2+incompatible
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/gorilla/websocket v1.4.1
	github.com/hashicorp/golang-lru v0.5.3
	github.com/howeyc/fsnotify v0.9.0
	github.com/huin/goupnp v1.0.0
	github.com/ianlancetaylor/demangle v0.0.0-20181102032728-5e5cf60278f6
	github.com/influxdata/influxdb v1.7.8
	github.com/jackpal/go-nat-pmp v1.0.1
	github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af
	github.com/json-iterator/go v1.1.7
	github.com/julienschmidt/httprouter v1.2.0
	github.com/karalabe/hid v1.0.0 // indirect
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/kr/pretty v0.1.0 // indirect
	github.com/livepeer/joy4 v0.1.1
	github.com/livepeer/lpms v0.0.0-20191004153601-83352b59757e
	github.com/livepeer/m3u8 v0.11.0
	github.com/mattn/go-colorable v0.1.2
	github.com/mattn/go-isatty v0.0.8
	github.com/mattn/go-runewidth v0.0.3
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/mwitkow/go-conntrack v0.0.0-20161129095857-cc309e4a2223
	github.com/oklog/ulid v1.3.1
	github.com/olekukonko/tablewriter v0.0.1
	github.com/oschwald/maxminddb-golang v1.5.0
	github.com/pborman/uuid v1.2.0
	github.com/peterh/liner v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/procfs v0.0.3
	github.com/prometheus/tsdb v0.10.0
	github.com/rjeczalik/notify v0.9.2
	github.com/robertkrimen/otto v0.0.0-20180617131154-15f95af6e78d
	github.com/rs/cors v1.7.0
	github.com/sirupsen/logrus v1.2.0
	github.com/spaolacci/murmur3 v0.0.0-20180118202830-f09979ecbc72
	github.com/status-im/keycard-go v0.0.0-20190424133014-d95853db0f48
	github.com/steakknife/bloomfilter v0.0.0-20180922174646-6819c0d2a570
	github.com/steakknife/hamming v0.0.0-20180906055917-c99c65617cd3
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tyler-smith/go-bip39 v1.0.2
	github.com/urfave/cli v1.20.0
	github.com/wsddn/go-ecdh v0.0.0-20161211032359-48726bab9208
	go.opencensus.io v0.22.1
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/net v0.0.0-20190909003024-a7b16738d86b
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/text v0.3.2
	google.golang.org/api v0.10.0
	google.golang.org/appengine v1.5.0
	google.golang.org/grpc v1.23.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/fatih/set.v0 v0.2.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20190709231704-1e4459ed25ff // indirect
	gopkg.in/sourcemap.v1 v1.0.5
	gopkg.in/urfave/cli.v1 v1.0.0-00010101000000-000000000000 // indirect
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ethereum/go-ethereum => github.com/livepeer/go-ethereum v1.8.4-0.20190523183241-7e95cbcfcd82

replace gopkg.in/urfave/cli.v1 => github.com/urfave/cli v1.22.2-0.20191002033821-63cd2e3d6bb5
