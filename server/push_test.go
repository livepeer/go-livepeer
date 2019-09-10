package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
)

func requestSetup(s *LivepeerServer) (http.Handler, *strings.Reader, *httptest.ResponseRecorder) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.HandlePush(w, r)
	})
	reader := strings.NewReader("")
	writer := httptest.NewRecorder()
	return handler, reader, writer
}

func TestNoErrors(t *testing.T) {
	assert := assert.New(t)
	handler, reader, w := requestSetup(setupServer())
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)

	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(strings.TrimSpace(string(body)), "")
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
	require.Nil(t, err)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "Error reading http request body")
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

	assert.Equal(200, resp.StatusCode)
}
