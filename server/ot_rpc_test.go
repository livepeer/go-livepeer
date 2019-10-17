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
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
)

type stubTranscoder struct {
	called int
	fname  string
	err    error
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
	if st.err != nil {
		return nil, st.err
	}
	return testRemoteTranscoderResults, nil
}

// Tests that `runTranscode` actually calls transcoders `Transcoder` function,
// sends results back by HTTP, and doesn't panic if it can't contact orchestrator
func TestRemoteTranscoder(t *testing.T) {
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

		i++
	}
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
}
