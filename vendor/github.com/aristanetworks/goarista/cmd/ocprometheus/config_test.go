// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"regexp"
	"testing"

	"github.com/aristanetworks/goarista/test"
	"github.com/prometheus/client_golang/prometheus"
)

func TestParseConfig(t *testing.T) {
	tCases := []struct {
		input  []byte
		config Config
	}{
		{
			input: []byte(`
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
          path: /Sysdb/environment/cooling/fan/speed/value
          help: Fan Speed`),
			config: Config{
				DeviceLabels: map[string]prometheus.Labels{
					"10.1.1.1": {
						"lab1": "val1",
						"lab2": "val2",
					},
					"*": {
						"lab1": "val3",
						"lab2": "val4",
					},
				},
				Subscriptions: []string{
					"/Sysdb/environment/cooling/status",
					"/Sysdb/environment/power/status",
				},
				Metrics: []*MetricDef{
					{
						Path: "/Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter",
						re: regexp.MustCompile(
							"/Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter"),
						Name: "intfCounter",
						Help: "Per-Interface Bytes/Errors/Discards Counters",
						devDesc: map[string]*prometheus.Desc{
							"10.1.1.1": prometheus.NewDesc("intfCounter",
								"Per-Interface Bytes/Errors/Discards Counters",
								[]string{"unnamedLabel1", "intf"},
								prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
						},
						desc: prometheus.NewDesc("intfCounter",
							"Per-Interface Bytes/Errors/Discards Counters",
							[]string{"unnamedLabel1", "intf"},
							prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
					},
					{
						Path: "/Sysdb/environment/cooling/fan/speed/value",
						re:   regexp.MustCompile("/Sysdb/environment/cooling/fan/speed/value"),
						Name: "fanSpeed",
						Help: "Fan Speed",
						devDesc: map[string]*prometheus.Desc{
							"10.1.1.1": prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
								prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
						},
						desc: prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
							prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
					},
				},
			},
		},
		{
			input: []byte(`
devicelabels:
        '*':
                lab1: val3
                lab2: val4
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
metrics:
        - name: intfCounter
          path: /Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/fan/speed/value
          help: Fan Speed`),
			config: Config{
				DeviceLabels: map[string]prometheus.Labels{
					"*": {
						"lab1": "val3",
						"lab2": "val4",
					},
				},
				Subscriptions: []string{
					"/Sysdb/environment/cooling/status",
					"/Sysdb/environment/power/status",
				},
				Metrics: []*MetricDef{
					{
						Path: "/Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter",
						re: regexp.MustCompile(
							"/Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter"),
						Name:    "intfCounter",
						Help:    "Per-Interface Bytes/Errors/Discards Counters",
						devDesc: map[string]*prometheus.Desc{},
						desc: prometheus.NewDesc("intfCounter",
							"Per-Interface Bytes/Errors/Discards Counters",
							[]string{"intf"},
							prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
					},
					{
						Path:    "/Sysdb/environment/cooling/fan/speed/value",
						re:      regexp.MustCompile("/Sysdb/environment/cooling/fan/speed/value"),
						Name:    "fanSpeed",
						Help:    "Fan Speed",
						devDesc: map[string]*prometheus.Desc{},
						desc: prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
							prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
					},
				},
			},
		},
		{
			input: []byte(`
devicelabels:
        10.1.1.1:
                lab1: val1
                lab2: val2
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
metrics:
        - name: intfCounter
          path: /Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/fan/speed/value
          help: Fan Speed`),
			config: Config{
				DeviceLabels: map[string]prometheus.Labels{
					"10.1.1.1": {
						"lab1": "val1",
						"lab2": "val2",
					},
				},
				Subscriptions: []string{
					"/Sysdb/environment/cooling/status",
					"/Sysdb/environment/power/status",
				},
				Metrics: []*MetricDef{
					{
						Path: "/Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter",
						re: regexp.MustCompile(
							"/Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter"),
						Name: "intfCounter",
						Help: "Per-Interface Bytes/Errors/Discards Counters",
						devDesc: map[string]*prometheus.Desc{
							"10.1.1.1": prometheus.NewDesc("intfCounter",
								"Per-Interface Bytes/Errors/Discards Counters",
								[]string{"intf"},
								prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
						},
					},
					{
						Path: "/Sysdb/environment/cooling/fan/speed/value",
						re:   regexp.MustCompile("/Sysdb/environment/cooling/fan/speed/value"),
						Name: "fanSpeed",
						Help: "Fan Speed",
						devDesc: map[string]*prometheus.Desc{
							"10.1.1.1": prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
								prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
						},
					},
				},
			},
		},
		{
			input: []byte(`
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
metrics:
        - name: intfCounter
          path: /Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/fan/speed/value
          help: Fan Speed`),
			config: Config{
				DeviceLabels: map[string]prometheus.Labels{},
				Subscriptions: []string{
					"/Sysdb/environment/cooling/status",
					"/Sysdb/environment/power/status",
				},
				Metrics: []*MetricDef{
					{
						Path: "/Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter",
						re: regexp.MustCompile(
							"/Sysdb/(?:lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter"),
						Name:    "intfCounter",
						Help:    "Per-Interface Bytes/Errors/Discards Counters",
						devDesc: map[string]*prometheus.Desc{},
						desc: prometheus.NewDesc("intfCounter",
							"Per-Interface Bytes/Errors/Discards Counters",
							[]string{"intf"}, prometheus.Labels{}),
					},
					{
						Path:    "/Sysdb/environment/cooling/fan/speed/value",
						re:      regexp.MustCompile("/Sysdb/environment/cooling/fan/speed/value"),
						Name:    "fanSpeed",
						Help:    "Fan Speed",
						devDesc: map[string]*prometheus.Desc{},
						desc: prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
							prometheus.Labels{}),
					},
				},
			},
		},
	}

	for i, c := range tCases {
		cfg, err := parseConfig(c.input)
		if err != nil {
			t.Errorf("Unexpected error in case %d: %v", i+1, err)
			continue
		}
		if !test.DeepEqual(*cfg, c.config) {
			t.Errorf("Test case %d: mismatch %v", i+1, test.Diff(*cfg, c.config))
		}
	}
}

func TestGetDescAndLabels(t *testing.T) {
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

	tCases := []struct {
		src    source
		desc   *prometheus.Desc
		labels []string
	}{
		{
			src: source{
				addr: "10.1.1.1",
				path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
			},
			desc: prometheus.NewDesc("intfCounter", "Per-Interface Bytes/Errors/Discards Counters",
				[]string{"unnamedLabel1", "intf"},
				prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
			labels: []string{"lag", "Ethernet1"},
		},
		{
			src: source{
				addr: "10.2.2.2",
				path: "/Sysdb/lag/intfCounterDir/Ethernet1/intfCounter",
			},
			desc: prometheus.NewDesc("intfCounter", "Per-Interface Bytes/Errors/Discards Counters",
				[]string{"unnamedLabel1", "intf"},
				prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
			labels: []string{"lag", "Ethernet1"},
		},
		{
			src: source{
				addr: "10.2.2.2",
				path: "/Sysdb/environment/cooling/status/fan/speed/value",
			},
			desc: prometheus.NewDesc("fanSpeed", "Fan Speed",
				[]string{},
				prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
			labels: []string{},
		},
		{
			src: source{
				addr: "10.2.2.2",
				path: "/Sysdb/environment/nonexistent",
			},
			desc:   nil,
			labels: nil,
		},
	}

	for i, c := range tCases {
		desc, labels := cfg.getDescAndLabels(c.src)
		if !test.DeepEqual(desc, c.desc) {
			t.Errorf("Test case %d: desc mismatch %v", i+1, test.Diff(desc, c.desc))
		}
		if !test.DeepEqual(labels, c.labels) {
			t.Errorf("Test case %d: labels mismatch %v", i+1, test.Diff(labels, c.labels))
		}
	}
}

func TestGetAllDescs(t *testing.T) {
	tCases := []struct {
		config []byte
		descs  []*prometheus.Desc
	}{
		{
			config: []byte(`
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
          help: Fan Speed`),
			descs: []*prometheus.Desc{
				prometheus.NewDesc("intfCounter", "Per-Interface Bytes/Errors/Discards Counters",
					[]string{"unnamedLabel1", "intf"},
					prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
				prometheus.NewDesc("intfCounter", "Per-Interface Bytes/Errors/Discards Counters",
					[]string{"unnamedLabel1", "intf"},
					prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
				prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
					prometheus.Labels{"lab1": "val3", "lab2": "val4"}),
				prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
					prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
			},
		},
		{
			config: []byte(`
devicelabels:
        10.1.1.1:
                lab1: val1
                lab2: val2
subscriptions:
        - /Sysdb/environment/cooling/status
        - /Sysdb/environment/power/status
metrics:
        - name: intfCounter
          path: /Sysdb/(lag|slice/phy/.+)/intfCounterDir/(?P<intf>.+)/intfCounter
          help: Per-Interface Bytes/Errors/Discards Counters
        - name: fanSpeed
          path: /Sysdb/environment/cooling/status/fan/speed/value
          help: Fan Speed`),
			descs: []*prometheus.Desc{
				prometheus.NewDesc("intfCounter", "Per-Interface Bytes/Errors/Discards Counters",
					[]string{"unnamedLabel1", "intf"},
					prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
				prometheus.NewDesc("fanSpeed", "Fan Speed", []string{},
					prometheus.Labels{"lab1": "val1", "lab2": "val2"}),
			},
		},
	}

	for i, c := range tCases {
		cfg, err := parseConfig(c.config)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		ch := make(chan *prometheus.Desc, 10)
		cfg.getAllDescs(ch)
		j := 0
		for d := range ch {
			if !test.DeepEqual(c.descs[j], d) {
				t.Errorf("Test case %d: desc %d mismatch %v", i+1, j+1, test.Diff(c.descs[j], d))
			}
			j++
			if j == len(c.descs) {
				break
			}
		}
		select {
		case <-ch:
			t.Errorf("Test case %d: too many descs", i+1)
		default:
		}
	}
}
