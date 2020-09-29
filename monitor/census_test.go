package monitor

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	SegmentEmerged(1, 1, 3, 1)
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentFullyTranscoded(1, 1, "ps", "")
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentEmerged(1, 2, 3, 1)
	SegmentTranscodeFailed(SegmentTranscodeErrorOrchestratorBusy, 1, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
	SegmentEmerged(1, 3, 3, 1)
	SegmentTranscodeFailed(SegmentTranscodeErrorSessionEnded, 1, 3, fmt.Errorf("some"), true)
	SegmentEmerged(1, 4, 3, 1)
	SegmentFullyTranscoded(1, 4, "ps", "")
	if sr := census.successRate(); sr != 0.75 {
		t.Fatalf("Success rate should be 0.75, not %f", sr)
	}
	StreamEnded(1)
	if len(census.success) != 0 {
		t.Fatalf("Should be no streams, instead have %d", len(census.success))
	}

	StreamCreated("h1", 2)
	SegmentEmerged(2, 1, 3, 1)
	SegmentFullyTranscoded(2, 1, "ps", "")
	SegmentEmerged(2, 2, 3, 1)
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
	SegmentEmerged(3, 1, 3, 1)
	SegmentFullyTranscoded(3, 1, "ps", "")
	SegmentEmerged(3, 2, 3, 1)
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

func TestRingArray(t *testing.T) {
	assert := assert.New(t)
	ra := newRingArray(2)
	assert.False(ra.hasData())
	t1 := time.Now()
	ra.push(t1, 1)
	assert.True(ra.hasData())
	t1t, v1 := ra.pop()
	assert.Equal(t1, t1t)
	assert.Equal(1.0, v1)
	assert.False(ra.hasData())

	ra.push(t1, 1)
	assert.Equal(0, ra.tail)
	t2 := time.Now()
	ra.push(t2, 2)
	assert.Equal(1.5, ra.average())
	ra.pop()
	t3 := time.Now()
	ra.push(t3, 4)
	assert.Equal(3.0, ra.average())
	t4 := time.Now()
	ra.push(t4, 3)
	assert.Equal(3.0, ra.average())
	assert.Equal(0, ra.tail)
	assert.Equal(2, ra.head)
	t2t, v2 := ra.pop()
	assert.Equal(t2, t2t)
	assert.Equal(2.0, v2)
	t3t, v3 := ra.pop()
	assert.Equal(t3, t3t)
	assert.Equal(4.0, v3)
	t4t, v4 := ra.pop()
	assert.Equal(t4, t4t)
	assert.Equal(3.0, v4)
	assert.False(ra.hasData())
}

func TestRingArray2(t *testing.T) {
	assert := assert.New(t)
	ra := newRingArray(2)
	t0t, v0 := ra.pop()
	assert.True(t0t.IsZero())
	assert.Equal(0.0, v0)
	assert.Equal(0.0, ra.average())
	t0t, v0 = ra.getTail()
	assert.True(t0t.IsZero())
	assert.Equal(0.0, v0)
	t1 := time.Now()
	ra.push(t1, 1)
	assert.Equal(0, ra.tail)
	t2 := time.Now()
	ra.push(t2, 2)
	assert.Equal(1.5, ra.average())
	t3 := time.Now()
	ra.push(t3, 3)
	assert.Equal(2.0, ra.average())
	t1t, v1 := ra.getTail()
	assert.Equal(t1, t1t)
	assert.Equal(1.0, v1)
	t1t, v1 = ra.pop()
	assert.Equal(t1, t1t)
	assert.Equal(1.0, v1)
	t2t, v2 := ra.pop()
	assert.Equal(t2, t2t)
	assert.Equal(2.0, v2)
	t3t, v3 := ra.pop()
	assert.Equal(t3, t3t)
	assert.Equal(3.0, v3)
	assert.False(ra.hasData())
}

func TestRingArray3(t *testing.T) {
	assert := assert.New(t)
	ra := newRingArray(2)
	t1 := time.Now()
	ra.push(t1, 1)
	assert.Equal(0, ra.tail)
	t2 := time.Now()
	ra.push(t2, 2)
	ra.pop()
	t3 := time.Now()
	ra.push(t3, 3)
	t2t, v2 := ra.pop()
	assert.Equal(t2, t2t)
	assert.Equal(2.0, v2)
	t3t, v3 := ra.pop()
	assert.Equal(t3, t3t)
	assert.Equal(3.0, v3)
	assert.False(ra.hasData())
}

func TestMovingAverage(t *testing.T) {
	assert := assert.New(t)
	ma := newMovingAverage(time.Hour, 2)
	av := ma.addSample(time.Now(), 1)
	assert.Equal(1.0, av)
	av = ma.addSample(time.Now(), 1)
	assert.Equal(1.0, av)
	av = ma.addSample(time.Now(), 7)
	assert.Equal(3.0, av)

	ma = newMovingAverage(time.Millisecond, 2)
	av = ma.addSample(time.Now(), 1)
	assert.Equal(1.0, av)
	time.Sleep(5 * time.Millisecond)
	av = ma.addSample(time.Now(), 2)
	assert.Equal(2.0, av)
}
