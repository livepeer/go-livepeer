package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"
)

type Capability int
type CapabilityString []uint64
type Constraints struct {
	minVersion string
}
type Capabilities struct {
	bitstring   CapabilityString
	mandatories CapabilityString
	version     string
	constraints Constraints
	capacities  map[Capability]int
	mutex       sync.Mutex
}
type CapabilityTest struct {
	inVideoData []byte
	outProfile  ffmpeg.VideoProfile
}

// Do not rearrange these values! Only append.
const (
	Capability_Invalid Capability = iota - 2
	Capability_Unused
	Capability_H264
	Capability_MPEGTS
	Capability_MP4
	Capability_FractionalFramerates
	Capability_StorageDirect
	Capability_StorageS3
	Capability_StorageGCS
	Capability_ProfileH264Baseline
	Capability_ProfileH264Main
	Capability_ProfileH264High
	Capability_ProfileH264ConstrainedHigh
	Capability_GOP
	Capability_AuthToken
	Capability_SceneClassification // Deprecated, but can't remove because of Capability ordering
	Capability_MPEG7VideoSignature
	Capability_HEVC_Decode
	Capability_HEVC_Encode
	Capability_VP8_Decode
	Capability_VP9_Decode
	Capability_VP8_Encode
	Capability_VP9_Encode
	Capability_H264_Decode_444_8bit
	Capability_H264_Decode_422_8bit
	Capability_H264_Decode_444_10bit
	Capability_H264_Decode_422_10bit
	Capability_H264_Decode_420_10bit
	Capability_SegmentSlicing
)

var CapabilityNameLookup = map[Capability]string{
	Capability_Invalid:                    "Invalid",
	Capability_Unused:                     "Unused",
	Capability_H264:                       "H.264",
	Capability_MPEGTS:                     "MPEGTS",
	Capability_MP4:                        "MP4",
	Capability_FractionalFramerates:       "Fractional framerates",
	Capability_StorageDirect:              "Storage direct",
	Capability_StorageS3:                  "Storage S3",
	Capability_StorageGCS:                 "Storage GCS",
	Capability_ProfileH264Baseline:        "H264 Baseline profile",
	Capability_ProfileH264Main:            "H264 Main profile",
	Capability_ProfileH264High:            "H264 High profile",
	Capability_ProfileH264ConstrainedHigh: "H264 Constained High profile",
	Capability_GOP:                        "GOP",
	Capability_AuthToken:                  "Auth token",
	Capability_MPEG7VideoSignature:        "MPEG7 signature",
	Capability_HEVC_Decode:                "HEVC decode",
	Capability_HEVC_Encode:                "HEVC encode",
	Capability_VP8_Decode:                 "VP8 decode",
	Capability_VP9_Decode:                 "VP9 decode",
	Capability_VP8_Encode:                 "VP8 encode",
	Capability_VP9_Encode:                 "VP9 encode",
	Capability_H264_Decode_444_8bit:       "H264 Decode YUV444 8-bit",
	Capability_H264_Decode_422_8bit:       "H264 Decode YUV422 8-bit",
	Capability_H264_Decode_444_10bit:      "H264 Decode YUV444 10-bit",
	Capability_H264_Decode_422_10bit:      "H264 Decode YUV422 10-bit",
	Capability_H264_Decode_420_10bit:      "H264 Decode YUV420 10-bit",
	Capability_SegmentSlicing:             "Segment slicing",
}

var CapabilityTestLookup = map[Capability]CapabilityTest{
	// 145x145 is the lowest resolution supported by NVENC on Windows
	// Software encoder requires `width must be multiple of 2` so we use 146x146
	Capability_H264: {
		inVideoData: testSegment_H264,
		outProfile:  ffmpeg.VideoProfile{Resolution: "146x146", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_HEVC_Decode: {
		inVideoData: testSegment_HEVC,
		outProfile:  ffmpeg.VideoProfile{Resolution: "145x145", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_HEVC_Encode: {
		inVideoData: testSegment_H264,
		outProfile:  ffmpeg.VideoProfile{Resolution: "145x145", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS, Encoder: ffmpeg.H265},
	},
	Capability_VP8_Decode: {
		inVideoData: testSegment_VP8,
		outProfile:  ffmpeg.VideoProfile{Resolution: "145x145", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_VP9_Decode: {
		inVideoData: testSegment_VP9,
		outProfile:  ffmpeg.VideoProfile{Resolution: "145x145", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_H264_Decode_444_8bit: {
		inVideoData: testSegment_H264_444_8bit,
		outProfile:  ffmpeg.VideoProfile{Resolution: "146x146", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_H264_Decode_422_8bit: {
		inVideoData: testSegment_H264_422_8bit,
		outProfile:  ffmpeg.VideoProfile{Resolution: "146x146", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_H264_Decode_444_10bit: {
		inVideoData: testSegment_H264_444_10bit,
		outProfile:  ffmpeg.VideoProfile{Resolution: "146x146", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_H264_Decode_422_10bit: {
		inVideoData: testSegment_H264_422_10bit,
		outProfile:  ffmpeg.VideoProfile{Resolution: "146x146", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
	Capability_H264_Decode_420_10bit: {
		inVideoData: testSegment_H264_420_10bit,
		outProfile:  ffmpeg.VideoProfile{Resolution: "146x146", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
	},
}

var capFormatConv = errors.New("capability: unknown format")
var capStorageConv = errors.New("capability: unknown storage")
var capProfileConv = errors.New("capability: unknown profile")
var capCodecConv = errors.New("capability: unknown codec")
var capUnknown = errors.New("capability: unknown")

func DefaultCapabilities() []Capability {
	// Add to this list as new features are added.
	return []Capability{
		Capability_H264,
		Capability_MPEGTS,
		Capability_MP4,
		Capability_FractionalFramerates,
		Capability_StorageDirect,
		Capability_StorageS3,
		Capability_StorageGCS,
		Capability_ProfileH264Baseline,
		Capability_ProfileH264Main,
		Capability_ProfileH264High,
		Capability_ProfileH264ConstrainedHigh,
		Capability_GOP,
		Capability_AuthToken,
		Capability_MPEG7VideoSignature,
		Capability_SegmentSlicing,
	}
}

func OptionalCapabilities() []Capability {
	return []Capability{
		Capability_HEVC_Decode,
		Capability_HEVC_Encode,
		Capability_VP8_Decode,
		Capability_VP9_Decode,
		Capability_H264_Decode_444_8bit,
		Capability_H264_Decode_422_8bit,
		Capability_H264_Decode_444_10bit,
		Capability_H264_Decode_422_10bit,
		Capability_H264_Decode_420_10bit,
	}
}

func MandatoryOCapabilities() []Capability {
	// Add to this list as certain features become mandatory.
	// Use sparingly, as adding to this is a hard break with older nodes
	return []Capability{
		Capability_AuthToken,
	}
}

func RemoveCapability(haystack []Capability, needle Capability) []Capability {
	for i, c := range haystack {
		if c == needle {
			// TODO use slices.Delete once go-livepeer updates to latest golang
			return append(haystack[:i], haystack[i+1:]...)
		}
	}
	return haystack
}

func AddCapability(caps []Capability, newCap Capability) []Capability {
	return append(caps, newCap)
}

func NewCapabilityString(caps []Capability) CapabilityString {
	capStr := CapabilityString{}
	for _, v := range caps {
		if v <= Capability_Unused {
			continue
		}
		capStr.addCapability(v)
	}
	return capStr
}

func (c1 CapabilityString) CompatibleWith(c2 CapabilityString) bool {
	// checks: ( c1 AND c2 ) == c1
	if len(c1) > len(c2) {
		return false
	}
	for i := range c1 {
		if (c1[i] & c2[i]) != c1[i] {
			return false
		}
	}
	return true
}

type chromaDepth struct {
	Chroma ffmpeg.ChromaSubsampling
	Depth  ffmpeg.ColorDepthBits
}

var cap_420_8bit = chromaDepth{ffmpeg.ChromaSubsampling420, ffmpeg.ColorDepth8Bit}
var cap_444_8bit = chromaDepth{ffmpeg.ChromaSubsampling444, ffmpeg.ColorDepth8Bit}
var cap_422_8bit = chromaDepth{ffmpeg.ChromaSubsampling422, ffmpeg.ColorDepth8Bit}
var cap_444_10bit = chromaDepth{ffmpeg.ChromaSubsampling444, ffmpeg.ColorDepth10Bit}
var cap_422_10bit = chromaDepth{ffmpeg.ChromaSubsampling422, ffmpeg.ColorDepth10Bit}
var cap_420_10bit = chromaDepth{ffmpeg.ChromaSubsampling420, ffmpeg.ColorDepth10Bit}

func JobCapabilities(params *StreamParameters, segPar *SegmentParameters) (*Capabilities, error) {
	caps := make(map[Capability]bool)

	// Define any default capabilities (especially ones that may be mandatory)
	caps[Capability_AuthToken] = true
	if params.VerificationFreq > 0 {
		caps[Capability_MPEG7VideoSignature] = true
	}
	if segPar != nil {
		caps[Capability_SegmentSlicing] = true
	}

	// capabilities based on given input
	switch params.Codec {
	case ffmpeg.H264:
		chromaSubsampling, colorDepth, formatError := params.PixelFormat.Properties()
		caps[Capability_H264] = true
		if formatError == nil {
			feature := chromaDepth{chromaSubsampling, colorDepth}
			switch feature {
			case cap_444_8bit:
				caps[Capability_H264_Decode_444_8bit] = true
			case cap_422_8bit:
				caps[Capability_H264_Decode_422_8bit] = true
			case cap_444_10bit:
				caps[Capability_H264_Decode_444_10bit] = true
			case cap_422_10bit:
				caps[Capability_H264_Decode_422_10bit] = true
			case cap_420_10bit:
				caps[Capability_H264_Decode_420_10bit] = true
			case cap_420_8bit:
			default:
				return nil, fmt.Errorf("capability: unsupported pixel format chroma=%d colorBits=%d", chromaSubsampling, colorDepth)
			}
		}
	}

	// capabilities based on requested output
	for _, v := range params.Profiles {
		// set format
		c, err := formatToCapability(v.Format)
		if err != nil {
			return nil, err
		}
		caps[c] = true

		// set encoder
		encodeCap, err := outputCodecToCapability(v.Encoder)
		if err != nil {
			return nil, err
		}
		caps[encodeCap] = true

		// fractional framerates
		if v.FramerateDen > 0 {
			caps[Capability_FractionalFramerates] = true
		}

		// set profiles
		c, err = profileToCapability(v.Profile)
		if err != nil {
			return nil, err
		}
		caps[c] = true

		// gop
		if v.GOP != 0 {
			caps[Capability_GOP] = true
		}
	}

	// capabilities based on broadacster or stream properties

	// set expected storage
	storageCap, err := storageToCapability(params.OS)
	if err != nil {
		return nil, err
	}
	caps[storageCap] = true

	// capabilities based on detected input codec
	decodeCap, err := inputCodecToCapability(params.Codec)
	if err != nil {
		return nil, err
	}
	caps[decodeCap] = true

	// generate bitstring
	capList := []Capability{}
	for k := range caps {
		capList = append(capList, k)
	}

	return &Capabilities{bitstring: NewCapabilityString(capList)}, nil
}

func (bcast *Capabilities) LivepeerVersionCompatibleWith(orch *net.Capabilities) bool {
	if bcast == nil || orch == nil || bcast.constraints.minVersion == "" {
		// should not happen, but just in case, return true by default
		return true
	}
	if orch.Version == "" || orch.Version == "undefined" {
		// Orchestrator/Transcoder version is not set, so it's incompatible
		return false
	}

	minVer, err := semver.NewVersion(bcast.constraints.minVersion)
	if err != nil {
		glog.Warningf("error while parsing minVersion: %v", err)
		return true
	}
	ver, err := semver.NewVersion(orch.Version)
	if err != nil {
		glog.Warningf("error while parsing version: %v", err)
		return false
	}

	// Ignore prerelease versions as in go-livepeer we actually define post-release suffixes
	minVerNoSuffix, _ := minVer.SetPrerelease("")
	verNoSuffix, _ := ver.SetPrerelease("")

	return !verNoSuffix.LessThan(&minVerNoSuffix)
}

func (bcast *Capabilities) CompatibleWith(orch *net.Capabilities) bool {
	// Ensure bcast and orch are compatible with one another.

	if bcast == nil {
		// Weird golang behavior: interface value can evaluate to non-nil
		// even if the underlying concrete type is nil.
		// cf. common.CapabilityComparator
		return false
	}
	if !bcast.LivepeerVersionCompatibleWith(orch) {
		return false
	}

	// For now, check this:
	// ( orch.mandatories  AND bcast.bitstring ) == orch.mandatories &&
	// ( bcast.bitstring   AND orch.bitstring  ) == bcast.bitstring

	// TODO Can simplify later on to:
	// ( bcast.bitstring AND orch.bitstring ) == ( bcast.bistring OR orch.mandatories )

	if !CapabilityString(orch.Mandatories).CompatibleWith(bcast.bitstring) {
		return false
	}

	return bcast.bitstring.CompatibleWith(orch.Bitstring)
}

func (c *Capabilities) ToNetCapabilities() *net.Capabilities {
	if c == nil {
		return nil
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	netCaps := &net.Capabilities{Bitstring: c.bitstring, Mandatories: c.mandatories, Version: c.version, Capacities: make(map[uint32]uint32), Constraints: &net.Capabilities_Constraints{MinVersion: c.constraints.minVersion}}
	for capability, capacity := range c.capacities {
		netCaps.Capacities[uint32(capability)] = uint32(capacity)
	}
	return netCaps
}

func CapabilitiesFromNetCapabilities(caps *net.Capabilities) *Capabilities {
	if caps == nil {
		return nil
	}
	coreCaps := &Capabilities{
		bitstring:   caps.Bitstring,
		mandatories: caps.Mandatories,
		capacities:  make(map[Capability]int),
		version:     caps.Version,
		constraints: Constraints{minVersion: caps.Constraints.GetMinVersion()},
	}
	if caps.Capacities == nil || len(caps.Capacities) == 0 {
		// build capacities map if not present (struct received from previous versions)
		for arrIdx := 0; arrIdx < len(caps.Bitstring); arrIdx++ {
			for capIdx := 0; capIdx < 64; capIdx++ {
				capInt := arrIdx*64 + capIdx
				if caps.Bitstring[arrIdx]&uint64(1<<capIdx) > 0 {
					coreCaps.capacities[Capability(capInt)] = 1
				}
			}
		}
	} else {
		for capabilityInt, capacity := range caps.Capacities {
			coreCaps.capacities[Capability(capabilityInt)] = int(capacity)
		}
	}
	return coreCaps
}

func NewCapabilities(caps []Capability, m []Capability) *Capabilities {
	c := &Capabilities{capacities: make(map[Capability]int), version: LivepeerVersion}
	if len(caps) > 0 {
		c.bitstring = NewCapabilityString(caps)
		// initialize capacities to 1 by default, mandatory capabilities doesn't have capacities
		for _, capability := range caps {
			c.capacities[capability] = 1
		}
	}
	if len(m) > 0 {
		c.mandatories = NewCapabilityString(m)
	}
	return c
}

func (cap *Capabilities) AddCapacity(newCaps *Capabilities) {
	cap.mutex.Lock()
	defer cap.mutex.Unlock()
	for capability, capacity := range newCaps.capacities {
		curCapacity, e := cap.capacities[capability]
		if !e {
			cap.capacities[capability] = 0
		}
		cap.capacities[capability] = curCapacity + capacity
		arrIdx := int(capability) / 64
		bitIdx := int(capability) % 64
		if arrIdx >= len(cap.bitstring) {
			cap.bitstring = append(cap.bitstring, 0)
		}
		cap.bitstring[arrIdx] |= uint64(1 << bitIdx)
	}
}

func (cap *Capabilities) RemoveCapacity(goneCaps *Capabilities) {
	cap.mutex.Lock()
	defer cap.mutex.Unlock()
	for capability, capacity := range goneCaps.capacities {
		curCapacity, e := cap.capacities[capability]
		if !e {
			continue
		}
		newCapacity := curCapacity - capacity
		if newCapacity <= 0 {
			delete(cap.capacities, capability)
			cap.bitstring.removeCapability(capability)
		} else {
			cap.capacities[capability] = newCapacity
		}
	}
}

func (capStr *CapabilityString) removeCapability(capability Capability) {
	arrIdx := int(capability) / 64 // floors automatically
	bitIdx := int(capability) % 64
	if arrIdx >= len(*capStr) {
		// don't have this capability byte
		return
	}
	(*capStr)[arrIdx] &= ^uint64(1 << bitIdx)
}

func (capStr *CapabilityString) addCapability(capability Capability) {
	int_index := int(capability) / 64 // floors automatically
	bit_index := int(capability) % 64
	// grow capStr until it's of length int_index
	for len(*capStr) <= int_index {
		*capStr = append(*capStr, 0)
	}
	(*capStr)[int_index] |= uint64(1 << bit_index)
}

func CapabilityToName(capability Capability) (string, error) {
	capName, found := CapabilityNameLookup[capability]
	if !found {
		return "", capUnknown
	}
	return capName, nil
}

func HasCapability(caps []Capability, capability Capability) bool {
	for _, c := range caps {
		if capability == c {
			return true
		}
	}
	return false
}

func inputCodecToCapability(codec ffmpeg.VideoCodec) (Capability, error) {
	switch codec {
	case ffmpeg.H264:
		return Capability_H264, nil
	case ffmpeg.H265:
		return Capability_HEVC_Decode, nil
	case ffmpeg.VP8:
		return Capability_VP8_Decode, nil
	case ffmpeg.VP9:
		return Capability_VP9_Decode, nil
	}
	return Capability_Invalid, capCodecConv
}

func outputCodecToCapability(codec ffmpeg.VideoCodec) (Capability, error) {
	switch codec {
	case ffmpeg.H264:
		return Capability_H264, nil
	case ffmpeg.H265:
		return Capability_HEVC_Encode, nil
	case ffmpeg.VP8:
		return Capability_VP8_Encode, nil
	case ffmpeg.VP9:
		return Capability_VP9_Encode, nil
	}
	return Capability_Invalid, capCodecConv
}

func formatToCapability(format ffmpeg.Format) (Capability, error) {
	switch format {
	case ffmpeg.FormatNone:
		return Capability_MPEGTS, nil
	case ffmpeg.FormatMPEGTS:
		return Capability_MPEGTS, nil
	case ffmpeg.FormatMP4:
		return Capability_MP4, nil
	}
	return Capability_Invalid, capFormatConv
}

func storageToCapability(os drivers.OSSession) (Capability, error) {
	if os == nil || os.GetInfo() == nil {
		return Capability_Unused, nil // unused
	}
	switch os.GetInfo().StorageType {
	case drivers.OSInfo_S3:
		return Capability_StorageS3, nil
	case drivers.OSInfo_GOOGLE:
		return Capability_StorageGCS, nil
	case drivers.OSInfo_DIRECT:
		return Capability_StorageDirect, nil
	}
	return Capability_Invalid, capStorageConv
}

func profileToCapability(profile ffmpeg.Profile) (Capability, error) {
	switch profile {
	case ffmpeg.ProfileNone:
		return Capability_Unused, nil
	case ffmpeg.ProfileH264Baseline:
		return Capability_ProfileH264Baseline, nil
	case ffmpeg.ProfileH264Main:
		return Capability_ProfileH264Main, nil
	case ffmpeg.ProfileH264High:
		return Capability_ProfileH264High, nil
	case ffmpeg.ProfileH264ConstrainedHigh:
		return Capability_ProfileH264ConstrainedHigh, nil
	}
	return Capability_Invalid, capProfileConv
}

// Fixed forever - don't change this list unless removing interoperability
// with nodes that don't support capability discovery
// (in which case, just remove everything)
var legacyCapabilities = []Capability{
	Capability_H264,
	Capability_MPEGTS,
	Capability_MP4,
	Capability_FractionalFramerates,
	Capability_StorageDirect,
	Capability_StorageS3,
	Capability_StorageGCS,
	Capability_ProfileH264Baseline,
	Capability_ProfileH264Main,
	Capability_ProfileH264High,
	Capability_ProfileH264ConstrainedHigh,
	Capability_GOP,
}
var legacyCapabilityString = NewCapabilityString(legacyCapabilities)

func (bcast *Capabilities) LegacyOnly() bool {
	if bcast == nil {
		// Weird golang behavior: interface value can evaluate to non-nil
		// even if the underlying concrete type is nil.
		// cf. common.CapabilityComparator
		return false
	}
	return bcast.bitstring.CompatibleWith(legacyCapabilityString)
}

func (bcast *Capabilities) SetMinVersionConstraint(minVersionConstraint string) {
	if bcast != nil {
		bcast.constraints.minVersion = minVersionConstraint
	}
}

func (bcast *Capabilities) MinVersionConstraint() string {
	if bcast != nil {
		return bcast.constraints.minVersion
	}
	return ""
}
