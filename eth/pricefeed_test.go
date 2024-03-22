package eth

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputePriceData(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid data", func(t *testing.T) {
		roundID := big.NewInt(1)
		updatedAt := big.NewInt(1626192000)
		answer := big.NewInt(420666000)
		decimals := uint8(6)

		data := computePriceData(roundID, updatedAt, answer, decimals)

		assert.EqualValues(int64(1), data.RoundID, "Round ID didn't match")
		assert.Equal("210333/500", data.Price.RatString(), "The Price Rat didn't match")
		assert.Equal("2021-07-13 16:00:00 +0000 UTC", data.UpdatedAt.UTC().String(), "The updated at time did not match")
	})

	t.Run("zero answer", func(t *testing.T) {
		roundID := big.NewInt(2)
		updatedAt := big.NewInt(1626192000)
		answer := big.NewInt(0)
		decimals := uint8(18)

		data := computePriceData(roundID, updatedAt, answer, decimals)

		assert.EqualValues(int64(2), data.RoundID, "Round ID didn't match")
		assert.Equal("0", data.Price.RatString(), "The Price Rat didn't match")
		assert.Equal("2021-07-13 16:00:00 +0000 UTC", data.UpdatedAt.UTC().String(), "The updated at time did not match")
	})

	t.Run("zero decimals", func(t *testing.T) {
		roundID := big.NewInt(3)
		updatedAt := big.NewInt(1626192000)
		answer := big.NewInt(13)
		decimals := uint8(0)

		data := computePriceData(roundID, updatedAt, answer, decimals)

		assert.EqualValues(int64(3), data.RoundID, "Round ID didn't match")
		assert.Equal("13", data.Price.RatString(), "The Price Rat didn't match")
		assert.Equal("2021-07-13 16:00:00 +0000 UTC", data.UpdatedAt.UTC().String(), "The updated at time did not match")
	})
}
