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
	ctx := context.Background()
	assert := assert.New(t)

	// simulate two packets spaced at expectedWallclockDelta, tsDelta=expectedWallclockDelta*90k
	t0 := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	dtNs := time.Duration((1.0/fps)*0.5*1e9) * time.Nanosecond
	t1 := t0.Add(dtNs)

	tc := NewTimestampCorrector(TimestampCorrectorConfig{
		FPS:   fps,
		Clock: makeClock([]time.Time{t0, t1}),
	})

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

func TestTimestampCorrector_Fix(t *testing.T) {

	// Table driven tests for a broken set of timestamps
	// under various conditions

	tests := []struct {
		name      string
		userAgent string
		disable   bool
		wantFix   bool
	}{
		{
			name:    "defaults",
			wantFix: true,
		},
		{
			name:      "UA matched",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15",
			wantFix:   true,
		},
		{
			name:      "UA did not match",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
			wantFix:   false,
		},
		{
			name:    "Disabled",
			disable: true,
			wantFix: false,
		},
		{
			name:      "Matching UA but disabled",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15",
			disable:   true,
			wantFix:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			fps := 30.0
			ctx := context.Background()

			// simulate two wall-clock timestamps spaced by expected dt
			t0 := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
			dtNs := time.Duration((1.0/fps)*1e9) * time.Nanosecond
			t1 := t0.Add(dtNs)

			conf := TimestampCorrectorConfig{
				FPS:       fps,
				Clock:     makeClock([]time.Time{t0, t1}),
				UserAgent: tc.userAgent,
				Disable:   tc.disable,
			}
			corr := NewTimestampCorrector(conf)

			// first packet: always unmodified
			ts1 := int64(1000)
			out1 := corr.Process(ctx, ts1)
			assert.Equal(ts1, out1, "first call must not change ts")

			// second packet: inflated tsDelta to trigger or not trigger fix
			dtSec := dtNs.Seconds()
			largeDelta := int64((corr.thresholdFreq * dtSec) + 2)
			ts2 := ts1 + largeDelta
			out2 := corr.Process(ctx, ts2)

			// expected: either raw ts2 or fixed via multiplyAndDivide
			want2 := ts2
			if tc.wantFix {
				want2 = multiplyAndDivide(ts2, 90000, 1000000)
			}
			assert.Equal(want2, out2, "second call")

			// third packet: follow same rule as second
			ts3 := ts2 + 20000
			out3 := corr.Process(ctx, ts3)
			want3 := ts3
			if tc.wantFix {
				want3 = multiplyAndDivide(ts3, 90000, 1000000)
			}
			assert.Equal(want3, out3, "third call")
		})
	}
}

func TestTimestampCorrector_Burst(t *testing.T) {
	fps := 30.0
	ctx := context.Background()
	assert := assert.New(t)

	// simulate burst: dt < expectedWallclockDelta
	t0 := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	dtNs := time.Duration((1.0/fps)*1e9) * time.Nanosecond
	fastNs := dtNs / 2 // corresponds to roughly 17ms - below 33ms minimum
	t1 := t0.Add(fastNs)
	// then a normal interval
	t2 := t0.Add(dtNs)

	tc := NewTimestampCorrector(TimestampCorrectorConfig{
		FPS:   fps,
		Clock: makeClock([]time.Time{t0, t1, t2}),
	})

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

func TestTimestampCorrector_UAFilter(t *testing.T) {
	matches := []struct {
		name string
		ua   string
	}{
		{
			name: "Safari on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15",
		},
		{
			name: "Safari on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 15_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6.1 Mobile/15E148 Safari/604.1",
		},
		{
			name: "Chrome on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/137.0.7151.79 Mobile/15E148 Safari/604.1",
		},
		{
			name: "Twitter on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/22F76 Twitter for iPhone/11.1.5",
		},
		{
			name: "Firefox on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/139.1 Mobile/15E148 Safari/605.1.15",
		},
		{
			name: "Facebook on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/510.0.0.47.116;FBBV/743276974;FBDV/iPhone17,3;FBMD/iPhone;FBSN/iOS;FBSV/18.5;FBSS/3;FBCR/;FBID/phone;FBLC/en_US;FBOP/80]",
		},
		{
			name: "Edge on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_7_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 EdgiOS/137.3296.65 Mobile/15E148 Safari/605.1.15",
		},
	}
	noMatches := []struct {
		name string
		ua   string
	}{
		{
			name: "Chrome on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
		},
		{
			name: "Older Safari on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
		},
		{
			name: "Safari 18.6 on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15",
		},
		{
			name: "Safari 18.6 on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1",
		},
		{
			name: "Headless Chrome on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/137.0.0.0 Safari/537.36",
		},
		{
			name: "Firefox on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:139.0) Gecko/20100101 Firefox/139.0",
		},
		{
			name: "Edge on MacOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36 Edg/137.0.3296.68",
		},
		{
			name: "Chrome on Android",
			ua:   "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36",
		},
		{
			name: "Edge on Android",
			ua:   "Mozilla/5.0 (Linux; Android 10; HD1913) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.7151.73 Mobile Safari/537.36 EdgA/137.0.3296.65",
		},
		{
			name: "Chrome on Linux",
			ua:   "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
		},
		{
			name: "Chrome on Windows",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
		},
		{
			name: "Edge on Windows",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36 Edg/137.0.3296.68",
		},
	}

	t.Run("Matches", func(t *testing.T) {
		for _, tc := range matches {
			t.Run(tc.name, func(t *testing.T) {
				assert.True(t, shouldFix(tc.ua), tc.ua)
			})
		}
	})

	t.Run("Does not match", func(t *testing.T) {
		for _, tc := range noMatches {
			t.Run(tc.name, func(t *testing.T) {
				assert.False(t, shouldFix(tc.ua), tc.ua)
			})
		}
	})

}
