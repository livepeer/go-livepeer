package monitor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastPoster returns a poster pointed at url with near-zero backoff so retry
// tests run quickly and deterministically.
func fastPoster(url, secret string) *UsageIngestPoster {
	p := NewUsageIngestPoster(url, secret)
	if p != nil {
		p.baseBackoff = time.Millisecond
		p.maxBackoff = time.Millisecond
		p.overallTimeout = 2 * time.Second
	}
	return p
}

func sampleEvent() SignedTicketUsageEvent {
	return SignedTicketUsageEvent{
		RequestID:      "req-123",
		ComputedFeeWei: "547000000000",
		Pixels:         1920 * 1080,
		Pipeline:       "live-video-to-video",
	}
}

const sampleAuthID = "app_98575870d7ae33589a3f0660:naap-storyboard-preview"

// (a) Flag OFF (no URL) => poster is nil and no POST is ever made, even when an
// endpoint is available. This is the zero-regression default.
func TestUsageIngest_FlagOff_NoPost(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Empty URL => nil poster (the OFF state).
	p := NewUsageIngestPoster("", "secret")
	require.Nil(t, p, "no ingest URL configured must yield a nil (inert) poster")

	// Calling the best-effort entry point on a nil poster must be a safe no-op.
	assert.NotPanics(t, func() { p.SendSignedTicket(sampleAuthID, sampleEvent()) })

	assert.Equal(t, int32(0), atomic.LoadInt32(&hits), "flag OFF must make zero POSTs")
}

// (b) Flag ON, success => exactly one POST with the correct body + auth header.
func TestUsageIngest_FlagOn_SinglePostWithBodyAndAuth(t *testing.T) {
	var hits int32
	var gotAuth, gotContentType string
	var gotBody usageIngestRequestBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"ingested":true,"duplicate":false}`))
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "s3cr3t")
	require.NotNil(t, p)

	err := p.postSignedTicket(context.Background(), sampleAuthID, sampleEvent())
	require.NoError(t, err)

	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "exactly one POST on success")
	assert.Equal(t, "Bearer s3cr3t", gotAuth, "bearer secret must be sent verbatim")
	assert.Equal(t, "application/json", gotContentType)

	// Body must match the pymthouse #178 contract.
	assert.Equal(t, "create_signed_ticket", gotBody.Type)
	assert.Equal(t, "app_98575870d7ae33589a3f0660", gotBody.Data.ClientID)
	assert.Equal(t, "naap-storyboard-preview", gotBody.Data.UsageSubject)
	assert.Equal(t, "req-123", gotBody.Data.RequestID)
	assert.Equal(t, "547000000000", gotBody.Data.ComputedFee)
	assert.Equal(t, "live-video-to-video", gotBody.Data.Pipeline)
	assert.Equal(t, "2073600", gotBody.Data.Pixels)
}

// (c) Retry on 5xx then success.
func TestUsageIngest_RetriesOn5xxThenSucceeds(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "")
	require.NotNil(t, p)

	err := p.postSignedTicket(context.Background(), sampleAuthID, sampleEvent())
	require.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&hits), "two 5xx then success = 3 attempts")
}

// (d) Bounded give-up on persistent failure: returns an error after exactly
// maxAttempts, without blocking indefinitely or panicking.
func TestUsageIngest_GivesUpAfterBoundedRetries(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "")
	require.NotNil(t, p)

	done := make(chan error, 1)
	go func() { done <- p.postSignedTicket(context.Background(), sampleAuthID, sampleEvent()) }()

	select {
	case err := <-done:
		require.Error(t, err, "persistent 5xx must surface an error after retries")
	case <-time.After(5 * time.Second):
		t.Fatal("postSignedTicket blocked far longer than the bounded retry budget")
	}
	assert.Equal(t, int32(usageIngestMaxAttempts), atomic.LoadInt32(&hits), "must stop after maxAttempts")
}

// (e) A 2xx duplicate (idempotent replay) is treated as success.
func TestUsageIngest_DuplicateIsSuccess(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"ingested":true,"duplicate":true}`))
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "")
	require.NotNil(t, p)

	err := p.postSignedTicket(context.Background(), sampleAuthID, sampleEvent())
	require.NoError(t, err, "a 2xx duplicate response must be treated as success")
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}

// Non-retryable 4xx (e.g. validation/auth/unknown-client) fails immediately with
// no retries, so transient-retry never masks a permanent rejection.
func TestUsageIngest_NonRetryable4xxFailsImmediately(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusNotFound} {
		var hits int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(status)
		}))

		p := fastPoster(srv.URL, "")
		require.NotNil(t, p)

		err := p.postSignedTicket(context.Background(), sampleAuthID, sampleEvent())
		require.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "4xx (%d) must not be retried", status)
		srv.Close()
	}
}

// 429 Too Many Requests is treated as transient and retried.
func TestUsageIngest_429IsRetried(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "")
	require.NotNil(t, p)

	err := p.postSignedTicket(context.Background(), sampleAuthID, sampleEvent())
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&hits))
}

// A missing/invalid auth_id must skip the POST entirely (no client_id/subject to
// attribute usage to), surfacing an error rather than sending a malformed event.
func TestUsageIngest_InvalidAuthIDSkipsPost(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "")
	require.NotNil(t, p)

	for _, bad := range []string{"", "no-colon", ":only-subject", "only-client:"} {
		err := p.postSignedTicket(context.Background(), bad, sampleEvent())
		assert.Error(t, err, "auth_id %q must be rejected", bad)
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(&hits), "invalid auth_id must make zero POSTs")
}

// Context cancellation bounds total time even against an unreachable endpoint.
func TestUsageIngest_RespectsContextDeadline(t *testing.T) {
	// Unreachable port: dials fail fast; ensure we still give up promptly.
	p := fastPoster("http://127.0.0.1:1/never", "")
	require.NotNil(t, p)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- p.postSignedTicket(ctx, sampleAuthID, sampleEvent()) }()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("postSignedTicket did not respect the context deadline")
	}
}

// SendSignedTicket is the best-effort entry point used by the signer: it must
// never panic, even when delivery ultimately fails.
func TestUsageIngest_SendSignedTicketNeverPanicsOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := fastPoster(srv.URL, "")
	require.NotNil(t, p)
	assert.NotPanics(t, func() { p.SendSignedTicket(sampleAuthID, sampleEvent()) })
}

func TestSplitUsageAuthID(t *testing.T) {
	cases := []struct {
		in      string
		client  string
		subject string
		ok      bool
	}{
		{"app_abc:user-1", "app_abc", "user-1", true},
		{"app_abc:ns:user-1", "app_abc", "ns:user-1", true}, // split on first colon only
		{"", "", "", false},
		{"nocolon", "", "", false},
		{":user", "", "", false},
		{"client:", "", "", false},
	}
	for _, c := range cases {
		client, subject, ok := splitUsageAuthID(c.in)
		assert.Equal(t, c.ok, ok, "ok for %q", c.in)
		assert.Equal(t, c.client, client, "client for %q", c.in)
		assert.Equal(t, c.subject, subject, "subject for %q", c.in)
	}
}
