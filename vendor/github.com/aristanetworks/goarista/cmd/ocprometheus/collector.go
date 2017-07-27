// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/aristanetworks/glog"
	"github.com/golang/protobuf/proto"
	"github.com/openconfig/reference/rpc/openconfig"
	"github.com/prometheus/client_golang/prometheus"
)

// A metric source.
type source struct {
	addr string
	path string
}

// Since the labels are fixed per-path and per-device we can cache them here,
// to avoid recomputing them.
type labelledMetric struct {
	metric prometheus.Metric
	labels []string
}

type collector struct {
	// Protects access to metrics map
	m       sync.Mutex
	metrics map[source]*labelledMetric

	config *Config
}

func newCollector(config *Config) *collector {
	return &collector{
		metrics: make(map[source]*labelledMetric),
		config:  config,
	}
}

// Process a notfication and update or create the corresponding metrics.
func (c *collector) update(addr string, message proto.Message) {
	resp, ok := message.(*openconfig.SubscribeResponse)
	if !ok {
		glog.Errorf("Unexpected type of message: %T", message)
		return
	}
	notif := resp.GetUpdate()
	if notif == nil {
		return
	}

	device := strings.Split(addr, ":")[0]
	prefix := "/" + strings.Join(notif.Prefix.Element, "/")

	// Process deletes first
	for _, del := range notif.Delete {
		path := prefix + "/" + strings.Join(del.Element, "/")
		key := source{addr: device, path: path}
		c.m.Lock()
		delete(c.metrics, key)
		c.m.Unlock()
	}

	// Process updates next
	for _, update := range notif.Update {
		// We only use JSON encoded values
		if update.Value == nil || update.Value.Type != openconfig.Type_JSON {
			glog.V(9).Infof("Ignoring incompatible update value in %s", update)
			continue
		}

		path := prefix + "/" + strings.Join(update.Path.Element, "/")
		value, suffix, ok := parseValue(update)
		if !ok {
			continue
		}
		if suffix != "" {
			path += "/" + suffix
		}
		src := source{addr: device, path: path}
		c.m.Lock()
		// Use the cached labels and descriptor if available
		if m, ok := c.metrics[src]; ok {
			m.metric = prometheus.MustNewConstMetric(m.metric.Desc(), prometheus.GaugeValue, value,
				m.labels...)
			c.m.Unlock()
			continue
		}
		c.m.Unlock()

		// Get the descriptor and labels for this source
		desc, labelValues := c.config.getDescAndLabels(src)
		if desc == nil {
			glog.V(8).Infof("Ignoring unmatched update at %s:%s: %+v", device, path, update.Value)
			continue
		}

		c.m.Lock()
		// Save the metric and labels in the cache
		metric := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelValues...)
		c.metrics[src] = &labelledMetric{
			metric: metric,
			labels: labelValues,
		}
		c.m.Unlock()
	}
}

func parseValue(update *openconfig.Update) (float64, string, bool) {
	// All metrics in Prometheus are floats, so only try to unmarshal as float64.
	var intf interface{}
	if err := json.Unmarshal(update.Value.Value, &intf); err != nil {
		glog.Errorf("Can't parse value in update %v: %v", update, err)
		return 0, "", false
	}

	switch value := intf.(type) {
	case float64:
		return value, "", true
	case map[string]interface{}:
		if vIntf, ok := value["value"]; ok {
			if val, ok := vIntf.(float64); ok {
				return val, "value", true
			}
		}
	default:
		glog.V(9).Infof("Ignorig non-numeric update: %v", update)
	}

	return 0, "", false
}

// Describe implements prometheus.Collector interface
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	c.config.getAllDescs(ch)
}

// Collect implements prometheus.Collector interface
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.m.Lock()
	for _, m := range c.metrics {
		ch <- m.metric
	}
	c.m.Unlock()
}
