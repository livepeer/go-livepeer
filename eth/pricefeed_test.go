package eth

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEthClient struct {
	mock.Mock
}

func (m *mockEthClient) CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error) {
	args := m.Called(ctx, contract, blockNumber)
	return args.Get(0).([]byte), args.Error(1)
}

func TestIsContractAddress(t *testing.T) {
	assert := assert.New(t)

	t.Run("invalid address", func(t *testing.T) {
		addr := "0x123"
		mockClient := new(mockEthClient)
		assert.False(isContractAddress(addr, mockClient))
	})

	t.Run("empty address", func(t *testing.T) {
		addr := ""
		mockClient := new(mockEthClient)
		assert.False(isContractAddress(addr, mockClient))
	})

	t.Run("valid address but not contract", func(t *testing.T) {
		addr := "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"
		mockClient := new(mockEthClient)
		mockClient.On("CodeAt", mock.Anything, common.HexToAddress(addr), mock.Anything).Return([]byte{}, nil)
		assert.False(isContractAddress(addr, mockClient))
	})

	t.Run("valid contract address", func(t *testing.T) {
		addr := "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"
		mockClient := new(mockEthClient)
		mockClient.On("CodeAt", mock.Anything, common.HexToAddress(addr), mock.Anything).Return([]byte{0x1}, nil)
		assert.True(isContractAddress(addr, mockClient))
	})
}

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
