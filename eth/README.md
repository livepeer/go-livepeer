Tests rely on a forked version of `go-ethereum` that has fast block times and Solidity library linking functionality added to the `abigen` tool found here: https://github.com/yondonfu/go-ethereum/tree/lpTest

```
git clone https://github.com/yondonfu/go-ethereum.git $GOPATH/src/github.com/ethereum/go-ethereum
cd $GOPATH/src/github.com/ethereum/go-ethereum
git checkout lpTest
go install ./...
```

# Running tests

Start geth

```
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth
bash init.sh
```

In a separate window

```
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth
go test -v -args -v 3 -logtostderr true
```

When tests are complete

```
bash cleanup.sh
```

# Generating Go bindings

The `contracts` folder contains generated Go bindings for the Livepeer protocol smart contracts.

If the smart contracts are updated you can generate new Go bindings by doing the following:

```
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth
git clone https://github.com/livepeer/protocol.git $GOPATH/src/github.com/livepeer/go-livepeer/eth/protocol
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth/protocol
truffle compile --all
node parseArtifacts.js
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth
go generate client.go
```
