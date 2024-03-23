package core

import (
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/assert"
)

func TestCurrencyToWeiMultiplier(t *testing.T) {
	tests := []struct {
		name         string
		data         eth.PriceData
		baseCurrency string
		expectedWei  *big.Rat
	}{
		{
			name:         "Base currency is ETH",
			data:         eth.PriceData{Price: big.NewRat(500, 1)}, // 500 USD per ETH
			baseCurrency: "ETH",
			expectedWei:  big.NewRat(1e18, 500), // (1 / 500 USD/ETH) * 1e18 wei/ETH
		},
		{
			name:         "Base currency is not ETH",
			data:         eth.PriceData{Price: big.NewRat(1, 2000)}, // 1/2000 ETH per USD
			baseCurrency: "USD",
			expectedWei:  big.NewRat(5e14, 1), // (1 * 1/2000 ETH/USD) * 1e18 wei/ETH
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := currencyToWeiMultiplier(tt.data, tt.baseCurrency)
			assert.Equal(t, 0, tt.expectedWei.Cmp(result))
		})
	}
}
