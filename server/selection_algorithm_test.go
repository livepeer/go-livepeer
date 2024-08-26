package server

import (
	"context"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const testPriceExpFactor = 100

func TestFilter(t *testing.T) {
	tests := []struct {
		name                   string
		orchMinPerfScore       float64
		maxPrice               float64
		prices                 map[string]float64
		orchPerfScores         map[string]float64
		orchestrators          []string
		want                   []string
		ignoreMaxPriceIfNeeded bool
	}{
		{
			name:             "Some Orchestrators pass the filter",
			orchMinPerfScore: 0.7,
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.6,
				"0x0000000000000000000000000000000000000002": 0.8,
				"0x0000000000000000000000000000000000000003": 0.9,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
				"0x0000000000000000000000000000000000000004",
			},
			want: []string{
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
		},
		{
			name:             "No orchestrator Scores defined",
			orchMinPerfScore: 0.7,
			orchPerfScores:   nil,
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
			},
			want: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
			},
		},
		{
			name:             "No min score defined",
			orchMinPerfScore: 0,
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.6,
				"0x0000000000000000000000000000000000000002": 0.8,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
			},
			want: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
			},
		},
		{
			name:             "No Orchestrators pass the filter",
			orchMinPerfScore: 0.99,
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.6,
				"0x0000000000000000000000000000000000000002": 0.8,
				"0x0000000000000000000000000000000000000003": 0.9,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
				"0x0000000000000000000000000000000000000004",
			},
			want: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
				"0x0000000000000000000000000000000000000004",
			},
		},
		{

			orchMinPerfScore: 0.7,
			maxPrice:         2000,
			prices: map[string]float64{
				"0x0000000000000000000000000000000000000001": 500,
				"0x0000000000000000000000000000000000000002": 1500,
				"0x0000000000000000000000000000000000000003": 1000,
			},
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.6,
				"0x0000000000000000000000000000000000000002": 0.8,
				"0x0000000000000000000000000000000000000003": 0.9,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
			want: []string{
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
		},
		{
			name:                   "All prices above max price and ignoreMaxPriceIfNeeded enabled",
			orchMinPerfScore:       0.7,
			maxPrice:               100,
			ignoreMaxPriceIfNeeded: true,
			prices: map[string]float64{
				"0x0000000000000000000000000000000000000001": 500,
				"0x0000000000000000000000000000000000000002": 1500,
				"0x0000000000000000000000000000000000000003": 1000,
			},
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.6,
				"0x0000000000000000000000000000000000000002": 0.8,
				"0x0000000000000000000000000000000000000003": 0.9,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
			want: []string{
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
		},
		{
			name:             "All prices below max price",
			orchMinPerfScore: 0.7,
			maxPrice:         100,
			prices: map[string]float64{
				"0x0000000000000000000000000000000000000001": 500,
				"0x0000000000000000000000000000000000000002": 1500,
				"0x0000000000000000000000000000000000000003": 1000,
			},
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.6,
				"0x0000000000000000000000000000000000000002": 0.8,
				"0x0000000000000000000000000000000000000003": 0.9,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
			want: []string{},
		},
		{
			name:             "Mix of prices relative to max price",
			orchMinPerfScore: 0.7,
			maxPrice:         750,
			prices: map[string]float64{
				"0x0000000000000000000000000000000000000001": 500,
				"0x0000000000000000000000000000000000000002": 1500,
				"0x0000000000000000000000000000000000000003": 700,
			},
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.8,
				"0x0000000000000000000000000000000000000002": 0.6,
				"0x0000000000000000000000000000000000000003": 0.9,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
			want: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000003",
			},
		},
		{
			name:             "Exact match with max price",
			orchMinPerfScore: 0.7,
			maxPrice:         1000,
			prices: map[string]float64{
				"0x0000000000000000000000000000000000000001": 500,
				"0x0000000000000000000000000000000000000002": 1000, // exactly max
				"0x0000000000000000000000000000000000000003": 1500,
			},
			orchPerfScores: map[string]float64{
				"0x0000000000000000000000000000000000000001": 0.8,
				"0x0000000000000000000000000000000000000002": 0.9,
				"0x0000000000000000000000000000000000000003": 0.6,
			},
			orchestrators: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
				"0x0000000000000000000000000000000000000003",
			},
			want: []string{
				"0x0000000000000000000000000000000000000001",
				"0x0000000000000000000000000000000000000002",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addrs []ethcommon.Address
			var maxPrice *big.Rat
			prices := map[ethcommon.Address]*big.Rat{}
			perfScores := map[ethcommon.Address]float64{}
			for _, o := range tt.orchestrators {
				addr := ethcommon.HexToAddress(o)
				addrs = append(addrs, addr)
				perfScores[addr] = tt.orchPerfScores[o]
				if price, ok := tt.prices[o]; ok {
					prices[addr] = new(big.Rat).SetFloat64(price)
				}
			}
			if tt.maxPrice > 0 {
				maxPrice = new(big.Rat).SetFloat64(tt.maxPrice)
			}
			sa := &ProbabilitySelectionAlgorithm{
				MinPerfScore:           tt.orchMinPerfScore,
				IgnoreMaxPriceIfNeeded: tt.ignoreMaxPriceIfNeeded,
			}

			res := sa.filter(context.Background(), addrs, maxPrice, prices, perfScores)

			var exp []ethcommon.Address
			for _, o := range tt.want {
				exp = append(exp, ethcommon.HexToAddress(o))
			}
			require.Equal(t, exp, res)
		})
	}
}

func TestCalculateProbabilities(t *testing.T) {
	tests := []struct {
		name        string
		addrs       []string
		stakes      []int64
		prices      []float64
		stakeWeight float64
		priceWeight float64
		randWeight  float64
		want        []float64
	}{
		{
			name:        "Stake and Price weights",
			addrs:       []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:      []int64{100, 100000, 300},
			prices:      []float64{400, 700, 1000},
			stakeWeight: 0.3,
			priceWeight: 0.7,
			want:        []float64{0.665530, 0.331925, 0.002545},
		},
		{
			name:       "Random selection",
			addrs:      []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:     []int64{100, 100000, 300},
			prices:     []float64{400, 700, 1000},
			randWeight: 1.0,
			want:       []float64{0.333333, 0.333333, 0.333333},
		},
		{
			name:        "Price selection",
			addrs:       []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:      []int64{100, 100000, 300},
			prices:      []float64{1, 1, 100000000},
			priceWeight: 1.0,
			want:        []float64{0.5, 0.5, 0.0},
		},
		{
			name:        "Stake selection",
			addrs:       []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000003"},
			stakes:      []int64{100, 100, 800},
			prices:      []float64{400, 700, 1000},
			stakeWeight: 1.0,
			want:        []float64{0.1, 0.1, 0.8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var orchs []ethcommon.Address
			stakes := map[ethcommon.Address]int64{}
			prices := map[ethcommon.Address]*big.Rat{}
			expProbs := map[ethcommon.Address]float64{}
			for i, addrStr := range tt.addrs {
				addr := ethcommon.HexToAddress(addrStr)
				orchs = append(orchs, addr)
				stakes[addr] = tt.stakes[i]
				prices[addr] = new(big.Rat).SetFloat64(tt.prices[i])
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
	// use constant seed to avoid test flakiness
	random.Seed(0)
	iters := 100000

	probs := map[ethcommon.Address]float64{
		ethcommon.HexToAddress("0x0000000000000000000000000000000000000001"): 0.11,
		ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"): 0.29,
		ethcommon.HexToAddress("0x0000000000000000000000000000000000000003"): 0.6,
	}

	selected := map[ethcommon.Address]int64{}
	for i := 0; i < iters; i++ {
		selected[selectBy(probs)]++
	}

	for addr, prob := range probs {
		selectedRatio := float64(selected[addr]) / float64(iters)
		require.InDelta(t, prob, selectedRatio, 0.01)
	}
}
