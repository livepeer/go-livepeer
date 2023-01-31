# Ethereum

## Reward

The node can run a reward service that will automatically call a smart contract function to mint LPT rewards each round that the node's on-chain registered address is in the active set. Note that at the moment, only the on-chain registered address can call the smart contract function to mint LPT rewards.

If the node detects that its address is registered on-chain, it will automatically start the reward service. The reward service can also be explicitly disabled by starting the node with `-reward=false` and explicitly enabled by starting the node with `-reward`.

## Round Initialization

The node can run a round initialization service that will automatically call a smart contract function to initialize the current round.

The round initialization service is disabled by default and can be enabled by starting the node with `-initializeRound`.

## Gas Prices

After the EIP-1559 upgrade on Ethereum, the node treats the gas price as priority fee + base fee.

### Max gas price

The `maxGasPrice` parameter makes sure the transaction fee never exceeds the specified limit.
- If the current network gas price is higher than `maxGasPrice`, the transaction is not sent
- The transaction parameter `maxFeePerGas` is set to `maxGasPrice`
	- **Note: As of v0.5.24, this is not true, but another release will be published to resolve this**

The following options can be used to get the max gas price:

- `curl localhost:7935/maxGasPrice`
- Run `livepeer_cli` and observe the max gas price in the node stats

The following options can be used to set the max gas price to `<MAX_GAS_PRICE>`, a Wei denominated value:

- Start the node with `-maxGasPrice <MAX_GAS_PRICE>`
- `curl localhost:7935/setMaxGasPrice?maxGasPrice=<MAX_GAS_PRICE>`
- Run `livepeer_cli` and select the set max gas price option

### Min gas price

The following options can be used to get the min gas price:

- `curl localhost:7935/minGasPrice`
- Run `livepeer_cli` and observe the min gas price in the node stats

The following options can be used to set the min gas price to `<MIN_GAS_PRICE>`, a Wei denominated value:

- Start the node with `-minGasPrice <MIN_GAS_PRICE>`
- `curl localhost:7935/setMinGasPrice?minGasPrice=<MIN_GAS_PRICE>`
- Run `livepeer_cli` and select the set min gas price option

### Known edge-cases
A known edge-case that affects the initialization of new rounds and the ticket redemption occurs when the L2 block-rate is significantly slower than the L1 block-rate.
This may result in:
- go-livepeer not attempting to initialize a new round based on the expected `roundLength` interval - which is based on L1 block values.
- go-livepeer attempting to redeem a winning ticket after its on-chain validity period already expired due to the block-rate difference between L1 and L2, which may result in the [`ticket is expired` error](https://github.com/livepeer/protocol/blob/confluence/contracts/pm/mixins/MixinTicketProcessor.sol#L64).
- go-livepeer attempting to redeem a winning ticket after its [off-chain validity period](https://github.com/livepeer/go-livepeer/blob/3230eb1ac29fd86f88f1e6f768ff6bfbeef95572/pm/recipient.go#L24) already expired due to the block-rate difference between L1 and L2, which may result in the [`TicketParams expired` error](https://github.com/livepeer/go-livepeer/blob/master/pm/recipient.go#L17).
