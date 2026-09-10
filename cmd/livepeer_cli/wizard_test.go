package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// httpGet returns the response body whatever the status, so a handler that reports an
// error with a 500 and a plain-text body looks to the caller like a successful read. The
// reward caller wizard offers to unset the value it reads back, so it must be able to tell
// an error body from an address.
func TestHttpGetWithSuccess(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantBody string
		wantOk   bool
	}{
		{
			name:     "ok with a value",
			status:   http.StatusOK,
			body:     "0x2222222222222222222222222222222222222222",
			wantBody: "0x2222222222222222222222222222222222222222",
			wantOk:   true,
		},
		{
			name:   "ok with an empty body",
			status: http.StatusOK,
			wantOk: true,
		},
		{
			name:     "server error carrying an error string",
			status:   http.StatusInternalServerError,
			body:     "Error getting reward caller: execution reverted",
			wantBody: "Error getting reward caller: execution reverted",
			wantOk:   false,
		},
		{
			name:   "bad request",
			status: http.StatusBadRequest,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			body, ok := httpGetWithSuccess(srv.URL)
			assert.Equal(t, tt.wantBody, body)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

// httpGet keeps its existing shape for the callers that do not check the status.
func TestHttpGetUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	body, ok := httpGetWithSuccess(url)
	assert.Equal(t, "", body)
	assert.False(t, ok)
	assert.Equal(t, "", httpGet(url))
}
