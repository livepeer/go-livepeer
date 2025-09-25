package common

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"math/rand"
	"mime"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/gpu"
	"github.com/jaypipes/ghw/pkg/pci"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/oapi-codegen/runtime/types"
	"github.com/pkg/errors"
	"google.golang.org/grpc/peer"
)

// HTTPDialTimeout timeout used to establish an HTTP connection between nodes
var HTTPDialTimeout = 2 * time.Second

// HTTPTimeout timeout used in HTTP connections between nodes
var HTTPTimeout = 8 * time.Second

// SegHttpPushTimeoutMultiplier used in the HTTP connection for pushing the segment
var SegHttpPushTimeoutMultiplier = 4.0

// SegUploadTimeoutMultiplier used in HTTP connection for uploading the segment
var SegUploadTimeoutMultiplier = 0.5

// MinSegmentUploadTimeout defines the minimum timeout enforced for uploading a segment to orchestrators
var MinSegmentUploadTimeout = 2 * time.Second

// WebhookDiscoveryRefreshInterval defines for long the Webhook Discovery values should be cached
var WebhookDiscoveryRefreshInterval = 1 * time.Minute

// Max Segment Duration
var MaxDuration = (5 * time.Minute)

// Max Input Bitrate (bits/sec)
var maxInputBitrate = 8000 * 1000 // 8000kbps

// Max Segment Size in bytes (cap reading HTTP response body at this size)
var MaxSegSize = int(MaxDuration.Seconds()) * (maxInputBitrate / 8)

const maxInt64 = int64(math.MaxInt64)

// using a scaleFactor of 1000 for orchestrator prices
// resulting in max decimal places of 3
const priceScalingFactor = int64(1000)

var (
	ErrParseBigInt = fmt.Errorf("failed to parse big integer")

	ErrChromaFormat = fmt.Errorf("unknown VideoProfile ChromaFormat")
	ErrFormatProto  = fmt.Errorf("unknown VideoProfile format for protobufs")
	ErrFormatMime   = fmt.Errorf("unknown VideoProfile format for mime type")
	ErrFormatExt    = fmt.Errorf("unknown VideoProfile format for extension")
	ErrProfProto    = fmt.Errorf("unknown VideoProfile profile for protobufs")
	ErrProfEncoder  = fmt.Errorf("unknown VideoProfile encoder for protobufs")
	ErrProfName     = fmt.Errorf("unknown VideoProfile profile name")

	ErrAudioDurationCalculation = fmt.Errorf("audio duration calculation failed")
	ErrNoExtensionsForType      = fmt.Errorf("no extensions exist for mime type")

	ext2mime = map[string]string{
		".ts":  "video/mp2t",
		".mp4": "video/mp4",
	}
	mime2ext = map[string]string{
		"video/mp2t": ".ts",
		"video/mp4":  ".mp4",
		"image/png":  ".png",
		"audio/wav":  ".wav",
	}
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
			name = "ffmpeg_" + ffmpeg.DefaultProfileName(width, height, bitrate)
		}
		format := net.VideoProfile_MPEGTS
		switch profile.Format {
		case ffmpeg.FormatNone:
		case ffmpeg.FormatMPEGTS:
		case ffmpeg.FormatMP4:
			format = net.VideoProfile_MP4
		default:
			return nil, ErrFormatProto
		}
		encoderProf := net.VideoProfile_ENCODER_DEFAULT
		switch profile.Profile {
		case ffmpeg.ProfileNone:
		case ffmpeg.ProfileH264Baseline:
			encoderProf = net.VideoProfile_H264_BASELINE
		case ffmpeg.ProfileH264Main:
			encoderProf = net.VideoProfile_H264_MAIN
		case ffmpeg.ProfileH264High:
			encoderProf = net.VideoProfile_H264_HIGH
		case ffmpeg.ProfileH264ConstrainedHigh:
			encoderProf = net.VideoProfile_H264_CONSTRAINED_HIGH
		default:
			return nil, ErrProfProto
		}
		encoder := net.VideoProfile_H264
		switch profile.Encoder {
		case ffmpeg.H264:
			encoder = net.VideoProfile_H264
		case ffmpeg.H265:
			encoder = net.VideoProfile_H265
		case ffmpeg.VP8:
			encoder = net.VideoProfile_VP8
		case ffmpeg.VP9:
			encoder = net.VideoProfile_VP9
		default:
			return nil, ErrProfEncoder
		}
		gop := int32(0)
		if profile.GOP < 0 {
			gop = int32(profile.GOP)
		} else {
			gop = int32(profile.GOP.Milliseconds())
		}
		var chromaFormat net.VideoProfile_ChromaSubsampling
		switch profile.ChromaFormat {
		case ffmpeg.ChromaSubsampling420:
			chromaFormat = net.VideoProfile_CHROMA_420
		case ffmpeg.ChromaSubsampling422:
			chromaFormat = net.VideoProfile_CHROMA_422
		case ffmpeg.ChromaSubsampling444:
			chromaFormat = net.VideoProfile_CHROMA_444
		default:
			return nil, ErrChromaFormat
		}
		fullProfile := net.VideoProfile{
			Name:         name,
			Width:        int32(width),
			Height:       int32(height),
			Bitrate:      int32(bitrate),
			Fps:          uint32(profile.Framerate),
			FpsDen:       uint32(profile.FramerateDen),
			Format:       format,
			Profile:      encoderProf,
			Gop:          gop,
			Encoder:      encoder,
			ColorDepth:   int32(profile.ColorDepth),
			ChromaFormat: chromaFormat,
			Quality:      uint32(profile.Quality),
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

func ProfileExtensionFormat(ext string) ffmpeg.Format {
	p, ok := ffmpeg.ExtensionFormats[ext]
	if !ok {
		return ffmpeg.FormatNone
	}
	return p
}

func ProfileFormatExtension(f ffmpeg.Format) (string, error) {
	ext, ok := ffmpeg.FormatExtensions[f]
	if !ok {
		return "", ErrFormatExt
	}
	return ext, nil
}

func ProfileFormatMimeType(f ffmpeg.Format) (string, error) {
	ext, err := ProfileFormatExtension(f)
	if err != nil {
		return "", err
	}
	return TypeByExtension(ext)
}

func TypeByExtension(ext string) (string, error) {
	if m, ok := ext2mime[ext]; ok && m != "" {
		return m, nil
	}
	m := mime.TypeByExtension(ext)
	if m == "" {
		return "", ErrFormatMime
	}
	return m, nil
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
	return ratToFixed(price, priceScalingFactor)
}

// FixedToPrice converts a fixed point number with 3 decimal places represented as in int64 into a big.Rat
func FixedToPrice(price int64) *big.Rat {
	return big.NewRat(price, priceScalingFactor)
}

// PriceToInt64 converts a *big.Rat to an *int64 if possible, otherwise returns an error.
func PriceToInt64(price *big.Rat) (*big.Rat, error) {
	fixed := new(big.Int).Div(price.Num(), price.Denom())
	if !fixed.IsInt64() {
		return nil, errors.New("price cannot be converted to int64")
	}

	return big.NewRat(fixed.Int64(), 1), nil
}

// BaseTokenAmountToFixed converts the base amount of a token (i.e. ETH/LPT) represented as a big.Int into a fixed point number represented
// as a int64 using a scalingFactor of 100000 resulting in max decimal places of 5
func BaseTokenAmountToFixed(baseAmount *big.Int) (int64, error) {
	// The base token amount is denominated in base units of a token
	// Assume that there are 10 ** 18 base units for each token
	// Convert the base token amount to a whole token amount by representing the base token amount
	// as a fraction with the denominator set to 10 ** 18
	var rat *big.Rat
	if baseAmount != nil {
		maxDecimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		rat = new(big.Rat).SetFrac(baseAmount, maxDecimals)
	}

	return ratToFixed(rat, 100000)
}

func ratToFixed(rat *big.Rat, scalingFactor int64) (int64, error) {
	if rat == nil {
		return 0, fmt.Errorf("reference to rat is nil")
	}
	scaled := new(big.Rat).Mul(rat, big.NewRat(scalingFactor, 1))
	fp, _ := new(big.Float).SetRat(scaled).Int64()
	return fp, nil
}

// Package-level RNG; tests can override it.
var PkgRNG = rand.New(rand.NewSource(time.Now().UnixNano()))

// RandomIDGenerator generates random hexadecimal string of specified length
// defined as variable for unit tests
var RandomIDGenerator = func(length uint) string {
	return hex.EncodeToString(RandomBytesGenerator(length))
}

var RandomBytesGenerator = func(length uint) []byte {
	x := make([]byte, length, length)
	PkgRNG.Read(x)
	return x
}

// RandName generates random hexadecimal string
func RandName() string {
	return RandomIDGenerator(10)
}

var RandomUintUnder = func(max uint) uint {
	return uint(rand.Uint32()) % max
}

func RandomUint64() uint64 {
	return binary.LittleEndian.Uint64(RandomBytesGenerator(8))
}

func ToInt64(val *big.Int) int64 {
	if val.Cmp(big.NewInt(maxInt64)) > 0 {
		return maxInt64
	}

	return val.Int64()
}

func RatPriceInfo(priceInfo *net.PriceInfo) (*big.Rat, error) {
	if priceInfo == nil {
		return nil, nil
	}

	pixelsPerUnit := priceInfo.PixelsPerUnit
	if pixelsPerUnit == 0 {
		return nil, errors.New("pixels per unit is 0")
	}

	return big.NewRat(priceInfo.PricePerUnit, pixelsPerUnit), nil
}

func JoinURL(url, path string) string {
	if !strings.HasSuffix(url, "/") {
		return url + "/" + path
	}
	return url + path
}

// Read at most n bytes from an io.Reader
func ReadAtMost(r io.Reader, n int) ([]byte, error) {
	// Reading one extra byte to check if input Reader
	// had more than n bytes
	limitedReader := io.LimitReader(r, int64(n)+1)
	b, err := ioutil.ReadAll(limitedReader)
	if err == nil && len(b) > n {
		return nil, errors.New("input bigger than max buffer size")
	}
	return b, err
}

func getGPUDefault() ([]*gpu.GraphicsCard, error) {
	gpu, err := ghw.GPU()

	if err != nil {
		return nil, err
	}

	return gpu.GraphicsCards, nil
}

func getPCIDefault() ([]*pci.Device, error) {
	pci, err := ghw.PCI()

	if err != nil {
		return nil, err
	}

	return pci.ListDevices(), nil
}

var getGPU = getGPUDefault
var getPCI = getPCIDefault

func detectNvidiaDevices() ([]string, error) {
	nvidiaCardCount := 0
	re := regexp.MustCompile("(?i)nvidia") // case insensitive match

	cards, err := getGPU()
	if err != nil {
		return nil, err
	}

	if len(cards) != 0 {
		for _, card := range cards {
			if card.DeviceInfo != nil && re.MatchString(card.DeviceInfo.Vendor.Name) {
				nvidiaCardCount += 1
			}
		}
	} else { // on VMs gpu.GraphicsCards may be empty
		rePCI := regexp.MustCompile("(?i)display ?controller")

		pci, err := getPCI()
		if err != nil {
			return nil, err
		}

		for _, device := range pci {
			// Make sure that the current device is a graphics card.
			// On some VMs driver may be misreported as vfio-pci, try to rely on device.Class.Name with a "Display controller"
			// See: https://github.com/jaypipes/ghw/issues/314#issuecomment-1113334378
			if device.Vendor != nil && re.MatchString(device.Vendor.Name) && (re.MatchString(device.Driver) || rePCI.MatchString(device.Class.Name)) {
				nvidiaCardCount += 1
			}
		}
	}

	if nvidiaCardCount == 0 {
		return nil, errors.New("no devices found with vendor name 'Nvidia'")
	}

	devices := []string{}

	for i := 0; i < nvidiaCardCount; i++ {
		s := strconv.Itoa(i)
		devices = append(devices, s)
	}

	return devices, nil
}

func ParseAccelDevices(devices string, acceleration ffmpeg.Acceleration) ([]string, error) {
	if acceleration == ffmpeg.Nvidia && devices == "all" {
		return detectNvidiaDevices()
	}
	return strings.Split(devices, ","), nil
}

func ParseEthAddr(strJsonKey string) (string, error) {
	var keyJson map[string]interface{}
	if err := json.Unmarshal([]byte(strJsonKey), &keyJson); err == nil {
		if address, ok := keyJson["address"].(string); ok {
			return address, nil
		}
	}
	return "", errors.New("Error parsing address from keyfile")
}

// CalculateAudioDuration calculates audio file duration using the lpms/ffmpeg package.
func CalculateAudioDuration(audio types.File) (int64, error) {
	read, err := audio.Reader()
	if err != nil {
		return 0, err
	}
	defer read.Close()

	bytearr, _ := audio.Bytes()
	_, mediaFormat, err := ffmpeg.GetCodecInfoBytes(bytearr)
	if err != nil {
		return 0, errors.New("Error getting codec info")
	}

	duration := int64(mediaFormat.DurSecs)
	if duration <= 0 {
		return 0, ErrAudioDurationCalculation
	}

	return duration, nil
}

// ValidateServiceURI checks if the serviceURI is valid.
func ValidateServiceURI(serviceURI *url.URL) bool {
	return !strings.Contains(serviceURI.Host, "0.0.0.0")
}

// MimeTypeToExtension returns the file extension for a given MIME type.
func MimeTypeToExtension(mimeType string) (string, error) {
	mimeType = strings.ToLower(mimeType)
	if ext, ok := mime2ext[mimeType]; ok {
		return ext, nil
	}
	return "", ErrNoExtensionsForType
}

func AppendHostname(urlPath string, host string) (*url.URL, error) {
	if urlPath == "" {
		return nil, fmt.Errorf("invalid url from orch")
	}
	pu, err := url.Parse(urlPath)
	if err != nil {
		return nil, err
	}
	if pu.Hostname() != "" {
		// url has a hostname already so use it
		return pu, nil
	} else {
		// no hostname, so append one
		if !strings.HasPrefix(urlPath, "/") {
			urlPath = "/" + urlPath
		}
		u, err := url.Parse(host + urlPath)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
}
