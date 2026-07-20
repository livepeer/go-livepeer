package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLivePaymentProcessorProcessesConfiguredUnits(t *testing.T) {
	start := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	lv2vUnits := int64(defaultSegInfo.Height) * int64(defaultSegInfo.Width) * int64(defaultSegInfo.FPS)
	tests := []struct {
		name    string
		units   int64
		elapsed time.Duration
		want    int64
	}{
		{name: "LV2V", units: lv2vUnits, elapsed: 1500 * time.Millisecond, want: lv2vUnits * 3 / 2},
		{name: "live whole seconds", units: 1, elapsed: 2 * time.Second, want: 2},
		{name: "fractional unit truncation", units: 1, elapsed: 1500 * time.Millisecond, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var processed int64
			p := &LivePaymentProcessor{
				units:              tt.units,
				lastProcessedAt:    start,
				processSegmentFunc: func(units int64) error { processed = units; return nil },
			}

			p.processOne(context.Background(), start.Add(tt.elapsed))

			require.Equal(t, tt.want, processed)
			require.Equal(t, start.Add(tt.elapsed), p.lastProcessedAt)
		})
	}
}
