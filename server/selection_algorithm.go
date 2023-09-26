package server

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"math"
	"math/rand"
)

type SelectionAlgorithm interface {
	Select(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]int64) ethcommon.Address
}

type probabilitySelectionAlgorithm struct {
	stakeWeight float64
	priceWeight float64
	randWeight  float64

	priceExpFactor float64
}

func (sa probabilitySelectionAlgorithm) Select(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]int64) ethcommon.Address {
	probabilities := sa.calculateProbabilities(addrs, stakes, prices)
	return selectBy(probabilities)
}

func (sa probabilitySelectionAlgorithm) calculateProbabilities(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]int64) map[ethcommon.Address]float64 {
	pricesNorm := map[ethcommon.Address]float64{}
	for _, addr := range addrs {
		p := prices[addr]
		pricesNorm[addr] = math.Exp(-1 * float64(p) / sa.priceExpFactor)
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

		probs[addr] = sa.priceWeight*priceProb + sa.stakeWeight*stakeProb + sa.randWeight*randProb
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
