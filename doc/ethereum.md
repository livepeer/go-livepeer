# Ethereum

## Connecting to an Ethereum network

When connecting to an Ethereum network, an Ethereum RPC provider needs to be specified via the `-ethUrl` flag.

- To connect to mainnet, the node should be started with `-network mainnet` and the URL for `-ethUrl` should be for a mainnet Ethereum node.
- To connect to Rinkeby, the node should be started with `-network rinkeby` and the URL for `-ethUrl` should be for a Rinkeby Ethereum node.
- To connect to a private network, the node should be started with `-network <NETWORK_NAME>` (`<NETWORK_NAME>` is the name of the private network), the URL for `-ethUrl` should be for a private network Ethereum node and value for `-ethController` should be the address of the Controller contract deployed on the private network.
- To connect to an off-chain network, the node should be started without the `-network` flag (the default value is `offchain`). The `-ethUrl` and the `-ethController` flags are unnecessary.

See [this guide](https://livepeer.readthedocs.io/en/latest/quickstart.html#connecting-to-an-ethereum-node) for instructions on obtaining a URL that be used with the `-ethUrl` flag.

## Reward

The node can run a reward service that will automatically call a smart contract function to mint LPT rewards each round that the node's on-chain registered address is in the active set. Note that at the moment, only the on-chain registered address can call the smart contract function to mint LPT rewards.

If the node detects that its address is registered on-chain, it will automatically start the reward service. The reward service can also be explicitly disabled by starting the node with `-reward=false` and explicitly enabled by starting the node with `-reward`.

## Round Initialization

The node can run a round initialization service that will automatically call a smart contract function to initialize the current round.

The round initialization service is disabled by default and can be enabled by starting the node with `-initializeRound`.