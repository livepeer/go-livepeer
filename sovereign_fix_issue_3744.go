# Sovereign Bounty Fix — Issue #3744
# Repo: livepeer/go-livepeer

# Sovereign Fix: Arbitrum Gas Spikes Cause PPP to Exceed Limits and Drop Sessions
package pm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// txCost calculates the estimated cost of a transaction based on a gas limit
// and the current or fallback gas price. It incorporates the orchestrator's
// configured maxGasPrice to prevent overshooting during gas spikes.
func (r *Recipient) txCost(gasLimit *big.Int) *big.Int {
	// Default fallback gas price, adjusted for Arbitrum Nitro.
	// This value is used if r.currentGasPrice is not available or zero.
	// 15 Gwei is a more realistic baseline for Arbitrum Nitro than the legacy 3 Gwei,
	// helping to prevent incorrect gas estimates during periods of low activity
	// or when the current gas price cannot be fetched.
	const fallbackGasPriceGwei = 15
	fallbackGasPrice := new(big.Int).Mul(big.NewInt(fallbackGasPriceGwei), big.NewInt(1e9)) // 15 Gwei

	// Determine the gas price to use for calculation.
	// Prioritize the dynamically fetched currentGasPrice if available and valid.
	gasPriceToUse := fallbackGasPrice
	if r.currentGasPrice != nil && r.currentGasPrice.Cmp(common.Big0) > 0 {
		gasPriceToUse = r.currentGasPrice
	}

	// Apply the orchestrator's configured maxGasPrice cap.
	// This is crucial for preventing session drops during transient gas spikes.
	// If the determined gas price (either current or fallback) exceeds maxGasPrice,
	// we cap it at maxGasPrice. This ensures that the calculated ticket face value
	// (and thus the effective PPP) does not exceed what the orchestrator is
	// willing to accept, allowing transcoding to continue and tickets to be
	// redeemed later when gas prices normalize.
	if r.maxGasPrice != nil && r.maxGasPrice.Cmp(common.Big0) > 0 && gasPriceToUse.Cmp(r.maxGasPrice) > 0 {
		gasPriceToUse = r.maxGasPrice
	}

	// Calculate the total transaction cost.
	return new(big.Int).Mul(gasLimit, gasPriceToUse)
}