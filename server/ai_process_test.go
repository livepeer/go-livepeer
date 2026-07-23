package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeprecatedPipelineHandler_Returns410(t *testing.T) {
	// The batch AI pipeline routes are deprecated and now served by a stub that
	// responds with HTTP 410 Gone plus deprecation headers.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/text-to-image", nil)
	deprecatedPipelineHandler("text-to-image").ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected status 410 Gone, got %d", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Errorf("expected Deprecation: true header, got %q", got)
	}
	if rec.Header().Get("Sunset") == "" {
		t.Error("expected a Sunset header")
	}
}

func TestEncodeReqMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     string
	}{
		{
			name: "valid metadata",
			metadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			want: `{"key1":"value1","key2":"value2"}`,
		},
		{
			name:     "empty metadata",
			metadata: map[string]string{},
			want:     `{}`,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			want:     `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeReqMetadata(tt.metadata)
			if got != tt.want {
				t.Errorf("encodeReqMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNoCapacityError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "insufficient capacity error",
			err:  errors.New("Insufficient capacity"),
			want: true,
		},
		{
			name: "INSUFFICIENT capacity ERROR",
			err:  errors.New("Insufficient capacity"),
			want: true,
		},
		{
			name: "non-insufficient capacity error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoCapacityError(tt.err); got != tt.want {
				t.Errorf("isNoCapacityError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isInvalidTicketSenderNonc(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "invalid ticket sendernonce",
			err:  errors.New("invalid ticket sendernonce"),
			want: true,
		},
		{
			name: "INVALID ticket sendernonce",
			err:  errors.New("Invalid ticket sendernonce"),
			want: true,
		},
		{
			name: "non-insufficient capacity error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInvalidTicketSenderNonce(tt.err); got != tt.want {
				t.Errorf("isNoCapacityError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ticketparams expired",
			err:  errors.New("ticketparams expired"),
			want: true,
		},
		{
			name: "TICKETPARAMS expired",
			err:  errors.New("TICKETPARAMS expired"),
			want: true,
		},
		{
			name: "non-retryable error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Errorf("isRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}
