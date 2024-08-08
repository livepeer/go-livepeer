package core

import (
	"context"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestCapability_Capacities(t *testing.T) {
	assert := assert.New(t)
	// create capabilities
	capabilities := []Capability{128, 2, 3}
	mandatory := []Capability{4}
	caps := NewCapabilities(capabilities, mandatory)
	// convert to net.Capabilities and back
	caps = CapabilitiesFromNetCapabilities(caps.ToNetCapabilities())
	// check all capacities present and set to 1
	assert.Equal(len(caps.capacities), len(capabilities))
	for _, c := range capabilities {
		v, exist := caps.capacities[c]
		assert.True(exist)
		assert.Equal(1, v)
	}
	// test increase capacity
	newCaps := NewCapabilities([]Capability{2, 3, 5}, mandatory)
	newCaps.capacities[2] = 2
	newCaps.capacities[5] = 2
	caps.AddCapacity(newCaps)
	assert.Equal(3, caps.capacities[2])
	assert.Equal(2, caps.capacities[5])
	// check new capability appeared in bitstring
	assert.True(caps.bitstring[0]&uint64(1<<5) > 0)
	// test decrease capacity
	caps.RemoveCapacity(newCaps)
	assert.Equal(1, caps.capacities[2])
	// check new cap is gone from capacities and bitstring
	_, exists := caps.capacities[5]
	assert.False(exists)
	assert.True(caps.bitstring[0]&uint64(1<<5) == 0)
	// decrease again and check only capability 1 left
	caps.RemoveCapacity(newCaps)
	assert.Equal(1, len(caps.capacities))
	assert.Equal(1, caps.capacities[128])
	assert.True(caps.bitstring[0] == 0)
	assert.True(caps.bitstring[1] == 0)
	assert.True(caps.bitstring[2] == 1)
	// check compatibility
	caps = NewCapabilities(capabilities, mandatory)
	netCapsLegacy := caps.ToNetCapabilities()
	netCapsLegacy.Capacities = nil
	legacyCaps := CapabilitiesFromNetCapabilities(netCapsLegacy)
	for _, c := range capabilities {
		v, exist := legacyCaps.capacities[c]
		assert.True(exist)
		assert.Equal(1, v)
	}
}

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
		nbCaps := rapid.IntRange(0, 512).Draw(t, "nbCaps")
		isSet := rapid.IntRange(0, 1)
		caps := []Capability{}
		for i := 0; i < nbCaps; i++ {
			if 1 == isSet.Draw(t, "isSet") {
				caps = append(caps, Capability(i))
			}
		}

		// generate a subset of caps
		reductionSz := rapid.IntRange(0, len(caps)).Draw(t, "reductionSz")
		subsetCaps := make([]Capability, len(caps))
		copy(subsetCaps, caps)
		for i := 0; i < reductionSz; i++ {
			// select an index k, and remove it
			k := rapid.IntRange(0, len(subsetCaps)-1).Draw(t, "k")
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

// We need this in order to call `NvidiaTranscoder::Transcode()` properly
func setupWorkDir(t *testing.T) (string, func()) {
	tmp := t.TempDir()
	WorkDir = tmp
	cleanup := func() {
		WorkDir = ""
	}
	return tmp, cleanup
}

func TestCapability_TranscoderCapabilities(t *testing.T) {
	tmpdir, cleanup := setupWorkDir(t)
	defer cleanup()

	// nvidia test
	devices, err := common.ParseAccelDevices("all", ffmpeg.Nvidia)
	devicesAvailable := err == nil && len(devices) > 0
	if devicesAvailable {
		nvidiaCaps, err := TestTranscoderCapabilities(devices, NewNvidiaTranscoder)
		assert.Nil(t, err)
		assert.False(t, HasCapability(nvidiaCaps, Capability_H264_Decode_444_8bit), "Nvidia device should not support decode of 444_8bit")
		assert.False(t, HasCapability(nvidiaCaps, Capability_H264_Decode_422_8bit), "Nvidia device should not support decode of 422_8bit")
		assert.False(t, HasCapability(nvidiaCaps, Capability_H264_Decode_444_10bit), "Nvidia device should not support decode of 444_10bit")
		assert.False(t, HasCapability(nvidiaCaps, Capability_H264_Decode_422_10bit), "Nvidia device should not support decode of 422_10bit")
		assert.False(t, HasCapability(nvidiaCaps, Capability_H264_Decode_420_10bit), "Nvidia device should not support decode of 420_10bit")
	}

	// Same test with software transcoder:
	softwareCaps, err := TestSoftwareTranscoderCapabilities(tmpdir)
	assert.Nil(t, err)
	// Software transcoder supports: [h264_444_8bit h264_422_8bit h264_444_10bit h264_422_10bit h264_420_10bit]
	assert.True(t, HasCapability(softwareCaps, Capability_H264_Decode_444_8bit), "software decoder should support 444_8bit input")
	assert.True(t, HasCapability(softwareCaps, Capability_H264_Decode_422_8bit), "software decoder should support 422_8bit input")
	assert.True(t, HasCapability(softwareCaps, Capability_H264_Decode_444_10bit), "software decoder should support 444_10bit input")
	assert.True(t, HasCapability(softwareCaps, Capability_H264_Decode_422_10bit), "software decoder should support 422_10bit input")
	assert.True(t, HasCapability(softwareCaps, Capability_H264_Decode_420_10bit), "software decoder should support 420_10bit input")
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
		jobCaps, err := JobCapabilities(params, nil)
		ret := assert.Nil(err)
		expectedCaps := &Capabilities{bitstring: NewCapabilityString(caps)}
		ret = assert.Equal(expectedCaps, jobCaps) && ret
		return ret
	}

	checkPixelFormat := func(constValue int, expected []Capability) bool {
		streamParams := &StreamParameters{Codec: ffmpeg.H264, PixelFormat: ffmpeg.PixelFormat{RawValue: constValue}}
		jobCaps, err := JobCapabilities(streamParams, nil)
		ret := assert.Nil(err)
		expectedCaps := &Capabilities{bitstring: NewCapabilityString(expected)}
		ret = assert.Equal(jobCaps, expectedCaps, "failed decode capability check") && ret
		return ret
	}
	// Capability_AuthToken appears to be mandatory
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV420P, []Capability{Capability_AuthToken, Capability_H264}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUYV422, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_422_8bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV422P, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_422_8bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV444P, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_444_8bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatUYVY422, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_422_8bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatNV12, []Capability{Capability_AuthToken, Capability_H264}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatNV21, []Capability{Capability_AuthToken, Capability_H264}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV420P10BE, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_420_10bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV420P10LE, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_420_10bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV422P10BE, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_422_10bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV422P10LE, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_422_10bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV444P10BE, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_444_10bit}))
	assert.True(checkPixelFormat(ffmpeg.PixelFormatYUV444P10LE, []Capability{Capability_AuthToken, Capability_H264, Capability_H264_Decode_444_10bit}))

	// check with everything empty
	assert.True(checkSuccess(&StreamParameters{}, []Capability{
		Capability_H264,
		Capability_AuthToken,
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
		Capability_AuthToken,
	}), "failed with everything enabled")

	// check fractional framerates
	params.Profiles = []ffmpeg.VideoProfile{{FramerateDen: 1}}
	params.OS = nil
	assert.True(checkSuccess(params, []Capability{
		Capability_H264,
		Capability_MPEGTS,
		Capability_FractionalFramerates,
		Capability_AuthToken,
	}), "failed with fractional framerates")

	// check MPEG7VideoSignature
	params.VerificationFreq = 1
	assert.True(checkSuccess(params, []Capability{
		Capability_H264,
		Capability_MPEGTS,
		Capability_FractionalFramerates,
		Capability_AuthToken,
		Capability_MPEG7VideoSignature,
	}), "failed with fast verification enabled")

	// check error case with format
	params.Profiles = []ffmpeg.VideoProfile{{Format: -1}}
	_, err = JobCapabilities(params, nil)
	assert.Equal(capFormatConv, err)

	// check error case with profiles
	params.Profiles = []ffmpeg.VideoProfile{{Profile: -1}}
	_, err = JobCapabilities(params, nil)
	assert.Equal(capProfileConv, err)

	// check error case with storage
	params.Profiles = nil
	params.OS = &stubOS{storageType: -1}
	_, err = JobCapabilities(params, nil)
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

	// broadcaster is not compatible with orchestrator - old O's version
	orch = NewCapabilities(nil, nil)
	bcast = NewCapabilities(nil, nil)
	bcast.constraints.minVersion = "0.4.1"
	orch.version = "0.4.0"
	assert.False(bcast.CompatibleWith(orch.ToNetCapabilities()))

	// broadcaster is not compatible with orchestrator - the same version
	orch = NewCapabilities(nil, nil)
	bcast = NewCapabilities(nil, nil)
	bcast.constraints.minVersion = "0.4.1"
	orch.version = "0.4.1"
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
			randCapsLen := rapid.IntRange(0, 256).Draw(t, "capLen")
			randCaps := rapid.IntRange(0, 512)
			capList := []Capability{}
			for i := 0; i < randCapsLen; i++ {
				capList = append(capList, Capability(randCaps.Draw(t, "cap")))
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

func (os *stubOS) GetInfo() *drivers.OSInfo {
	if os.storageType == stubOSMagic {
		return nil
	}
	return &drivers.OSInfo{StorageType: drivers.OSInfo_StorageType(os.storageType)}
}
func (os *stubOS) EndSession() {}
func (os *stubOS) SaveData(context.Context, string, io.Reader, map[string]string, time.Duration) (string, error) {
	return "", nil
}
func (os *stubOS) IsExternal() bool      { return false }
func (os *stubOS) IsOwn(url string) bool { return true }
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

func TestCapability_RemoveCapability(t *testing.T) {
	tests := []struct {
		name     string
		caps     []Capability
		toRemove Capability
		expect   []Capability
	}{{
		name:     "empty capability list",
		caps:     nil,
		toRemove: Capability_H264,
		expect:   nil,
	}, {
		name:     "capability not in list",
		caps:     []Capability{Capability_H264, Capability_MPEGTS},
		toRemove: Capability_MP4,
		expect:   []Capability{Capability_H264, Capability_MPEGTS},
	}, {
		name:     "capability at beginning of list",
		caps:     []Capability{Capability_H264, Capability_MPEGTS},
		toRemove: Capability_H264,
		expect:   []Capability{Capability_MPEGTS},
	}, {
		name:     "capability in middle of list",
		caps:     []Capability{Capability_H264, Capability_MP4, Capability_MPEGTS},
		toRemove: Capability_MP4,
		expect:   []Capability{Capability_H264, Capability_MPEGTS},
	}, {
		name:     "capability at end of list",
		caps:     []Capability{Capability_H264, Capability_MPEGTS},
		toRemove: Capability_MPEGTS,
		expect:   []Capability{Capability_H264},
	}, {
		name:     "last capability",
		caps:     []Capability{Capability_H264},
		toRemove: Capability_H264,
		expect:   []Capability{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, RemoveCapability(tt.caps, tt.toRemove))
		})
	}
}

func TestLiveeerVersionCompatibleWith(t *testing.T) {
	tests := []struct {
		name                  string
		broadcasterMinVersion string
		transcoderVersion     string
		expected              bool
	}{
		{
			name:                  "broadcaster required version is the same as the transcoder version",
			broadcasterMinVersion: "0.4.1",
			transcoderVersion:     "0.4.1",
			expected:              true,
		},
		{
			name:                  "broadcaster required version is less than the transcoder version",
			broadcasterMinVersion: "0.4.0",
			transcoderVersion:     "0.4.1",
			expected:              true,
		},
		{
			name:                  "broadcaster required version is more than the transcoder version",
			broadcasterMinVersion: "0.4.2",
			transcoderVersion:     "0.4.1",
			expected:              false,
		},
		{
			name:                  "broadcaster required version is the same as the transcoder dirty version",
			broadcasterMinVersion: "0.4.1",
			transcoderVersion:     "0.4.1-b3278dce-dirty",
			expected:              true,
		},
		{
			name:                  "broadcaster required version is before the transcoder dirty version",
			broadcasterMinVersion: "0.4.0",
			transcoderVersion:     "0.4.1-b3278dce-dirty",
			expected:              true,
		},
		{
			name:                  "broadcaster required version is after the transcoder dirty version",
			broadcasterMinVersion: "0.4.2",
			transcoderVersion:     "0.4.1-b3278dce-dirty",
			expected:              false,
		},
		{
			name:                  "broadcaster required version is empty",
			broadcasterMinVersion: "",
			transcoderVersion:     "0.4.1",
			expected:              true,
		},
		{
			name:                  "both versions are undefined",
			broadcasterMinVersion: "",
			transcoderVersion:     "",
			expected:              true,
		},
		{
			name:                  "transcoder version is empty",
			broadcasterMinVersion: "0.4.0",
			transcoderVersion:     "",
			expected:              false,
		},
		{
			name:                  "transcoder version is undefined",
			broadcasterMinVersion: "0.4.0",
			transcoderVersion:     "undefined",
			expected:              false,
		},
		{
			name:                  "unparsable broadcaster's min version",
			broadcasterMinVersion: "nonparsablesemversion",
			transcoderVersion:     "0.4.1",
			expected:              true,
		},
		{
			name:                  "unparsable transcoder's version",
			broadcasterMinVersion: "0.4.1",
			transcoderVersion:     "nonparsablesemversion",
			expected:              false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bCapabilities := &Capabilities{constraints: Constraints{minVersion: tt.broadcasterMinVersion}}
			tCapabilities := &Capabilities{version: tt.transcoderVersion}
			assert.Equal(t, tt.expected, bCapabilities.LivepeerVersionCompatibleWith(tCapabilities.ToNetCapabilities()))
		})
	}
}
