package server

import (
	"bytes"
	"crypto/tls"
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
)

type stubTranscoder struct {
	called   int
	profiles []ffmpeg.VideoProfile
	fname    string
	err      error
}

var testRemoteTranscoderResults = &core.TranscodeData{
	Segments: []*core.TranscodedSegmentData{
		&core.TranscodedSegmentData{Data: []byte("body1"), Pixels: 777},
		&core.TranscodedSegmentData{Data: []byte("body2"), Pixels: 888},
	},
	Pixels: 999,
}

func (st *stubTranscoder) Transcode(job string, fname string, profiles []ffmpeg.VideoProfile) (*core.TranscodeData, error) {
	st.called++
	st.fname = fname
	st.profiles = profiles
	if st.err != nil {
		return nil, st.err
	}
	return testRemoteTranscoderResults, nil
}

// Tests that `runTranscode` actually calls transcoders `Transcoder` function,
// sends results back by HTTP, and doesn't panic if it can't contact orchestrator
func TestRemoteTranscoder_Profiles(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}

	assert := assert.New(t)
	assert.Nil(nil)
	notify := &net.NotifySegment{
		TaskId:   742,
		Profiles: common.ProfilesToTranscodeOpts(profiles),
		Url:      "linktomanifest",
	}
	tr := &stubTranscoder{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.Transcoder = tr

	runTranscode(node, "badaddress", httpc, notify)
	assert.Equal(1, tr.called)
	assert.Equal(profiles, tr.profiles)
	assert.Equal("linktomanifest", tr.fname)

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
	assert.Equal(2, tr.called)
	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal("999", headers.Get("Pixels"))
	assert.Equal("multipart/mixed; boundary=17b336b6e6ae071e928f", headers.Get("Content-Type"))
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

func TestRemoteTranscoder_FullProfiles(t *testing.T) {
	assert := assert.New(t)
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

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
			Format:     ffmpeg.FormatMP4,
		},
	}

	fullProfiles, err := common.FFmpegProfiletoNetProfile(profiles)
	assert.Nil(err)

	notify := &net.NotifySegment{
		TaskId:       742,
		FullProfiles: fullProfiles,
		Url:          "linktomanifest",
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
	profiles[0].Bitrate = "432000"
	profiles[1].Bitrate = "765000"
	profiles[0].Format = ffmpeg.FormatMPEGTS
	assert.Equal(profiles, tr.profiles)
	assert.Equal("linktomanifest", tr.fname)

	// Test deserialization failure from invalid full profile format
	notify.FullProfiles[1].Format = -1
	runTranscode(node, "", httpc, notify)
	assert.Equal(2, tr.called)
	assert.Nil(nil, tr.profiles)
}

func TestRemoteTranscoderError(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}

	assert := assert.New(t)
	assert.Nil(nil)
	notify := &net.NotifySegment{
		TaskId:   742,
		Profiles: common.ProfilesToTranscodeOpts(profiles),
		Url:      "linktomanifest",
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
	notify.Profiles = common.ProfilesToTranscodeOpts(profiles)
	runTranscode(node, parsedURL.Host, httpc, notify)
	assert.Equal(2, tr.called)
	assert.NotNil(body)
	assert.Equal("segment / profile mismatch", string(body))
}
