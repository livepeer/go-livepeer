package server

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"testing"
)

const testPriceExpFactor = 100

func TestCalculateProbabilities(t *testing.T) {
	tests := []struct {
		name        string
		addrs       []string
		stakes      []int64
		prices      []int64
		stakeWeight float64
		priceWeight float64
		randWeight  float64
		want        []float64
	}{
		{
			name:        "Stake and Price weights",
			addrs:       []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:      []int64{100, 100000, 300},
			prices:      []int64{400, 700, 1000},
			stakeWeight: 0.3,
			priceWeight: 0.7,
			want:        []float64{0.665530, 0.331925, 0.002545},
		},
		{
			name:       "Random selection",
			addrs:      []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:     []int64{100, 100000, 300},
			prices:     []int64{400, 700, 1000},
			randWeight: 1.0,
			want:       []float64{0.333333, 0.333333, 0.333333},
		},
		{
			name:        "Price selection",
			addrs:       []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:      []int64{100, 100000, 300},
			prices:      []int64{1, 1, 100000000},
			priceWeight: 1.0,
			want:        []float64{0.5, 0.5, 0.0},
		},
		{
			name:        "Stake selection",
			addrs:       []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:      []int64{100, 100, 800},
			prices:      []int64{400, 700, 1000},
			stakeWeight: 1.0,
			want:        []float64{0.1, 0.1, 0.8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var orchs []ethcommon.Address
			stakes := map[ethcommon.Address]int64{}
			prices := map[ethcommon.Address]int64{}
			expProbs := map[ethcommon.Address]float64{}
			for i, addrStr := range tt.addrs {
				addr := ethcommon.HexToAddress(addrStr)
				orchs = append(orchs, addr)
				stakes[addr] = tt.stakes[i]
				prices[addr] = tt.prices[i]
				expProbs[addr] = tt.want[i]
			}

			sa := ProbabilitySelectionAlgorithm{
				StakeWeight:    tt.stakeWeight,
				PriceWeight:    tt.priceWeight,
				RandWeight:     tt.randWeight,
				PriceExpFactor: testPriceExpFactor,
			}

			probs := sa.calculateProbabilities(orchs, stakes, prices)

			require.Len(t, probs, len(expProbs))
			for addr, expProb := range expProbs {
				require.InDelta(t, expProb, probs[addr], 0.0001)
			}
		})
	}
}

func TestSelectByProbability(t *testing.T) {
	probs := map[ethcommon.Address]float64{
		ethcommon.HexToAddress("0x0000000000000000000000000000000000000001"): 0.1,
		ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"): 0.3,
		ethcommon.HexToAddress("0x0000000000000000000000000000000000000003"): 0.6,
	}

	selected := selectBy(probs)

	// all we can check is that one of input addresses was selected
	require.Contains(t, probs, selected)
}
