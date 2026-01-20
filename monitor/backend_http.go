package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type httpBackend struct {
	client  *http.Client
	target  string
	headers map[string]string
	method  string
}

func init() {
	RegisterBackendFactory("http", newHTTPBackend)
	RegisterBackendFactory("https", newHTTPBackend)
}

func newHTTPBackend(u *url.URL, opts BackendOptions) (EventBackend, error) {
	query := u.Query()

	method := strings.ToUpper(strings.TrimSpace(query.Get("method")))
	if method == "" {
		method = http.MethodPost
	}

	timeout := 10 * time.Second
	if rawTimeout := strings.TrimSpace(query.Get("timeout")); rawTimeout != "" {
		dur, err := time.ParseDuration(rawTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q for http sink %s: %w", rawTimeout, u.String(), err)
		}
		timeout = dur
	}

	// remove internal configuration parameters
	query.Del("method")
	query.Del("timeout")
	cleaned := *u
	cleaned.RawQuery = query.Encode()

	headers := make(map[string]string, len(opts.Headers))
	for k, v := range opts.Headers {
		headers[k] = v
	}

	return &httpBackend{
		client:  &http.Client{Timeout: timeout},
		target:  cleaned.String(),
		headers: headers,
		method:  method,
	}, nil
}

func (b *httpBackend) Start(_ context.Context) error {
	return nil
}

func (b *httpBackend) Publish(ctx context.Context, batch []EventEnvelope) error {
	if len(batch) == 0 {
		return nil
	}

	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, b.method, b.target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("http sink %s returned status %d", b.target, resp.StatusCode)
}

func (b *httpBackend) Stop(_ context.Context) error {
	return nil
}
