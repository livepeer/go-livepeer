geth --datadir ~/.lpTest init ../genesis.json
geth --datadir ~/.lpTest --networkid 777 --nodiscover --maxpeers 0 --rpc
geth --datadir ~/.lpTest --exec 'loadScript("startMiner.js")' attach ~/.lpTest/geth.ipc
