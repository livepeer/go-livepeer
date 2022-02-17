package core

import (
	"errors"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
)

type Capability int
type CapabilityString []uint64
type Constraints struct{}
type Capabilities struct {
	bitstring   CapabilityString
	mandatories CapabilityString
	constraints Constraints
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
	Capability_SceneClassification
	Capability_MPEG7VideoSignature
	Capability_HEVC_Decode
	Capability_HEVC_Encode
	Capability_VP8_Decode
	Capability_VP9_Decode
	Capability_VP8_Encode
	Capability_VP9_Encode
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
	Capability_SceneClassification:        "Scene slassification",
	Capability_MPEG7VideoSignature:        "MPEG7 signature",
	Capability_HEVC_Decode:                "HEVC decode",
	Capability_HEVC_Encode:                "HEVC encode",
	Capability_VP8_Decode:                 "VP8 decode",
	Capability_VP9_Decode:                 "VP9 decode",
	Capability_VP8_Encode:                 "VP8 encode",
	Capability_VP9_Encode:                 "VP9 encode",
}

var CapabilityTestLookup = map[Capability]CapabilityTest{
	// 145x145 is the lowest resolution supported by NVENC on Windows
	Capability_H264: {
		inVideoData: testSegment_H264,
		outProfile:  ffmpeg.VideoProfile{Resolution: "145x145", Bitrate: "1000k", Format: ffmpeg.FormatMPEGTS},
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
	}
}

func OptionalCapabilities() []Capability {
	return []Capability{
		Capability_HEVC_Decode,
		Capability_HEVC_Encode,
		Capability_VP8_Decode,
		Capability_VP9_Decode,
	}
}

func ExperimentalCapabilities() []Capability {
	// Add experimental capabilities if enabled during build
	return experimentalCapabilities
}

func MandatoryOCapabilities() []Capability {
	// Add to this list as certain features become mandatory.
	// Use sparingly, as adding to this is a hard break with older nodes
	return []Capability{
		Capability_AuthToken,
	}
}

func NewCapabilityString(caps []Capability) CapabilityString {
	capStr := []uint64{}
	for _, v := range caps {
		if v <= Capability_Unused {
			continue
		}
		int_index := int(v) / 64 // floors automatically
		bit_index := int(v) % 64
		// grow capStr until it's of length int_index
		for len(capStr) <= int_index {
			capStr = append(capStr, 0)
		}
		capStr[int_index] |= uint64(1 << bit_index)
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

func JobCapabilities(params *StreamParameters) (*Capabilities, error) {
	caps := make(map[Capability]bool)

	// Define any default capabilities (especially ones that may be mandatory)
	caps[Capability_AuthToken] = true
	if params.VerificationFreq > 0 {
		caps[Capability_MPEG7VideoSignature] = true
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

	// capabilities based on detector profiles
	for _, profile := range params.Detection.Profiles {
		switch profile.Type() {
		case ffmpeg.SceneClassification:
			caps[Capability_SceneClassification] = true
		}
	}

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

func (bcast *Capabilities) CompatibleWith(orch *net.Capabilities) bool {
	// Ensure bcast and orch are compatible with one another.

	if bcast == nil {
		// Weird golang behavior: interface value can evaluate to non-nil
		// even if the underlying concrete type is nil.
		// cf. common.CapabilityComparator
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
	return &net.Capabilities{Bitstring: c.bitstring, Mandatories: c.mandatories}
}

func CapabilitiesFromNetCapabilities(caps *net.Capabilities) *Capabilities {
	if caps == nil {
		return nil
	}
	return &Capabilities{
		bitstring:   caps.Bitstring,
		mandatories: caps.Mandatories,
	}
}

func NewCapabilities(caps []Capability, m []Capability) *Capabilities {
	c := &Capabilities{}
	if len(caps) > 0 {
		c.bitstring = NewCapabilityString(caps)
	}
	if len(m) > 0 {
		c.mandatories = NewCapabilityString(m)
	}
	return c
}

func CapabilityToName(capability Capability) (string, error) {
	capName, found := CapabilityNameLookup[capability]
	if !found {
		return "", capUnknown
	}
	return capName, nil
}

func InArray(capability Capability, caps []Capability) bool {
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
	case net.OSInfo_S3:
		return Capability_StorageS3, nil
	case net.OSInfo_GOOGLE:
		return Capability_StorageGCS, nil
	case net.OSInfo_DIRECT:
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
