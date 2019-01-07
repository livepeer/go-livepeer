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
