package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/vidplayer"
)

func requestSetup(s *LivepeerServer) (http.Handler, *strings.Reader, *httptest.ResponseRecorder) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.HandlePush(w, r)
	})
	reader := strings.NewReader("")
	writer := httptest.NewRecorder()
	return handler, reader, writer
}

func TestPush_ShouldReturn422ForNonRetryable(t *testing.T) {
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()

	d, _ := ioutil.ReadFile("./test.flv")
	reader := bytes.NewReader(d)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani/18.ts", reader)

	dummyRes := func(err string) *net.TranscodeResult {
		return &net.TranscodeResult{
			Result: &net.TranscodeResult_Error{Error: err},
		}
	}

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	tr := dummyRes("unknown error (test)")
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	bsm := bsmWithSessList([]*BroadcastSession{sess})

	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")

	pl := core.NewBasicPlaylistManager("xx", osSession, nil)

	cxn := &rtmpConnection{
		mid:         core.ManifestID("mani"),
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
		params:      &core.StreamParameters{Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p25fps16x9}},
	}

	s.rtmpConnections["mani"] = cxn

	// Should return 503 if got a retryable error with no sessions to use
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(503, resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(err)
	assert.Contains(string(body), "No sessions available")

	// Should return 422 if max attempts reached with unknown error
	oldAttempts := MaxAttempts
	defer func() {
		MaxAttempts = oldAttempts
	}()
	MaxAttempts = 1

	sess = StubBroadcastSession(ts.URL)
	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm
	tr = dummyRes("unknown error (test)")
	buf, err = proto.Marshal(tr)
	require.Nil(t, err)
	w = httptest.NewRecorder()
	reader = bytes.NewReader(d)
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	assert.Equal(422, resp.StatusCode)
	body, err = ioutil.ReadAll(resp.Body)
	assert.NoError(err)
	assert.Contains(string(body), "unknown error (test)")

	// Should return 422 if error is non-retryable due to bad input
	sess = StubBroadcastSession(ts.URL)
	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm
	tr = dummyRes("No keyframes in input")
	buf, err = proto.Marshal(tr)
	require.Nil(t, err)
	w = httptest.NewRecorder()
	reader = bytes.NewReader(d)
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	assert.Equal(422, resp.StatusCode)
	body, err = ioutil.ReadAll(resp.Body)
	assert.NoError(err)
	assert.Contains(string(body), "No keyframes in input")
}

func TestPush_MultipartReturn(t *testing.T) {
	assert := assert.New(t)
	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	reader := strings.NewReader("InsteadOf.TS")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani/17.ts", reader)

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

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	segPath := "/transcoded/segment.ts"
	tSegData := []*net.TranscodedSegmentData{{Url: ts.URL + segPath, Pixels: 100}}
	tr := dummyRes(tSegData)
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	mux.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transcoded binary data"))
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.Params.ManifestID = "mani"
	bsm := bsmWithSessList([]*BroadcastSession{sess})

	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")

	oldjpqt := core.JsonPlaylistQuitTimeout
	defer func() {
		core.JsonPlaylistQuitTimeout = oldjpqt
	}()
	core.JsonPlaylistQuitTimeout = 0 * time.Second
	pl := core.NewBasicPlaylistManager("xx", osSession, nil)

	cxn := &rtmpConnection{
		mid:         core.ManifestID("mani"),
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
		params:      &core.StreamParameters{Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p25fps16x9}},
	}

	s.rtmpConnections["mani"] = cxn

	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(200, resp.StatusCode)

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr := multipart.NewReader(resp.Body, params["boundary"])
	var i int
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		mediaType, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Contains(params, "name")
		assert.Len(params, 1)
		assert.Equal(params["name"], "P144p25fps16x9_17.txt")
		assert.Equal(`attachment; filename="P144p25fps16x9_17.txt"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		bodyPart, err := ioutil.ReadAll(p)
		assert.NoError(err)
		assert.Equal("application/vnd+livepeer.uri", mediaType)
		up, err := url.Parse(string(bodyPart))
		assert.Nil(err)
		assert.Equal(segPath, up.Path)

		i++
	}
	assert.Equal(1, i)
	assert.Equal(uint64(12), cxn.sourceBytes)
	assert.Equal(uint64(0), cxn.transcodedBytes)

	bsm.trustedPool.sel.Clear()
	bsm.trustedPool.sel.Add([]*BroadcastSession{sess})

	sess.BroadcasterOS = osSession
	// Body should be empty if no Accept header specified
	reader.Seek(0, 0)
	req = httptest.NewRequest("POST", "/live/mani/15.ts", reader)
	w = httptest.NewRecorder()
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal("", strings.TrimSpace(string(body)))

	// Binary data should be returned
	bsm.trustedPool.sel.Clear()
	bsm.trustedPool.sel.Add([]*BroadcastSession{sess})

	reader.Seek(0, 0)
	reader = strings.NewReader("InsteadOf.TS")
	req = httptest.NewRequest("POST", "/live/mani/12.ts", reader)
	w = httptest.NewRecorder()
	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	assert.Equal(200, resp.StatusCode)

	mediaType, params, err = mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr = multipart.NewReader(resp.Body, params["boundary"])
	i = 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		mediaType, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Contains(params, "name")
		assert.Len(params, 1)
		assert.Equal("P144p25fps16x9_12.ts", params["name"])
		assert.Equal(`attachment; filename="P144p25fps16x9_12.ts"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		bodyPart, err := ioutil.ReadAll(p)
		assert.Nil(err)
		assert.Equal("video/mp2t", strings.ToLower(mediaType))
		assert.Equal("transcoded binary data", string(bodyPart))

		i++
	}
	assert.Equal(1, i)
	assert.Equal(uint64(36), cxn.sourceBytes)
	assert.Equal(uint64(44), cxn.transcodedBytes)

	// No sessions error
	cxn.sessManager.trustedPool.sel.Clear()
	cxn.sessManager.trustedPool.lastSess = nil
	cxn.sessManager.trustedPool.sessMap = make(map[string]*BroadcastSession)

	reader.Seek(0, 0)
	req = httptest.NewRequest("POST", "/live/mani/13.ts", reader)
	w = httptest.NewRecorder()
	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal("No sessions available\n", string(body))
	assert.Equal(503, resp.StatusCode)

	// Input Segment bigger than MaxSegSize
	reader.Seek(0, 0)
	req = httptest.NewRequest("POST", "/live/mani/14.ts", reader)
	w = httptest.NewRecorder()
	req.Header.Set("Accept", "multipart/mixed")
	// set seg size to something small
	tmpSegSize := common.MaxSegSize
	common.MaxSegSize = 1 // 1 byte
	defer func() { common.MaxSegSize = tmpSegSize }()
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal("Error reading http request body: input bigger than max buffer size\n", string(body))
	assert.Equal(500, resp.StatusCode)
	pl.Cleanup()
}

func TestPush_MemoryRequestError(t *testing.T) {
	// assert http request body error returned
	assert := assert.New(t)
	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	handler, _, w := requestSetup(s)
	f, err := os.Open(`doesn't exist`)
	require.NotNil(t, err)
	req := httptest.NewRequest("POST", "/live/seg.ts", f)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "Error reading http request body")
}

func TestPush_EmptyURLError(t *testing.T) {
	// assert http request body error returned
	assert := assert.New(t)
	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/.ts", nil)
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(string(body), "Bad URL")
}

func TestPush_ShouldUpdateLastUsed(t *testing.T) {
	assert := assert.New(t)
	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani1/1.ts", nil)
	s.HandlePush(w, req)
	resp := w.Result()
	resp.Body.Close()
	lu := s.rtmpConnections["mani1"].lastUsed
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani1/1.ts", nil)
	s.HandlePush(w, req)
	resp = w.Result()
	resp.Body.Close()
	assert.True(lu.Before(s.rtmpConnections["mani1"].lastUsed))
}

func TestPush_HTTPIngest(t *testing.T) {
	assert := assert.New(t)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
	n.NodeType = core.BroadcasterNode
	reader := strings.NewReader("")
	req := httptest.NewRequest("POST", "/live/name/1.mp4", reader)

	ctx, cancel := context.WithCancel(context.Background())
	// HTTP ingest disabled
	s, _ := NewLivepeerServer("127.0.0.1:1938", n, false, "")
	s.SetContextFromUnitTest(ctx)
	h, pattern := s.HTTPMux.Handler(req)
	assert.Equal("", pattern)

	writer := httptest.NewRecorder()
	h.ServeHTTP(writer, req)
	resp := writer.Result()
	defer resp.Body.Close()
	assert.Equal(404, resp.StatusCode)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	// HTTP ingest enabled
	s, _ = NewLivepeerServer("127.0.0.1:1938", n, true, "")
	s.SetContextFromUnitTest(ctx)
	h, pattern = s.HTTPMux.Handler(req)
	assert.Equal("/live/", pattern)

	writer = httptest.NewRecorder()
	h.ServeHTTP(writer, req)
	resp = writer.Result()
	defer resp.Body.Close()
	assert.Equal(503, resp.StatusCode)

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal("No sessions available", strings.TrimSpace(string(body)))
	cancel()
}

func TestPush_MP4(t *testing.T) {
	oldProfs := BroadcastJobVideoProfiles
	defer func() { BroadcastJobVideoProfiles = oldProfs }()
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P720p25fps16x9}

	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	// Do a bunch of setup. Would be nice to simplify this one day...
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	s.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	defer func() { s.rtmpConnections = map[core.ManifestID]*rtmpConnection{} }()
	segHandler := getHLSSegmentHandler(s)
	ts, mux := stubTLSServer()
	defer ts.Close()

	// sometimes LivepeerServer needs time  to start
	// esp if this is the only test in the suite being run (eg, via `-run)
	time.Sleep(10 * time.Millisecond)

	sd := &stubDiscovery{}
	sd.infos = []*net.OrchestratorInfo{{Transcoder: ts.URL, AuthToken: stubAuthToken}}
	s.LivepeerNode.OrchestratorPool = sd

	dummyRes := func(tSegData []*net.TranscodedSegmentData) *net.TranscodeResult {
		return &net.TranscodeResult{
			Result: &net.TranscodeResult_Data{
				Data: &net.TranscodeData{
					Segments: tSegData,
				},
			},
		}
	}
	segPath := "/random"
	tSegData := []*net.TranscodedSegmentData{{Url: ts.URL + segPath, Pixels: 100}}
	tr := dummyRes(tSegData)
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	mux.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transcoded binary data"))
	})

	// Check default response: should be empty, with OS populated
	handler, reader, writer := requestSetup(s)
	reader = strings.NewReader("a video file goes here")
	req := httptest.NewRequest("POST", "/live/name/1.mp4", reader)
	handler.ServeHTTP(writer, req)
	resp := writer.Result()
	defer resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	assert.Empty(body)
	// Check OS for source
	vpath, err := url.Parse("/stream/name/source/1.mp4")
	assert.Nil(err)
	body, err = segHandler(vpath)
	assert.Nil(err)
	assert.Equal("a video file goes here", string(body))
	// Check OS for transcoded rendition
	vpath, err = url.Parse("/stream/name/P720p25fps16x9/1.mp4")
	assert.Nil(err)
	body, err = segHandler(vpath)
	assert.Nil(err)
	assert.Equal("transcoded binary data", string(body))
	// Sanity check version with mpegts extension doesn't exist
	vpath, err = url.Parse("/stream/name/source/1.ts")
	assert.Nil(err)
	body, err = segHandler(vpath)
	assert.Equal(vidplayer.ErrNotFound, err)
	assert.Empty(body)
	vpath, err = url.Parse("/stream/name/P720p25fps16x9/1.ts")
	assert.Nil(err)
	body, err = segHandler(vpath)
	assert.Equal(vidplayer.ErrNotFound, err)
	assert.Empty(body)
	// We can't actually test the returned content type here.
	// (That is handled within LPMS, so assume it's fine from here.)

	// Check multipart response for MP4s
	reader = strings.NewReader("a new video goes here")
	writer = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/name/2.mp4", reader)
	req.Header.Set("Accept", "multipart/mixed")
	handler.ServeHTTP(writer, req)
	resp = writer.Result()
	assert.Equal(200, resp.StatusCode)
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr := multipart.NewReader(resp.Body, params["boundary"])
	i := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		mediaType, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Contains(params, "name")
		assert.Len(params, 1)
		assert.Equal(params["name"], "P720p25fps16x9_2.mp4")
		assert.Equal(`attachment; filename="P720p25fps16x9_2.mp4"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P720p25fps16x9", p.Header.Get("Rendition-Name"))
		bodyPart, err := ioutil.ReadAll(p)
		assert.Nil(err)
		assert.Equal("video/mp4", mediaType)
		assert.Equal("transcoded binary data", string(bodyPart))

		i++
	}
	assert.Equal(1, i)

	// Check formats
	for _, cxn := range s.rtmpConnections {
		assert.Equal(ffmpeg.FormatMP4, cxn.profile.Format)
		for _, p := range cxn.params.Profiles {
			assert.Equal(ffmpeg.FormatMP4, p.Format)
		}
	}
}

func TestPush_SetVideoProfileFormats(t *testing.T) {
	oldProfs := BroadcastJobVideoProfiles
	defer func() { BroadcastJobVideoProfiles = oldProfs }()
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P720p25fps16x9, ffmpeg.P720p60fps16x9}

	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	// s := setupServer()
	// s, cancel := setupServerWithCancelAndPorts()
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	// sometimes LivepeerServer needs time  to start
	// esp if this is the only test in the suite being run (eg, via `-run)
	// time.Sleep(10 * time.Millisecond)
	s.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	defer func() { s.rtmpConnections = map[core.ManifestID]*rtmpConnection{} }()

	// Base case, mpegts
	h, r, w := requestSetup(s)
	req := httptest.NewRequest("POST", "/live/seg/0.ts", r)
	h.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	assert.Len(s.rtmpConnections, 1)
	for _, cxn := range s.rtmpConnections {
		assert.Equal(ffmpeg.FormatMPEGTS, cxn.profile.Format)
		assert.Len(cxn.params.Profiles, 2)
		assert.Len(BroadcastJobVideoProfiles, 2)
		for i, p := range cxn.params.Profiles {
			assert.Equal(ffmpeg.FormatMPEGTS, p.Format)
			// HTTP push mutates the profiles, causing undesirable changes to
			// the default set of broadcast profiles that persist to subsequent
			// streams. Make sure this doesn't happen!
			assert.Equal(ffmpeg.FormatNone, BroadcastJobVideoProfiles[i].Format)
		}
	}

	// Sending a MP4 under the same stream name doesn't change assigned profiles
	h, r, w = requestSetup(s)
	req = httptest.NewRequest("POST", "/live/seg/1.ts", r)
	h.ServeHTTP(w, req)
	resp = w.Result()
	defer resp.Body.Close()

	assert.Len(s.rtmpConnections, 1)
	for _, cxn := range s.rtmpConnections {
		assert.Equal(ffmpeg.FormatMPEGTS, cxn.profile.Format)
		assert.Len(cxn.params.Profiles, 2)
		assert.Len(BroadcastJobVideoProfiles, 2)
		for i, p := range cxn.params.Profiles {
			assert.Equal(ffmpeg.FormatMPEGTS, p.Format)
			assert.Equal(ffmpeg.FormatNone, BroadcastJobVideoProfiles[i].Format)
		}
	}

	// Sending a MP4 under a new stream name sets the profile correctly
	h, r, w = requestSetup(s)
	req = httptest.NewRequest("POST", "/live/new/0.mp4", r)
	h.ServeHTTP(w, req)
	resp = w.Result()
	defer resp.Body.Close()

	assert.Len(s.rtmpConnections, 2)
	cxn, ok := s.rtmpConnections["new"]
	assert.True(ok, "stream did not exist")
	assert.Equal(ffmpeg.FormatMP4, cxn.profile.Format)
	assert.Len(cxn.params.Profiles, 2)
	assert.Len(BroadcastJobVideoProfiles, 2)
	for i, p := range cxn.params.Profiles {
		assert.Equal(ffmpeg.FormatMP4, p.Format)
		assert.Equal(ffmpeg.FormatNone, BroadcastJobVideoProfiles[i].Format)
	}

	hookCalled := 0
	// Sanity check that default profile with webhook is copied
	// Checking since there is special handling for the default set of profiles
	// within the webhook hander.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := authWebhookResponse{ManifestID: "intweb"}
		val, err := json.Marshal(auth)
		assert.Nil(err, "invalid auth webhook response")
		w.Write(val)
		hookCalled++
	}))
	defer ts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, ts.URL)

	h, r, w = requestSetup(s)
	req = httptest.NewRequest("POST", "/live/web/0.mp4", r)
	h.ServeHTTP(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	assert.Equal(1, hookCalled)

	assert.Len(s.rtmpConnections, 3)
	cxn, ok = s.rtmpConnections["web"]
	assert.False(ok, "stream should not exist")
	cxn, ok = s.rtmpConnections["intweb"]
	assert.True(ok, "stream did not exist")
	assert.Equal(ffmpeg.FormatMP4, cxn.profile.Format)
	assert.Len(cxn.params.Profiles, 2)
	assert.Len(BroadcastJobVideoProfiles, 2)
	for i, p := range cxn.params.Profiles {
		assert.Equal(ffmpeg.FormatMP4, p.Format)
		assert.Equal(ffmpeg.FormatNone, BroadcastJobVideoProfiles[i].Format)
	}
	// Server has empty sessions list, so it will return 503
	assert.Equal(503, resp.StatusCode)

	h, r, w = requestSetup(s)
	req = httptest.NewRequest("POST", "/live/web/1.mp4", r)
	h.ServeHTTP(w, req)
	resp = w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	// webhook should not be called again
	assert.Equal(1, hookCalled)

	assert.Len(s.rtmpConnections, 3)
	cxn, ok = s.rtmpConnections["web"]
	assert.False(ok, "stream should not exist")
	cxn, ok = s.rtmpConnections["intweb"]
	assert.True(ok, "stream did not exist")
	glog.Errorf("===> body: %s", body)
	assert.Equal(503, resp.StatusCode)
}

func TestPush_ShouldRemoveSessionAfterTimeoutIfInternalMIDIsUsed(t *testing.T) {
	defer goleak.VerifyNone(t, common.IgnoreRoutines()...)

	oldRI := httpPushTimeout
	httpPushTimeout = 100 * time.Millisecond
	defer func() { httpPushTimeout = oldRI }()
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()

	hookCalled := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := authWebhookResponse{ManifestID: "intmid"}
		val, err := json.Marshal(auth)
		assert.Nil(err, "invalid auth webhook response")
		w.Write(val)
		hookCalled++
	}))
	defer ts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, ts.URL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/extmid1/1.ts", nil)
	s.HandlePush(w, req)
	resp := w.Result()
	resp.Body.Close()
	assert.Equal(1, hookCalled)
	s.connectionLock.Lock()
	_, exists := s.rtmpConnections["intmid"]
	_, existsExt := s.rtmpConnections["extmid1"]
	intmid := s.internalManifests["extmid1"]
	s.connectionLock.Unlock()
	assert.Equal("intmid", string(intmid))
	assert.True(exists)
	assert.False(existsExt)
	time.Sleep(150 * time.Millisecond)
	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["intmid"]
	_, extEx := s.internalManifests["extmid1"]
	s.connectionLock.Unlock()
	cancel()
	assert.False(exists)
	assert.False(extEx)
}

func TestPush_ShouldRemoveSessionAfterTimeout(t *testing.T) {
	defer goleak.VerifyNone(t, common.IgnoreRoutines()...)

	oldRI := httpPushTimeout
	httpPushTimeout = 100 * time.Millisecond
	defer func() { httpPushTimeout = oldRI }()
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani3/1.ts", nil)
	s.HandlePush(w, req)
	resp := w.Result()
	resp.Body.Close()
	s.connectionLock.Lock()
	_, exists := s.rtmpConnections["mani3"]
	s.connectionLock.Unlock()
	assert.True(exists)
	time.Sleep(150 * time.Millisecond)
	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["mani3"]
	s.connectionLock.Unlock()
	cancel()
	assert.False(exists)
}

func TestPush_ShouldNotPanicIfSessionAlreadyRemoved(t *testing.T) {
	oldRI := httpPushTimeout
	httpPushTimeout = 100 * time.Millisecond
	defer func() { httpPushTimeout = oldRI }()
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani2/1.ts", nil)
	s.HandlePush(w, req)
	resp := w.Result()
	resp.Body.Close()
	s.connectionLock.Lock()
	_, exists := s.rtmpConnections["mani2"]
	s.connectionLock.Unlock()
	assert.True(exists)
	s.connectionLock.Lock()
	delete(s.rtmpConnections, "mani2")
	s.connectionLock.Unlock()
	time.Sleep(200 * time.Millisecond)
	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["mani2"]
	s.connectionLock.Unlock()
	assert.False(exists)
}

func TestPush_ResetWatchdog(t *testing.T) {
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()

	waitBarrier := func(ch chan struct{}) bool {
		select {
		case <-ch:
			return true
		case <-time.After(1 * time.Second):
			return false
		}
	}

	// override reset func with our own instrumentation
	cancelCount := 0
	resetCount := 0
	var wrappedCancel func()
	var wg sync.WaitGroup // to synchronize on cancels of the watchdog
	timerCreationBarrier := make(chan struct{})
	oldResetTimer := httpPushResetTimer
	httpPushResetTimer = func() (context.Context, context.CancelFunc) {
		wg.Add(1)
		ctx, cancel := context.WithCancel(context.Background())
		resetCount++
		wrappedCancel = func() {
			cancelCount++
			cancel()
			wg.Done()
		}
		timerCreationBarrier <- struct{}{}
		return ctx, wrappedCancel
	}
	defer func() { httpPushResetTimer = oldResetTimer }()

	// sanity check : normal flow should result in a single cancel
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/name/0.ts", nil)
	s.HandlePush(w, req)
	assert.True(waitBarrier(timerCreationBarrier), "timer creation timed out")
	assert.True(wgWait(&wg), "watchdog did not exit")
	assert.Equal(1, cancelCount)
	assert.Equal(1, resetCount)

	// set up for "long transcode" : one timeout before returning normally
	ts, mux := stubTLSServer()
	defer ts.Close()
	serverBarrier := make(chan struct{})
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		assert.True(waitBarrier(serverBarrier), "server barrier timed out")
	})
	sess := StubBroadcastSession(ts.URL)
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	s.connectionLock.Lock()
	cxn, exists := s.rtmpConnections["name"]
	assert.True(exists)
	cxn.sessManager = bsm
	s.connectionLock.Unlock()

	cancelCount = 0
	resetCount = 0
	pushFuncBarrier := make(chan struct{})
	go func() { s.HandlePush(w, req); pushFuncBarrier <- struct{}{} }()

	assert.True(waitBarrier(timerCreationBarrier), "timer creation timed out")
	assert.Equal(0, cancelCount)
	assert.Equal(1, resetCount)
	cxn.lastUsed = time.Time{} // reset. prob should be locked

	// induce a timeout via cancellation
	wrappedCancel()
	assert.True(waitBarrier(timerCreationBarrier), "timer creation timed out")
	assert.Equal(1, cancelCount)
	assert.Equal(2, resetCount)
	assert.NotEqual(time.Time{}, cxn.lastUsed, "lastUsed was not reset")

	// check things with a normal return
	cxn.lastUsed = time.Time{}  // reset again
	serverBarrier <- struct{}{} // induce server to return
	assert.True(waitBarrier(pushFuncBarrier), "push func timed out")
	assert.True(wgWait(&wg), "watchdog did not exit")
	assert.Equal(2, cancelCount)
	assert.Equal(2, resetCount)
	assert.Equal(time.Time{}, cxn.lastUsed, "lastUsed was reset")

	// check lastUsed is not reset if session disappears
	cancelCount = 0
	resetCount = 0
	go func() { s.HandlePush(w, req); pushFuncBarrier <- struct{}{} }()
	assert.True(waitBarrier(timerCreationBarrier), "timer creation timed out")
	assert.Equal(0, cancelCount)
	assert.Equal(1, resetCount)
	s.connectionLock.Lock()
	cxn, exists = s.rtmpConnections["name"]
	assert.True(exists)
	delete(s.rtmpConnections, "name") // disappear the session
	assert.NotEqual(time.Time{}, cxn.lastUsed, "lastUsed was not reset")
	cxn.lastUsed = time.Time{} // use time zero value as a sentinel
	s.connectionLock.Unlock()

	wrappedCancel() // induce tick
	assert.True(waitBarrier(timerCreationBarrier), "timer creation timed out")
	assert.Equal(1, cancelCount)
	assert.Equal(2, resetCount)
	assert.Equal(time.Time{}, cxn.lastUsed)

	// clean up and some more sanity checks
	serverBarrier <- struct{}{}
	assert.True(waitBarrier(pushFuncBarrier), "push func timed out")
	assert.True(wgWait(&wg), "watchdog did not exit")
	assert.Equal(2, cancelCount)
	assert.Equal(2, resetCount)
	assert.Equal(time.Time{}, cxn.lastUsed, "lastUsed was reset")

	// cancelling again should not lead to a timer reset since push is complete
	assert.Panics(wrappedCancel)
	assert.Equal(3, cancelCount)
	assert.Equal(2, resetCount)
}

func TestPush_FileExtensionError(t *testing.T) {
	// assert file extension error returned
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	handler, reader, w := requestSetup(s)
	req := httptest.NewRequest("POST", "/live/seg.m3u8", reader)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "ignoring file extension")
}

func TestPush_StorageError(t *testing.T) {
	// assert storage error
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	handler, reader, w := requestSetup(s)

	tempStorage := drivers.NodeStorage
	drivers.NodeStorage = nil
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	mid := parseManifestID(req.URL.Path)
	err := removeRTMPStream(context.TODO(), s, mid)
	assert.Equal(errUnknownStream, err)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "ErrStorage")

	// reset drivers.NodeStorage to original value
	drivers.NodeStorage = tempStorage
}

func TestPush_ForAuthWebhookFailure(t *testing.T) {
	// assert app data error
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	handler, reader, w := requestSetup(s)

	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = &url.URL{Path: "notaurl"}
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusForbidden, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "Could not create stream ID")
}

func TestPush_ResolutionWithoutContentResolutionHeader(t *testing.T) {
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	server, cancel := setupServerWithCancel()
	defer serverCleanup(server)
	defer cancel()
	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	handler, reader, w := requestSetup(server)
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	defaultRes := "0x0"

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	assert.Len(server.rtmpConnections, 1)
	for _, cxn := range server.rtmpConnections {
		assert.Equal(cxn.profile.Resolution, defaultRes)
	}

	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
}

func TestPush_ResolutionWithContentResolutionHeader(t *testing.T) {
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	server, cancel := setupServerWithCancel()
	defer serverCleanup(server)
	defer cancel()
	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	handler, reader, w := requestSetup(server)
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	resolution := "123x456"
	req.Header.Set("Content-Resolution", resolution)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	assert.Len(server.rtmpConnections, 1)
	for _, cxn := range server.rtmpConnections {
		assert.Equal(resolution, cxn.profile.Resolution)
	}

	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
}

func TestPush_OSPerStream(t *testing.T) {
	oldjpqt := core.JsonPlaylistQuitTimeout
	defer func() {
		core.JsonPlaylistQuitTimeout = oldjpqt
	}()
	core.JsonPlaylistQuitTimeout = 0 * time.Second

	lpmon.NodeID = "testNode"
	drivers.Testing = true
	assert := assert.New(t)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
	s, _ := NewLivepeerServer("127.0.0.1:1939", n, true, "")
	serverCtx, serverCancel := context.WithCancel(context.Background())
	s.SetContextFromUnitTest(serverCtx)
	defer serverCleanup(s)

	whts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, _ := ioutil.ReadAll(r.Body)
		var req authWebhookReq
		err := json.Unmarshal(out, &req)
		if err != nil {
			glog.Error("Error parsing URL: ", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		assert.Equal(req.URL, "http://example.com/live/sess1/1.ts")
		w.Write([]byte(`{"manifestID":"OSTEST01", "objectStore": "memory://store1", "recordObjectStore": "memory://store2"}`))
	}))

	defer whts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, whts.URL)

	ts, mux := stubTLSServer()
	defer ts.Close()

	// sometimes LivepeerServer needs time  to start
	// esp if this is the only test in the suite being run (eg, via `-run)
	time.Sleep(10 * time.Millisecond)

	oldProfs := BroadcastJobVideoProfiles
	defer func() { BroadcastJobVideoProfiles = oldProfs }()
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P720p25fps16x9}

	sd := &stubDiscovery{}
	sd.infos = []*net.OrchestratorInfo{{Transcoder: ts.URL, AuthToken: stubAuthToken}}
	s.LivepeerNode.OrchestratorPool = sd

	dummyRes := func(tSegData []*net.TranscodedSegmentData) *net.TranscodeResult {
		return &net.TranscodeResult{
			Result: &net.TranscodeResult_Data{
				Data: &net.TranscodeData{
					Segments: tSegData,
				},
			},
		}
	}
	segPath := "/random"
	tSegData := []*net.TranscodedSegmentData{{Url: ts.URL + segPath, Pixels: 100}}
	tr := dummyRes(tSegData)
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	mux.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transcoded binary data"))
	})

	handler, reader, w := requestSetup(s)
	reader = strings.NewReader("segmentbody")
	req := httptest.NewRequest("POST", "/live/sess1/1.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	assert.NotNil(drivers.TestMemoryStorages)
	assert.Contains(drivers.TestMemoryStorages, "store1")
	assert.Contains(drivers.TestMemoryStorages, "store2")
	store1 := drivers.TestMemoryStorages["store1"]
	sess1 := store1.GetSession("OSTEST01")
	assert.NotNil(sess1)
	ctx := context.Background()
	fi, err := sess1.ReadData(ctx, "OSTEST01/source/1.ts")
	assert.Nil(err)
	assert.NotNil(fi)
	body, _ := ioutil.ReadAll(fi.Body)
	assert.Equal("segmentbody", string(body))
	assert.Equal("OSTEST01/source/1.ts", fi.Name)

	fi, err = sess1.ReadData(ctx, "OSTEST01/P720p25fps16x9/1.ts")
	assert.Nil(err)
	assert.NotNil(fi)
	body, _ = ioutil.ReadAll(fi.Body)
	assert.Equal("transcoded binary data", string(body))

	// Saving to record store is async so sleep for a bit
	time.Sleep(100 * time.Millisecond)

	store2 := drivers.TestMemoryStorages["store2"]
	sess2 := store2.GetSession("sess1/" + lpmon.NodeID)
	assert.NotNil(sess2)
	fi, err = sess2.ReadData(ctx, fmt.Sprintf("sess1/%s/source/1.ts", lpmon.NodeID))
	assert.Nil(err)
	assert.NotNil(fi)
	body, _ = ioutil.ReadAll(fi.Body)
	assert.Equal("segmentbody", string(body))

	fi, err = sess2.ReadData(ctx, fmt.Sprintf("sess1/%s/P720p25fps16x9/1.ts", lpmon.NodeID))
	assert.Nil(err)
	assert.NotNil(fi)
	body, _ = ioutil.ReadAll(fi.Body)
	assert.Equal("transcoded binary data", string(body))

	assert.Equal(200, resp.StatusCode)
	body, _ = ioutil.ReadAll(resp.Body)
	assert.True(len(body) > 0)

	// check that segment with 0-frame is not saved to recording store
	w = httptest.NewRecorder()
	breader := bytes.NewReader(zero_frame_ts)
	req = httptest.NewRequest("POST", "/live/sess1/2.ts", breader)
	req.Header.Set("Accept", "multipart/mixed")
	handler.ServeHTTP(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	zfr, err := sess1.ReadData(ctx, "OSTEST01/source/2.ts")
	assert.Nil(err)
	zfb, zerr := ioutil.ReadAll(zfr.Body)
	assert.Nil(zerr)
	assert.Equal(zero_frame_ts, zfb)
	_, err = sess2.ReadData(ctx, "OSTEST01/source/2.ts")
	assert.Error(err, "Not found")
	assert.Equal(200, resp.StatusCode)
	assert.Equal(int64(-1), resp.ContentLength)
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr := multipart.NewReader(resp.Body, params["boundary"])
	i := 0
	for {
		p, merr := mr.NextPart()
		if merr == io.EOF {
			break
		}
		assert.Nil(merr)
		mediaType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		disposition, dispParams, err := mime.ParseMediaType(p.Header.Get("Content-Disposition"))
		assert.Nil(err)
		body, merr := ioutil.ReadAll(p)
		assert.Nil(merr)
		assert.True(len(body) > 0)
		assert.Equal("video/mp2t", mediaType)
		assert.Equal("attachment", disposition)
		assert.Equal("P720p25fps16x9_2.ts", dispParams["filename"])
		assert.Equal(zero_frame_ts, body)
		i++
	}
	assert.Equal(len(BroadcastJobVideoProfiles), i)

	fi, err = sess2.ReadData(ctx, fmt.Sprintf("sess1/%s/source/2.ts", lpmon.NodeID))
	assert.EqualError(err, "Not found")
	assert.Nil(fi)

	serverCancel()
}

func TestPush_ConcurrentSegments(t *testing.T) {
	assert := assert.New(t)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
	n.NodeType = core.BroadcasterNode
	s, _ := NewLivepeerServer("127.0.0.1:1938", n, true, "")
	serverCtx, serverCancel := context.WithCancel(context.Background())
	s.SetContextFromUnitTest(serverCtx)
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = nil

	var wg sync.WaitGroup
	start := make(chan struct{})
	sendSeg := func(url string) {
		reader := strings.NewReader("")
		req := httptest.NewRequest("POST", url, reader)
		h, pattern := s.HTTPMux.Handler(req)
		assert.Equal("/live/", pattern)
		writer := httptest.NewRecorder()
		<-start
		h.ServeHTTP(writer, req)
		resp := writer.Result()
		defer resp.Body.Close()
		assert.Equal(503, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err)
		assert.Equal("No sessions available", strings.TrimSpace(string(body)))
		wg.Done()
	}
	// Send concurrent segments on the same streamID
	wg.Add(2)
	go sendSeg("/live/streamID/0.ts")
	go sendSeg("/live/streamID/1.ts")
	time.Sleep(300 * time.Millisecond)
	// Send signal to go-routines so the requests are as-close-together-as-possible
	close(start)
	// Wait for goroutines to end
	wg.Wait()
	serverCancel()
}

func TestPush_ReuseIntmidWithDiffExtmid(t *testing.T) {
	oldjpqt := core.JsonPlaylistQuitTimeout
	defer func() {
		core.JsonPlaylistQuitTimeout = oldjpqt
	}()
	defer goleak.VerifyNone(t, common.IgnoreRoutines()...)
	core.JsonPlaylistQuitTimeout = 0 * time.Second

	reader := strings.NewReader("InsteadOf.TS")
	oldRI := httpPushTimeout
	httpPushTimeout = 100 * time.Millisecond
	defer func() { httpPushTimeout = oldRI }()
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()

	hookCalled := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := authWebhookResponse{ManifestID: "intmid"}
		val, err := json.Marshal(auth)
		assert.Nil(err, "invalid auth webhook response")
		w.Write(val)
		hookCalled++
	}))
	defer ts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, ts.URL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/extmid1/0.ts", reader)
	s.HandlePush(w, req)
	resp := w.Result()
	assert.Equal(503, resp.StatusCode)
	resp.Body.Close()
	assert.Equal(1, hookCalled)
	s.connectionLock.Lock()
	_, exists := s.rtmpConnections["intmid"]
	intmid := s.internalManifests["extmid1"]
	s.connectionLock.Unlock()
	assert.Equal("intmid", string(intmid))
	assert.True(exists)

	time.Sleep(4 * time.Millisecond)

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/extmid2/0.ts", reader)
	s.HandlePush(w, req)
	resp = w.Result()
	assert.Equal(503, resp.StatusCode)
	resp.Body.Close()
	assert.Equal(2, hookCalled)
	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["intmid"]
	intmid = s.internalManifests["extmid2"]
	_, existsOld := s.internalManifests["extmid1"]
	s.connectionLock.Unlock()
	assert.Equal("intmid", string(intmid))
	assert.True(exists)
	assert.False(existsOld)

	time.Sleep(200 * time.Millisecond)

	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["intmid"]
	_, extEx := s.internalManifests["extmid1"]
	_, extEx2 := s.internalManifests["extmid2"]
	s.connectionLock.Unlock()
	cancel()
	assert.False(exists)
	assert.False(extEx)
	assert.False(extEx2)
	serverCleanup(s)
}

func TestPush_MultipartReturnMultiSession(t *testing.T) {
	assert := assert.New(t)
	// need real video data for fast verification
	transcodeddata, err := ioutil.ReadFile("../core/test.ts")
	assert.NoError(err)
	goodHash, err := ioutil.ReadFile("../core/test.phash")
	assert.NoError(err)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	reader := strings.NewReader("InsteadOf.TS")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani/17.ts", reader)

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

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	segPath := "/transcoded/segment.ts"
	tSegData := []*net.TranscodedSegmentData{{Url: ts.URL + segPath, Pixels: 100, PerceptualHashUrl: ts.URL + segPath + ".phash"}}
	tr := dummyRes(tSegData)
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	mux.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(transcodeddata)
	})
	mux.HandleFunc(segPath+".phash", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(goodHash)
	})

	ts2, mux2 := stubTLSServer()
	defer ts2.Close()
	tSegData2 := []*net.TranscodedSegmentData{{Url: ts2.URL + segPath, Pixels: 100, PerceptualHashUrl: ts2.URL + segPath + ".phash"}}
	tr2 := dummyRes(tSegData2)
	buf2, err := proto.Marshal(tr2)
	require.Nil(t, err)
	mux2.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf2)
	})
	mux2.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(transcodeddata)
	})
	unverifiedHash := goodHash
	unverifiedHashCalled := 0
	mux2.HandleFunc(segPath+".phash", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(unverifiedHash)
		unverifiedHashCalled++
	})

	ts3, mux3 := stubTLSServer()
	defer ts3.Close()
	tSegData3 := []*net.TranscodedSegmentData{{Url: ts3.URL + segPath, Pixels: 100, PerceptualHashUrl: ts3.URL + segPath + ".phash"}}
	tr3 := dummyRes(tSegData3)
	buf3, err := proto.Marshal(tr3)
	require.Nil(t, err)
	mux3.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		// delay so it will be chosen second
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write(buf3)
	})
	mux3.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(transcodeddata)
	})
	mux3.HandleFunc(segPath+".phash", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(goodHash)
	})

	sess1 := StubBroadcastSession(ts.URL)
	sess1.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess1.Params.ManifestID = "mani"

	sess2 := StubBroadcastSession(ts2.URL)
	sess2.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess2.Params.ManifestID = "mani"
	sess2.OrchestratorScore = common.Score_Untrusted

	sess3 := StubBroadcastSession(ts3.URL)
	sess3.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess3.Params.ManifestID = "mani"
	sess3.OrchestratorScore = common.Score_Untrusted

	bsm := bsmWithSessListExt([]*BroadcastSession{sess1}, []*BroadcastSession{sess3, sess2}, false)
	bsm.VerificationFreq = 1
	assert.Equal(0, bsm.untrustedPool.sus.count)
	// hack: stop pool from refreshing
	bsm.untrustedPool.refreshing = true

	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")
	sess1.BroadcasterOS = osSession
	sess2.BroadcasterOS = osSession
	sess3.BroadcasterOS = osSession

	oldjpqt := core.JsonPlaylistQuitTimeout
	defer func() {
		core.JsonPlaylistQuitTimeout = oldjpqt
	}()
	core.JsonPlaylistQuitTimeout = 0 * time.Second
	pl := core.NewBasicPlaylistManager("xx", osSession, nil)

	cxn := &rtmpConnection{
		mid:         core.ManifestID("mani"),
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
		params:      &core.StreamParameters{Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p25fps16x9}, VerificationFreq: 1},
	}

	s.rtmpConnections["mani"] = cxn

	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(200, resp.StatusCode)

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr := multipart.NewReader(resp.Body, params["boundary"])
	var i int
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		mediaType, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Contains(params, "name")
		assert.Len(params, 1)
		assert.Equal("P144p25fps16x9_17.ts", params["name"])
		assert.Equal(`attachment; filename="P144p25fps16x9_17.ts"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		assert.Equal("video/mp2t", strings.ToLower(mediaType))
		i++
	}
	assert.Equal(1, i)
	assert.Equal(uint64(12), cxn.sourceBytes)

	// now make unverified to respond with bad hash
	unverifiedHash = []byte{0}
	reader = strings.NewReader("InsteadOf.TS")
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)

	req.Header.Set("Accept", "multipart/mixed")
	s.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	assert.Equal(200, resp.StatusCode)

	mediaType, params, err = mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr = multipart.NewReader(resp.Body, params["boundary"])
	i = 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		mediaType, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Contains(params, "name")
		assert.Len(params, 1)
		assert.Equal("P144p25fps16x9_18.ts", params["name"])
		assert.Equal(`attachment; filename="P144p25fps16x9_18.ts"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		assert.Equal("video/mp2t", strings.ToLower(mediaType))

		i++
	}
	assert.Equal(1, i)
	assert.Equal(uint64(12*2), cxn.sourceBytes)
	assert.Equal(2, unverifiedHashCalled)
	assert.Contains(bsm.untrustedPool.sus.list, ts2.URL)
	assert.Equal(0, bsm.untrustedPool.sus.count)
}

func createStubTranscoder(transcodedSegData []byte, transcodedSegPhash []byte, tCallback func()) *httptest.Server {
	// Create stub server
	ts, mux := stubTLSServer()

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

	segPath := "/transcoded/segment.ts"
	tSegData := []*net.TranscodedSegmentData{{Url: ts.URL + segPath, Pixels: 100, PerceptualHashUrl: ts.URL + segPath + ".phash"}}
	tr := dummyRes(tSegData)
	buf, _ := proto.Marshal(tr)
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	mux.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(transcodedSegData)
		if tCallback != nil {
			tCallback()
		}
	})
	mux.HandleFunc(segPath+".phash", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(transcodedSegPhash)
	})
	return ts
}

func initServerWithBSM(srv *LivepeerServer, trustedSessList []*BroadcastSession, untrustedSessList []*BroadcastSession, verificationFreq uint) *BroadcastSessionsManager {
	bsm := bsmWithSessListExt(trustedSessList, untrustedSessList, false)
	bsm.VerificationFreq = verificationFreq

	// hack: stop pool from refreshing
	bsm.untrustedPool.refreshing = true

	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")
	for _, sess := range trustedSessList {
		sess.BroadcasterOS = osSession
	}
	for _, sess := range untrustedSessList {
		sess.BroadcasterOS = osSession
	}

	pl := core.NewBasicPlaylistManager("xx", osSession, nil)

	cxn := &rtmpConnection{
		mid:         core.ManifestID("mani"),
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
		params:      &core.StreamParameters{Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p25fps16x9}, VerificationFreq: 1},
	}

	srv.rtmpConnections["mani"] = cxn
	return bsm
}

func TestPush_FastVerificationFlow(t *testing.T) {
	assert := assert.New(t)
	// need real video data for fast verification
	segmentgooddata, err := ioutil.ReadFile("../core/test.ts")
	assert.NoError(err)
	segmentbaddata, err := ioutil.ReadFile("../core/test2.ts")
	assert.NoError(err)
	goodHash, err := ioutil.ReadFile("../core/test.phash")
	assert.NoError(err)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	srv, cancel := setupServerWithCancel()
	defer serverCleanup(srv)
	defer cancel()

	lastUsedT := ""

	trustedT := createStubTranscoder(segmentgooddata, goodHash, func() { lastUsedT = "trusted" })
	defer trustedT.Close()

	untrustedTBadSegment := createStubTranscoder(segmentbaddata, goodHash, func() { lastUsedT = "untrustedBadSegment" })
	defer untrustedTBadSegment.Close()

	untrustedTBadHash := createStubTranscoder(segmentgooddata, []byte{1, 2, 3}, func() { lastUsedT = "untrustedBadHash" })
	defer untrustedTBadHash.Close()

	untrustedTGood := createStubTranscoder(segmentgooddata, goodHash, func() { lastUsedT = "untrustedGood" })
	defer untrustedTGood.Close()

	trustedSess := StubBroadcastSession(trustedT.URL)
	trustedSess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	trustedSess.Params.ManifestID = "mani"

	untrustedSessBadSegment := StubBroadcastSession(untrustedTBadSegment.URL)
	untrustedSessBadSegment.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	untrustedSessBadSegment.Params.ManifestID = "mani"
	untrustedSessBadSegment.OrchestratorScore = common.Score_Untrusted

	untrustedSessBadHash := StubBroadcastSession(untrustedTBadHash.URL)
	untrustedSessBadHash.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	untrustedSessBadHash.Params.ManifestID = "mani"
	untrustedSessBadHash.OrchestratorScore = common.Score_Untrusted

	untrustedSessGood := StubBroadcastSession(untrustedTGood.URL)
	untrustedSessGood.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	untrustedSessGood.Params.ManifestID = "mani"
	untrustedSessGood.OrchestratorScore = common.Score_Untrusted

	// submit sequentially to get results ordered same as sessions
	submitMultiSession = submitSegment

	oldjpqt := core.JsonPlaylistQuitTimeout
	defer func() {
		core.JsonPlaylistQuitTimeout = oldjpqt
	}()
	core.JsonPlaylistQuitTimeout = 0 * time.Second
	reader := strings.NewReader("InsteadOf.TS")

	// pass sessions in reverse order because LIFO selector will pick up in reverse order
	bsm := initServerWithBSM(srv, []*BroadcastSession{trustedSess}, []*BroadcastSession{untrustedSessGood, untrustedSessBadSegment}, 1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani/17.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	// check that untrusted bad session is suspended after first segment, note that if "good" untrusted session is verified first,
	// "bad" session will never get verified and suspended
	validateMultipartResponse(assert, resp, 17, 1)
	assert.Contains(bsm.untrustedPool.sus.list, untrustedTBadSegment.URL)

	// check that untrusted good session is used for next segment
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	validateMultipartResponse(assert, resp, 18, 1)
	assert.Equal("untrustedGood", lastUsedT)

	// reinit and check that 'bad hash' session causes suspension as well
	bsm = initServerWithBSM(srv, []*BroadcastSession{trustedSess}, []*BroadcastSession{untrustedSessGood, untrustedSessBadHash}, 1)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	validateMultipartResponse(assert, resp, 18, 1)
	assert.Contains(bsm.untrustedPool.sus.list, untrustedTBadHash.URL)

	// check that trusted session is used, when both untrusted sessions are bad
	bsm = initServerWithBSM(srv, []*BroadcastSession{trustedSess}, []*BroadcastSession{untrustedSessBadSegment, untrustedSessBadHash}, 1)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	validateMultipartResponse(assert, resp, 18, 1)
	// we do not suspend sessions if none of the session from untrusted pool match results of trusted session
	assert.Equal(0, len(bsm.untrustedPool.sus.list))

	// check that trusted session is used for next segment in the above case
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/19.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	validateMultipartResponse(assert, resp, 19, 1)
	assert.Equal("trusted", lastUsedT)

	// check multiple sessions and verification every segment
	bsm = initServerWithBSM(srv, []*BroadcastSession{trustedSess}, []*BroadcastSession{untrustedSessBadSegment,
		untrustedSessGood, untrustedSessGood, untrustedSessGood, untrustedSessGood, untrustedSessGood, untrustedSessGood,
		untrustedSessGood, untrustedSessGood, untrustedSessGood}, math.MaxUint)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/18.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	validateMultipartResponse(assert, resp, 18, 1)
	assert.NotEqual("trusted", lastUsedT)

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/live/mani/19.ts", reader)
	req.Header.Set("Accept", "multipart/mixed")
	srv.HandlePush(w, req)
	resp = w.Result()
	defer resp.Body.Close()
	validateMultipartResponse(assert, resp, 19, 1)
	assert.NotEqual("trusted", lastUsedT)
}

func validateMultipartResponse(assert *assert.Assertions, resp *http.Response, segNo int, nparts int) {
	assert.Equal(200, resp.StatusCode)
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	assert.Equal("multipart/mixed", mediaType)
	assert.Nil(err)
	mr := multipart.NewReader(resp.Body, params["boundary"])
	var i = 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		mediaType, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Contains(params, "name")
		assert.Len(params, 1)
		name := fmt.Sprintf("P144p25fps16x9_%d.ts", segNo)
		assert.Equal(name, params["name"])
		assert.Equal(fmt.Sprintf(`attachment; filename="%s"`, name), p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		assert.Equal("video/mp2t", strings.ToLower(mediaType))
		i++
	}
	assert.Equal(nparts, i)
}

func TestPush_Slice(t *testing.T) {
	// assert http request body error returned
	assert := assert.New(t)
	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/10.ts", nil)
	req.Header.Set("Content-Slice-From", "100")
	req.Header.Set("Content-Slice-To", "10")
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(string(body), "Invalid slice config from=100ms to=10ms")
}

func TestPush_SlicePass(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	orch := &mockOrchestrator{}

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
	orch.On("AuthToken", mock.Anything, mock.Anything).Return(stubAuthToken)
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		seg := r.Header.Get(segmentHeader)
		md, _, err := verifySegCreds(context.TODO(), orch, seg, ethcommon.Address{})
		require.NoError(err)
		require.NotNil(md)
		require.NotNil(md.SegmentParameters)
		require.Equal(100*time.Millisecond, md.SegmentParameters.Clip.From)
		require.Equal(200*time.Millisecond, md.SegmentParameters.Clip.To)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.Params.ManifestID = "mani"
	bsm := bsmWithSessList([]*BroadcastSession{sess})

	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")

	pl := core.NewBasicPlaylistManager("xx", osSession, nil)

	cxn := &rtmpConnection{
		mid:         core.ManifestID("mani"),
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
		params: &core.StreamParameters{
			Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p25fps16x9},
		},
	}

	s.rtmpConnections["mani"] = cxn

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/mani/18.ts", nil)
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Content-Slice-From", "100")
	req.Header.Set("Content-Slice-To", "200")
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode)
}
