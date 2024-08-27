package server

import (
	"context"
	"math"
	"math/big"
	"math/rand"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

type ProbabilitySelectionAlgorithm struct {
	MinPerfScore float64

	StakeWeight float64
	PriceWeight float64
	RandWeight  float64

	PriceExpFactor         float64
	IgnoreMaxPriceIfNeeded bool
}

func (sa ProbabilitySelectionAlgorithm) Select(ctx context.Context, addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, maxPrice *big.Rat, prices map[ethcommon.Address]*big.Rat, perfScores map[ethcommon.Address]float64) ethcommon.Address {
	filtered := sa.filter(ctx, addrs, maxPrice, prices, perfScores)
	probabilities := sa.calculateProbabilities(filtered, stakes, prices)
	return selectBy(probabilities)
}

func (sa ProbabilitySelectionAlgorithm) filter(ctx context.Context, addrs []ethcommon.Address, maxPrice *big.Rat, prices map[ethcommon.Address]*big.Rat, perfScores map[ethcommon.Address]float64) []ethcommon.Address {
	filteredByPerfScore := sa.filterByPerfScore(ctx, addrs, perfScores)
	return sa.filterByMaxPrice(ctx, filteredByPerfScore, maxPrice, prices)
}

func (sa ProbabilitySelectionAlgorithm) filterByPerfScore(ctx context.Context, addrs []ethcommon.Address, scores map[ethcommon.Address]float64) []ethcommon.Address {
	if sa.MinPerfScore <= 0 || len(scores) == 0 {
		// Performance Score filter not defined, return all Orchestrators
		return addrs
	}

	var res []ethcommon.Address
	for _, addr := range addrs {
		if scores[addr] >= sa.MinPerfScore {
			res = append(res, addr)
		}
	}

	if len(res) == 0 {
		// If no orchestrators pass the perf filter, return all Orchestrators.
		// That may mean some issues with the PerfScore service.
		clog.Warningf(ctx, "No Orchestrators passed min performance score filter, not using the filter, numAddrs=%d, minPerfScore=%v, scores=%v, addrs=%v", len(addrs), sa.MinPerfScore, scores, addrs)
		return addrs
	}
	return res
}

func (sa ProbabilitySelectionAlgorithm) filterByMaxPrice(ctx context.Context, addrs []ethcommon.Address, maxPrice *big.Rat, prices map[ethcommon.Address]*big.Rat) []ethcommon.Address {
	if maxPrice == nil || len(prices) == 0 {
		// Max price filter not defined, return all Orchestrators
		return addrs
	}

	var res []ethcommon.Address
	for _, addr := range addrs {
		price := prices[addr]
		if price != nil && price.Cmp(maxPrice) <= 0 {
			res = append(res, addr)
		}
	}

	if len(res) == 0 && sa.IgnoreMaxPriceIfNeeded {
		// If no orchestrators pass the filter, return all Orchestrators
		// It means that no orchestrators are below the max price
		clog.Warningf(ctx, "No Orchestrators passed max price filter, not using the filter, numAddrs=%d, maxPrice=%v, prices=%v, addrs=%v", len(addrs), maxPrice, prices, addrs)
		return addrs
	}
	return res
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
