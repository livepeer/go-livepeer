package common

import (
	"fmt"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var (
	ErrParseBigInt = fmt.Errorf("failed to parse big integer")
	ErrProfile     = fmt.Errorf("failed to parse profile")
)

func ParseBigInt(num string) (*big.Int, error) {
	bigNum := new(big.Int)
	_, ok := bigNum.SetString(num, 10)

	if !ok {
		return nil, ErrParseBigInt
	} else {
		return bigNum, nil
	}
}

func WaitUntil(waitTime time.Duration, condition func() bool) {
	start := time.Now()
	for time.Since(start) < waitTime {
		if condition() == false {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
}

func WaitAssert(t *testing.T, waitTime time.Duration, condition func() bool, msg string) {
	start := time.Now()
	for time.Since(start) < waitTime {
		if condition() == false {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}

	if condition() == false {
		t.Errorf(msg)
	}
}

func Retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return Retry(attempts, 2*sleep, fn)
		}
		return err
	}

	return nil
}

func TxDataToVideoProfile(txData string) ([]ffmpeg.VideoProfile, error) {
	profiles := make([]ffmpeg.VideoProfile, 0)

	if len(txData) == 0 {
		return profiles, nil
	}
	if len(txData) < VideoProfileIDSize {
		return nil, ErrProfile
	}

	for i := 0; i+VideoProfileIDSize <= len(txData); i += VideoProfileIDSize {
		txp := txData[i : i+VideoProfileIDSize]

		p, ok := ffmpeg.VideoProfileLookup[VideoProfileNameLookup[txp]]
		if !ok {
			glog.Errorf("Cannot find video profile for job: %v", txp)
			return nil, ErrProfile // monitor to see if this is too aggressive
		}
		profiles = append(profiles, p)
	}

	return profiles, nil
}

func ProfilesToTranscodeOpts(profiles []ffmpeg.VideoProfile) []byte {
	//Sort profiles first
	sort.Sort(ffmpeg.ByName(profiles))
	transOpts := []byte{}
	for _, prof := range profiles {
		transOpts = append(transOpts, crypto.Keccak256([]byte(prof.Name))[0:4]...)
	}
	return transOpts
}
