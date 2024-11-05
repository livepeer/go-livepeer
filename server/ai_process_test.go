package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/livepeer/ai-worker/worker"
)

func Test_submitLLM(t *testing.T) {
	type args struct {
		ctx    context.Context
		params aiRequestParams
		sess   *AISession
		req    worker.GenLLMFormdataRequestBody
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
