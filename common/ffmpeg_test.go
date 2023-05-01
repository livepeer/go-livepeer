package common

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/gpu"
	"github.com/jaypipes/ghw/pkg/pci"
	"github.com/jaypipes/pcidb"
	"github.com/livepeer/go-livepeer/net"
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
			Name:         "prof1",
			Bitrate:      "432k",
			Framerate:    uint(560),
			Resolution:   "123x456",
			Profile:      ffmpeg.ProfileH264Main,
			GOP:          123 * 1000000, //  milliseconds (in nanoseconds)
			ColorDepth:   ffmpeg.ColorDepth8Bit,
			ChromaFormat: ffmpeg.ChromaSubsampling420,
		},
		ffmpeg.VideoProfile{
			Name:         "prof2",
			Bitrate:      "765k",
			Framerate:    uint(876),
			FramerateDen: uint(12),
			Resolution:   "456x987",
			GOP:          -100,
			ColorDepth:   ffmpeg.ColorDepth10Bit,
			ChromaFormat: ffmpeg.ChromaSubsampling444,
		},
	}

	// empty name should return automatically generated name
	profiles[0].Name = ""
	fullProfiles, err := FFmpegProfiletoNetProfile(profiles)

	assert.Equal(fullProfiles[0].ColorDepth, int32(ffmpeg.ColorDepth8Bit))
	assert.Equal(fullProfiles[1].ColorDepth, int32(ffmpeg.ColorDepth10Bit))
	assert.Equal(fullProfiles[0].ChromaFormat, net.VideoProfile_CHROMA_420)
	assert.Equal(fullProfiles[1].ChromaFormat, net.VideoProfile_CHROMA_444)
	profiles[0].ColorDepth = ffmpeg.ColorDepth12Bit
	profiles[0].ChromaFormat = ffmpeg.ChromaSubsampling422
	profiles[1].ColorDepth = ffmpeg.ColorDepth16Bit

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

	assert.Equal(fullProfiles[0].ColorDepth, int32(ffmpeg.ColorDepth12Bit))
	assert.Equal(fullProfiles[0].ChromaFormat, net.VideoProfile_CHROMA_422)
	assert.Equal(fullProfiles[1].ColorDepth, int32(ffmpeg.ColorDepth16Bit))

	// Empty bitrate should return parsing error
	profiles[0].Bitrate = ""
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Contains(err.Error(), "strconv.Atoi: parsing")
	profiles[0].Bitrate = "432k"

	// Empty resolution should return ErrTranscoderRes
	profiles[0].Resolution = ""
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Equal(ffmpeg.ErrTranscoderRes, err)
	profiles[0].Resolution = "123x456"

	// Unset format should be mpegts by default
	assert.Equal(profiles[0].Format, ffmpeg.FormatNone)
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)
	assert.Equal(fullProfiles[0].Format, net.VideoProfile_MPEGTS)

	profiles[0].Format = ffmpeg.FormatMP4
	profiles[1].Format = ffmpeg.FormatMPEGTS
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)
	assert.Equal(fullProfiles[0].Format, net.VideoProfile_MP4)
	assert.Equal(fullProfiles[1].Format, net.VideoProfile_MPEGTS)

	// Verify FPS denominator behaviour
	assert.Equal(fullProfiles[0].FpsDen, uint32(0))
	assert.Equal(fullProfiles[1].FpsDen, uint32(profiles[1].FramerateDen))

	// Verify Encoder profile behaviour
	assert.Equal(fullProfiles[0].Profile, net.VideoProfile_H264_MAIN)
	assert.Equal(fullProfiles[1].Profile, net.VideoProfile_ENCODER_DEFAULT)

	// Verify GOP behavior
	assert.Equal(fullProfiles[0].Gop, int32(123))
	assert.Equal(fullProfiles[1].Gop, int32(-100))

	// Invalid format should return error
	profiles[1].Format = -1
	fullProfiles, err = FFmpegProfiletoNetProfile(profiles)
	assert.Equal(ErrFormatProto, err)
	assert.Nil(fullProfiles)
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

func TestVideoProfile_FormatMimeType(t *testing.T) {
	inp := []ffmpeg.Format{ffmpeg.FormatNone, ffmpeg.FormatMPEGTS, ffmpeg.FormatMP4}
	exp := []string{"video/mp2t", "video/mp2t", "video/mp4"}
	for i, v := range inp {
		m, err := ProfileFormatMimeType(v)
		m = strings.ToLower(m)
		if m != exp[i] || err != nil {
			t.Error("Mismatched format; expected ", exp[i], " got ", m)
		}
	}
	if _, err := ProfileFormatMimeType(-1); err != ErrFormatExt {
		t.Error("Did not get expected error")
	}

	// test error with unknown mime type (eg, could be missing from system)
	ffmpeg.FormatExtensions[-1] = "invalid"
	if _, ok := ffmpeg.FormatExtensions[-1]; !ok {
		t.Error("Sanity check failed; did not add extension")
	}
	if _, err := ProfileFormatMimeType(-1); err != ErrFormatMime {
		t.Error("Did not get expected error")
	}
	delete(ffmpeg.FormatExtensions, -1)
	if _, ok := ffmpeg.FormatExtensions[-1]; ok {
		t.Error("Sanity check failed; did not clean up extension")
	}
}

func TestVideoProfile_FormatExtension(t *testing.T) {
	inp := []ffmpeg.Format{ffmpeg.FormatNone, ffmpeg.FormatMPEGTS, ffmpeg.FormatMP4}
	exp := []string{".ts", ".ts", ".mp4"}
	if len(inp) != len(ffmpeg.FormatExtensions) {
		t.Error("Format lengths did not match; missing a new format?")
	}
	for i, v := range inp {
		m, err := ProfileFormatExtension(v)
		if m != exp[i] || err != nil {
			t.Error("Mismatched format; expected ", exp[i], " got ", m)
		}
	}
	if _, err := ProfileFormatExtension(-1); err != ErrFormatExt {
		t.Error("Did not get expected error")
	}
}

func TestVideoProfile_ProfileNameToValue(t *testing.T) {
	assert := assert.New(t)
	inp := []string{"", "h264baseline", "h264main", "h264high", "h264constrainedhigh"}
	assert.Len(inp, len(ffmpeg.ProfileParameters), "Missing a new profile?")
	outp := []ffmpeg.Profile{ffmpeg.ProfileNone, ffmpeg.ProfileH264Baseline, ffmpeg.ProfileH264Main, ffmpeg.ProfileH264High, ffmpeg.ProfileH264ConstrainedHigh}
	assert.Len(inp, len(outp))
	for i, profile := range inp {
		p, err := ffmpeg.EncoderProfileNameToValue(profile)
		assert.Nil(err)
		assert.Equal(outp[i], p)
	}
	p, _ := ffmpeg.EncoderProfileNameToValue("none")
	assert.Equal(ffmpeg.ProfileNone, p)
	_, err := ffmpeg.EncoderProfileNameToValue("invalid")
	assert.Equal(ErrProfName, err, "Could not get profile value")
}

func TestParseAccelDevices_FailedDetection(t *testing.T) {
	assert := assert.New(t)

	getGPU = func() ([]*gpu.GraphicsCard, error) {
		return []*gpu.GraphicsCard{}, nil
	}
	getPCI = func() ([]*pci.Device, error) {
		return []*pci.Device{}, nil
	}

	ids, err := ParseAccelDevices("all", ffmpeg.Nvidia)

	assert.NotNil(err)
	assert.Equal(len(ids), 0)
}

func TestParseAccessDevices_Gpu(t *testing.T) {
	assert := assert.New(t)

	originGetGPU := getGPU
	originGetPCI := getPCI

	getGPU = func() ([]*gpu.GraphicsCard, error) {
		gpus := []*gpu.GraphicsCard{}
		for i := 0; i < 3; i++ {
			gpus = append(gpus, &gpu.GraphicsCard{
				DeviceInfo: &ghw.PCIDevice{
					Vendor: &pcidb.Vendor{
						Name: "--Nvidia Corp",
					},
				},
			})
		}

		return gpus, nil
	}
	ids, err := ParseAccelDevices("all", ffmpeg.Nvidia)

	assert.Nil(err)
	assert.Equal(len(ids), 3)
	assert.Equal(ids[0], "0")
	assert.Equal(ids[1], "1")
	assert.Equal(ids[2], "2")

	getGPU = originGetGPU
	getPCI = originGetPCI
}

func TestParseAccessDevices_GpuFailedProbing(t *testing.T) {
	assert := assert.New(t)

	originGetGPU := getGPU
	originGetPCI := getPCI

	getGPU = func() ([]*gpu.GraphicsCard, error) {
		return []*gpu.GraphicsCard{}, nil
	}

	getPCI = func() ([]*pci.Device, error) {
		pcis := []*pci.Device{}
		for i := 0; i < 2; i++ {
			pcis = append(pcis, &pci.Device{
				Vendor: &pcidb.Vendor{
					Name: "--Nvidia Corp",
				},
				Driver: "nvidia",
			})
		}
		return pcis, nil
	}

	ids, err := ParseAccelDevices("all", ffmpeg.Nvidia)

	assert.Nil(err)
	assert.Equal(len(ids), 2)
	assert.Equal(ids[0], "0")
	assert.Equal(ids[1], "1")

	getGPU = originGetGPU
	getPCI = originGetPCI
}

func TestParseAccelDevices_WrongDriver(t *testing.T) {
	assert := assert.New(t)

	originGetGPU := getGPU
	originGetPCI := getPCI

	getGPU = func() ([]*gpu.GraphicsCard, error) {
		return []*gpu.GraphicsCard{}, nil
	}

	getPCI = func() ([]*pci.Device, error) {
		pcis := []*pci.Device{}
		for i := 0; i < 4; i++ {
			pcis = append(pcis, &pci.Device{
				Vendor: &pcidb.Vendor{
					Name: "--Nvidia Corp",
				},
				Class: &pcidb.Class{
					Name: "Display Controller",
				},
			})
		}

		return pcis, nil
	}

	ids, err := ParseAccelDevices("all", ffmpeg.Nvidia)

	assert.Nil(err)
	assert.Equal(len(ids), 4)
	assert.Equal(ids[0], "0")
	assert.Equal(ids[1], "1")
	assert.Equal(ids[2], "2")
	assert.Equal(ids[3], "3")

	getGPU = originGetGPU
	getPCI = originGetPCI
}

func TestParseAccelDevices_CustomSelection(t *testing.T) {
	assert := assert.New(t)

	ids, _ := ParseAccelDevices("0,3,1", ffmpeg.Nvidia)
	assert.Equal(len(ids), 3)
	assert.Equal(ids[0], "0")
	assert.Equal(ids[1], "3")
	assert.Equal(ids[2], "1")
}
