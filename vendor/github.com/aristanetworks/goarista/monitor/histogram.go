// Copyright (C) 2015  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package monitor

import (
	"expvar"
	"strings"
	"time"

	"github.com/aristanetworks/goarista/monitor/stats"
)

// LatencyHistogram contains the data needed to properly export itself to expvar
// and provide a pretty printed version
type LatencyHistogram struct {
	name        string
	histogram   *stats.Histogram
	latencyUnit time.Duration
}

// NewLatencyHistogram creates a new histogram and registers an HTTP handler for it.
func NewLatencyHistogram(name string, latencyUnit time.Duration, numBuckets int,
	growth float64, smallest float64, minValue int64) *LatencyHistogram {

	histogramOptions := stats.HistogramOptions{
		NumBuckets:         numBuckets,
		GrowthFactor:       growth,
		SmallestBucketSize: smallest,
		MinValue:           minValue,
	}
	histogram := &LatencyHistogram{
		name:        name,
		histogram:   stats.NewHistogram(histogramOptions),
		latencyUnit: latencyUnit,
	}
	expvar.Publish(name, histogram)
	return histogram
}

// Print returns the histogram as a chart
func (h *LatencyHistogram) Print() string {
	return h.addUnits(h.histogram.Delta1m().String()) +
		h.addUnits(h.histogram.Delta10m().String()) +
		h.addUnits(h.histogram.Delta1h().String()) +
		h.addUnits(h.histogram.Value().String())
}

// String returns the histogram as JSON.
func (h *LatencyHistogram) String() string {
	return h.histogram.String()
}

// UpdateLatencyValues updates the LatencyHistogram's buckets with the new
// datapoint and updates the string associated with the expvar.String
func (h *LatencyHistogram) UpdateLatencyValues(delta time.Duration) {
	h.histogram.Add(int64(delta / h.latencyUnit))
}

func (h *LatencyHistogram) addUnits(hist string) string {
	i := strings.Index(hist, "\n")
	return hist[:i] + "µs" + hist[i:]
}
