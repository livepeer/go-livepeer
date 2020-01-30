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

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
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
		mediaType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
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
		mediaType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		assert.Nil(err)
		assert.Equal(`attachment; filename="P144p25fps16x9_12.ts"`, p.Header.Get("Content-Disposition"))
		assert.Equal("P144p25fps16x9", p.Header.Get("Rendition-Name"))
		bodyPart, err := ioutil.ReadAll(p)
		assert.Nil(err)
		assert.Equal("video/mp2t", mediaType)
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
	handler, _, w := requestSetup(setupServer())
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

func TestShouldRemoveSessionAfterTimeout(t *testing.T) {
	oldRI := refreshIntervalHttpPush
	refreshIntervalHttpPush = 2 * time.Millisecond
	assert := assert.New(t)
	s := setupServer()
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
	assert.False(exists)
	refreshIntervalHttpPush = oldRI
}

func TestShouldNotPanicIfSessionAlreadyRemoved(t *testing.T) {
	oldRI := refreshIntervalHttpPush
	refreshIntervalHttpPush = 5 * time.Millisecond
	assert := assert.New(t)
	s := setupServer()
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
	handler, reader, w := requestSetup(setupServer())
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
	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	handler, reader, w := requestSetup(server)
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	defaultRes := "0x0"

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	for _, cxn := range server.rtmpConnections {
		assert.Equal(cxn.profile.Resolution, defaultRes)
	}

	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
}

func TestResolutionWithContentResolutionHeader(t *testing.T) {
	assert := assert.New(t)
	server := setupServer()
	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
	handler, reader, w := requestSetup(server)
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	resolution := "123x456"
	req.Header.Set("Content-Resolution", resolution)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	for _, cxn := range server.rtmpConnections {
		assert.Equal(resolution, cxn.profile.Resolution)
	}

	server.rtmpConnections = map[core.ManifestID]*rtmpConnection{}
}

func TestWebhookRequestURL(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()

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
