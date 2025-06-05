package media

import (
	"context"
	"time"

	"github.com/livepeer/go-livepeer/clog"
)

// TimestampCorrector detects a bug with iOS 18.4.1+ where
// timestamps come in with microsecond frequency -
// 1000khz rather than the 90khz that RTP requires
// If needed, divides incoming timestamps by 90_000 / 1_000_000.
// https://bugs.webkit.org/show_bug.cgi?id=292273
type TimestampCorrector struct {
	// configuration
	frameInterval float64          // expected time between successive frames
	thresholdFreq float64          // threshold in Hz to trigger correction
	now           func() time.Time // function to obtain current time, injected for testing

	// detection state
	firstTS       int64
	firstArrival  time.Time
	detectionDone bool
	needsFix      bool
}

// NewTimestampCorrector creates a detector based on your target fps.
func NewTimestampCorrector(fps float64) *TimestampCorrector {
	return &TimestampCorrector{
		frameInterval: 1.0 / fps,
		thresholdFreq: 800_000,  // anything above 800 kHz flags the bug
		now:           time.Now, // default to real clock
	}
}

// Process inspects the supplied timestamp against wall‐clock arrival time,
// decides whether the iOS bug is present, and if so converts to 90khz from 1000khz
// (1000khz == microsecond time base) Returns the (possibly corrected) timestamp.
func (c *TimestampCorrector) Process(ctx context.Context, ts int64) int64 {

	// detection phase
	if !c.detectionDone {
		now := c.now()

		if c.firstArrival.IsZero() {
			if ts != 0 {
				// Normally this is not a problem *except* if this client
				// has the iOS bug and we need to convert the first ts.
				// Could result in sending a very large first ts then small ones
				//
				// NB: we can't simply add rescaled deltas to the first timestamp
				// 		 because that could break audio sync
				clog.Info(ctx, "TSCorrector: First timestamp was not zero!", "ts", ts)
			}
			c.firstArrival = now
			c.firstTS = ts
			return ts
		}

		if c.firstTS == ts {
			// handle frames split across packets
			return ts
		}

		dt := now.Sub(c.firstArrival).Seconds()
		if dt < c.frameInterval {
			// burst / flush (too fast)
			// usually from waiting for a late / lost packet
			clog.Info(ctx, "TSCorrector: Burst packet, speculatively setting wallclock delta", "dt_ms", dt*1000, "diff_ms", (c.frameInterval-dt)*1000)
			dt = c.frameInterval
		}

		tsDelta := float64(ts - c.firstTS)
		freq := tsDelta / dt
		if freq > c.thresholdFreq {
			// TODO remove this entire file once we stop seeing this log line
			clog.Info(ctx, "TSCorrector: Frequency too high! Starting microsecond timestamp mode", "freq", freq, "first", c.firstTS, "ts", ts, "delta", tsDelta, "dt_ms", dt*1000)
			c.needsFix = true
		} else {
			clog.Info(ctx, "TSCorrector: All good", "freq", freq, "first", c.firstTS, "ts", ts, "delta", tsDelta, "dt_ms", dt*1000)
		}
		c.detectionDone = true
	}

	// apply correction once flagged
	if c.needsFix {
		return multiplyAndDivide(ts, 90_000, 1_000_000)
	}

	return ts
}
