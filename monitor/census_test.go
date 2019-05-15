package monitor

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAveragerCanBeRemoved(t *testing.T) {
	a1 := newAverager()
	if !a1.canBeRemoved() {
		t.Fatal("Should be able to remove empty buffer")
	}
	a1.addEmerged(1)
	time.Sleep(time.Millisecond)
	a1.addEmerged(2)
	rate, has := a1.successRate()
	if rate != 1 {
		t.Fatalf("Rate should be 1, got %v", rate)
	}
	if has {
		t.Fatalf("Rate shouldn't be found at this point")
	}
	if a1.canBeRemoved() {
		t.Fatal("Should not be able to remove buffer with not transcoded segments till timeout passes")
	}
	a1.segments[0].transcoded = 1
	a1.segments[1].failed = true
	if !a1.canBeRemoved() {
		t.Fatal("Should be able to remove buffer with all transcoded segments")
	}
	a2 := newAverager()
	a2.addEmerged(1)
	old := timeToWaitForError
	timeToWaitForError = time.Millisecond
	time.Sleep(10 * time.Millisecond)
	if !a2.canBeRemoved() {
		t.Fatal("Should be able to remove buffer with timeouted segments")
	}
	timeToWaitForError = old
}

func TestLastSegmentTimeout(t *testing.T) {
	unitTestMode = true
	defer func() { unitTestMode = false }()
	InitCensus("tst", "testid", "testversion")
	// defer func() {
	// 	shutDown <- nil
	// }()
	StreamCreated("h1", 1)
	if len(census.success) != 1 {
		t.Fatal("Should be one stream")
	}
	SegmentEmerged(1, 1, 3)
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentFullyTranscoded(1, 1, "ps", "")
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentEmerged(1, 2, 3)
	SegmentTranscodeFailed(SegmentTranscodeErrorOrchestratorBusy, 1, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
	SegmentEmerged(1, 3, 3)
	SegmentTranscodeFailed(SegmentTranscodeErrorSessionEnded, 1, 3, fmt.Errorf("some"), true)
	SegmentEmerged(1, 4, 3)
	SegmentFullyTranscoded(1, 4, "ps", "")
	if sr := census.successRate(); sr != 0.75 {
		t.Fatalf("Success rate should be 0.75, not %f", sr)
	}
	StreamEnded(1)
	if len(census.success) != 0 {
		t.Fatalf("Should be no streams, instead have %d", len(census.success))
	}

	StreamCreated("h1", 2)
	SegmentEmerged(2, 1, 3)
	SegmentFullyTranscoded(2, 1, "ps", "")
	SegmentEmerged(2, 2, 3)
	StreamEnded(2)
	if len(census.success) != 1 {
		t.Fatalf("Should be one stream, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	old1 := timeToWaitForError
	timeToWaitForError = time.Nanosecond
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
	go census.timeoutWatcher(context.Background())
	time.Sleep(10 * time.Millisecond)
	if len(census.success) != 0 {
		t.Fatalf("Should be streams, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	timeToWaitForError = old1

	StreamCreated("h3", 3)
	SegmentEmerged(3, 1, 3)
	SegmentFullyTranscoded(3, 1, "ps", "")
	SegmentEmerged(3, 2, 3)
	StreamEnded(3)
	if len(census.success) != 1 {
		t.Fatalf("Should be one stream, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentTranscodeFailed(SegmentTranscodeErrorOrchestratorBusy, 3, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
}
