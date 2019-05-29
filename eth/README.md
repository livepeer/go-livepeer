# Generating Go bindings for contracts

The `contracts` folder contains Go bindings for the Livepeer protocol smart contracts generated using the 
[abigen](https://github.com/ethereum/go-ethereum/tree/master/cmd/abigen) tool.

If the smart contracts are updated you can generate new Go bindings by doing the following:

```
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth
git clone https://github.com/livepeer/protocol.git $GOPATH/src/github.com/livepeer/go-livepeer/eth/protocol
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth/protocol
npm install
npm run compile
node scripts/parseArtifacts.js
cd $GOPATH/src/github.com/livepeer/go-livepeer/eth
go generate client.go
```
