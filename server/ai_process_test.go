package server

import (
	"context"
	"reflect"
	"testing"

	"github.com/livepeer/ai-worker/worker"
)

func Test_submitLlmGenerate(t *testing.T) {
	type args struct {
		ctx    context.Context
		params aiRequestParams
		sess   *AISession
		req    worker.LlmGenerateFormdataRequestBody
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
			got, err := submitLlmGenerate(tt.args.ctx, tt.args.params, tt.args.sess, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("submitLlmGenerate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("submitLlmGenerate() = %v, want %v", got, tt.want)
			}
		})
	}
}
