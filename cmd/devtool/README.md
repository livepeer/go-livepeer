# devtool

An on-chain workflow testing tool that supports the following:

- Automatically submitting the necessary setup transactions for each node type
- Generating a Bash script with default CLI flags to start each node type

This tool requires access to an ETH node connected to private network.

## Setting up a private ETH network

```
docker pull livepeer/geth-with-livepeer-protocol:pm
docker run -p 8545:8545 -p 8546:8546 --name geth-with-livepeer-protocol livepeer/geth-with-livepeer-protocol:pm
```

## Setting up a broadcaster

`go run cmd/devtool/devtool.go setup broadcaster`

This command will submit the setup transactions for a broadcaster and generate the Bash script
`run_broadcaster_<ETH_ACCOUNT>.sh` which can be used to start a broadcaster node.

## Setting up a orchestrator/transcoder

`go run cmd/devtool/devtool.go setup transcoder`

This command will submit the setup transactions for an orchestrator/transcoder and generate the Bash scripts 
`run_orchestrator_<ETH_ACCOUNT>.sh` which can be used to start an orchestrator node and `run_transcoder_<ETH_ACCOUNT>.sh` which can be used to start a transcoder node.
