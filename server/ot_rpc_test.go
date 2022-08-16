package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTranscoder struct {
	called   int
	profiles []ffmpeg.VideoProfile
	fname    string
	err      error
}

var testRemoteTranscoderResults = &core.TranscodeData{
	Segments: []*core.TranscodedSegmentData{
		{Data: []byte("body1"), Pixels: 777},
		{Data: []byte("body2"), Pixels: 888},
	},
	Pixels: 999,
}

func (st *stubTranscoder) Transcode(ctx context.Context, md *core.SegTranscodingMetadata) (*core.TranscodeData, error) {
	st.called++
	st.fname = md.Fname
	st.profiles = md.Profiles
	if st.err != nil {
		return nil, st.err
	}
	return testRemoteTranscoderResults, nil
}

func (st *stubTranscoder) EndTranscodingSession(sessionId string) {

}

// Tests that `runTranscode` actually calls transcoders `Transcoder` function,
// sends results back by HTTP, and doesn't panic if it can't contact orchestrator
func TestRemoteTranscoder_Profiles(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}

	assert := assert.New(t)
	segData, err := core.NetSegData(&core.SegTranscodingMetadata{Profiles: profiles})
	assert.Nil(err)

	var segmentRead int
	segmentTs := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := ioutil.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("segment's binary data"))
		segmentRead++
	}))
	defer segmentTs.Close()

	notify := &net.NotifySegment{
		TaskId:  742,
		SegData: segData,
		Url:     segmentTs.URL,
	}
	tr := &stubTranscoder{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.Transcoder = tr

	runTranscode(node, "badaddress", httpc, notify)
	assert.Equal(1, segmentRead)
	assert.Equal(1, tr.called)
	// reset some things that are different when using profiles
	profiles[0].Bitrate = "6000000" // presets use "k" abbrev; conversions don't
	profiles[1].Bitrate = "400000"
	profiles[0].AspectRatio = "" // unused across the wire
	profiles[1].AspectRatio = ""
	profiles[0].Format = ffmpeg.FormatMPEGTS // default is ffmpeg.FormatNone
	profiles[1].Format = ffmpeg.FormatMPEGTS
	assert.Equal(profiles, tr.profiles)
	assert.True(strings.HasSuffix(tr.fname, ".tempfile"))

	var headers http.Header
	var body []byte
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := ioutil.ReadAll(r.Body)
		assert.NoError(err)
		headers = r.Header
		body = out
		w.Write(nil)
	}))
	defer ts.Close()
	parsedURL, _ := url.Parse(ts.URL)
	rand.Seed(123)
	runTranscode(node, parsedURL.Host, httpc, notify)
	assert.Equal(2, segmentRead)
	assert.Equal(2, tr.called)
	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal("999", headers.Get("Pixels"))
	assert.Equal("multipart/mixed; boundary=57bddd1eb02b9ffae8dc", headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerLPT, headers.Get("Authorization"))
	mediaType, params, err := mime.ParseMediaType(headers.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.NoError(err)
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var i int
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		bodyPart, err := ioutil.ReadAll(p)
		assert.NoError(err)
		assert.Equal(testRemoteTranscoderResults.Segments[i].Data, bodyPart)

		pixels, err := strconv.ParseInt(p.Header.Get("Pixels"), 10, 64)
		assert.NoError(err)
		assert.Equal(testRemoteTranscoderResults.Segments[i].Pixels, pixels)

		assert.Equal("video/mp2t", strings.ToLower(p.Header.Get("Content-Type")))

		i++
	}
}

func TestTranscodeResults_ErrorsWhenAuthHeaderMissing(t *testing.T) {
	var l lphttp
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/TranscodeResults", nil)
	require.NoError(t, err)

	l.TranscodeResults(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "Unauthorized")
}

func TestTranscodeResults_ErrorsWhenCredentialsInvalid(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/TranscodeResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerLPT)
	r.Header.Set("Credentials", "BAD CREDENTIALS")

	l.TranscodeResults(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "Unauthorized")
}

func TestTranscodeResults_ErrorsWhenContentTypeMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/TranscodeResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerLPT)
	r.Header.Set("Credentials", "")

	l.TranscodeResults(w, r)
	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	require.Contains(t, w.Body.String(), "mime: no media type")
}

func TestTranscodeResults_ErrorsWhenTaskIDMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/TranscodeResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerLPT)
	r.Header.Set("Credentials", "")
	r.Header.Set("Content-Type", "video/mp4")

	l.TranscodeResults(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Invalid Task ID")
}

func TestTranscodeResults_DoesNotErrorWhenSceneDetectionHeaderMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/TranscodeResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerLPT)
	r.Header.Set("Credentials", "")
	r.Header.Set("Content-Type", "video/mp4")
	r.Header.Set("TaskId", "123")
	r.Header.Set("Pixels", "1")

	l.TranscodeResults(w, r)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestTranscodeResults_ErrorsWhenPixelsHeaderMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/TranscodeResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerLPT)
	r.Header.Set("Credentials", "")
	r.Header.Set("Content-Type", "video/mp4")
	r.Header.Set("TaskId", "123")

	l.TranscodeResults(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Invalid Pixels")
}

func TestRemoteTranscoder_FullProfiles(t *testing.T) {
	assert := assert.New(t)
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	profiles := []ffmpeg.VideoProfile{
		{
			Name:       "prof1",
			Bitrate:    "432k",
			Framerate:  uint(560),
			Resolution: "123x456",
		},
		{
			Name:       "prof2",
			Bitrate:    "765k",
			Framerate:  uint(876),
			Resolution: "456x987",
			Format:     ffmpeg.FormatMP4,
		},
	}

	segData, err := core.NetSegData(&core.SegTranscodingMetadata{Profiles: profiles})
	assert.Nil(err)

	var segmentRead int
	segmentTs := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := ioutil.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("segment's binary data"))
		segmentRead++
	}))
	defer segmentTs.Close()

	notify := &net.NotifySegment{
		TaskId:  742,
		Url:     segmentTs.URL,
		SegData: segData,
	}
	tr := &stubTranscoder{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.Transcoder = tr

	assert.Equal(0, tr.called)
	assert.Empty(tr.profiles)
	assert.Empty(tr.fname)
	runTranscode(node, "badaddress", httpc, notify)
	assert.Equal(1, tr.called)
	assert.Equal(1, segmentRead)
	profiles[0].Bitrate = "432000"
	profiles[1].Bitrate = "765000"
	profiles[0].Format = ffmpeg.FormatMPEGTS
	assert.Equal(profiles, tr.profiles)
	assert.True(strings.HasSuffix(tr.fname, ".tempfile"))

	// Test deserialization failure from invalid full profile format
	notify.SegData.FullProfiles2[1].Format = -1
	runTranscode(node, "", httpc, notify)
	assert.Equal(1, segmentRead)
	assert.Equal(1, tr.called)
	assert.Nil(nil, tr.profiles)
}

func TestRemoteTranscoder_Error(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}

	assert := assert.New(t)
	assert.Nil(nil)
	var segmentRead int
	segmentTs := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := ioutil.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("segment's binary data"))
		segmentRead++
	}))
	defer segmentTs.Close()
	notify := &net.NotifySegment{
		TaskId:   742,
		Profiles: common.ProfilesToTranscodeOpts(profiles),
		Url:      segmentTs.URL,
	}
	tr := &stubTranscoder{}
	errText := "Some error"
	tr.err = fmt.Errorf(errText)
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.Transcoder = tr

	var headers http.Header
	var body []byte
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := ioutil.ReadAll(r.Body)
		assert.NoError(err)
		headers = r.Header
		body = out
		w.Write(nil)
	}))
	defer ts.Close()
	parsedURL, _ := url.Parse(ts.URL)
	runTranscode(node, parsedURL.Host, httpc, notify)
	assert.Equal(0, tr.called)
	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(transcodingErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerLPT, headers.Get("Authorization"))
	assert.Equal("empty seg data", string(body))

	segData, err := core.NetSegData(&core.SegTranscodingMetadata{Profiles: profiles})
	assert.Nil(err)
	notify.SegData = segData

	runTranscode(node, parsedURL.Host, httpc, notify)
	assert.Equal(1, tr.called)
	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(transcodingErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerLPT, headers.Get("Authorization"))
	assert.Equal(errText, string(body))

	// mismatched segment / profile error
	// stub transcoder returns 2 profiles, so we ask for just 1
	tr.err = nil
	assert.Len(profiles, 2) // sanity
	profiles = []ffmpeg.VideoProfile{profiles[0]}
	segData, err = core.NetSegData(&core.SegTranscodingMetadata{Profiles: profiles})
	assert.Nil(err)
	notify.SegData = segData
	runTranscode(node, parsedURL.Host, httpc, notify)
	assert.Equal(2, tr.called)
	assert.NotNil(body)
	assert.Equal("segment / profile mismatch", string(body))

	// unrecoverable error
	// send the response and panic
	tr.err = core.NewUnrecoverableError(errors.New("some error"))
	panicked := false
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	runTranscode(node, parsedURL.Host, httpc, notify)
	assert.Equal(3, tr.called)
	assert.NotNil(body)
	assert.Equal("some error", string(body))
	assert.True(panicked)
}
