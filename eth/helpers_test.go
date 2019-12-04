package eth

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromPerc_DefaultDenominator(t *testing.T) {
	assert.Equal(t, big.NewInt(1000000), FromPerc(100.0))

	assert.Equal(t, big.NewInt(500000), FromPerc(50.0))

	assert.Equal(t, big.NewInt(0), FromPerc(0.0))
}

func TestFromPercOfUint256_Given100Percent_ResultWithinEpsilon(t *testing.T) {
	actual := FromPercOfUint256(100.0)

	diff := new(big.Int).Sub(maxUint256, actual)
	assert.True(t, diff.Int64() < 100)
}
func TestFromPercOfUint256_Given50Percent_ResultWithinEpsilon(t *testing.T) {
	half := new(big.Int).Div(maxUint256, big.NewInt(2))

	actual := FromPercOfUint256(50.0)

	diff := new(big.Int).Sub(half, actual)
	assert.True(t, diff.Int64() < 100)
}

func TestFromPercOfUint256_Given0Percent_ReturnsZero(t *testing.T) {
	assert.Equal(t, int64(0), FromPercOfUint256(0.0).Int64())
}

func TestToBaseAmount(t *testing.T) {
	exp := "100000000000000000" // 0.1 eth
	wei, err := ToBaseAmount("0.1")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	exp = "20000000000000000" // 0.02 eth
	wei, err = ToBaseAmount("0.02")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	exp = "1250000000000000000" // 1.25 eth
	wei, err = ToBaseAmount("1.25")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	exp = "3333333333333333" // 0.003333333333333333 eth
	wei, err = ToBaseAmount("0.003333333333333333")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	// return an error if value has more than 18 decimals
	wei, err = ToBaseAmount("0.111111111111111111111111111111111")
	assert.Nil(t, wei)
	assert.EqualError(t, err, "submitted value has more than 18 decimals")
}

func TestFromBaseAmount(t *testing.T) {
	wei, _ := new(big.Int).SetString("100000000000000000", 10)
	ethVal := FromBaseAmount(wei) // 0.1 eth
	assert.Equal(t, ethVal, "0.1")

	wei, _ = new(big.Int).SetString("20000000000000000", 10)
	ethVal = FromBaseAmount(wei) // 0.02 eth
	assert.Equal(t, ethVal, "0.02")

	wei, _ = new(big.Int).SetString("1250000000000000000", 10)
	ethVal = FromBaseAmount(wei) // 1.25 eth
	assert.Equal(t, ethVal, "1.25")

	wei, _ = new(big.Int).SetString("3333333333333333", 10) // 0.003333333333333333 eth
	ethVal = FromBaseAmount(wei)
	assert.Equal(t, ethVal, "0.003333333333333333")

	// test that no decimals return no trailing "."
	wei, _ = new(big.Int).SetString("1000000000000000000", 10)
	ethVal = FromBaseAmount(wei) // 1 eth
	assert.Equal(t, ethVal, "1")
}
