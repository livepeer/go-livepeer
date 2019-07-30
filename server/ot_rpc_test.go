package server

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
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
}

var testRemoteTranscoderResults = [][]byte{[]byte("body1"), []byte("body2")}

func (st *stubTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	st.called++
	st.fname = fname
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
		assert.Equal(testRemoteTranscoderResults[i], bodyPart)
		i++
	}
}
