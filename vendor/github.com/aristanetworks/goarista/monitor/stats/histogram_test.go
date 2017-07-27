// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package stats_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aristanetworks/goarista/monitor/stats"
)

func testJSON(h *stats.Histogram) error {
	js, err := h.Value().MarshalJSON()
	if err != nil {
		return err
	}
	var v interface{}
	err = json.Unmarshal(js, &v)
	if err != nil {
		return fmt.Errorf("Failed to parse JSON: %s\nJSON was: %s", err, js)
	}
	return nil
}

// Ensure we can JSONify the histogram into valid JSON.
func TestJSON(t *testing.T) {
	h := stats.NewHistogram(
		stats.HistogramOptions{NumBuckets: 10, GrowthFactor: 0,
			SmallestBucketSize: 20, MinValue: 0})
	testJSON(h)
	h.Add(42)
	testJSON(h)
}
