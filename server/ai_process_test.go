package server

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/livepeer/go-livepeer/ai/worker"
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

func Test_isRetryableError(t *testing.T) {
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
