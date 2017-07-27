// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"testing"

	"github.com/aristanetworks/goarista/test"
	"github.com/openconfig/reference/rpc/openconfig"
	"github.com/prometheus/client_golang/prometheus"
)

func makeMetrics(cfg *Config, expValues map[source]float64) map[source]*labelledMetric {
	expMetrics := map[source]*labelledMetric{}
	for k, v := range expValues {
		desc, labels := cfg.getDescAndLabels(k)
		if desc == nil || labels == nil {
			panic("cfg.getDescAndLabels returned nil")
		}
		expMetrics[k] = &labelledMetric{
			metric: prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, labels...),
			labels: labels,
		}
	}

	return expMetrics
}

func makeResponse(notif *openconfig.Notification) *openconfig.SubscribeResponse {
	return &openconfig.SubscribeResponse{
		Response: &openconfig.SubscribeResponse_Update{Update: notif},
	}
}

func TestUpdate(t *testing.T) {
	config := []byte(`
devicelabels:
        10.1.1.1:
                lab1: val1
                lab2: val2
        '*':
                lab1: val3
                lab2: val4
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
metrics:
        - name: intfCounter
          path: /Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/status/fan/speed/value
          help: Fan Speed`)
	cfg, err := parseConfig(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	coll := newCollector(cfg)

	notif := &openconfig.Notification{
		Prefix: &openconfig.Path{Element: []string{"Sysdb"}},
		Update: []*openconfig.Update{
			{
				Path: &openconfig.Path{
					Element: []string{"lag", "intfCounterDir", "Ethernet1", "intfCounter"},
				},
				Value: &openconfig.Value{
					Type:  openconfig.Type_JSON,
					Value: []byte("42"),
				},
			},
			{
				Path: &openconfig.Path{
					Element: []string{"environment", "cooling", "status", "fan", "speed"},
				},
				Value: &openconfig.Value{
					Type:  openconfig.Type_JSON,
					Value: []byte("{\"value\": 45}"),
				},
			},
		},
	}
	expValues := map[source]float64{
		source{
			addr: "10.1.1.1",
			path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
		}: 42,
		source{
			addr: "10.1.1.1",
			path: "/Sysdb/environment/cooling/status/fan/speed/value",
		}: 45,
	}

	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics := makeMetrics(cfg, expValues)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Update one value, and one path which is not a metric
	notif = &openconfig.Notification{
		Prefix: &openconfig.Path{Element: []string{"Sysdb"}},
		Update: []*openconfig.Update{
			{
				Path: &openconfig.Path{
					Element: []string{"lag", "intfCounterDir", "Ethernet1", "intfCounter"},
				},
				Value: &openconfig.Value{
					Type:  openconfig.Type_JSON,
					Value: []byte("52"),
				},
			},
			{
				Path: &openconfig.Path{
					Element: []string{"environment", "doesntexist", "status"},
				},
				Value: &openconfig.Value{
					Type:  openconfig.Type_JSON,
					Value: []byte("{\"value\": 45}"),
				},
			},
		},
	}
	src := source{
		addr: "10.1.1.1",
		path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
	}
	expValues[src] = 52

	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Same path, different device
	notif = &openconfig.Notification{
		Prefix: &openconfig.Path{Element: []string{"Sysdb"}},
		Update: []*openconfig.Update{
			{
				Path: &openconfig.Path{
					Element: []string{"lag", "intfCounterDir", "Ethernet1", "intfCounter"},
				},
				Value: &openconfig.Value{
					Type:  openconfig.Type_JSON,
					Value: []byte("42"),
				},
			},
		},
	}
	src.addr = "10.1.1.2"
	expValues[src] = 42

	coll.update("10.1.1.2:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Delete a path
	notif = &openconfig.Notification{
		Prefix: &openconfig.Path{Element: []string{"Sysdb"}},
		Delete: []*openconfig.Path{
			{
				Element: []string{"lag", "intfCounterDir", "Ethernet1", "intfCounter"},
			},
		},
	}
	src.addr = "10.1.1.1"
	delete(expValues, src)

	coll.update("10.1.1.1:6042", makeResponse(notif))
	expMetrics = makeMetrics(cfg, expValues)
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}

	// Non-numeric update
	notif = &openconfig.Notification{
		Prefix: &openconfig.Path{Element: []string{"Sysdb"}},
		Update: []*openconfig.Update{
			{
				Path: &openconfig.Path{
					Element: []string{"lag", "intfCounterDir", "Ethernet1", "intfCounter"},
				},
				Value: &openconfig.Value{
					Type:  openconfig.Type_JSON,
					Value: []byte("\"test\""),
				},
			},
		},
	}

	coll.update("10.1.1.1:6042", makeResponse(notif))
	if !test.DeepEqual(expMetrics, coll.metrics) {
		t.Errorf("Mismatched metrics: %v", test.Diff(expMetrics, coll.metrics))
	}
}
