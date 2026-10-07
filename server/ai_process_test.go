package server

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/core"
)

func Test_submitLLM(t *testing.T) {
	type args struct {
		ctx    context.Context
		params aiRequestParams
		sess   *AISession
		req    worker.GenLLMJSONRequestBody
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := submitLLM(tt.args.ctx, tt.args.params, tt.args.sess, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("submitLLM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("submitLLM() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_submitAudioToText(t *testing.T) {
	type args struct {
		ctx    context.Context
		params aiRequestParams
		sess   *AISession
		req    worker.GenAudioToTextMultipartRequestBody
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "invalid request (no file)",
			args: args{
				ctx:    context.Background(),
				params: aiRequestParams{},
				sess:   &AISession{},
				req:    worker.GenAudioToTextMultipartRequestBody{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil session",
			args: args{
				ctx:    context.Background(),
				params: aiRequestParams{},
				sess:   nil,
				req:    worker.GenAudioToTextMultipartRequestBody{},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := submitAudioToText(tt.args.ctx, tt.args.params, tt.args.sess, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("submitAudioToText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("submitAudioToText() = %v, want %v", got, tt.want)
			}
		})
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

// Test_processAIRequest_NoOrchestrators_NonLive ensures that a non-live AI
// request (liveParams == nil) returns a ServiceUnavailableError instead of
// panicking when no orchestrators are available.
func Test_processAIRequest_NoOrchestrators_NonLive(t *testing.T) {
	cap := core.Capability_AudioToText
	modelID := "openai/whisper-large-v3"

	node := &core.LivepeerNode{
		OrchestratorPool:          &stubDiscovery{},
		AIProcesssingRetryTimeout: time.Second,
	}

	// Seed the selector cache with empty pools so Select returns nil without
	// triggering a network refresh (empty OrchestratorPool keeps the refresh
	// threshold at zero).
	sel := &AISessionSelector{
		warmPool:        NewAISessionPool(&LIFOSelector{}, newSuspender(), 0),
		coldPool:        NewAISessionPool(&LIFOSelector{}, newSuspender(), 0),
		ttl:             time.Hour,
		lastRefreshTime: time.Now(),
		cap:             cap,
		modelID:         modelID,
		node:            node,
		suspender:       newSuspender(),
	}
	mgr := NewAISessionManager(node, time.Hour)
	mgr.selectors[strconv.Itoa(int(cap))+"_"+modelID] = sel

	params := aiRequestParams{
		node:        node,
		sessManager: mgr,
		// liveParams intentionally nil to mimic a non-live pipeline request.
	}

	req := worker.GenAudioToTextMultipartRequestBody{ModelId: &modelID}

	resp, err := processAIRequest(context.Background(), params, req)
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}
	var unavailable *ServiceUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected ServiceUnavailableError, got %v", err)
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
