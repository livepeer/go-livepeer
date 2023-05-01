package common

import (
	"math"
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
)

func TestPriceToFixed(t *testing.T) {
	assert := assert.New(t)

	// if rat is nil returns 0 and error
	fp, err := PriceToFixed(nil)
	assert.Zero(fp)
	assert.Error(err)

	// 1/10 rat returns 100
	fp, err = PriceToFixed(big.NewRat(1, 10))
	assert.Nil(err)
	assert.Equal(fp, int64(100))

	// 500/1 returns 500000 with
	fp, err = PriceToFixed(big.NewRat(500, 1))
	assert.Nil(err)
	assert.Equal(fp, int64(500000))

	// 125/100 returns 1250
	fp, err = PriceToFixed(big.NewRat(125, 100))
	assert.Nil(err)
	assert.Equal(fp, int64(1250))

	// rat smaller than 1/1000 returns 0
	fp, err = PriceToFixed(big.NewRat(1, 5000))
	assert.Nil(err)
	assert.Zero(fp)
}

func TestBaseTokenAmountToFixed(t *testing.T) {
	assert := assert.New(t)

	// Check when nil is passed in
	fp, err := BaseTokenAmountToFixed(nil)
	assert.EqualError(err, "reference to rat is nil")
	assert.Zero(fp)

	baseTokenAmount, _ := new(big.Int).SetString("1844486980754712220549592", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(184448698075), fp)

	baseTokenAmount, _ = new(big.Int).SetString("1238827039830161692185743", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(123882703983), fp)

	baseTokenAmount, _ = new(big.Int).SetString("451288400383394091574336", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(45128840038), fp)

	// math.MaxInt64 = 9223372036854775807 is the largest fixed point number that can be represented as a int64
	// This corresponds to 92233720368547.75807 tokens or 92233720368547.75807 * (10 ** 18) base token units
	baseTokenAmount, _ = new(big.Int).SetString("92233720368547758070000000000000", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(math.MaxInt64), fp)

	// Base token amount = 92233720368547.75808 * (10 ** 18). This is 1 more than the largest base token amount that can be represented as a fixed point number
	// Should return math.MaxInt64
	baseTokenAmount, _ = new(big.Int).SetString("92233720368547758080000000000000", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(math.MaxInt64), fp)

	// Base token amount = 92233720368547.75808 * (10 ** 18). This is a magnitude greater than the largest base token amount that can be represented as a fixed point number
	// Should return math.MaxInt64
	baseTokenAmount, _ = new(big.Int).SetString("922337203685477580700000000000000", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(math.MaxInt64), fp)

	// Base token amount = .00001 * (10 ** 18). This is the smallest base token amount that can be represented as a fixed point number
	// Should return 1
	baseTokenAmount, _ = new(big.Int).SetString("10000000000000", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Equal(int64(1), fp)

	// Base token amount = .000009 * (10 ** 18). This is smaller than the minimum base token amount for fixed point number conversion
	// Should return 0
	baseTokenAmount, _ = new(big.Int).SetString("9000000000000", 10)
	fp, err = BaseTokenAmountToFixed(baseTokenAmount)
	assert.Nil(err)
	assert.Zero(fp)
}

func TestToInt64(t *testing.T) {
	// test val > math.MaxInt64 => val = math.MaxInt64
	val, _ := new(big.Int).SetString("9223372036854775808", 10) // 2^63
	assert.Equal(t, int64(math.MaxInt64), ToInt64(val))
	// test val < math.MaxInt64
	assert.Equal(t, int64(5), ToInt64(big.NewInt(5)))
}

func TestRatPriceInfo(t *testing.T) {
	assert := assert.New(t)

	// Test nil priceInfo
	priceInfo, err := RatPriceInfo(nil)
	assert.Nil(priceInfo)
	assert.Nil(err)

	// Test priceInfo.pixelsPerUnit = 0
	_, err = RatPriceInfo(&net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 0})
	assert.EqualError(err, "pixels per unit is 0")

	// Test valid priceInfo
	priceInfo, err = RatPriceInfo(&net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 2})
	assert.Nil(err)
	assert.Zero(priceInfo.Cmp(big.NewRat(7, 2)))
}
