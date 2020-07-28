package core

import (
	"context"
	"sort"
	"testing"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestCapability_NewString(t *testing.T) {
	assert := assert.New(t)

	// simple case
	str := NewCapabilityString([]Capability{-10, -1, 0, 1, 2, 3, 4, 5})
	assert.Equal(CapabilityString([]uint64{63}), str)

	// test skipping
	str = NewCapabilityString([]Capability{193, 192})
	assert.Equal(CapabilityString([]uint64{0, 0, 0, 3}), str)

	// out of order inserts
	str = NewCapabilityString([]Capability{193, 54, 192, 79})
	assert.Equal(CapabilityString([]uint64{1 << 54, 1 << 15, 0, 3}), str)

}

func TestCapability_CompatibleBitstring(t *testing.T) {
	// sanity check a simple case
	compatible := NewCapabilityString([]Capability{0, 1, 2, 3}).CompatibleWith([]uint64{15})
	assert.True(t, compatible)

	rapid.Check(t, func(t *rapid.T) {
		assert := assert.New(t) // in order to pick up the rapid rng

		// generate initial list of caps
		nbCaps := rapid.IntRange(0, 512).Draw(t, "nbCaps").(int)
		isSet := rapid.IntRange(0, 1)
		caps := []Capability{}
		for i := 0; i < nbCaps; i++ {
			if 1 == isSet.Draw(t, "isSet").(int) {
				caps = append(caps, Capability(i))
			}
		}

		// generate a subset of caps
		reductionSz := rapid.IntRange(0, len(caps)).Draw(t, "reductionSz").(int)
		subsetCaps := make([]Capability, len(caps))
		copy(subsetCaps, caps)
		for i := 0; i < reductionSz; i++ {
			// select an index k, and remove it
			k := rapid.IntRange(0, len(subsetCaps)-1).Draw(t, "k").(int)
			subsetCaps[k] = subsetCaps[len(subsetCaps)-1]
			subsetCaps = subsetCaps[:len(subsetCaps)-1]
		}
		assert.Len(subsetCaps, len(caps)-reductionSz) // sanity check

		c1 := NewCapabilityString(subsetCaps)
		c2 := NewCapabilityString(caps)

		// caps should be compatible with subset
		assert.True(c1.CompatibleWith(c2), "caps is not compatible with subset")

		if reductionSz > 0 {
			// subset should not be compatible with caps
			assert.False(c2.CompatibleWith(c1), "subset was compatible with caps")
		} else {
			assert.Equal(c2, c1)
		}
	})
}

func TestCapability_JobCapabilities(t *testing.T) {
	assert := assert.New(t)

	// TODO We really want to ensure that the *invariant* of having an
	// invalid configuration will make all other capabilities fail out,
	// regardless of the position in which the invalid setting is present.
	//
	// Similarly for valid configurations; most importantly, we need
	// to ensure that later configurations don't overwrite earlier
	// valid configs. Eg. one rendition has a format of MP4, another
	// has a format of mpegts, and a third has no format specified;
	// the output capability string should have both mpegts and mp4.
	// Regardless of ordering of the original inputs.
	//
	// Use a rapid check to facilitate this.

	checkSuccess := func(params *StreamParameters, caps []Capability) bool {
		jobCaps, err := JobCapabilities(params)
		ret := assert.Nil(err)
		expectedCaps := &Capabilities{bitstring: NewCapabilityString(caps)}
		ret = assert.Equal(expectedCaps, jobCaps) && ret
		return ret
	}

	// check with everything empty
	assert.True(checkSuccess(&StreamParameters{}, []Capability{
		Capability_H264,
	}), "failed with empty params")

	// check with everything enabled
	profs := []ffmpeg.VideoProfile{
		{Format: ffmpeg.FormatMPEGTS},
		{Format: ffmpeg.FormatMP4},
		{FramerateDen: 1},
		{Profile: ffmpeg.ProfileH264Main},
		{Profile: ffmpeg.ProfileH264High},
		{GOP: 1},
	}
	storageURI := "s3+http://K:P@localhost:9000/bucket"
	os, err := drivers.ParseOSURL(storageURI, false)
	assert.Nil(err)
	params := &StreamParameters{Profiles: profs, OS: os.NewSession("")}
	assert.True(checkSuccess(params, []Capability{
		Capability_H264,
		Capability_MP4,
		Capability_MPEGTS,
		Capability_FractionalFramerates,
		Capability_StorageS3,
		Capability_ProfileH264Main,
		Capability_ProfileH264High,
		Capability_GOP,
	}), "failed with everything enabled")

	// check fractional framerates
	params.Profiles = []ffmpeg.VideoProfile{{FramerateDen: 1}}
	params.OS = nil
	assert.True(checkSuccess(params, []Capability{
		Capability_H264,
		Capability_MPEGTS,
		Capability_FractionalFramerates,
	}), "failed with fractional framerates")

	// check error case with format
	params.Profiles = []ffmpeg.VideoProfile{{Format: -1}}
	_, err = JobCapabilities(params)
	assert.Equal(capFormatConv, err)

	// check error case with profiles
	params.Profiles = []ffmpeg.VideoProfile{{Profile: -1}}
	_, err = JobCapabilities(params)
	assert.Equal(capProfileConv, err)

	// check error case with storage
	params.Profiles = nil
	params.OS = &stubOS{storageType: -1}
	_, err = JobCapabilities(params)
	assert.Equal(capStorageConv, err)
}

func TestCapability_CompatibleWithNetCap(t *testing.T) {
	assert := assert.New(t)

	// sanity checks
	bcast := NewCapabilities(nil, nil)
	orch := NewCapabilities(nil, nil)
	assert.Nil(bcast.bitstring)
	assert.Nil(orch.bitstring)
	assert.True(bcast.CompatibleWith(orch.ToNetCapabilities()))
	assert.True(orch.CompatibleWith(bcast.ToNetCapabilities()))

	// orchestrator is not compatible with broadcaster - empty cap set
	bcast = NewCapabilities([]Capability{1}, nil)
	assert.Empty(orch.bitstring)
	assert.False(bcast.CompatibleWith(orch.ToNetCapabilities()))
	assert.True(orch.CompatibleWith(bcast.ToNetCapabilities())) // sanity check; not commutative

	// orchestrator is not compatible with broadcaster - different cap set
	orch = NewCapabilities([]Capability{2}, nil)
	assert.False(bcast.CompatibleWith(orch.ToNetCapabilities()))

	// B / O are equivalent
	orch = NewCapabilities([]Capability{1}, nil)
	assert.Equal(bcast.bitstring, orch.bitstring)
	assert.True(bcast.CompatibleWith(orch.ToNetCapabilities()))
	assert.True(orch.CompatibleWith(bcast.ToNetCapabilities()))

	// O supports a superset of B's capabilities
	orch = NewCapabilities([]Capability{1, 2}, nil)
	assert.True(bcast.CompatibleWith(orch.ToNetCapabilities()))

	// check a mandatory capability - no match
	mandatory := []Capability{3}
	orch = NewCapabilities([]Capability{1, 2, 3}, mandatory)
	assert.False(bcast.CompatibleWith(orch.ToNetCapabilities()))
	assert.False(orch.CompatibleWith(bcast.ToNetCapabilities()))

	// check a mandatory capability - match only the single mandatory capability
	assert.Equal(NewCapabilityString(mandatory), orch.mandatories)
	bcast = NewCapabilities(mandatory, nil)
	assert.True(bcast.CompatibleWith(orch.ToNetCapabilities()))

	// check a mandatory capability - match with B's multiple capabilities
	bcast = NewCapabilities([]Capability{1, 3}, nil)
	assert.True(bcast.CompatibleWith(orch.ToNetCapabilities()))

	// broadcaster "mandatory" capabilities have no effect during regular match
	orch = NewCapabilities(nil, nil)
	bcast = NewCapabilities(nil, []Capability{1})
	assert.True(bcast.CompatibleWith(orch.ToNetCapabilities()))
}

func TestCapability_RoundTrip_Net(t *testing.T) {
	// check invariant:
	// cap == CapabilitiesFromNetCapabilities(cap.ToNetCapabilies())
	// and vice versa:
	// net == CapabilitiesFromNetCapabilities(net).ToNetCapabilities()

	rapid.Check(t, func(t *rapid.T) {
		assert := assert.New(t) // in order to pick up the rapid rng

		makeCapList := func() []Capability {
			randCapsLen := rapid.IntRange(0, 256).Draw(t, "capLen").(int)
			randCaps := rapid.IntRange(0, 512)
			capList := []Capability{}
			for i := 0; i < randCapsLen; i++ {
				capList = append(capList, Capability(randCaps.Draw(t, "cap").(int)))
			}
			return capList
		}

		// cap == CapabilitiesFromNetCapabilities(cap.ToNetCapabilies())
		caps := NewCapabilities(makeCapList(), makeCapList())
		assert.Equal(caps, CapabilitiesFromNetCapabilities(caps.ToNetCapabilities()))

		// net == CapabilitiesFromNetCapabilities(net).ToNetCapabilities()
		netCaps := NewCapabilities(makeCapList(), makeCapList()).ToNetCapabilities()
		assert.Equal(netCaps, CapabilitiesFromNetCapabilities(netCaps).ToNetCapabilities())
	})
}

func TestCapability_FormatToCapability(t *testing.T) {
	assert := assert.New(t)
	// Ensure all ffmpeg-enumerated formats are represented during conversion
	for _, format := range ffmpeg.ExtensionFormats {
		_, err := formatToCapability(format)
		assert.Nil(err)
	}
	// ensure error is triggered for unrepresented values
	c, err := formatToCapability(-100)
	assert.Equal(Capability_Invalid, c)
	assert.Equal(capFormatConv, err)
}

const stubOSMagic = 0x1337

type stubOS struct {
	//storageType net.OSInfo_StorageType
	storageType int32
}

func (os *stubOS) GetInfo() *net.OSInfo {
	if os.storageType == stubOSMagic {
		return nil
	}
	return &net.OSInfo{StorageType: net.OSInfo_StorageType(os.storageType)}
}
func (os *stubOS) EndSession()                                                {}
func (os *stubOS) SaveData(string, []byte, map[string]string) (string, error) { return "", nil }
func (os *stubOS) IsExternal() bool                                           { return false }
func (os *stubOS) IsOwn(url string) bool                                      { return true }
func (os *stubOS) ListFiles(ctx context.Context, prefix, delim string) (drivers.PageInfo, error) {
	return nil, nil
}
func (os *stubOS) ReadData(ctx context.Context, name string) (*drivers.FileInfoReader, error) {
	return nil, nil
}
func (os *stubOS) OS() drivers.OSDriver {
	return nil
}

func TestCapability_StorageToCapability(t *testing.T) {
	assert := assert.New(t)
	for _, storageType := range net.OSInfo_StorageType_value {
		os := &stubOS{storageType: storageType}
		_, err := storageToCapability(os)
		assert.Nil(err)
	}

	// test error case
	c, err := storageToCapability(&stubOS{storageType: -1})
	assert.Equal(Capability_Invalid, c)
	assert.Equal(capStorageConv, err)

	// test unused caps
	c, err = storageToCapability(&stubOS{storageType: stubOSMagic})
	assert.Equal(Capability_Unused, c)
	assert.Nil(err)

	c, err = storageToCapability(nil)
	assert.Equal(Capability_Unused, c)
	assert.Nil(err)
}

func TestCapability_ProfileToCapability(t *testing.T) {
	assert := assert.New(t)
	caps := []Capability{Capability_Unused, Capability_ProfileH264Baseline, Capability_ProfileH264Main, Capability_ProfileH264High, Capability_ProfileH264ConstrainedHigh}

	// iterate through lpms-defined profiles to ensure all are accounted for
	// need to put into a slice and sort to ensure consistent ordering
	profs := []int{}
	for k, _ := range ffmpeg.ProfileParameters {
		profs = append(profs, int(k))
	}
	sort.Ints(profs)
	for i := range profs {
		c, err := profileToCapability(ffmpeg.Profile(profs[i]))
		assert.Nil(err)
		assert.Equal(caps[i], c)
	}

	// check invalid profile handling
	c, err := profileToCapability(-1)
	assert.Equal(Capability_Invalid, c)
	assert.Equal(capProfileConv, err)
}

func TestCapabilities_LegacyCheck(t *testing.T) {
	assert := assert.New(t)
	legacyLen := len(legacyCapabilities)
	caps := legacyCapabilities
	capStr := NewCapabilityString(caps)

	assert.True(capStr.CompatibleWith(legacyCapabilityString)) // sanity check

	// adding a non-legacy cap should fail the check
	caps = append(caps, Capability(50))
	capStr = NewCapabilityString(caps)
	assert.False(capStr.CompatibleWith(legacyCapabilityString))

	// having a subset of legacy caps should still be ok
	// TODO randomly select subset via rapid check
	caps = legacyCapabilities
	caps = caps[:len(caps)-1]
	caps = caps[:len(caps)-1]
	assert.Len(caps, len(legacyCapabilities)-2) // sanity check
	capStr = NewCapabilityString(caps)
	assert.True(capStr.CompatibleWith(legacyCapabilityString))

	assert.Len(legacyCapabilities, legacyLen) // sanity check no modifications
}
