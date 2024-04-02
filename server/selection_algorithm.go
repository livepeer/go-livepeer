package server

import (
	"math"
	"math/big"
	"math/rand"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

type ProbabilitySelectionAlgorithm struct {
	MinPerfScore float64

	StakeWeight float64
	PriceWeight float64
	RandWeight  float64

	PriceExpFactor float64
}

func (sa ProbabilitySelectionAlgorithm) Select(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, maxPrice *big.Rat, prices map[ethcommon.Address]*big.Rat, perfScores map[ethcommon.Address]float64) ethcommon.Address {
	filtered := sa.filter(addrs, maxPrice, prices, perfScores)
	probabilities := sa.calculateProbabilities(filtered, stakes, prices)
	return selectBy(probabilities)
}

func (sa ProbabilitySelectionAlgorithm) filter(addrs []ethcommon.Address, maxPrice *big.Rat, prices map[ethcommon.Address]*big.Rat, perfScores map[ethcommon.Address]float64) []ethcommon.Address {
	hasScores := sa.MinPerfScore > 0 && len(perfScores) > 0
	hasMaxPrice := maxPrice != nil && len(prices) > 0
	if !hasScores && !hasMaxPrice {
		// No filters defined, return all Orchestrators
		return addrs
	}

	applyFilter := func(pred func(ethcommon.Address) bool) []ethcommon.Address {
		var res []ethcommon.Address
		for _, addr := range addrs {
			if pred(addr) {
				res = append(res, addr)
			}
		}
		return res
	}

	filtered := applyFilter(func(addr ethcommon.Address) bool {
		if hasScores && perfScores[addr] < sa.MinPerfScore {
			return false
		}
		oPrice := prices[addr]
		if hasMaxPrice && oPrice != nil && oPrice.Cmp(maxPrice) > 0 {
			return false
		}
		return true
	})
	if len(filtered) == 0 && hasScores {
		// If no orchestrators pass the filter, try ignoring the max price.
		filtered = applyFilter(func(addr ethcommon.Address) bool {
			return perfScores[addr] >= sa.MinPerfScore
		})
	}

	if len(filtered) == 0 {
		// If no orchestrators pass the perf filter, return all Orchestrators.
		// That may mean some issues with the PerfScore service.
		glog.Warning("No Orchestrators passed min performance score filter, not using the filter")
		return addrs
	}
	return filtered
}

func (sa ProbabilitySelectionAlgorithm) calculateProbabilities(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]*big.Rat) map[ethcommon.Address]float64 {
	pricesNorm := map[ethcommon.Address]float64{}
	for _, addr := range addrs {
		p, _ := prices[addr].Float64()
		pricesNorm[addr] = math.Exp(-1 * p / sa.PriceExpFactor)
	}

	var priceSum, stakeSum float64
	for _, addr := range addrs {
		priceSum += pricesNorm[addr]
		stakeSum += float64(stakes[addr])
	}

	probs := map[ethcommon.Address]float64{}
	for _, addr := range addrs {
		priceProb := 1.0
		if priceSum != 0 {
			priceProb = pricesNorm[addr] / priceSum
		}
		stakeProb := 1.0
		if stakeSum != 0 {
			stakeProb = float64(stakes[addr]) / stakeSum
		}
		randProb := 1.0 / float64(len(addrs))

		probs[addr] = sa.PriceWeight*priceProb + sa.StakeWeight*stakeProb + sa.RandWeight*randProb
	}

	return probs
}

func selectBy(probabilities map[ethcommon.Address]float64) ethcommon.Address {
	if len(probabilities) == 0 {
		return ethcommon.Address{}
	}

	var addrs []ethcommon.Address
	var cumProbs []float64
	var cumProb float64
	for addr, prob := range probabilities {
		addrs = append(addrs, addr)
		cumProb += prob
		cumProbs = append(cumProbs, cumProb)
	}

	r := random.Float64()
	for i, cumProb := range cumProbs {
		if r <= cumProb {
			return addrs[i]
		}
	}

	// return any Orchestrator is none was found with the probabilities
	// should not happen, but just to be on the safe side if we encounter some super corner case with the float
	// number precision
	return addrs[0]
}
