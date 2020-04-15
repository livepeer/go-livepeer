# Development

## Testing

Some tests depend on access to the JSON-RPC API of an Ethereum node connected to mainnet or Rinkeby.

- To run mainnet tests, the `MAINNET_ETH_URL` environment variable should be set. If the variable is not set, the mainnet tests will be skipped.
- To run Rinkeby tests, the `RINKEBY_ETH_URL` environment variable should be set. If the variable is not set, the Rinkeby tests will b eskipped

To run tests:

```
bash test.sh
```