# HTTP endpoint

The Livepeer node exposes an HTTP interface for monitoring and managing the node. This is how the `livepeer_cli` tool interfaces with a running node.
By default, the CLI listens to localhost:7935. This can be adjusted with the -cliAddr `<interface>:<port>` flag.

Routes that can submit Ethereum transactions, sign messages, or change transaction gas controls are not registered by default. Start the node with `-enableCliTxRoutes` to enable them. Keep the CLI listener restricted to loopback or another trusted network when enabling this flag.

Without `-enableCliTxRoutes`, the following routes return `404 Not Found`:

- Rounds and orchestrator registration: `/initializeRound`, `/activateOrchestrator`, `/setOrchestratorConfig`
- Bonding, withdrawals, and rewards: `/bond`, `/rebond`, `/unbond`, `/withdrawStake`, `/withdrawFees`, `/claimEarnings`, `/reward`
- Wallet and governance operations: `/transferTokens`, `/requestTokens`, `/signMessage`, `/vote`, `/voteOnProposal`
- Transaction gas controls: `/setMaxGasPrice`, `/setMinGasPrice`
- Ticket Broker transactions: `/fundDepositAndReserve`, `/fundDeposit`, `/unlock`, `/cancelUnlock`, `/withdraw`

The flag controls only the CLI HTTP routes. Automatic services such as reward processing, round initialization, and ticket redemption are unaffected. Other read-only and local-configuration CLI routes remain available.

## Available endpoints:



`/getLogLevel` returns current verbosity level in the body of response

`/setLogLevel` sets verbosity current level. Level to set should be provided in body of the request, encoded as `application/x-www-form-urlencoded`. Parameter should be named `loglevel`.
It can be used from command like this:

`curl -F loglevel=6 http://localhost:7935/setLogLevel`

Log level should be integer from 0 to 6, where 6 means most verbose logging.
