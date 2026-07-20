package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLivePaymentProcessorProcessesConfiguredUnits(t *testing.T) {
	start := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	var processed []int64
	p := &LivePaymentProcessor{
		units:           lv2vPixelsPerSecond,
		lastProcessedAt: start,
		processUnitsFunc: func(units int64) error {
			processed = append(processed, units)
			return nil
		},
	}

	p.processOne(context.Background(), start.Add(1500*time.Millisecond))

	require.Equal(t, []int64{lv2vPixelsPerSecond * 3 / 2}, processed)
	require.Equal(t, start.Add(1500*time.Millisecond), p.lastProcessedAt)
}

func TestLivePaymentProcessorTruncatesFractionalUnits(t *testing.T) {
	start := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	var processed []int64
	p := &LivePaymentProcessor{
		units:           1,
		lastProcessedAt: start,
		processUnitsFunc: func(units int64) error {
			processed = append(processed, units)
			return nil
		},
	}

	p.processOne(context.Background(), start.Add(1500*time.Millisecond))
	p.processOne(context.Background(), start.Add(2*time.Second))

	require.Equal(t, []int64{1, 0}, processed)
	require.Equal(t, start.Add(2*time.Second), p.lastProcessedAt)
}

func TestNewLivePaymentProcessorUsesLV2VUnits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := NewLivePaymentProcessor(ctx, time.Second, func(int64) error { return nil })

	require.Equal(t, lv2vPixelsPerSecond, p.units)
}
