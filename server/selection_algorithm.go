package server

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"math"
	"math/rand"
	"time"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

type ProbabilitySelectionAlgorithm struct {
	MinPerfScore float64

	StakeWeight float64
	PriceWeight float64
	RandWeight  float64

	PriceExpFactor float64
}

func (sa ProbabilitySelectionAlgorithm) Select(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]float64, perfScores map[ethcommon.Address]float64) ethcommon.Address {
	filtered := sa.filter(addrs, perfScores)
	probabilities := sa.calculateProbabilities(filtered, stakes, prices)
	return selectBy(probabilities)
}

func (sa ProbabilitySelectionAlgorithm) filter(addrs []ethcommon.Address, scores map[ethcommon.Address]float64) []ethcommon.Address {
	if sa.MinPerfScore <= 0 || scores == nil || len(scores) == 0 {
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
		// If no orchestrators pass the filter, then returns all Orchestrators
		// That may mean some issues with the PerfScore service.
		glog.Warning("No Orchestrators passed min performance score filter, not using the filter")
		return addrs
	}
	return res
}

func (sa ProbabilitySelectionAlgorithm) calculateProbabilities(addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, prices map[ethcommon.Address]float64) map[ethcommon.Address]float64 {
	// Find the biggest stake in the current working set
	var maxStake float64 = 0.0
	for _, addr := range addrs {
		var thisStake = float64(stakes[addr])
		if thisStake > maxStake {
			maxStake = thisStake
		}
	}

	probs := map[ethcommon.Address]float64{}
	for _, addr := range addrs {
		randProb := 1.0 / float64(len(addrs))
		// Normalize price-weighted curve exponentially
		var priceProb = math.Exp(-1 * prices[addr] / sa.PriceExpFactor)
		// Normalize stake-weighted curve linearly relative to the highest staked orchestrator in the set
		var stakeProb = float64(stakes[addr]) / maxStake
		probs[addr] = (sa.PriceWeight*priceProb)*(sa.StakeWeight*stakeProb) + sa.RandWeight*randProb
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
