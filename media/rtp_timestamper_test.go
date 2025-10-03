package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	mod32 = 1 << 32 // 4 294 967 296
)

func TestRTPTimestamper(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []int64
		wantRtp []uint32
	}{
		{
			name:    "Initialization",
			inputs:  []int64{0},
			wantRtp: []uint32{0},
		},
		{
			name:    "Monotonic increase",
			inputs:  []int64{0, 1, 2, 1000},
			wantRtp: []uint32{0, 1, 2, 1000},
		},
		{
			name:    "Small backwards jump (no wrap)",
			inputs:  []int64{1000, 990},
			wantRtp: []uint32{1000, 990},
		},
		{
			name:   "Forward 33-bit PTS wrap",
			inputs: []int64{mpegtsMaxPts - 2, 2},
			wantRtp: []uint32{
				uint32((mpegtsMaxPts - 2) % mod32),
				uint32((2 + mpegtsMaxPts) % mod32),
			},
		},
		{
			name:   "Backward 33-bit PTS wrap",
			inputs: []int64{5, mpegtsMaxPts - 3},
			wantRtp: []uint32{
				5,
				uint32(mod32 - 3),
			},
		},
		{
			name:    "Crossing 32-bit boundary (wrap in RTP)",
			inputs:  []int64{halfPts - 1, halfPts, halfPts + 1},
			wantRtp: []uint32{uint32(halfPts - 1), 0, 1},
		},
		{
			name:   "Random jitter near wrap threshold",
			inputs: []int64{mpegtsMaxPts - halfPts/2, 2},
			wantRtp: []uint32{
				uint32((mpegtsMaxPts - halfPts/2) % mod32),
				2,
			},
		},
		{
			name:    "Large monotonic advance (< halfPts)",
			inputs:  []int64{100, halfPts - 1},
			wantRtp: []uint32{100, uint32(halfPts - 1)},
		},
		{
			name:   "Multiple successive wraps",
			inputs: []int64{mpegtsMaxPts - 1, 2, mpegtsMaxPts - 1, 2},
			wantRtp: []uint32{
				uint32((mpegtsMaxPts - 1) % mod32),
				uint32((2 + mpegtsMaxPts) % mod32),
				uint32((mpegtsMaxPts - 1) % mod32),
				uint32((2 + mpegtsMaxPts) % mod32),
			},
		},
		{
			name:    "Idempotent Unwrap (same raw twice)",
			inputs:  []int64{500, 500},
			wantRtp: []uint32{500, 500},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := NewRTPTimestamper()
			gotRtp := make([]uint32, len(tc.inputs))
			for i, raw := range tc.inputs {
				ext := u.ToTS(raw)
				gotRtp[i] = ext
			}
			require.Equal(t, tc.wantRtp, gotRtp)
		})
	}
}
