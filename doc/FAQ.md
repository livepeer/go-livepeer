
# Livepeer FAQ

## General Questions

### What is Livepeer?
Livepeer is a decentralized video infrastructure network that enables developers to build scalable video applications on Ethereum and other EVM-compatible chains.

### How does Livepeer work?
Livepeer combines decentralized storage and computing to provide a scalable video infrastructure. It uses a network of independent nodes that process and deliver video streams.

## Technical Questions

### How do I estimate transaction costs in the CLI?
When working with Livepeer transactions, it's important to be aware of gas costs to avoid high-fee accidents. While the Livepeer CLI doesn't currently provide built-in cost estimation, you can use these strategies:

1. **Use Ethereum Gas Trackers**: Before broadcasting transactions, check current gas prices using tools like:
   - [Etherscan Gas Tracker](https://etherscan.io/gastracker)
   - [GasNow](https://www.gasnow.org/)
   - [EtherGasStation](https://etherscan.io/gasstation)

2. **Estimate Gas Manually**: For common transactions, you can estimate gas costs:
   - Standard transactions typically require 21,000 gas
   - Complex transactions may require 50,000-100,000 gas or more
   - Multiply gas units by current gas price to estimate fees

3. **Monitor Network Conditions**: During periods of high network activity, gas prices can increase significantly. Check gas prices before submitting transactions.

4. **Use Transaction Builders**: Tools like MetaMask or MyEtherWallet can help estimate transaction costs before submission.

### How can I check my node's status?
You can check your node's status using the CLI command:
```
livepeer status
```

This will provide information about your node's health, current tasks, and connection status.

## Troubleshooting

### My node is not connecting to the network
Please check your network configuration and ensure your node is properly configured in the Livepeer network. You can use:
```
livepeer config show
```
to verify your settings.

### How do I update my node?
To update your node, simply run:
```
livepeer update
```
This will check for updates and install them if available.
