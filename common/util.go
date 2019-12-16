package common

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"google.golang.org/grpc/peer"
)

// HTTPTimeout timeout used in HTTP connections between nodes
const HTTPTimeout = 8 * time.Second

var (
	ErrParseBigInt = fmt.Errorf("failed to parse big integer")
	ErrProfile     = fmt.Errorf("failed to parse profile")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

func BytesToVideoProfile(txData []byte) ([]ffmpeg.VideoProfile, error) {
	profiles := make([]ffmpeg.VideoProfile, 0)

	if len(txData) == 0 {
		return profiles, nil
	}
	if len(txData) < VideoProfileIDBytes {
		return nil, ErrProfile
	}

	for i := 0; i+VideoProfileIDBytes <= len(txData); i += VideoProfileIDBytes {
		var txp [VideoProfileIDBytes]byte
		copy(txp[:], txData[i:i+VideoProfileIDBytes])

		p, ok := ffmpeg.VideoProfileLookup[VideoProfileByteLookup[txp]]
		if !ok {
			glog.Errorf("Cannot find video profile for job: %v", txp)
			return nil, ErrProfile // monitor to see if this is too aggressive
		}
		profiles = append(profiles, p)
	}

	return profiles, nil
}

func DefaultProfileName(width int, height int, bitrate int) string {
	return fmt.Sprintf("%dx%d_%d", width, height, bitrate)
}

func FFmpegProfiletoNetProfile(ffmpegProfiles []ffmpeg.VideoProfile) ([]*net.VideoProfile, error) {
	profiles := make([]*net.VideoProfile, 0, len(ffmpegProfiles))
	for _, profile := range ffmpegProfiles {
		width, height, err := ffmpeg.VideoProfileResolution(profile)
		if err != nil {
			return nil, err
		}
		br := strings.Replace(profile.Bitrate, "k", "000", 1)
		bitrate, err := strconv.Atoi(br)
		if err != nil {
			return nil, err
		}
		name := profile.Name
		if name == "" {
			name = "ffmpeg_" + DefaultProfileName(width, height, bitrate)
		}
		fullProfile := net.VideoProfile{
			Name:    name,
			Width:   int32(width),
			Height:  int32(height),
			Bitrate: int32(bitrate),
			Fps:     uint32(profile.Framerate),
		}
		profiles = append(profiles, &fullProfile)
	}
	return profiles, nil
}

func ProfilesToTranscodeOpts(profiles []ffmpeg.VideoProfile) []byte {
	transOpts := []byte{}
	for _, prof := range profiles {
		transOpts = append(transOpts, crypto.Keccak256([]byte(prof.Name))[0:4]...)
	}
	return transOpts
}

func ProfilesToHex(profiles []ffmpeg.VideoProfile) string {
	return hex.EncodeToString(ProfilesToTranscodeOpts(profiles))
}

func ProfilesNames(profiles []ffmpeg.VideoProfile) string {
	names := make(sort.StringSlice, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	names.Sort()
	return strings.Join(names, ",")
}

func GetConnectionAddr(ctx context.Context) string {
	from := "unknown"
	if p, ok := peer.FromContext(ctx); ok {
		from = p.Addr.String()
	}
	return from
}

// GenErrRegex generates a regexp `(err1)|(err2)|(err3)` given a list of
// error strings [err1, err2, err3]
func GenErrRegex(errStrings []string) *regexp.Regexp {
	groups := []string{}
	for _, v := range errStrings {
		groups = append(groups, fmt.Sprintf("(%v)", v))
	}
	return regexp.MustCompile(strings.Join(groups, "|"))
}

// PriceToFixed converts a big.Rat into a fixed point number represented as int64
// using a scaleFactor of 1000 resulting in max decimal places of 3
func PriceToFixed(price *big.Rat) (int64, error) {
	scalingFactor := int64(1000)
	if price == nil {
		return 0, fmt.Errorf("reference to rat is nil")
	}
	scaled := new(big.Rat).Mul(price, big.NewRat(scalingFactor, 1))
	fp, _ := new(big.Float).SetRat(scaled).Int64()
	return fp, nil
}

// RandomIDGenerator generates random hexadecimal string of specified length
// defined as variable for unit tests
var RandomIDGenerator = func(length uint) string {
	x := make([]byte, length, length)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return hex.EncodeToString(x)
}

// RandName generates random hexadecimal string
func RandName() string {
	return RandomIDGenerator(10)
}
