//go:build !race

package server

import (
	"encoding/json"
	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPush_WebhookRequestURL(t *testing.T) {
	assert := assert.New(t)

	// wait for any earlier tests to complete
	assert.True(wgWait(&pushResetWg), "timed out waiting for earlier tests")

	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()

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

	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, ts.URL)
	handler, reader, w := requestSetup(s)
	req := httptest.NewRequest("POST", "/live/seg.ts", reader)
	handler.ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Server has empty sessions list, so it will return 503
	assert.Equal(503, resp.StatusCode)
}
