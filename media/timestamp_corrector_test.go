package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// makeClock returns a Now() function that returns successive times from the slice.
func makeClock(times []time.Time) (nowFunc func() time.Time) {
	idx := 0
	return func() time.Time {
		if idx < len(times) {
			t := times[idx]
			idx++
			return t
		}
		return times[len(times)-1]
	}
}

func TestTimestampCorrector_NoFix_NormalClock(t *testing.T) {
	fps := 30.0
	tc := NewTimestampCorrector(fps)
	ctx := context.Background()
	assert := assert.New(t)

	// simulate two packets spaced at expectedWallclockDelta, tsDelta=expectedWallclockDelta*90k
	t0 := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	dtNs := time.Duration((1.0/fps)*0.5*1e9) * time.Nanosecond
	t1 := t0.Add(dtNs)
	tc.now = makeClock([]time.Time{t0, t1})

	ts1 := int64(1000)
	out1 := tc.Process(ctx, ts1)
	assert.Equal(ts1, out1, "first call changed ts")

	// tsDelta should correspond to 90k Hz, below threshold
	ts2 := ts1 + int64((dtNs.Seconds() * 90000))
	out2 := tc.Process(ctx, ts2)
	assert.Equal(ts2, out2, "normal-clock second call changed ts")

	// subsequent calls remain unmodified
	ts3 := ts2 + 3000
	out3 := tc.Process(ctx, ts3)
	assert.Equal(ts3, out3, "subsequent normal-clock call changed ts")
}

func TestTimestampCorrector_Fix_iOS_Bug(t *testing.T) {
	fps := 30.0
	tc := NewTimestampCorrector(fps)
	ctx := context.Background()
	assert := assert.New(t)

	// simulate two packets spaced normally but with an inflated tsDelta to trigger fix
	t0 := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	dtNs := time.Duration(tc.frameInterval*1e9) * time.Nanosecond
	t1 := t0.Add(dtNs)
	tc.now = makeClock([]time.Time{t0, t1})

	ts1 := int64(1000)
	out1 := tc.Process(ctx, ts1)
	assert.Equal(ts1, out1, "first call changed ts")

	// choose a delta large enough: > thresholdFreq*dt
	dtSec := dtNs.Seconds()
	largeDelta := int64((tc.thresholdFreq * dtSec) + 2)
	ts2 := ts1 + largeDelta
	out2 := tc.Process(ctx, ts2)
	expected2 := multiplyAndDivide(ts2, 90_000, 1_000_000)
	assert.Equal(expected2, out2, "bug-clock second call")

	// subsequent calls also fixed
	ts3 := ts2 + 20000
	out3 := tc.Process(ctx, ts3)
	expected3 := multiplyAndDivide(ts3, 90_000, 1_000_000)
	assert.Equal(expected3, out3, "subsequent bug-clock call")
}

func TestTimestampCorrector_Burst(t *testing.T) {
	fps := 30.0
	tc := NewTimestampCorrector(fps)
	ctx := context.Background()
	assert := assert.New(t)

	// simulate burst: dt < expectedWallclockDelta
	t0 := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	dtNs := time.Duration(tc.frameInterval*1e9) * time.Nanosecond
	fastNs := dtNs / 2 // corresponds to roughly 17ms - below 33ms minimum
	t1 := t0.Add(fastNs)
	// then a normal interval
	t2 := t0.Add(dtNs)
	tc.now = makeClock([]time.Time{t0, t1, t2})

	ts1 := int64(0)
	out1 := tc.Process(ctx, ts1)
	assert.Equal(ts1, out1, "first call mutated ts")

	// we can have a gap of roughly 280ms during a <33ms burst
	ts2 := int64(26000)
	out2 := tc.Process(ctx, ts2)
	assert.Equal(ts2, out2, "flush-phase call changed ts")

	// now a normal spacing with small tsDelta → detectionDone becomes true but no fix
	ts3 := ts2 + 100
	out3 := tc.Process(ctx, ts3)
	assert.Equal(ts3, out3, "post-flush normal call changed ts")
}
