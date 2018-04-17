[![Build Status](https://circleci.com/gh/livepeer/go-livepeer-basicnet.svg?style=shield&circle-token=e33534f6f4e2a6af19bb1596d7b72767a246cbab)](https://circleci.com/gh/livepeer/go-livepeer-basicnet/tree/master)

# go-livepeer-basicnet
Implementation of the Livepeer VideoNetwork interface.  Built with libp2p.

## Installation
gx and gx-go are required.

```
go get -u github.com/whyrusleeping/gx
go get -u github.com/whyrusleeping/gx-go
gx-go get github.com/livepeer/go-livepeer-basicnet
```

## Testing
To run a specific test: `go test -run Reconnect` to run TestReconnect from the
`basic_network_test.go` file.

To run all the tests in a specific file: `make network_node_test` to run
`network_node_test.go`

To run all tests: `go test`
