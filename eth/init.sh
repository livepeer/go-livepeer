geth --datadir ~/.lpTest init genesis.json
cp $GOPATH/src/github.com/livepeer/libp2p-livepeer/eth/keys/* ~/.lpTest/keystore
geth --datadir ~/.lpTest --networkid 777 --nodiscover --maxpeers 0 --rpc --mine --minerthreads 1 --etherbase "0x0161e041aad467a890839d5b08b138c1e6373072"
