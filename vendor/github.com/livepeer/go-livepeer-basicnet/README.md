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

## Updating libp2p packages
For now this is an interative and error-prone process.
```
# Turn gx hashes into github.com import paths
gx-go uw
# Update package.json as needed (usually can just copy from go-ipfs)
gx-go rw
# Remove gx directory in GOPATH/src
rm -rf $GOPATH/src/gx # maybe back this up first!
gx install
```

Follow instructions on screen. If it fails due to a dependency or IPFS error:
ignore, remove gx-workspace-update.json, and retry the command. Repeat until
`git diff` looks somewhat correct. In particular, the package.json should be
updated and the gx hashes replaced by Github paths.

Run `gx-go rewrite` to rewrite the gx hashes and then check with `go test`.

If compilation fails, and the errors seem to be clustered around version
conflicts of a particular package, you might have to `gx workspace-update` that
package manually to knock it down to a compatible version.

Good luck, have fun.
