# Multi Orchestrator Setup

This document describes the various ways an operator could run multiple Orchestrator nodes. Each Orchestrator node will still have its own keypair and will still accept payments on behalf of the on-chain registered Ethereum address. This setup allows an operator to separate node operations; e.g. one node responsible for redeeming winning tickets, one node for calling `reward` and multiple nodes solely responsible for handling transcode requests. 

It is also possible to connect multiple Transcoder nodes to an Orchestrator node. The documentation about scaling transcoding by splitting Orchestrator and Transcoder nodes can be found [here](https://livepeer.org/docs/video-miners/how-to-guides/o-t-split).

## Unlocking Account and Connecting to Ethereum

The Livepeer node requires you to unlock your Ethereum account and provide connection information for an Ethereum JSON-RPC provider in order to interact with the [Livepeer Protocol](https://github.com/livepeer/protocol), unless the node is in [standalone transcoder](https://livepeer.org/docs/video-miners/how-to-guides/o-t-split) mode.

For brevity we'll exclude these flags in the examples and replace them with `<...ETH SETUP...>`. If no Ethereum account is available the Livepeer node will create one for you.

More detailed instructions for connecting to Ethereum can be found [here](https://livepeer.org/docs/installation/connect-to-ethereum). 

```
livepeer \
    -network <mainnet|rinkeby> \
    -ethUrl <JSON_RPC_PROVIDER> \
    -ethKeystorePath <PATH_TO_KEYSTORE_DIR> \
    -ethAcctAddr <ETHEREUM_ADDRESS> \
    -ethPassword <ETHEREUM_ACCOUNT_PASSWORD (string|path)> \
    -ethController <CONTROLLER_CONTRACT_ADDRESS (required if '-network' not provided)>
```

## Single Orchestrator with Redeemer

This setup allows an operator to use a separate Ethereum account for redeeming winning tickets on-chain and paying for that transaction while the ticket recipient is still the operator's on-chain registered Ethereum address.

The Orchestrator node will still be responsible for calling `reward`*.

\* _currently, only the on-chain registered address can call `reward`_

1. Start the Redeemer 

```shell
livepeer \
    <...ETH SETUP...> \
    -redeemer \ 
    -ethOrchAddr <ORCHESTRATOR_ON_CHAIN_ETH_ADDR (also the recipient address)>
```

2. Start the Orchestrator node


```shell
livepeer \
    <...ETH SETUP...> \
    -orchestrator -transcoder \
    -redeemerAddr <REDEEMER_HTTP_ADDR> \
    -pricePerUnit <PRICE (wei/pixel if '-pixelsPerUnit' is not set)>
```


## Blockchain Service Node with (Multiple) Orchestrator node(s)

In this setup a node started with the keys for the on-chain registered address will be responsible for all transactions. 

In order to use multiple Orchestrator nodes with this setup, the cluster of Orchestrator nodes responsible for transcoding would have to be behind a load balancer (e.g. DNS load balancing). The URI of the load balancer will be the on-chain registered Service URI.

1. Start the Blockchain Service Node 

```shell
livepeer \
    <...ETH SETUP...> \
    -redeemer \ 
    -httpAddr <REDEEMER_HTTP_ADDR (host):port> \
```

2. Start an Orchestrator node
```shell
livepeer \
    <...ETH SETUP...> \
    -orchestrator -transcoder \
    -ethOrchAddr <ORCHESTRATOR_ON_CHAIN_ETH_ADDR> \
    -redeemerAddr <REDEEMER_HTTP_ADDR> \
    -pricePerUnit <PRICE (wei/pixel if '-pixelsPerUnit' is not set)>
```

## Redeemer + RewardService + Multiple Orchestrator Nodes

This is the "coldest" setup possible for on-chain registered addresses. The keys for the on-chain registered address will be responsible for calling reward.

To use this setup with multiple Orchestrator nodes a load balancer is required as described above. 

1. Start the Redeemer

```shell
livepeer \
    <...ETH SETUP...> \
    -redeemer \ 
    -httpAddr <REDEEMER_HTTP_ADDR (host):port> \ 
    -ethOrchAddr <ORCHESTRATOR_ON_CHAIN_ETH_ADDR>
```

2. Start the RewardService

The address specified via `-ethAcctAddr` should be the on-chain registered address. 

```shell
livepeer \
    <...ETH SETUP...> \
```

3. Start an Orchestrator  node

The main difference is to use the `-ethOrchAddr` flag and specify the on-chain registered Ethereum address, in this case that of the RewardService node. 

```shell
livepeer \
    <...ETH SETUP...> \
    -orchestrator -transcoder \
    -ethOrchAddr <ORCHESTRATOR_ON_CHAIN_ETH_ADDR (also the recipient address)> \
    -redeemerAddr <REDEEMER_HTTP_ADDR> \
    -pricePerUnit <PRICE (wei/pixel if '-pixelsPerUnit' is not set)>
```
