package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

// Durable usage-ingest delivery.
//
// Billed BYOC usage historically reached OpenMeter only through the best-effort,
// fire-and-forget pipeline in kafka.go (SendQueueEventAsync -> batched producer
// -> Redpanda -> Benthos collector -> OpenMeter), which has several silent-drop
// branches and no delivery guarantee. This file adds a strictly additive,
// synchronous, bounded-retry HTTP POST that delivers each create_signed_ticket
// usage event to the durable, idempotent pymthouse ingest endpoint. It is only
// active when an ingest URL is configured; otherwise it is inert and the legacy
// Kafka path is unchanged.
const (
	usageIngestMaxAttempts       = 3
	usageIngestBaseBackoff       = 100 * time.Millisecond
	usageIngestMaxBackoff        = 500 * time.Millisecond
	usageIngestPerAttemptTimeout = 2 * time.Second
	usageIngestOverallTimeout    = 6 * time.Second
	// UsageIngestEventType matches the legacy Kafka event type and the pymthouse
	// ingest contract (`type: "create_signed_ticket"`).
	UsageIngestEventType = "create_signed_ticket"
)

// SignedTicketUsageEvent carries the per-served-job usage data delivered to the
// durable ingest endpoint. Values are sourced verbatim from the data already
// computed in the remote signer, so the durable path and the legacy Kafka path
// describe the same event.
type SignedTicketUsageEvent struct {
	// RequestID is the unique per-job id. It is the CloudEvent id and the
	// idempotency key on the pymthouse side (dedupe on (clientId, requestId)).
	RequestID string
	// ComputedFeeWei is the deterministic per-job fee in wei (fee.FloatString(0)),
	// identical to the legacy event's "computed_fee" field. pymthouse converts it
	// to USD micros, mirroring the Benthos collector.
	ComputedFeeWei string
	Pixels         int64
	Pipeline       string
}

// usageIngestRequestBody mirrors the request contract of pymthouse
// `POST /api/v1/internal/ingest/signed-ticket` (PR #178): the endpoint reads the
// usage fields from the nested `data` object.
type usageIngestRequestBody struct {
	Type string                 `json:"type"`
	Data usageIngestRequestData `json:"data"`
}

type usageIngestRequestData struct {
	ClientID     string `json:"client_id"`
	UsageSubject string `json:"usage_subject"`
	RequestID    string `json:"request_id"`
	// ComputedFee is the fee in wei. pymthouse reads "computed_fee_wei" or
	// "computed_fee" and converts to USD micros via the collector formula.
	ComputedFee string `json:"computed_fee,omitempty"`
	Pixels      string `json:"pixels,omitempty"`
	Pipeline    string `json:"pipeline,omitempty"`
}

// UsageIngestPoster performs a synchronous, bounded-retry HTTP POST of a
// signed-ticket usage event to the durable pymthouse ingest endpoint. It is
// strictly additive to the legacy fire-and-forget Kafka path and is only
// constructed when an ingest URL is configured, so when unset the behavior is
// byte-identical to the legacy code (no new POST is ever made).
type UsageIngestPoster struct {
	url            string
	secret         string
	httpClient     *http.Client
	maxAttempts    int
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	overallTimeout time.Duration
}

// NewUsageIngestPoster returns a configured poster, or nil when no ingest URL is
// set. A nil poster is the OFF (default) state: callers treat it as "no durable
// delivery configured" and keep only the legacy path.
func NewUsageIngestPoster(rawURL, secret string) *UsageIngestPoster {
	if strings.TrimSpace(rawURL) == "" {
		return nil
	}
	return &UsageIngestPoster{
		url:            strings.TrimSpace(rawURL),
		secret:         strings.TrimSpace(secret),
		httpClient:     &http.Client{Timeout: usageIngestPerAttemptTimeout},
		maxAttempts:    usageIngestMaxAttempts,
		baseBackoff:    usageIngestBaseBackoff,
		maxBackoff:     usageIngestMaxBackoff,
		overallTimeout: usageIngestOverallTimeout,
	}
}

// SendSignedTicket delivers the usage event best-effort: it never panics and
// never returns an error to the caller. Delivery runs under a detached,
// bounded context so it cannot block the signing path indefinitely. On final
// failure (after bounded retries) it logs and increments a metric so drops are
// observable. authID is the signer auth ID (`client_id:external_user_id`).
func (p *UsageIngestPoster) SendSignedTicket(authID string, evt SignedTicketUsageEvent) {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.overallTimeout)
	defer cancel()

	if err := p.postSignedTicket(ctx, authID, evt); err != nil {
		glog.Errorf("usage ingest: durable signed-ticket POST failed request_id=%s err=%v", evt.RequestID, err)
		UsageIngestSendError(UsageIngestEventType)
	}
}

// postSignedTicket builds the request body from authID + evt and delivers it
// with bounded retries. It returns an error on a non-retryable failure or once
// the retry budget is exhausted. Exposed (unexported) for hermetic testing.
func (p *UsageIngestPoster) postSignedTicket(ctx context.Context, authID string, evt SignedTicketUsageEvent) error {
	clientID, externalUserID, ok := splitUsageAuthID(authID)
	if !ok {
		return fmt.Errorf("usage ingest: missing or invalid auth_id for request_id=%s", evt.RequestID)
	}

	body, err := json.Marshal(usageIngestRequestBody{
		Type: UsageIngestEventType,
		Data: usageIngestRequestData{
			ClientID:     clientID,
			UsageSubject: externalUserID,
			RequestID:    evt.RequestID,
			ComputedFee:  evt.ComputedFeeWei,
			Pixels:       pixelsString(evt.Pixels),
			Pipeline:     evt.Pipeline,
		},
	})
	if err != nil {
		return fmt.Errorf("usage ingest: failed to marshal request body: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("usage ingest: context done before attempt %d: %w", attempt, ctx.Err())
			case <-time.After(p.backoffFor(attempt - 1)):
			}
		}

		retryable, err := p.doOnce(ctx, body)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable {
			return err
		}
	}
	return fmt.Errorf("usage ingest: giving up after %d attempts: %w", p.maxAttempts, lastErr)
}

// doOnce performs a single attempt and reports whether the failure (if any) is
// retryable. A nil error means success (2xx, including an idempotent duplicate).
func (p *UsageIngestPoster) doOnce(ctx context.Context, body []byte) (retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		// Malformed URL etc. is a configuration error; retrying will not help.
		return false, fmt.Errorf("usage ingest: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.secret != "" {
		req.Header.Set("Authorization", "Bearer "+p.secret)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		// Network / timeout error: transient, retry.
		return true, fmt.Errorf("usage ingest: request failed: %w", err)
	}
	defer resp.Body.Close()
	// Drain a bounded amount so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<14))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// 2xx, including an idempotent duplicate, is success.
		return false, nil
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return true, fmt.Errorf("usage ingest: transient server status %d", resp.StatusCode)
	default:
		// Other 4xx (auth, validation, unknown client): not retryable.
		return false, fmt.Errorf("usage ingest: non-retryable status %d", resp.StatusCode)
	}
}

// backoffFor returns the capped exponential backoff for the given completed
// attempt count (1-based exponent).
func (p *UsageIngestPoster) backoffFor(completedAttempts int) time.Duration {
	if completedAttempts < 1 {
		completedAttempts = 1
	}
	backoff := p.baseBackoff << (completedAttempts - 1)
	if backoff <= 0 || backoff > p.maxBackoff {
		backoff = p.maxBackoff
	}
	return backoff
}

// splitUsageAuthID splits the signer auth ID (`client_id:external_user_id`) into
// its OpenMeter subject parts. It returns ok=false when either part is missing.
func splitUsageAuthID(authID string) (clientID, externalUserID string, ok bool) {
	idx := strings.Index(authID, ":")
	if idx <= 0 || idx >= len(authID)-1 {
		return "", "", false
	}
	return authID[:idx], authID[idx+1:], true
}

func pixelsString(pixels int64) string {
	if pixels <= 0 {
		return ""
	}
	return strconv.FormatInt(pixels, 10)
}
