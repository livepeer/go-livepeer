package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type TestVideoSegmenter struct {
	count int
}

func (t *TestVideoSegmenter) RTMPToHLS(ctx context.Context, cleanup bool) error {
	t.count++
	if t.count < RetryCount {
		return errors.New("Test Retry")
	}
	return nil
}

func (t *TestVideoSegmenter) GetCount() int {
	return t.count
}

func TestRetryRTMPToHLS(t *testing.T) {
	var testVideoSegmenter = &TestVideoSegmenter{}
	SegmenterRetryWait = time.Millisecond
	rtmpToHLS(testVideoSegmenter, context.Background(), true)
	count := testVideoSegmenter.GetCount()
	if count != RetryCount {
		t.Error("Not enough retries attempted")
		t.Fail()
	}
}
