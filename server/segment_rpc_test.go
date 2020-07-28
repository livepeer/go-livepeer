package server

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func serveSegmentHandler(orch Orchestrator) http.Handler {
	lp := lphttp{
		orchestrator: orch,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lp.ServeSegment(w, r)
	})
}
func TestServeSegment_GetPaymentError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	headers := map[string]string{
		paymentHeader: "foo",
	}
	resp := httpPostResp(handler, nil, headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusPaymentRequired, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "base64")
}

func TestServeSegment_VerifySegCredsError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: "foo",
	}
	resp := httpPostResp(handler, nil, headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusForbidden, resp.StatusCode)
	assert.Equal(errSegEncoding.Error(), strings.TrimSpace(string(body)))
}

func TestServeSegment_MismatchHashError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles:   []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9},
		},
	}
	creds, err := genSegCreds(s, &stream.HLSSegment{})
	require.Nil(t, err)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)
	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader([]byte("foo")), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusForbidden, resp.StatusCode)
	assert.Equal("Forbidden", strings.TrimSpace(string(body)))
}

func TestServeSegment_TranscodeSegError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles:   []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)
	orch.On("TranscodeSeg", md, seg).Return(nil, errors.New("TranscodeSeg error"))
	orch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Error)
	assert.True(ok)
	assert.Equal("TranscodeSeg error", res.Error)
}

func TestVerifySegCreds_Duration(t *testing.T) {
	assert := assert.New(t)
	orch := &stubOrchestrator{offchain: true}
	runVerify := func(sd *net.SegData) (*core.SegTranscodingMetadata, error) {
		data, err := proto.Marshal(sd)
		assert.Nil(err)
		creds := base64.StdEncoding.EncodeToString(data)
		md, err := verifySegCreds(orch, creds, ethcommon.Address{})
		md2, err2 := coreSegMetadata(sd)
		assert.Equal(md, md2, "Did verifySegCreds call coreSegMetadata?")
		assert.Equal(err, err2)
		return md, err
	}

	// check default value
	md, err := runVerify(&net.SegData{})
	assert.Nil(err)
	assert.Equal(2000*time.Millisecond, md.Duration)

	// check regular value
	md, err = runVerify(&net.SegData{Duration: int32(123)})
	assert.Nil(err)
	assert.Equal(123*time.Millisecond, md.Duration)

	// check invalid value : less than zero
	md, err = runVerify(&net.SegData{Duration: -1})
	assert.Equal(errDuration, err)
	assert.Nil(md)

	// check invalid value : greater than max duration
	md, err = runVerify(&net.SegData{Duration: int32(maxDuration.Milliseconds() + 1)})
	assert.Equal(errDuration, err)
	assert.Nil(md)
}

func TestCoreSegMetadata_Profiles(t *testing.T) {
	assert := assert.New(t)
	// testing with the following profiles doesn't work: ffmpeg.P720p60fps16x9, ffmpeg.P144p25fps16x9
	profiles := []ffmpeg.VideoProfile{ffmpeg.P576p30fps16x9, ffmpeg.P240p30fps4x3}
	segData := &net.SegData{
		ManifestId: []byte("manifestID"),
		Profiles:   common.ProfilesToTranscodeOpts(profiles),
	}
	md, err := coreSegMetadata(segData)
	assert.Nil(err)
	assert.Equal(profiles, md.Profiles)

	// Check error handling with the default invalid Profiles
	segData, err = core.NetSegData(&core.SegTranscodingMetadata{})
	assert.Nil(err)
	assert.Equal([]byte("invalid"), segData.Profiles)
	md, err = coreSegMetadata(segData)
	assert.Nil(md)
	assert.Equal(common.ErrProfile, err)
}

func TestGenSegCreds_FullProfiles(t *testing.T) {
	assert := assert.New(t)
	profiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfile{
			Name:       "prof1",
			Bitrate:    "432k",
			Framerate:  uint(560),
			Resolution: "123x456",
			Format:     ffmpeg.FormatMPEGTS,
		},
		ffmpeg.VideoProfile{
			Name:       "prof2",
			Bitrate:    "765k",
			Framerate:  uint(876),
			Resolution: "456x987",
			Format:     ffmpeg.FormatMP4,
		},
		ffmpeg.VideoProfile{Resolution: "0x0", Bitrate: "0"},
	}

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles:   profiles,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}

	data, err := genSegCreds(s, seg)
	assert.Nil(err)

	buf, err := base64.StdEncoding.DecodeString(data)
	assert.Nil(err)

	segData := net.SegData{}
	err = proto.Unmarshal(buf, &segData)
	assert.Nil(err)

	expectedProfiles, err := common.FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)
	assert.Equal([]byte("invalid"), segData.Profiles)
	assert.Empty(segData.FullProfiles)
	assert.Equal(expectedProfiles, segData.FullProfiles2)

	// Check when we have a MP4 sandwiched in between two mpegts
	assert.Len(s.Params.Profiles, 3)
	assert.Equal(ffmpeg.FormatMPEGTS, s.Params.Profiles[0].Format)
	assert.Equal(ffmpeg.FormatMP4, s.Params.Profiles[1].Format)
	assert.Equal(ffmpeg.FormatNone, s.Params.Profiles[2].Format)
	data, err = genSegCreds(s, seg)
	assert.Nil(err)
	buf, err = base64.StdEncoding.DecodeString(data)
	assert.Nil(err)
	err = proto.Unmarshal(buf, &segData)
	assert.Nil(err)
	expectedProfiles, err = common.FFmpegProfiletoNetProfile(profiles)
	assert.Equal([]byte("invalid"), segData.Profiles)
	assert.Empty(segData.FullProfiles)
	assert.Equal(expectedProfiles, segData.FullProfiles2)

	// Check that FullProfiles field is used for none/mpegts (not FullProfiles2)
	s.Params.Profiles[1].Format = ffmpeg.FormatMPEGTS
	data, err = genSegCreds(s, seg)
	assert.Nil(err)
	buf, err = base64.StdEncoding.DecodeString(data)
	assert.Nil(err)
	err = proto.Unmarshal(buf, &segData)
	assert.Nil(err)
	expectedProfiles, err = common.FFmpegProfiletoNetProfile(profiles)
	assert.Equal([]byte("invalid"), segData.Profiles)
	assert.Equal(expectedProfiles, segData.FullProfiles)
	assert.Empty(segData.FullProfiles2)

	// Check that FullProfiles3 field is used if any profile has FPS denominator
	s.Params.Profiles[1].FramerateDen = uint(12)
	data, err = genSegCreds(s, seg)
	assert.Nil(err)
	buf, err = base64.StdEncoding.DecodeString(data)
	assert.Nil(err)
	err = proto.Unmarshal(buf, &segData)
	assert.Nil(err)
	expectedProfiles, err = common.FFmpegProfiletoNetProfile(profiles)
	assert.Equal(expectedProfiles[1].FpsDen, uint32(s.Params.Profiles[1].FramerateDen))
	assert.Equal([]byte("invalid"), segData.Profiles)
	assert.Equal(expectedProfiles, segData.FullProfiles3)
	assert.Empty(segData.FullProfiles)
	assert.Empty(segData.FullProfiles2)

	// Check that FullProfiles3 field is used if any profile has non-empty encoder profile
	s.Params.Profiles[1].FramerateDen = uint(0) // unset FramerateDen as that also triggers FullProfile3
	s.Params.Profiles[0].Profile = ffmpeg.ProfileH264High
	data, err = genSegCreds(s, seg)
	assert.Nil(err)
	buf, err = base64.StdEncoding.DecodeString(data)
	assert.Nil(err)
	err = proto.Unmarshal(buf, &segData)
	assert.Nil(err)
	expectedProfiles, err = common.FFmpegProfiletoNetProfile(profiles)
	assert.Equal(expectedProfiles[0].Profile, net.VideoProfile_H264_HIGH)
	assert.Equal([]byte("invalid"), segData.Profiles)
	assert.Equal(expectedProfiles, segData.FullProfiles3)
	assert.Empty(segData.FullProfiles)
	assert.Empty(segData.FullProfiles2)

	// Check that FullProfiles3 is used if any profile has a GOP set
	s.Params.Profiles[0].Profile = ffmpeg.ProfileNone // unset Profile as that also triggers FullProfile3
	s.Params.Profiles[1].GOP = time.Duration(123) * time.Millisecond
	data, err = genSegCreds(s, seg)
	assert.Nil(err)
	buf, err = base64.StdEncoding.DecodeString(data)
	assert.Nil(err)
	expectedProfiles, err = common.FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)
	assert.Equal(int32(0), expectedProfiles[0].Gop)
	assert.Equal(int32(123), expectedProfiles[1].Gop)
	assert.Empty(segData.FullProfiles)
	assert.Empty(segData.FullProfiles2)
	assert.Equal([]byte("invalid"), segData.Profiles)

	// Check that profile format errors propagate
	s.Params.Profiles[1].Format = -1
	data, err = genSegCreds(s, seg)
	assert.Empty(data)
	assert.Equal(common.ErrFormatProto, err)
}

func TestGenSegCreds_Profiles(t *testing.T) {
	assert := assert.New(t)
	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P360p30fps16x9}
	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles:   profiles,
		},
	}

	seg := &stream.HLSSegment{Data: []byte("foo")}

	data, err := genSegCreds(s, seg)
	assert.Nil(err)

	buf, err := base64.StdEncoding.DecodeString(data)
	assert.Nil(err)

	segData := net.SegData{}
	err = proto.Unmarshal(buf, &segData)
	assert.Nil(err)

	expectedProfiles, err := common.FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)
	assert.Equal(expectedProfiles, segData.FullProfiles)
}

func TestCoreSegMetadata_FullProfiles(t *testing.T) {
	assert := assert.New(t)

	profiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfile{
			Name:       "prof1",
			Bitrate:    "432k",
			Framerate:  uint(560),
			Resolution: "123x456",
		},
		ffmpeg.VideoProfile{
			Name:         "prof2",
			Bitrate:      "765k",
			Framerate:    uint(876),
			FramerateDen: uint(12),
			Resolution:   "456x987",
			Format:       ffmpeg.FormatMP4,
			Profile:      ffmpeg.ProfileH264ConstrainedHigh,
		},
	}

	fullProfiles, err := common.FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)

	segData := &net.SegData{
		ManifestId:   []byte("manifestID"),
		FullProfiles: fullProfiles,
	}

	md, err := coreSegMetadata(segData)
	assert.Nil(err)
	profiles[0].Bitrate = "432000"
	profiles[1].Bitrate = "765000"
	profiles[0].Format = ffmpeg.FormatMPEGTS
	assert.Equal(profiles, md.Profiles)

	// Test deserialization failure from invalid full profile format
	segData.FullProfiles[1].Format = -1
	md, err = coreSegMetadata(segData)
	assert.Nil(md)
	assert.Equal(errFormat, err)

	// Test deserialization with FullProfiles2
	// (keep invalid FullProfiles populated to exhibit precedence)
	segData.FullProfiles2 = []*net.VideoProfile{&net.VideoProfile{Name: "prof3"}}
	md, err = coreSegMetadata(segData)
	expected := []ffmpeg.VideoProfile{{Name: "prof3",
		Bitrate: "0", Resolution: "0x0",
		Format: ffmpeg.FormatMPEGTS}}
	assert.Equal(expected, md.Profiles)

	// Test deserialization with FullProfiles3
	// (keep invalid FullProfiles populated to exhibit precedence)
	segData.FullProfiles3 = []*net.VideoProfile{&net.VideoProfile{Name: "prof4"}}
	md, err = coreSegMetadata(segData)
	expected = []ffmpeg.VideoProfile{{Name: "prof4",
		Bitrate: "0", Resolution: "0x0",
		Framerate: uint(0), FramerateDen: uint(0),
		Format: ffmpeg.FormatMPEGTS}}
	assert.Equal(expected, md.Profiles)

}

func TestMakeFfmpegVideoProfiles(t *testing.T) {
	assert := assert.New(t)
	videoProfiles := []*net.VideoProfile{
		{
			Name:    "prof1",
			Width:   int32(123),
			Height:  int32(456),
			Bitrate: int32(789),
			Fps:     uint32(912),
			FpsDen:  uint32(15),
		},
		{
			Name:    "prof2",
			Width:   int32(987),
			Height:  int32(654),
			Bitrate: int32(321),
			Fps:     uint32(198),
			Profile: net.VideoProfile_H264_BASELINE,
			Gop:     int32(123),
		},
	}

	// testing happy case scenario
	expectedProfiles := []ffmpeg.VideoProfile{
		{
			Name:         videoProfiles[0].Name,
			Bitrate:      fmt.Sprint(videoProfiles[0].Bitrate),
			Framerate:    uint(videoProfiles[0].Fps),
			FramerateDen: uint(videoProfiles[0].FpsDen),
			Resolution:   fmt.Sprintf("%dx%d", videoProfiles[0].Width, videoProfiles[0].Height),
			Format:       ffmpeg.FormatMPEGTS,
			Profile:      ffmpeg.ProfileNone,
		},
		{
			Name:         videoProfiles[1].Name,
			Bitrate:      fmt.Sprint(videoProfiles[1].Bitrate),
			Framerate:    uint(videoProfiles[1].Fps),
			FramerateDen: uint(0),
			Resolution:   fmt.Sprintf("%dx%d", videoProfiles[1].Width, videoProfiles[1].Height),
			Format:       ffmpeg.FormatMPEGTS,
			Profile:      ffmpeg.ProfileH264Baseline,
			GOP:          time.Duration(123) * time.Millisecond,
		},
	}

	ffmpegProfiles, err := makeFfmpegVideoProfiles(videoProfiles)
	assert.Nil(err)
	expectedResolution := fmt.Sprintf("%dx%d", videoProfiles[0].Width, videoProfiles[0].Height)
	assert.Equal(expectedProfiles, ffmpegProfiles)
	assert.Equal(ffmpegProfiles[0].Resolution, expectedResolution)
	assert.Equal(ffmpegProfiles[0].GOP, time.Duration(0))

	// empty name should return automatically generated name
	videoProfiles[0].Name = ""
	expectedName := "net_" + fmt.Sprintf("%dx%d_%d", videoProfiles[0].Width, videoProfiles[0].Height, videoProfiles[0].Bitrate)
	ffmpegProfiles, err = makeFfmpegVideoProfiles(videoProfiles)
	assert.Nil(err)
	assert.Equal(ffmpegProfiles[0].Name, expectedName)

	// Unset format should default to mpegts
	assert.Equal(videoProfiles[0].Format, videoProfiles[1].Format)
	assert.Equal(videoProfiles[0].Format, net.VideoProfile_MPEGTS)

	videoProfiles[0].Format = net.VideoProfile_MP4
	videoProfiles[1].Format = net.VideoProfile_MPEGTS
	ffmpegProfiles, err = makeFfmpegVideoProfiles(videoProfiles)
	assert.Nil(err)
	assert.Equal(ffmpegProfiles[0].Format, ffmpeg.FormatMP4)
	assert.Equal(ffmpegProfiles[1].Format, ffmpeg.FormatMPEGTS)

	// Invalid format should return error
	videoProfiles[1].Format = -1
	ffmpegProfiles, err = makeFfmpegVideoProfiles(videoProfiles)
	assert.Nil(ffmpegProfiles)
	assert.Equal(errFormat, err)
}

func TestServeSegment_SaveDataFormat(t *testing.T) {
	assert := assert.New(t)
	os := &stubOSSession{}
	tData := &core.TranscodeData{Segments: []*core.TranscodedSegmentData{
		&core.TranscodedSegmentData{Data: []byte("unused1")},
		&core.TranscodedSegmentData{Data: []byte("unused2")},
	}}
	tRes := &core.TranscodeResult{
		TranscodeData: tData,
		OS:            os,
	}
	orch := &stubOrchestrator{res: tRes, offchain: true}
	handler := serveSegmentHandler(orch)

	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	defer func() { drivers.NodeStorage = oldStorage }()

	sess := &BroadcastSession{
		Params:        &core.StreamParameters{Profiles: []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9, ffmpeg.P720p60fps16x9}},
		Broadcaster:   stubBroadcaster2(),
		BroadcasterOS: os,
	}
	creds, err := genSegCreds(sess, &stream.HLSSegment{})
	assert.Nil(err)
	headers := map[string]string{segmentHeader: creds}

	for _, p := range sess.Params.Profiles {
		assert.Equal(ffmpeg.FormatNone, p.Format) // sanity check
	}

	// no format in profiles
	resp := httpPostResp(handler, bytes.NewReader([]byte("")), headers)
	defer resp.Body.Close()
	assert.Equal([]string{"P720p30fps16x9/0.ts", "P720p60fps16x9/0.ts"}, os.saved)

	// mp4 format
	for i, _ := range sess.Params.Profiles {
		sess.Params.Profiles[i].Format = ffmpeg.FormatMP4
	}
	assert.Equal(ffmpeg.FormatNone, ffmpeg.P720p30fps16x9.Format) // sanity
	assert.Equal(ffmpeg.FormatNone, ffmpeg.P720p60fps16x9.Format) // sanity
	creds, err = genSegCreds(sess, &stream.HLSSegment{})
	os = &stubOSSession{} // reset for simplicity
	tRes.OS = os
	headers = map[string]string{segmentHeader: creds}
	resp = httpPostResp(handler, bytes.NewReader([]byte("")), headers)
	defer resp.Body.Close()
	assert.Equal([]string{"P720p30fps16x9/0.mp4", "P720p60fps16x9/0.mp4"}, os.saved)

	// mpegts format
	for i, _ := range sess.Params.Profiles {
		sess.Params.Profiles[i].Format = ffmpeg.FormatMPEGTS
	}
	creds, err = genSegCreds(sess, &stream.HLSSegment{})
	os = &stubOSSession{} // reset for simplicity
	tRes.OS = os
	headers = map[string]string{segmentHeader: creds}
	resp = httpPostResp(handler, bytes.NewReader([]byte("")), headers)
	defer resp.Body.Close()
	assert.Equal([]string{"P720p30fps16x9/0.ts", "P720p60fps16x9/0.ts"}, os.saved)

	// Check for error in format extension detection prior to saving data
	// Simulate by removing one of the formats from the ffmpeg table
	assert.Contains(ffmpeg.FormatExtensions, ffmpeg.FormatMPEGTS, "Could not sanity check mpegts format extension")
	oldExt := ffmpeg.FormatExtensions[ffmpeg.FormatMPEGTS]
	delete(ffmpeg.FormatExtensions, ffmpeg.FormatMPEGTS)
	defer func() {
		ffmpeg.FormatExtensions[ffmpeg.FormatMPEGTS] = oldExt
		assert.Contains(ffmpeg.FormatExtensions, ffmpeg.FormatMPEGTS)
	}()
	assert.NotContains(ffmpeg.FormatExtensions, ffmpeg.FormatMPEGTS)
	os = &stubOSSession{}
	tRes.OS = os
	resp = httpPostResp(handler, bytes.NewReader([]byte("")), headers)
	defer resp.Body.Close()
	assert.Empty(os.saved)

	var tr net.TranscodeResult
	body, err := ioutil.ReadAll(resp.Body)
	err = proto.Unmarshal(body, &tr)
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Error)
	assert.True(ok)
	assert.Equal("unknown VideoProfile format for extension", res.Error)
}

func TestServeSegment_OSSaveDataError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	mos := &mockOSSession{}

	mos.On("SaveData", mock.Anything, mock.Anything).Return("", errors.New("SaveData error"))

	tData := &core.TranscodeData{Segments: []*core.TranscodedSegmentData{{Data: []byte("foo")}}}
	tRes := &core.TranscodeResult{
		TranscodeData: tData,
		Sig:           []byte("foo"),
		OS:            mos,
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)
	orch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(0, len(res.Data.Segments))
}

func TestServeSegment_ReturnSingleTranscodedSegmentData(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	tData := &core.TranscodeData{Segments: []*core.TranscodedSegmentData{&core.TranscodedSegmentData{Data: []byte("foo")}}}
	tRes := &core.TranscodeResult{
		TranscodeData: tData,
		Sig:           []byte("foo"),
		OS:            drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)
	orch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(1, len(res.Data.Segments))
}

func TestServeSegment_ReturnMultipleTranscodedSegmentData(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
				ffmpeg.P240p30fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	tData := &core.TranscodedSegmentData{Data: []byte("foo")}
	tRes := &core.TranscodeResult{
		TranscodeData: &core.TranscodeData{Segments: []*core.TranscodedSegmentData{tData, tData}},
		Sig:           []byte("foo"),
		OS:            drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)
	orch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(2, len(res.Data.Segments))
}

func TestServeSegment_ProcessPaymentError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)
	assert := assert.New(t)
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	_, err = verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	// Return an error to trigger bad request
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(errors.New("some error"), false).Once()

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}

	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("some error", strings.TrimSpace(string(body)))
	resp.Body.Close()

	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(errors.New("some error")).Once()
	resp = httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	require.Nil(err)

	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("some error", strings.TrimSpace(string(body)))
}

func TestServeSegment_UpdateOrchestratorInfo(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	params := &net.TicketParams{
		Recipient:         []byte("foo"),
		FaceValue:         big.NewInt(100).Bytes(),
		WinProb:           big.NewInt(100).Bytes(),
		RecipientRandHash: []byte("bar"),
		Seed:              []byte("baz"),
		ExpirationBlock:   big.NewInt(100).Bytes(),
	}

	price := &net.PriceInfo{
		PricePerUnit:  2,
		PixelsPerUnit: 3,
	}
	// trigger an update to orchestrator info
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	uri, err := url.Parse("http://google.com")
	require.Nil(err)
	addr := ethcommon.BytesToAddress([]byte("foo"))
	orch.On("ServiceURI").Return(uri)
	orch.On("Address").Return(addr)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(params, nil).Once()
	orch.On("PriceInfo", mock.Anything).Return(price, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil).Once()
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	tData := &core.TranscodeData{Segments: []*core.TranscodedSegmentData{&core.TranscodedSegmentData{Data: []byte("foo")}}}
	tRes := &core.TranscodeResult{
		TranscodeData: tData,
		Sig:           []byte("foo"),
		OS:            drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)
	orch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	assert.Equal(uri.String(), tr.Info.Transcoder)
	assert.Equal(params.Recipient, tr.Info.TicketParams.Recipient)
	assert.Equal(params.FaceValue, tr.Info.TicketParams.FaceValue)
	assert.Equal(params.WinProb, tr.Info.TicketParams.WinProb)
	assert.Equal(params.RecipientRandHash, tr.Info.TicketParams.RecipientRandHash)
	assert.Equal(params.Seed, tr.Info.TicketParams.Seed)
	assert.Equal(price.PricePerUnit, tr.Info.PriceInfo.PricePerUnit)
	assert.Equal(price.PixelsPerUnit, tr.Info.PriceInfo.PixelsPerUnit)
	assert.Equal(addr.Bytes(), tr.Info.Address)

	// Test orchestratorInfo error
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil).Once()
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(nil, errors.New("TicketParams error")).Once()

	resp = httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	require.Nil(err)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("Internal Server Error", strings.TrimSpace(string(body)))
}

func TestServeSegment_InsufficientBalanceError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)
	assert := assert.New(t)
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles:   []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	_, err = verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(false)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("Insufficient balance", strings.TrimSpace(string(body)))
}

func TestServeSegment_DebitFees_SingleRendition(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	tData := &core.TranscodeData{Segments: []*core.TranscodedSegmentData{&core.TranscodedSegmentData{Data: []byte("foo"), Pixels: int64(110592000)}}}
	tRes := &core.TranscodeResult{
		TranscodeData: tData,
		Sig:           []byte("foo"),
		OS:            drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)
	orch.On("DebitFees", mock.Anything, md.ManifestID, mock.Anything, tData.Segments[0].Pixels)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(1, len(res.Data.Segments))
	assert.Equal(res.Data.Segments[0].Pixels, tData.Segments[0].Pixels)
	orch.AssertCalled(t, "DebitFees", mock.Anything, md.ManifestID, mock.Anything, tData.Segments[0].Pixels)
}

func TestServeSegment_DebitFees_MultipleRenditions(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
				ffmpeg.P240p30fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	tData720 := &core.TranscodedSegmentData{
		Data:   []byte("foo"),
		Pixels: int64(110592000),
	}
	tData240 := &core.TranscodedSegmentData{
		Data:   []byte("foo"),
		Pixels: int64(6134400),
	}
	tRes := &core.TranscodeResult{
		TranscodeData: &core.TranscodeData{Segments: []*core.TranscodedSegmentData{tData720, tData240}},
		Sig:           []byte("foo"),
		OS:            drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)
	orch.On("DebitFees", mock.Anything, md.ManifestID, mock.Anything, tData720.Pixels+tData240.Pixels)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(2, len(res.Data.Segments))
	for i, seg := range res.Data.Segments {
		assert.Equal(seg.Pixels, tRes.TranscodeData.Segments[i].Pixels)
	}
	orch.AssertCalled(t, "DebitFees", mock.Anything, md.ManifestID, mock.Anything, tData720.Pixels+tData240.Pixels)
}

// break loop for adding pixelcounts when OS upload fails
func TestServeSegment_DebitFees_OSSaveDataError_BreakLoop(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
				ffmpeg.P240p30fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)

	mos := &mockOSSession{}

	tData720 := &core.TranscodedSegmentData{
		Data:   []byte("foo"),
		Pixels: int64(110592000),
	}
	tData240 := &core.TranscodedSegmentData{
		Data:   []byte("foo"),
		Pixels: int64(6134400),
	}
	tRes := &core.TranscodeResult{
		TranscodeData: &core.TranscodeData{Segments: []*core.TranscodedSegmentData{tData720, tData240}},
		Sig:           []byte("foo"),
		OS:            mos,
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)

	mos.On("SaveData", mock.Anything, mock.Anything).Return("720pdotcom", nil).Once()
	mos.On("SaveData", mock.Anything, mock.Anything).Return("", errors.New("SaveData error")).Once()

	orch.On("DebitFees", mock.Anything, md.ManifestID, mock.Anything, tData720.Pixels)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(1, len(res.Data.Segments))
	assert.Equal(res.Data.Segments[0].Pixels, tData720.Pixels)
	orch.AssertCalled(t, "DebitFees", mock.Anything, md.ManifestID, mock.Anything, tData720.Pixels)
}

func TestServeSegment_DebitFees_TranscodeSegError_ZeroPixelsBilled(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			Profiles: []ffmpeg.VideoProfile{
				ffmpeg.P720p60fps16x9,
			},
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	url, _ := url.Parse("foo")
	orch.On("ServiceURI").Return(url)
	orch.On("Address").Return(ethcommon.Address{})
	orch.On("PriceInfo", mock.Anything).Return(&net.PriceInfo{}, nil)
	orch.On("TicketParams", mock.Anything, mock.Anything).Return(&net.TicketParams{}, nil)
	orch.On("ProcessPayment", net.Payment{}, s.Params.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.Params.ManifestID).Return(true)
	orch.On("TranscodeSeg", md, seg).Return(nil, errors.New("TranscodeSeg error"))
	orch.On("DebitFees", mock.Anything, md.ManifestID, mock.Anything, int64(0))

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Error)
	assert.True(ok)
	assert.Equal("TranscodeSeg error", res.Error)
	orch.AssertCalled(t, "DebitFees", mock.Anything, md.ManifestID, mock.Anything, int64(0))
}

func TestSubmitSegment_GenSegCredsError(t *testing.T) {
	b := stubBroadcaster2()
	b.signErr = errors.New("Sign error")

	s := &BroadcastSession{
		Broadcaster: b,
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "Sign error", err.Error())
}

func TestSubmitSegment_RatPriceInfoError(t *testing.T) {
	b := stubBroadcaster2()

	s := &BroadcastSession{
		Broadcaster: b,
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			PriceInfo: &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 0},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.EqualError(t, err, "pixels per unit is 0")
}

func TestSubmitSegment_EstimateFeeError(t *testing.T) {
	b := stubBroadcaster2()

	s := &BroadcastSession{
		Broadcaster: b,
		Params: &core.StreamParameters{
			ManifestID: core.RandomManifestID(),
			// Contains invalid profile
			Profiles: []ffmpeg.VideoProfile{ffmpeg.VideoProfile{Resolution: "foo"}},
		},
		OrchestratorInfo: &net.OrchestratorInfo{
			PriceInfo: &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 1},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Error(t, err)
}

func TestSubmitSegment_NewBalanceUpdateError(t *testing.T) {
	b := stubBroadcaster2()
	sender := &pm.MockSender{}
	expErr := errors.New("EV error")
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), expErr)

	s := &BroadcastSession{
		Broadcaster: b,
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		Sender:      sender,
		Balance:     &mockBalance{},
		OrchestratorInfo: &net.OrchestratorInfo{
			PriceInfo: &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 1},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.EqualError(t, err, expErr.Error())
}

func TestSubmitSegment_GenPaymentError_CreateTicketBatchError(t *testing.T) {
	b := stubBroadcaster2()
	existingCredit := big.NewRat(5, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(1, nil, existingCredit)
	balance.On("Credit", existingCredit)
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	expErr := errors.New("CreateTicketBatch error")
	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(nil, expErr)

	oInfo := &net.OrchestratorInfo{
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  0,
			PixelsPerUnit: 1,
		},
	}

	s := &BroadcastSession{
		Broadcaster:      b,
		Params:           &core.StreamParameters{ManifestID: core.RandomManifestID()},
		Sender:           sender,
		Balance:          balance,
		OrchestratorInfo: oInfo,
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.EqualError(t, err, expErr.Error())
	// Check that completeBalanceUpdate() adds back the existing credit when the update status is Staged
	balance.AssertCalled(t, "Credit", existingCredit)
}

func TestSubmitSegment_GenPaymentError_ValidatePriceError(t *testing.T) {
	b := stubBroadcaster2()
	balance := &mockBalance{}
	existingCredit := big.NewRat(5, 1)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(1, nil, existingCredit)
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(1, 1), nil)
	balance.On("Credit", mock.Anything)

	oinfo := &net.OrchestratorInfo{
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 3,
		},
	}

	s := &BroadcastSession{
		Broadcaster:      b,
		Params:           &core.StreamParameters{ManifestID: core.RandomManifestID()},
		Sender:           sender,
		Balance:          balance,
		OrchestratorInfo: oinfo,
	}

	BroadcastCfg.SetMaxPrice(big.NewRat(1, 5))
	defer BroadcastCfg.SetMaxPrice(nil)

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.EqualErrorf(t, err, err.Error(), "Orchestrator price higher than the set maximum price of %v wei per %v pixels", int64(1), int64(5))
	balance.AssertCalled(t, "Credit", existingCredit)
}

func TestSubmitSegment_HttpPostError(t *testing.T) {
	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: "https://127.0.0.1",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Contains(t, err.Error(), "connection refused")

	// Test completeBalanceUpdate() adds back the existing credit when the update status is Staged
	existingCredit := big.NewRat(5, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, nil, existingCredit)
	balance.On("Credit", existingCredit)
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	s.Balance = balance
	s.Sender = sender

	_, err = SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Contains(t, err.Error(), "connection refused")
	balance.AssertCalled(t, "Credit", existingCredit)
}

func TestSubmitSegment_Non200StatusCode(t *testing.T) {
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server error", http.StatusInternalServerError)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "Server error", err.Error())

	// Test completeBalanceUpdate() does not add anything back when the update status is CreditSpent
	newCredit := big.NewRat(7, 1)
	existingCredit := big.NewRat(5, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit)
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	s.Balance = balance
	s.Sender = sender

	_, err = SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "Server error", err.Error())
	balance.AssertNotCalled(t, "Credit", mock.Anything)
}

func TestSubmitSegment_ProtoUnmarshalError(t *testing.T) {
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("foo"))
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Contains(t, err.Error(), "proto")

	// Test completeBalanceUpdate() does not add anything back when the update status is CreditSpent
	newCredit := big.NewRat(7, 1)
	existingCredit := big.NewRat(5, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit)
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	s.Balance = balance
	s.Sender = sender

	_, err = SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Contains(t, err.Error(), "proto")
	balance.AssertNotCalled(t, "Credit", mock.Anything)
}

func TestSubmitSegment_TranscodeResultError(t *testing.T) {
	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Error{Error: "TranscodeResult error"},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		},
	}

	_, err = SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "TranscodeResult error", err.Error())

	// Test completeBalanceUpdate() does not add anything back when the update status is CreditSpent
	newCredit := big.NewRat(7, 1)
	existingCredit := big.NewRat(5, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit)
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	s.Balance = balance
	s.Sender = sender

	_, err = SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "TranscodeResult error", err.Error())
	balance.AssertNotCalled(t, "Credit", mock.Anything)
}

func TestSubmitSegment_Timeout(t *testing.T) {
	assert := assert.New(t)

	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{},
		},
	}
	buf, err := proto.Marshal(tr)

	headerTimeout := 0 * time.Millisecond
	bodyTimeout := 0 * time.Millisecond
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(headerTimeout)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(bodyTimeout)
		w.Write(buf)
	})

	// sanity check no timeouts
	sess := &BroadcastSession{
		Broadcaster:      stubBroadcaster2(),
		Params:           &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{Transcoder: ts.URL},
	}
	seg := &stream.HLSSegment{Duration: 0.01}
	_, err = SubmitSegment(sess, seg, 0)
	assert.Nil(err)

	oldTimeout := common.HTTPTimeout
	defer func() { common.HTTPTimeout = oldTimeout }()
	common.HTTPTimeout = 0

	// time out header
	headerTimeout = 100 * time.Millisecond
	_, err = SubmitSegment(sess, seg, 0)
	assert.Contains(err.Error(), "header timeout")
	assert.Contains(err.Error(), "context deadline exceeded")

	// time out body
	headerTimeout = 0
	bodyTimeout = 100 * time.Millisecond
	_, err = SubmitSegment(sess, seg, 0)
	assert.NotNil(err)
	assert.Equal("body timeout: context deadline exceeded", err.Error())

	// sanity check, again
	bodyTimeout = 0
	_, err = SubmitSegment(sess, seg, 0)
	assert.Nil(err)

	// sanity check default timeouts with a bodyTimeout > seg.Duration
	common.HTTPTimeout = 1 * time.Second
	bodyTimeout = 100 * time.Millisecond
	assert.Greater(common.HTTPTimeout.Milliseconds(), bodyTimeout.Milliseconds())
	seg.Duration = 0.0 // missing duration
	_, err = SubmitSegment(sess, seg, 0)
	assert.Nil(err)

	seg.Duration = -0.01 // negative duration
	_, err = SubmitSegment(sess, seg, 0)
	assert.Nil(err)

	seg.Duration = 0.01 // 10ms; less than default timeout. should set default
	_, err = SubmitSegment(sess, seg, 0)
	assert.Nil(err)

	// ensure default timeout triggers
	common.HTTPTimeout = 10 * time.Millisecond
	assert.Greater(bodyTimeout.Milliseconds(), common.HTTPTimeout.Milliseconds())
	_, err = SubmitSegment(sess, seg, 0)
	assert.Equal("body timeout: context deadline exceeded", err.Error())
}

func TestSubmitSegment_Success(t *testing.T) {
	info := &net.OrchestratorInfo{
		Transcoder: "foo",
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
		TicketParams: &net.TicketParams{
			ExpirationBlock: big.NewInt(100).Bytes(),
		},
	}
	require := require.New(t)

	dummyRes := func(tSegData []*net.TranscodedSegmentData) *net.TranscodeResult {
		return &net.TranscodeResult{
			Info: info,
			Result: &net.TranscodeResult_Data{
				Data: &net.TranscodeData{
					Segments: tSegData,
					Sig:      []byte("bar"),
				},
			},
		}
	}

	tSegData := []*net.TranscodedSegmentData{
		&net.TranscodedSegmentData{Url: "foo"},
	}
	tr := dummyRes(tSegData)
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	var runChecks func(r *http.Request)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		if runChecks != nil {
			runChecks(r)
		}
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		},
	}

	assert := assert.New(t)

	segData := pm.RandBytes(1024)

	runChecks = func(r *http.Request) {
		assert.Equal("video/MP2T", r.Header.Get("Content-Type"))

		data, err := ioutil.ReadAll(r.Body)
		require.Nil(err)

		assert.Equal(segData, data)
	}

	noNameSeg := &stream.HLSSegment{Data: segData}
	tdata, err := SubmitSegment(s, noNameSeg, 0)

	assert.Nil(err)
	assert.Equal(1, len(tdata.Segments))
	assert.Equal("foo", tdata.Segments[0].Url)
	assert.Equal([]byte("bar"), tdata.Sig)
	assert.Equal(tdata.Info.Transcoder, info.Transcoder)
	assert.Equal(tdata.Info.GetPriceInfo().GetPricePerUnit(), info.GetPriceInfo().GetPricePerUnit())
	assert.Equal(tdata.Info.GetPriceInfo().GetPixelsPerUnit(), info.GetPriceInfo().GetPixelsPerUnit())
	assert.Equal(tdata.Info.GetTicketParams().GetExpirationBlock(), info.GetTicketParams().GetExpirationBlock())

	// Check that latency score calculation is different for different segment durations
	// The round trip duration calculated in SubmitSegment should be about the same across all calls
	noNameSeg.Duration = 5.0
	tdata, err = SubmitSegment(s, noNameSeg, 0)
	assert.Nil(err)
	latencyScore1 := tdata.LatencyScore

	noNameSeg.Duration = 10.0
	tdata, err = SubmitSegment(s, noNameSeg, 0)
	assert.Nil(err)
	latencyScore2 := tdata.LatencyScore

	noNameSeg.Duration = .5
	tdata, err = SubmitSegment(s, noNameSeg, 0)
	assert.Nil(err)
	latencyScore3 := tdata.LatencyScore

	assert.Less(latencyScore1, latencyScore3)
	assert.Less(latencyScore2, latencyScore1)

	// Check that a new OrchestratorInfo is returned from SubmitSegment()
	tr.Info = info
	buf, err = proto.Marshal(tr)
	require.Nil(err)
	assert.Equal(tr.Info, info)

	tdata, err = SubmitSegment(s, noNameSeg, 0)
	assert.Nil(err)
	assert.NotEqual(tdata.Info, s.OrchestratorInfo)
	assert.Equal(tdata.Info.Transcoder, info.Transcoder)
	assert.Equal(tdata.Info.GetPriceInfo().GetPricePerUnit(), info.GetPriceInfo().GetPricePerUnit())
	assert.Equal(tdata.Info.GetPriceInfo().GetPixelsPerUnit(), info.GetPriceInfo().GetPixelsPerUnit())
	assert.Equal(tdata.Info.GetTicketParams().GetExpirationBlock(), info.GetTicketParams().GetExpirationBlock())

	// Test when input data is uploaded
	runChecks = func(r *http.Request) {
		assert.Equal("application/vnd+livepeer.uri", r.Header.Get("Content-Type"))

		data, err := ioutil.ReadAll(r.Body)
		require.Nil(err)

		assert.Equal([]byte("foo"), data)
	}

	seg := &stream.HLSSegment{Name: "foo", Data: []byte("dummy")}
	SubmitSegment(s, seg, 0)

	// Test completeBalanceUpdate() adds back change when the update status is ReceivedChange

	// Use a custom matcher func to compare mocked big.Rat values
	ratMatcher := func(rat *big.Rat) interface{} {
		return mock.MatchedBy(func(x *big.Rat) bool { return x.Cmp(rat) == 0 })
	}

	// Debit should be 0 when len(tdata.Segments) = 0
	// Total credit should be update.NewCredit when update.ExistingCredit = 0
	newCredit := big.NewRat(7, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, big.NewRat(0, 1)).Once()
	balance.On("Credit", ratMatcher(newCredit)).Once()
	sender := &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	s.Balance = balance
	s.Sender = sender

	SubmitSegment(s, seg, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(newCredit))

	// Total credit should be update.ExistingCredit when update.NewCredit = 0
	existingCredit := big.NewRat(5, 1)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, big.NewRat(0, 1), existingCredit).Once()
	balance.On("Credit", ratMatcher(existingCredit)).Once()

	SubmitSegment(s, seg, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(existingCredit))

	// Total credit should be update.ExistingCredit + update.NewCredit when update.ExistingCredit > 0 && update.NewCredit > 0
	totalCredit := big.NewRat(12, 1)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit).Once()
	balance.On("Credit", ratMatcher(totalCredit)).Once()

	SubmitSegment(s, seg, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(totalCredit))

	// Debit should be calculated based on the pixel count of a single result when len(tdata.Segments) = 1
	tSegData[0].Pixels = 3
	tr = dummyRes(tSegData)
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	s.OrchestratorInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}
	change := big.NewRat(9, 1)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit).Once()
	balance.On("Credit", ratMatcher(change)).Once()

	SubmitSegment(s, seg, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(change))

	// Debit should be calculated based on the sum of the pixel counts of results when len(tdata.Segments) > 1
	tSegData = append(tSegData, &net.TranscodedSegmentData{Url: "duh", Pixels: 5})
	tr = dummyRes(tSegData)
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	change = big.NewRat(4, 1)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit).Once()
	balance.On("Credit", ratMatcher(change)).Once()

	SubmitSegment(s, seg, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(change))

	// Change should be negative if debit > total credit
	tSegData = append(tSegData, &net.TranscodedSegmentData{Url: "goo", Pixels: 100})
	tr = dummyRes(tSegData)
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	change = big.NewRat(-96, 1)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, newCredit, existingCredit).Once()
	balance.On("Credit", ratMatcher(change))

	SubmitSegment(s, seg, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(change))
}

func stubTLSServer() (*httptest.Server, *http.ServeMux) {
	mux := http.NewServeMux()

	ts := httptest.NewUnstartedServer(mux)
	ts.TLS = &tls.Config{
		// This config option fixes the "unexpected ALPN protocol ""; want h2" error
		NextProtos: []string{http2.NextProtoTLS},
	}
	ts.StartTLS()

	return ts, mux
}
