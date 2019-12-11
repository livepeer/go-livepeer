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
		ManifestID:  core.RandomManifestID(),
	}
	creds, err := genSegCreds(s, &stream.HLSSegment{})
	require.Nil(t, err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)
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
		ManifestID:  core.RandomManifestID(),
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)
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

func TestVerifySegCreds_Profiles(t *testing.T) {
	assert := assert.New(t)
	orch := &mockOrchestrator{}
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
	// testing with the following profiles doesn't work: ffmpeg.P720p60fps16x9, ffmpeg.P144p25fps16x9
	profiles := []ffmpeg.VideoProfile{ffmpeg.P576p30fps16x9, ffmpeg.P240p30fps4x3}
	segData := &net.SegData{
		ManifestId: []byte("manifestID"),
		Profiles:   common.ProfilesToTranscodeOpts(profiles),
	}
	data, err := proto.Marshal(segData)
	assert.Nil(err)

	creds := base64.StdEncoding.EncodeToString(data)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	assert.Nil(err)
	assert.Equal(profiles, md.Profiles)
}

func TestGenSegCreds_FullProfiles(t *testing.T) {
	assert := assert.New(t)
	profiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfile{
			Name:       "prof1",
			Bitrate:    "432k",
			Framerate:  uint(560),
			Resolution: "123x456",
		},
		ffmpeg.VideoProfile{
			Name:       "prof2",
			Bitrate:    "765k",
			Framerate:  uint(876),
			Resolution: "456x987",
		},
	}

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles:    profiles,
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

func TestGenSegCreds_Profiles(t *testing.T) {
	assert := assert.New(t)
	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P360p30fps16x9}
	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles:    profiles,
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

func TestVerifySegCreds_FullProfiles(t *testing.T) {
	assert := assert.New(t)
	orch := &mockOrchestrator{}
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	profiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfile{
			Name:       "prof1",
			Bitrate:    "432k",
			Framerate:  uint(560),
			Resolution: "123x456",
		},
		ffmpeg.VideoProfile{
			Name:       "prof2",
			Bitrate:    "765k",
			Framerate:  uint(876),
			Resolution: "456x987",
		},
	}

	fullProfiles, err := common.FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)

	segData := &net.SegData{
		ManifestId:   []byte("manifestID"),
		FullProfiles: fullProfiles,
	}

	data, err := proto.Marshal(segData)
	assert.Nil(err)

	creds := base64.StdEncoding.EncodeToString(data)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	assert.Nil(err)
	profiles[0].Bitrate = "432000"
	profiles[1].Bitrate = "765000"
	assert.Equal(profiles, md.Profiles)
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
		},
		{
			Name:    "prof2",
			Width:   int32(987),
			Height:  int32(654),
			Bitrate: int32(321),
			Fps:     uint32(198),
		},
	}

	// testing happy case scenario
	expectedProfiles := []ffmpeg.VideoProfile{
		{
			Name:       videoProfiles[0].Name,
			Bitrate:    fmt.Sprint(videoProfiles[0].Bitrate),
			Framerate:  uint(videoProfiles[0].Fps),
			Resolution: fmt.Sprintf("%dx%d", videoProfiles[0].Width, videoProfiles[0].Height),
		},
		{
			Name:       videoProfiles[1].Name,
			Bitrate:    fmt.Sprint(videoProfiles[1].Bitrate),
			Framerate:  uint(videoProfiles[1].Fps),
			Resolution: fmt.Sprintf("%dx%d", videoProfiles[1].Width, videoProfiles[1].Height),
		},
	}

	ffmpegProfiles := makeFfmpegVideoProfiles(videoProfiles)
	expectedResolution := fmt.Sprintf("%dx%d", videoProfiles[0].Width, videoProfiles[0].Height)
	assert.Equal(expectedProfiles, ffmpegProfiles)
	assert.Equal(ffmpegProfiles[0].Resolution, expectedResolution)

	// empty name should return automatically generated name
	videoProfiles[0].Name = ""
	expectedName := "net_" + fmt.Sprintf("%dx%d_%d", videoProfiles[0].Width, videoProfiles[0].Height, videoProfiles[0].Bitrate)
	ffmpegProfiles = makeFfmpegVideoProfiles(videoProfiles)
	assert.Equal(ffmpegProfiles[0].Name, expectedName)
}

func TestServeSegment_OSSaveDataError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

	mos := &mockOSSession{}

	mos.On("SaveData", mock.Anything, mock.Anything).Return("", errors.New("SaveData error"))

	tData := &core.TranscodeData{Segments: []*core.TranscodedSegmentData{&core.TranscodedSegmentData{Data: []byte("foo")}}}
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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
			ffmpeg.P240p30fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

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

func TestServeSegment_UnacceptableProcessPaymentError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)
	assert := assert.New(t)
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	_, err = verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	// Return an an unacceptable error to trigger bad request
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(pm.NewMockReceiveError(errors.New("some error"), false)).Once()

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

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(errors.New("some error")).Once()
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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
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
	}

	price := &net.PriceInfo{
		PricePerUnit:  2,
		PixelsPerUnit: 3,
	}
	// Return an acceptable payment error to trigger an update to orchestrator info
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(pm.NewMockReceiveError(errors.New("some error"), true)).Once()
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

	orch.On("TicketParams", mock.Anything).Return(params, nil).Once()
	orch.On("PriceInfo", mock.Anything).Return(price, nil)

	uri, err := url.Parse("http://google.com")
	require.Nil(err)
	orch.On("ServiceURI").Return(uri)

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

	// Return an acceptable payment error to trigger an update to orchestrator info
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(pm.NewMockReceiveError(errors.New("some other error"), true)).Once()
	orch.On("TicketParams", mock.Anything).Return(params, nil).Once()

	resp = httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	require.Nil(err)

	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert.Equal(http.StatusOK, resp.StatusCode)

	assert.Equal(uri.String(), tr.Info.Transcoder)
	assert.Equal(params.Recipient, tr.Info.TicketParams.Recipient)
	assert.Equal(params.FaceValue, tr.Info.TicketParams.FaceValue)
	assert.Equal(params.WinProb, tr.Info.TicketParams.WinProb)
	assert.Equal(params.RecipientRandHash, tr.Info.TicketParams.RecipientRandHash)
	assert.Equal(params.Seed, tr.Info.TicketParams.Seed)

	// Test orchestratorInfo error
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(pm.NewMockReceiveError(errors.New("some error"), true)).Once()
	orch.On("TicketParams", mock.Anything).Return(nil, errors.New("TicketParams error")).Once()

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
		ManifestID:  core.RandomManifestID(),
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	_, err = verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(false)

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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
			ffmpeg.P240p30fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
			ffmpeg.P240p30fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)

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
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("SufficientBalance", mock.Anything, s.ManifestID).Return(true)
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
		ManifestID:  core.RandomManifestID(),
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "Sign error", err.Error())
}

func TestSubmitSegment_RatPriceInfoError(t *testing.T) {
	b := stubBroadcaster2()

	s := &BroadcastSession{
		Broadcaster: b,
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			PriceInfo: &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 0},
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.EqualError(t, err, "invalid priceInfo.pixelsPerUnit")
}

func TestSubmitSegment_EstimateFeeError(t *testing.T) {
	b := stubBroadcaster2()

	s := &BroadcastSession{
		Broadcaster: b,
		ManifestID:  core.RandomManifestID(),
		// Contains invalid profile
		Profiles: []ffmpeg.VideoProfile{ffmpeg.VideoProfile{Resolution: "foo"}},
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
		ManifestID:  core.RandomManifestID(),
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
		ManifestID:       core.RandomManifestID(),
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
		ManifestID:       core.RandomManifestID(),
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
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: "https://foo.com",
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
		ManifestID:  core.RandomManifestID(),
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
		ManifestID:  core.RandomManifestID(),
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
		ManifestID:  core.RandomManifestID(),
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

func TestSubmitSegment_Success(t *testing.T) {
	require := require.New(t)

	dummyRes := func(tSegData []*net.TranscodedSegmentData) *net.TranscodeResult {
		return &net.TranscodeResult{
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

		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		},
	}

	assert := assert.New(t)

	runChecks = func(r *http.Request) {
		assert.Equal("video/MP2T", r.Header.Get("Content-Type"))

		data, err := ioutil.ReadAll(r.Body)
		require.Nil(err)

		assert.Equal([]byte("dummy"), data)
	}

	tdata, err := SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal(1, len(tdata.Segments))
	assert.Equal("foo", tdata.Segments[0].Url)
	assert.Equal([]byte("bar"), tdata.Sig)

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

func TestSubmitSegment_UpdateOrchestratorInfo(t *testing.T) {
	require := require.New(t)

	params := pm.TicketParams{
		Recipient:         ethcommon.Address{},
		FaceValue:         big.NewInt(100),
		WinProb:           big.NewInt(100),
		RecipientRandHash: pm.RandHash(),
		Seed:              big.NewInt(100),
	}

	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{
					&net.TranscodedSegmentData{Url: "foo", Pixels: 10},
				},
				Sig: []byte("bar"),
			},
		},
		Info: &net.OrchestratorInfo{
			Transcoder: "http://google.com",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
			TicketParams: &net.TicketParams{
				Recipient:         params.Recipient.Bytes(),
				FaceValue:         params.FaceValue.Bytes(),
				WinProb:           params.WinProb.Bytes(),
				RecipientRandHash: params.RecipientRandHash.Bytes(),
				Seed:              params.Seed.Bytes(),
			},
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	// Test with no pm.Sender in BroadcastSession

	newSess := func() *BroadcastSession {
		return &BroadcastSession{
			Broadcaster: stubBroadcaster2(),
			ManifestID:  core.RandomManifestID(),
			OrchestratorInfo: &net.OrchestratorInfo{
				Transcoder: ts.URL,
				PriceInfo: &net.PriceInfo{
					PricePerUnit:  2,
					PixelsPerUnit: 1,
				},
			},
		}
	}

	s := newSess()

	assert := assert.New(t)

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal("http://google.com", s.OrchestratorInfo.Transcoder)
	assert.Equal(tr.Info.TicketParams.Recipient, s.OrchestratorInfo.TicketParams.Recipient)
	assert.Equal(tr.Info.TicketParams.FaceValue, s.OrchestratorInfo.TicketParams.FaceValue)
	assert.Equal(tr.Info.TicketParams.WinProb, s.OrchestratorInfo.TicketParams.WinProb)
	assert.Equal(tr.Info.TicketParams.RecipientRandHash, s.OrchestratorInfo.TicketParams.RecipientRandHash)
	assert.Equal(tr.Info.TicketParams.Seed, s.OrchestratorInfo.TicketParams.Seed)

	// Test with pm.Sender in BroadcastSession

	sender := &pm.MockSender{}
	s = newSess()
	s.Sender = sender

	batch := &pm.TicketBatch{
		TicketParams: &pm.TicketParams{
			Recipient: pm.RandAddress(),
			FaceValue: big.NewInt(1234),
			WinProb:   big.NewInt(5678),
			Seed:      big.NewInt(7777),
		},
		TicketExpirationParams: &pm.TicketExpirationParams{},
		Sender:                 pm.RandAddress(),
		SenderParams: []*pm.TicketSenderParams{
			&pm.TicketSenderParams{SenderNonce: 777, Sig: pm.RandBytes(42)},
		},
	}

	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(batch, nil)
	sender.On("StartSession", params).Return("foobar")

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal("foobar", s.PMSessionID)
	sender.AssertCalled(t, "StartSession", params)

	// Test balance update completion with last known price and NOT the price
	// included in an updated OrchestratorInfo message

	ratMatcher := func(rat *big.Rat) interface{} {
		return mock.MatchedBy(func(x *big.Rat) bool { return x.Cmp(rat) == 0 })
	}

	existingCredit := big.NewRat(100, 1)
	// change = existingCredit - (pixelCount * price) = 100 - (10 * 2) = 80
	// Note that price = 2 which is the last known price and not the price = 1 included in the
	// updated OrchestratorInfo message
	change := big.NewRat(80, 1)
	balance := &mockBalance{}
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(0, big.NewRat(0, 1), existingCredit)
	balance.On("Credit", ratMatcher(change))
	sender = &pm.MockSender{}
	sender.On("EV", mock.Anything).Return(big.NewRat(0, 1), nil)
	sender.On("StartSession", params).Return("foobar")
	s = newSess()
	s.Balance = balance
	s.Sender = sender

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	balance.AssertCalled(t, "Credit", ratMatcher(change))

	// Test does not crash if OrchestratorInfo.TicketParams is nil

	sender = &pm.MockSender{}
	s = newSess()
	s.Sender = sender

	// Update stub server to return OrchestratorInfo without TicketParams
	tr.Info = &net.OrchestratorInfo{
		Transcoder: "http://google.com",
	}
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(batch, nil)
	sender.On("StartSession", mock.Anything).Return("foobar")

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal("http://google.com", s.OrchestratorInfo.Transcoder)
	sender.AssertNotCalled(t, "StartSession", mock.Anything)

	// Test update OrchestratorOS

	s = newSess()

	tr.Info = &net.OrchestratorInfo{
		Transcoder: "http://google.com",
		Storage: []*net.OSInfo{
			&net.OSInfo{
				StorageType: 1,
				S3Info: &net.S3OSInfo{
					Host: "http://apple.com",
				},
			},
		},
	}
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal(tr.Info.Storage[0].StorageType, s.OrchestratorOS.GetInfo().StorageType)
	assert.Equal(tr.Info.Storage[0].S3Info.Host, s.OrchestratorOS.GetInfo().S3Info.Host)
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
