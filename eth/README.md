Tests rely on a forked version of `go-ethereum` that has fast block times and Solidity library linking functionality added to the `abigen` tool found here: https://github.com/yondonfu/go-ethereum/tree/lpTest

```
git clone https://github.com/yondonfu/go-ethereum.git $GOPATH/src/github.com/ethereum/go-ethereum
cd $GOPATH/src/github.com/ethereum/go-ethereum
git checkout lpTest
go install ./...
```

# Running tests

```
cd $GOPATH/src/github.com/livepeer/libp2p-livepeer/eth
bash init.sh
go test -v
bash cleanup.sh
```
