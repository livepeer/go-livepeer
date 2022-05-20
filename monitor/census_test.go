package monitor

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	lpnet "github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
)

func TestAveragerCanBeRemoved(t *testing.T) {
	a1 := newAverager("test")
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
	a2 := newAverager("test")
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
	NodeID = "testid"

	InitCensus("bctr", "testversion")
	// defer func() {
	// 	shutDown <- nil
	// }()
	StreamCreated("h1", 1)
	if len(census.success) != 1 {
		t.Fatal("Should be one stream")
	}
	SegmentEmerged(context.TODO(), 1, 1, 3, 1)
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentFullyTranscoded(context.Background(), 1, 1, "ps", "", &lpnet.OrchestratorInfo{})
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentEmerged(context.TODO(), 1, 2, 3, 1)
	SegmentTranscodeFailed(context.TODO(), SegmentTranscodeErrorOrchestratorBusy, 1, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
	SegmentEmerged(context.TODO(), 1, 3, 3, 1)
	SegmentTranscodeFailed(context.TODO(), SegmentTranscodeErrorSessionEnded, 1, 3, fmt.Errorf("some"), true)
	SegmentEmerged(context.TODO(), 1, 4, 3, 1)
	SegmentFullyTranscoded(context.Background(), 1, 4, "ps", "", &lpnet.OrchestratorInfo{})
	if sr := census.successRate(); sr != 0.75 {
		t.Fatalf("Success rate should be 0.75, not %f", sr)
	}
	StreamEnded(context.TODO(), 1)
	if len(census.success) != 0 {
		t.Fatalf("Should be no streams, instead have %d", len(census.success))
	}

	StreamCreated("h1", 2)
	SegmentEmerged(context.TODO(), 2, 1, 3, 1)
	SegmentFullyTranscoded(context.Background(), 2, 1, "ps", "", &lpnet.OrchestratorInfo{})
	SegmentEmerged(context.TODO(), 2, 2, 3, 1)
	StreamEnded(context.TODO(), 2)
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
	SegmentEmerged(context.TODO(), 3, 1, 3, 1)
	SegmentFullyTranscoded(context.Background(), 3, 1, "ps", "", &lpnet.OrchestratorInfo{})
	SegmentEmerged(context.TODO(), 3, 2, 3, 1)
	StreamEnded(context.TODO(), 3)
	if len(census.success) != 1 {
		t.Fatalf("Should be one stream, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentTranscodeFailed(context.TODO(), SegmentTranscodeErrorOrchestratorBusy, 3, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
}

func TestWei2Gwei(t *testing.T) {
	assert := assert.New(t)

	wei := big.NewInt(gweiConversionFactor)
	assert.Equal(1.0, wei2gwei(wei))

	wei = big.NewInt(gweiConversionFactor / 2)
	assert.Equal(0.5, wei2gwei(wei))

	wei = big.NewInt(0)
	assert.Equal(0.0, wei2gwei(wei))

	wei = big.NewInt(gweiConversionFactor * 1.5)
	assert.Equal(1.5, wei2gwei(wei))

	wei = big.NewInt(gweiConversionFactor * 2)
	assert.Equal(2.0, wei2gwei(wei))
}

func TestFracWei2Gwei(t *testing.T) {
	assert := assert.New(t)

	wei := big.NewRat(gweiConversionFactor, 1)
	assert.Equal(1.0, fracwei2gwei(wei))

	wei = big.NewRat(gweiConversionFactor/2, 1)
	assert.Equal(0.5, fracwei2gwei(wei))

	wei = big.NewRat(0, 1)
	assert.Equal(0.0, fracwei2gwei(wei))

	wei = big.NewRat(gweiConversionFactor*1.5, 1)
	assert.Equal(1.5, fracwei2gwei(wei))

	wei = big.NewRat(gweiConversionFactor*2, 1)
	assert.Equal(2.0, fracwei2gwei(wei))

	delta := .000000001

	// Test wei amounts with a fractional part
	wei = big.NewRat(gweiConversionFactor/2, 6)
	assert.InDelta(.083333333, fracwei2gwei(wei), delta)

	wei = big.NewRat(gweiConversionFactor*1.5, 7)
	assert.InDelta(.214285714, fracwei2gwei(wei), delta)

	wei = big.NewRat(gweiConversionFactor*2, 7)
	assert.InDelta(.285714286, fracwei2gwei(wei), delta)
}
