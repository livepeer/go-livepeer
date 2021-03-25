# Ethereum

## Reward

The node can run a reward service that will automatically call a smart contract function to mint LPT rewards each round that the node's on-chain registered address is in the active set. Note that at the moment, only the on-chain registered address can call the smart contract function to mint LPT rewards.

If the node detects that its address is registered on-chain, it will automatically start the reward service. The reward service can also be explicitly disabled by starting the node with `-reward=false` and explicitly enabled by starting the node with `-reward`.

## Round Initialization

The node can run a round initialization service that will automatically call a smart contract function to initialize the current round.

The round initialization service is disabled by default and can be enabled by starting the node with `-initializeRound`.