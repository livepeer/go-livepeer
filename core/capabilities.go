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
	constraints Constraints
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
)

var capFormatConv = errors.New("capability: unknown format")
var capStorageConv = errors.New("capability: unknown storage")
var capProfileConv = errors.New("capability: unknown profile")

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
	caps[Capability_H264] = true

	// capabilities based on requested output
	for _, v := range params.Profiles {
		// set format
		c, err := formatToCapability(v.Format)
		if err != nil {
			return nil, err
		}
		caps[c] = true

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
	}

	// capabilities based on broadacster or stream properties

	// set expected storage
	storageCap, err := storageToCapability(params.OS)
	if err != nil {
		return nil, err
	}
	caps[storageCap] = true

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
	return bcast.bitstring.CompatibleWith(orch.Bitstring)
}

func (c *Capabilities) ToNetCapabilities() *net.Capabilities {
	return &net.Capabilities{Bitstring: c.bitstring}
}

func NewCapabilities(caps []Capability) *Capabilities {
	return &Capabilities{bitstring: NewCapabilityString(caps)}
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
