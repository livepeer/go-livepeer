package server

import (
	"errors"
	"testing"
)

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
