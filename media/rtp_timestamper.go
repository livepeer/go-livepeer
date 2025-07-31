package media

const (
	// MPEG‑TS PTS is 33 bits wide; it wraps at 2^33.
	mpegtsMaxPts = 1 << 33
	halfPts      = mpegtsMaxPts / 2
)

// RTPTimestamper keeps track of wrap‑arounds in a 33‑bit PTS stream.
type RTPTimestamper struct {
	lastRaw     int64 // last raw PTS seen
	wrapCount   int64 // how many times we've wrapped
	initialized bool  // false until first Unwrap call
}

// NewRTPTimestamper creates a fresh unwrapper.
func NewRTPTimestamper() *RTPTimestamper {
	return &RTPTimestamper{}
}

// returns a monotonic 32‑bit timeline by accounting for 33-bit wrap‑arounds.
func (u *RTPTimestamper) ToTS(rawPts int64) uint32 {
	if u.initialized {
		delta := rawPts - u.lastRaw
		if delta < -halfPts {
			// wrapped forward
			u.wrapCount++
		} else if delta > halfPts {
			// wrapped backward (unlikely in normal streams)
			u.wrapCount--
		}
	} else {
		u.initialized = true
	}
	u.lastRaw = rawPts
	return uint32(rawPts + u.wrapCount*mpegtsMaxPts)
}
