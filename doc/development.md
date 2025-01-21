# Development

## Testing

Some tests depend on access to the JSON-RPC API of an Ethereum node connected to mainnet or Rinkeby.

-   To run mainnet tests, the `MAINNET_ETH_URL` environment variable should be set. If the variable is not set, the mainnet tests will be skipped.
-   To run Rinkeby tests, the `RINKEBY_ETH_URL` environment variable should be set. If the variable is not set, the Rinkeby tests will b eskipped

To run tests:

```bash
bash test.sh
```

## Debugging

To debug the code, it is recommended to use [Visual Studio Code](https://code.visualstudio.com/) with the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go). Example VSCode configuration files are provided below. For more information on how to interact with the [go-livepeer](https://github.com/livepeer/go-livepeer) software, please check out the [Livepeer Docs](https://docs.livepeer.org/orchestrators/guides/get-started). Please ensure that you followed the steps in the [Build from Source documentation](https://docs.livepeer.org/orchestrators/guides/install-go-livepeer#build-from-source) and have the right dependencies and environment variables set in your shell configuration file (e.g., `.bashrc`, `.zshrc`).

### Configuration Files

<details>
<summary>Launch.json (transcoding)</summary>

<!-- prettier-ignore -->
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Run CLI",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer_cli",
      "console": "integratedTerminal",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        // "--http=7935", // Uncomment for Orch CLI.
        "--http=5935" // Uncomment for Gateway CLI.
      ]
    },
    {
      "name": "Launch O/T (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        "-orchestrator",
        "-transcoder",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all"
      ]
    },
    {
      "name": "Launch O (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        "-orchestrator",
        "-orchSecret=orchSecret",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6"
      ]
    },
    {
      "name": "Launch T (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        "-transcoder",
        "-orchSecret=orchSecret",
        "-orchAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all"
      ]
    },
    {
      "name": "Launch G (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        "-gateway",
        "-transcodingOptions=${env:HOME}/.lpData/offchain/transcodingOptions.json",
        "-orchAddr=0.0.0.0:8935",
        "-httpAddr=0.0.0.0:9935",
        "-v",
        "6"
      ]
    },
    {
      "name": "Launch O/T (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags=-extldflags=-lm", // Fix missing symbol error and enable mainnet.
      "args": [
        "-orchestrator",
        "-transcoder",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all",
        "-network=arbitrum-one-mainnet",
        "-ethUrl=https://arb1.arbitrum.io/rpc",
        "-ethPassword=<ETH_SECRET>",
        "-ethAcctAddr=<ETH_ACCT_ADDR>",
        "-ethOrchAddr=<ORCH_ADDR>",
        "-pricePerUnit=<PRICE_PER_UNIT>"
      ]
    },
    {
      "name": "Launch O (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags=-extldflags=-lm", // Fix missing symbol error and enable mainnet.
      "args": [
        "-orchestrator",
        "-orchSecret=orchSecret",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6",
        "-network=arbitrum-one-mainnet",
        "-ethUrl=https://arb1.arbitrum.io/rpc",
        "-ethPassword=<ETH_SECRET>",
        "-ethAcctAddr=<ETH_ACCT_ADDR>",
        "-ethOrchAddr=<ORCH_ADDR>",
        "-pricePerUnit=<PRICE_PER_UNIT>"
      ]
    },
    {
      "name": "Launch T (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags=-extldflags=-lm", // Fix missing symbol error and enable mainnet.
      "args": [
        "-transcoder",
        "-orchSecret=orchSecret",
        "-orchAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all"
      ]
    },
    {
      "name": "Launch G (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags=-extldflags=-lm", // Fix missing symbol error and enable mainnet.
      "args": [
        "-gateway",
        "-transcodingOptions=${env:HOME}/.lpData/offchain/transcodingOptions.json",
        "-orchAddr=0.0.0.0:8935",
        "-httpAddr=0.0.0.0:9935",
        "-v",
        "6",
        "-httpIngest",
        "-network=arbitrum-one-mainnet",
        "-ethUrl=https://arb1.arbitrum.io/rpc",
        "-ethPassword=<ETH_SECRET>",
        "-ethAcctAddr=<ETH_ACCT_ADDR>"
      ]
    }
  ],
  "compounds": [
    {
      "name": "Launch full stack (off-chain)",
      "configurations": ["Launch O/T (off-chain)", "Launch G (off-chain)"],
      "stopAll": true
    },
    {
      "name": "Launch full stack (on-chain)",
      "configurations": ["Launch O/T (on-chain)", "Launch G (on-chain)"],
      "stopAll": true
    }
  ]
}
```

</details>

<details>
<summary>Launch.json (AI)</summary>

<!-- prettier-ignore -->
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Run AI CLI",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer_cli",
      "console": "integratedTerminal",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        // "--http=7935", // Uncomment for Orch CLI.
        "--http=5935" // Uncomment for Gateway CLI.
      ]
    },
    {
      "name": "Launch AI O/W (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        "-orchestrator",
        "-aiWorker",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all",
        "-aiModels=${env:HOME}/.lpData/cfg/aiModels.json",
        "-aiModelsDir=${env:HOME}/.lpData/models"
      ]
    },
    {
      "name": "Launch AI O (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags='-extldflags=-lm -X github.com/livepeer/go-livepeer/core.LivepeerVersion=0.0.0'", // Fix missing symbol and version mismatch errors.
      "args": [
        "-orchestrator",
        "-orchSecret=orchSecret",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6"
      ]
    },
    {
      "name": "Launch AI W (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags='-extldflags=-lm -X github.com/livepeer/go-livepeer/core.LivepeerVersion=0.0.0'", // Fix missing symbol and version mismatch errors.
      "args": [
        "-aiWorker",
        "-orchSecret=orchSecret",
        "-orchAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all",
        "-aiModels=${env:HOME}/.lpData/cfg/aiModels.json",
        "-aiModelsDir=${env:HOME}/.lpData/models"
      ]
    },
    {
      "name": "Launch AI G (off-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-ldflags=-extldflags=-lm", // Fix missing symbol error.
      "args": [
        "-gateway",
        "-datadir=${env:HOME}/.lpData2",
        "-orchAddr=0.0.0.0:8935",
        "-httpAddr=0.0.0.0:9935",
        "-v",
        "6",
        "-httpIngest"
      ]
    },
    {
      "name": "Launch AI O/W (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags=-extldflags=-lm", // Fix missing symbol error and enable mainnet.
      "args": [
        "-orchestrator",
        "-aiWorker",
        "-aiServiceRegistry",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all",
        "-aiModels=${env:HOME}/.lpData/cfg/aiModels.json",
        "-aiModelsDir=${env:HOME}/.lpData/models",
        "-network=arbitrum-one-mainnet",
        "-ethUrl=https://arb1.arbitrum.io/rpc",
        "-ethPassword=<ETH_SECRET>",
        "-ethAcctAddr=<ETH_ACCT_ADDR>",
        "-ethOrchAddr=<ORCH_ADDR>"
      ]
    },
    {
      "name": "Launch AI O (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags='-extldflags=-lm -X github.com/livepeer/go-livepeer/core.LivepeerVersion=0.0.0'", // Fix missing symbol error, version mismatch error and enable mainnet.
      "args": [
        "-orchestrator",
        "-orchSecret=orchSecret",
        "-aiServiceRegistry",
        "-serviceAddr=0.0.0.0:8935",
        "-v=6",
        "-network=arbitrum-one-mainnet",
        "-ethUrl=https://arb1.arbitrum.io/rpc",
        "-ethPassword=<ETH_SECRET>",
        "-ethAcctAddr=<ETH_ACCT_ADDR>",
        "-ethOrchAddr=<ORCH_ADDR>",
        "-pricePerUnit=0"
      ]
    },
    {
      "name": "Launch AI W (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags='-extldflags=-lm -X github.com/livepeer/go-livepeer/core.LivepeerVersion=0.0.0'", // Fix missing symbol error, version mismatch error and enable mainnet.
      "args": [
        "-aiWorker",
        "-orchSecret=orchSecret",
        "-orchAddr=0.0.0.0:8935",
        "-v=6",
        "-nvidia=all",
        "-aiModels=${env:HOME}/.lpData/cfg/aiModels.json",
        "-aiModelsDir=${env:HOME}/.lpData/models"
      ]
    },
    {
      "name": "Launch AI G (on-chain)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "cmd/livepeer",
      "buildFlags": "-tags=mainnet,experimental -ldflags=-extldflags=-lm", // Fix missing symbol error and enable mainnet.
      "args": [
        "-gateway",
        "-aiServiceRegistry",
        "-datadir=${env:HOME}/.lpData2",
        "-orchAddr=0.0.0.0:8935",
        "-httpAddr=0.0.0.0:9935",
        "-v",
        "6",
        "-httpIngest",
        "-network=arbitrum-one-mainnet",
        "-ethUrl=https://arb1.arbitrum.io/rpc",
        "-ethPassword=<ETH_SECRET>",
        "-ethAcctAddr=<ETH_ACCT_ADDR>"
      ]
    }
  ],
  "compounds": [
    {
      "name": "Launch full AI stack (off-chain)",
      "configurations": [
        "Launch AI O/W (off-chain)",
        "Launch AI G (off-chain)"
      ],
      "stopAll": true
    },
    {
      "name": "Launch full AI stack (on-chain)",
      "configurations": ["Launch AI O/W (on-chain)", "Launch AI G (on-chain)"],
      "stopAll": true
    }
  ]
}
```

</details>
