// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"gopkg.in/yaml.v2"
)

// Config is the representation of ocprometheus's YAML config file.
type Config struct {
	// Per-device labels.
	DeviceLabels map[string]prometheus.Labels

	// Prefixes to subscribe to.
	Subscriptions []string

	// Metrics to collect and how to munge them.
	Metrics []*MetricDef
}

// MetricDef is the representation of a metric definiton in the config file.
type MetricDef struct {
	// Path is a regexp to match on the Update's full path.
	// The regexp must be a prefix match.
	// The regexp can define named capture groups to use as labels.
	Path string

	// Path compiled as a regexp.
	re *regexp.Regexp

	// Metric name.
	Name string

	// Metric help string.
	Help string

	// This map contains the metric descriptors for this metric for each device.
	devDesc map[string]*prometheus.Desc

	// This is the default metric descriptor for devices that don't have explicit descs.
	desc *prometheus.Desc
}

// Parses the config and creates the descriptors for each path and device.
func parseConfig(cfg []byte) (*Config, error) {
	config := &Config{
		DeviceLabels: make(map[string]prometheus.Labels),
	}
	if err := yaml.Unmarshal(cfg, config); err != nil {
		return nil, fmt.Errorf("Failed to parse config: %v", err)
	}

	for _, def := range config.Metrics {
		def.re = regexp.MustCompile(def.Path)

		// Extract label names
		reNames := def.re.SubexpNames()[1:]
		labelNames := make([]string, len(reNames))
		for i, n := range reNames {
			labelNames[i] = n
			if n == "" {
				labelNames[i] = "unnamedLabel" + strconv.Itoa(i+1)
			}
		}

		// Create a default descriptor only if there aren't any per-device labels,
		// or if it's explicitly declared
		if len(config.DeviceLabels) == 0 || len(config.DeviceLabels["*"]) > 0 {
			def.desc = prometheus.NewDesc(def.Name, def.Help, labelNames, config.DeviceLabels["*"])
		}
		// Add per-device descriptors
		def.devDesc = make(map[string]*prometheus.Desc)
		for device, labels := range config.DeviceLabels {
			if device == "*" {
				continue
			}
			def.devDesc[device] = prometheus.NewDesc(def.Name, def.Help, labelNames, labels)
		}
	}

	return config, nil
}

// Returns the descriptor corresponding to the device and path, and labels extracted from the path.
// If the device and path doesn't match any metrics, returns nil.
func (c *Config) getDescAndLabels(s source) (*prometheus.Desc, []string) {
	for _, def := range c.Metrics {
		if groups := def.re.FindStringSubmatch(s.path); groups != nil {
			if desc, ok := def.devDesc[s.addr]; ok {
				return desc, groups[1:]
			}

			return def.desc, groups[1:]
		}
	}

	return nil, nil
}

// Sends all the descriptors to the channel.
func (c *Config) getAllDescs(ch chan<- *prometheus.Desc) {
	for _, def := range c.Metrics {
		// Default descriptor might not be present
		if def.desc != nil {
			ch <- def.desc
		}

		for _, desc := range def.devDesc {
			ch <- desc
		}
	}
}
