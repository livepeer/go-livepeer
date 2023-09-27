package server

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"math"
	"math/rand"
)

type ProbabilitySelectionAlgorithm struct {
	StakeWeight float64
	PriceWeight float64
	RandWeight  float64

	PriceExpFactor float64
}

func (sa ProbabilitySelectionAlgorithm) Select(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]float64) ethcommon.Address {
	probabilities := sa.calculateProbabilities(addrs, stakes, prices)
	return selectBy(probabilities)
}

func (sa ProbabilitySelectionAlgorithm) calculateProbabilities(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]float64) map[ethcommon.Address]float64 {
	pricesNorm := map[ethcommon.Address]float64{}
	for _, addr := range addrs {
		p := prices[addr]
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

	r := rand.Float64()
	for i, cumProb := range cumProbs {
		if r <= cumProb {
			return addrs[i]
		}
	}

	return addrs[0]
}
