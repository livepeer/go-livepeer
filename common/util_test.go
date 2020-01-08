package common

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
)

func TestTxDataToVideoProfile(t *testing.T) {
	if res, err := TxDataToVideoProfile(""); err != nil && len(res) != 0 {
		t.Error("Unexpected return on empty input")
	}
	if _, err := TxDataToVideoProfile("abc"); err != ErrProfile {
		t.Error("Unexpected return on too-short input", err)
	}
	if _, err := TxDataToVideoProfile("abcdefghijk"); err != ErrProfile {
		t.Error("Unexpected return on invalid input", err)
	}
	res, err := TxDataToVideoProfile("93c717e7c0a6517a")
	if err != nil || res[1] != ffmpeg.P240p30fps16x9 || res[0] != ffmpeg.P360p30fps16x9 {
		t.Error("Unexpected profile! ", err, res)
	}
}

func TestVideoProfileBytes(t *testing.T) {
	if len(VideoProfileByteLookup) != len(VideoProfileNameLookup) {
		t.Error("Video profile byte map was not created correctly")
	}
	if res, err := BytesToVideoProfile(nil); err != nil && len(res) != 0 {
		t.Error("Unexpected return on empty input")
	}
	if res, err := BytesToVideoProfile([]byte{}); err != nil && len(res) != 0 {
		t.Error("Unexpected return on empty input")
	}
	if _, err := BytesToVideoProfile([]byte("abc")); err != ErrProfile {
		t.Error("Unexpected return on too-short input", err)
	}
	if _, err := BytesToVideoProfile([]byte("abcdefghijk")); err != ErrProfile {
		t.Error("Unexpected return on invalid input", err)
	}
	b, _ := hex.DecodeString("93c717e7c0a6517a")
	res, err := BytesToVideoProfile(b)
	if err != nil || res[1] != ffmpeg.P240p30fps16x9 || res[0] != ffmpeg.P360p30fps16x9 {
		t.Error("Unexpected profile! ", err, res)
	}
}

func TestProfilesToHex(t *testing.T) {
	// Sanity checking against an existing eth impl that we know works
	compare := func(profiles []ffmpeg.VideoProfile) {
		b1 := ethcommon.ToHex(ProfilesToTranscodeOpts(profiles))[2:]
		b2 := ProfilesToHex(profiles)
		if b1 != b2 {
			t.Error("Unequal profile hex ")
		}
	}
	// XXX double check which one is wrong! ethcommon method produces "0" zero string
	// compare(nil)
	// compare([]ffmpeg.VideoProfile{})
	compare([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9})
	compare([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9})
}

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
