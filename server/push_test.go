package server

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
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

func TestMultipartReturn(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
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
	tSegData := []*net.TranscodedSegmentData{
		&net.TranscodedSegmentData{Url: ts.URL + segPath, Pixels: 100},
	}
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
	sess.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.ManifestID = "mani"
	bsm := bsmWithSessList([]*BroadcastSession{sess})

	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")

	pl := core.NewBasicPlaylistManager("xx", osSession)

	cxn := &rtmpConnection{
		mid:         core.ManifestID("mani"),
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
		params:      &streamParameters{profiles: []ffmpeg.VideoProfile{ffmpeg.P144p25fps16x9}},
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

	bsm.sel.Clear()
	bsm.sel.Add([]*BroadcastSession{sess})
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
	reader.Seek(0, 0)
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
		assert.Equal(params["name"], "P144p25fps16x9_12.ts")
		assert.Equal(`attachment; filename="P144p25fps16x9_12.ts"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		bodyPart, err := ioutil.ReadAll(p)
		assert.Nil(err)
		assert.Equal("video/mp2t", strings.ToLower(mediaType))
		assert.Equal("transcoded binary data", string(bodyPart))

		i++
	}
	assert.Equal(1, i)

	// No sessions error
	cxn.sessManager.sel.Clear()
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
}

func TestMemoryRequestError(t *testing.T) {
	// assert http request body error returned
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	handler, _, w := requestSetup(s)
	f, err := os.Open(`doesn't exist`)
	req := httptest.NewRequest("POST", "/live/seg.ts", f)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "Error reading http request body")
}

func TestEmptyURLError(t *testing.T) {
	// assert http request body error returned
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/live/.ts", nil)
	s.HandlePush(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("Bad URL\n", string(body))
}

func TestShouldUpdateLastUsed(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
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

	reader := strings.NewReader("")
	req := httptest.NewRequest("POST", "/live/name/1.mp4", reader)

	// HTTP ingest disabled
	s := NewLivepeerServer("127.0.0.1:1938", n, false)
	h, pattern := s.HTTPMux.Handler(req)
	assert.Equal("", pattern)

	writer := httptest.NewRecorder()
	h.ServeHTTP(writer, req)
	resp := writer.Result()
	defer resp.Body.Close()
	assert.Equal(404, resp.StatusCode)

	// HTTP ingest enabled
	s = NewLivepeerServer("127.0.0.1:1938", n, true)
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
}

func TestPush_MP4(t *testing.T) {

	// Do a bunch of setup. Would be nice to simplify this one day...
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	s.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	defer func() { s.rtmpConnections = map[core.ManifestID]*rtmpConnection{} }()
	segHandler := getHLSSegmentHandler(s)
	ts, mux := stubTLSServer()
	defer ts.Close()

	// sometimes LivepeerServer needs time  to start
	// esp if this is the only test in the suite being run (eg, via `-run)
	time.Sleep(10 * time.Millisecond)

	oldProfs := BroadcastJobVideoProfiles
	defer func() { BroadcastJobVideoProfiles = oldProfs }()
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P720p25fps16x9}

	sd := &stubDiscovery{}
	sd.infos = []*net.OrchestratorInfo{&net.OrchestratorInfo{Transcoder: ts.URL}}
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
	tSegData := []*net.TranscodedSegmentData{
		&net.TranscodedSegmentData{Url: ts.URL + segPath, Pixels: 100},
	}
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
		for _, p := range cxn.params.profiles {
			assert.Equal(ffmpeg.FormatMP4, p.Format)
		}
	}
}

func TestPush_SetVideoProfileFormats(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	// sometimes LivepeerServer needs time  to start
	// esp if this is the only test in the suite being run (eg, via `-run)
	time.Sleep(10 * time.Millisecond)
	s.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	defer func() { s.rtmpConnections = map[core.ManifestID]*rtmpConnection{} }()

	oldProfs := BroadcastJobVideoProfiles
	defer func() { BroadcastJobVideoProfiles = oldProfs }()
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P720p25fps16x9, ffmpeg.P720p60fps16x9}

	// Base case, mpegts
	h, r, w := requestSetup(s)
	req := httptest.NewRequest("POST", "/live/seg/0.ts", r)
	h.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	assert.Len(s.rtmpConnections, 1)
	for _, cxn := range s.rtmpConnections {
		assert.Equal(ffmpeg.FormatMPEGTS, cxn.profile.Format)
		assert.Len(cxn.params.profiles, 2)
		assert.Len(BroadcastJobVideoProfiles, 2)
		for i, p := range cxn.params.profiles {
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
		assert.Len(cxn.params.profiles, 2)
		assert.Len(BroadcastJobVideoProfiles, 2)
		for i, p := range cxn.params.profiles {
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
	assert.Len(cxn.params.profiles, 2)
	assert.Len(BroadcastJobVideoProfiles, 2)
	for i, p := range cxn.params.profiles {
		assert.Equal(ffmpeg.FormatMP4, p.Format)
		assert.Equal(ffmpeg.FormatNone, BroadcastJobVideoProfiles[i].Format)
	}

	// Sanity check that default profile with webhook is copied
	// Checking since there is special handling for the default set of profiles
	// within the webhook hander.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := authWebhookResponse{ManifestID: "web"}
		val, err := json.Marshal(auth)
		assert.Nil(err, "invalid auth webhook response")
		w.Write(val)
	}))
	defer ts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = ts.URL

	h, r, w = requestSetup(s)
	req = httptest.NewRequest("POST", "/live/web/0.mp4", r)
	h.ServeHTTP(w, req)
	resp = w.Result()
	defer resp.Body.Close()

	assert.Len(s.rtmpConnections, 3)
	cxn, ok = s.rtmpConnections["web"]
	assert.True(ok, "stream did not exist")
	assert.Equal(ffmpeg.FormatMP4, cxn.profile.Format)
	assert.Len(cxn.params.profiles, 2)
	assert.Len(BroadcastJobVideoProfiles, 2)
	for i, p := range cxn.params.profiles {
		assert.Equal(ffmpeg.FormatMP4, p.Format)
		assert.Equal(ffmpeg.FormatNone, BroadcastJobVideoProfiles[i].Format)
	}
}

func ignoreRoutines() []goleak.Option {
	// goleak works by making list of all running goroutines and reporting error if it finds any
	// this list tells goleak to ignore these goroutines - we're not interested in these particular goroutines
	funcs2ignore := []string{"github.com/golang/glog.(*loggingT).flushDaemon", "go.opencensus.io/stats/view.(*worker).start",
		"github.com/rjeczalik/notify.(*recursiveTree).dispatch", "github.com/rjeczalik/notify._Cfunc_CFRunLoopRun", "github.com/ethereum/go-ethereum/metrics.(*meterArbiter).tick",
		"github.com/ethereum/go-ethereum/consensus/ethash.(*Ethash).remote", "github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache",
		"internal/poll.runtime_pollWait", "github.com/livepeer/go-livepeer/core.(*RemoteTranscoderManager).Manage", "github.com/livepeer/lpms/core.(*LPMS).Start",
		"github.com/livepeer/go-livepeer/server.(*LivepeerServer).StartMediaServer", "github.com/livepeer/go-livepeer/core.(*RemoteTranscoderManager).Manage.func1",
		"github.com/livepeer/go-livepeer/server.(*LivepeerServer).HandlePush.func1", "github.com/rjeczalik/notify.(*nonrecursiveTree).dispatch",
		"github.com/rjeczalik/notify.(*nonrecursiveTree).internal", "github.com/livepeer/lpms/stream.NewBasicRTMPVideoStream.func1"}

	res := make([]goleak.Option, 0, len(funcs2ignore))
	for _, f := range funcs2ignore {
		res = append(res, goleak.IgnoreTopFunction(f))
	}
	return res
}

func TestShouldRemoveSessionAfterTimeout(t *testing.T) {
	defer goleak.VerifyNone(t, ignoreRoutines()...)

	oldRI := refreshIntervalHttpPush
	refreshIntervalHttpPush = 2 * time.Millisecond
	assert := assert.New(t)
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
	time.Sleep(50 * time.Millisecond)
	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["mani3"]
	s.connectionLock.Unlock()
	cancel()
	assert.False(exists)
	refreshIntervalHttpPush = oldRI
}

func TestShouldNotPanicIfSessionAlreadyRemoved(t *testing.T) {
	oldRI := refreshIntervalHttpPush
	refreshIntervalHttpPush = 5 * time.Millisecond
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
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
	time.Sleep(10 * time.Millisecond)
	s.connectionLock.Lock()
	_, exists = s.rtmpConnections["mani2"]
	s.connectionLock.Unlock()
	assert.False(exists)
	refreshIntervalHttpPush = oldRI
}

func TestFileExtensionError(t *testing.T) {
	// assert file extension error returned
	assert := assert.New(t)
	s := setupServer()
	handler, reader, w := requestSetup(s)
	defer serverCleanup(s)
	req := httptest.NewRequest("POST", "/live/seg.m3u8", reader)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "ignoring file extension")
}

func TestStorageError(t *testing.T) {
	// assert storage error
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	handler, reader, w := requestSetup(s)

	tempStorage := drivers.NodeStorage
	drivers.NodeStorage = nil
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	mid := parseManifestID(req.URL.Path)
	err := removeRTMPStream(s, mid)

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

func TestForAuthWebhookFailure(t *testing.T) {
	// assert app data error
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	handler, reader, w := requestSetup(s)

	AuthWebhookURL = "notaurl"
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "Could not create stream ID")

	// reset AuthWebhookURL to original value
	AuthWebhookURL = ""
}

func TestResolutionWithoutContentResolutionHeader(t *testing.T) {
	assert := assert.New(t)
	server := setupServer()
	defer serverCleanup(server)
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

func TestResolutionWithContentResolutionHeader(t *testing.T) {
	assert := assert.New(t)
	server := setupServer()
	defer serverCleanup(server)
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

func TestWebhookRequestURL(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, _ := ioutil.ReadAll(r.Body)
		var req authWebhookReq
		err := json.Unmarshal(out, &req)
		if err != nil {
			glog.Error("Error parsing URL: ", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		assert.Equal(req.URL, "http://example.com/live/seg.ts")
		w.Write(nil)
	}))

	defer ts.Close()

	AuthWebhookURL = ts.URL
	handler, reader, w := requestSetup(s)
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Server has empty sessions list, so it will return 503
	assert.Equal(503, resp.StatusCode)
}
