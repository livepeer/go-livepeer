# devtool

An on-chain workflow testing tool that supports the following:

- Automatically submitting the necessary setup transactions for each node type
- Generating a Bash script with default CLI flags to start each node type

## Prerequisites

## Step 1: Set up a private ETH network with Livepeer protocol deployed

```
docker pull livepeer/geth-with-livepeer-protocol:streamflow
docker run -p 8545:8545 -p 8546:8546 --name geth-with-livepeer-protocol livepeer/geth-with-livepeer-protocol:streamflow

# Mac M1 ONLY
# docker pull darkdragon/geth-with-livepeer-protocol:streamflow
# docker run -p 8545:8545 -p 8546:8546 --name geth-with-livepeer-protocol darkdragon/geth-with-livepeer-protocol:streamflow

```


## Step 2: Set up a broadcaster

`go run cmd/devtool/devtool.go setup broadcaster`

This command will submit the setup transactions for a broadcaster and generate the Bash script
`run_broadcaster_<ETH_ACCOUNT>.sh` which can be used to start a broadcaster node.

## Step 3: Set up a orchestrator/transcoder

`go run cmd/devtool/devtool.go setup transcoder`

This command will submit the setup transactions for an orchestrator/transcoder and generate the Bash scripts 
`run_orchestrator_<ETH_ACCOUNT>.sh` which can be used to start an orchestrator node and `run_transcoder_<ETH_ACCOUNT>.sh` which can be used to start a transcoder node.
