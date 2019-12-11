package common

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"

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

func TestFFmpegProfiletoNetProfile(t *testing.T) {
	assert := assert.New(t)

	profiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfile{
			Name:       "prof1",
			Bitrate:    "432k",
			Framerate:  uint(560),
			Resolution: "123x456",
		},
		ffmpeg.VideoProfile{
			Name:       "prof2",
			Bitrate:    "765k",
			Framerate:  uint(876),
			Resolution: "456x987",
		},
	}

	// empty name should return automatically generated name
	profiles[0].Name = ""
	fullProfiles, err := FFmpegProfiletoNetProfile(profiles)

	width, height, err := ffmpeg.VideoProfileResolution(profiles[0])
	assert.Nil(err)

	br := strings.Replace(profiles[0].Bitrate, "k", "000", 1)
	bitrate, err := strconv.Atoi(br)
	assert.Nil(err)

	expectedName := "ffmpeg_" + fmt.Sprintf("%dx%d_%d", width, height, bitrate)
	assert.Equal(expectedName, fullProfiles[0].Name)

	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)
	profiles[0].Name = "prof1"

	// Empty bitrate should return parsing error
	profiles[0].Bitrate = ""
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Contains(err.Error(), "strconv.Atoi: parsing")
	profiles[0].Bitrate = "432k"

	// Empty resolution should return ErrTranscoderRes
	profiles[0].Resolution = ""
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Equal(ffmpeg.ErrTranscoderRes, err)
}

func TestProfilesToHex(t *testing.T) {
	assert := assert.New(t)
	// Sanity checking against an existing eth impl that we know works
	compare := func(profiles []ffmpeg.VideoProfile) {
		pCopy := make([]ffmpeg.VideoProfile, len(profiles))
		copy(pCopy, profiles)
		b1, err := hex.DecodeString(ProfilesToHex(profiles))
		assert.Nil(err, "Error hex encoding/decoding")
		b2, err := BytesToVideoProfile(b1)
		assert.Nil(err, "Error converting back to profile")
		assert.Equal(pCopy, b2)
	}
	// XXX double check which one is wrong! ethcommon method produces "0" zero string
	// compare(nil)
	// compare([]ffmpeg.VideoProfile{})
	compare([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9})
	compare([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9})
	compare([]ffmpeg.VideoProfile{ffmpeg.P360p30fps16x9, ffmpeg.P240p30fps16x9})
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
