package build

// SupportedChains is an enum that indicates the chains supported by the node
// All chains represented by an enum value less than or equal than this value are supported
// All chains represented by an enum value greater than this value are not supported
type SupportedChains int

const (
	// Dev is a development chain
	Dev SupportedChains = iota
	// Rinkeby is the Ethereum Rinkeby or Arbitrum Testnet test network chain
	Rinkeby
	// Mainnet is the Ethereum or Arbitrum main network chain
	Mainnet
)

// ChainSupported returns whether the node can connect to the chain with the given ID
func ChainSupported(chainID int64) bool {
	switch chainID {
	case 4, 421611:
		return Rinkeby <= HighestChain
	case 1, 42161:
		return Mainnet <= HighestChain
	default:
		return Dev <= HighestChain
	}
}
