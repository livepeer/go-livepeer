// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// The ocprometheus implements a Prometheus exporter for OpenConfig telemetry data.
package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/openconfig/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenaddr := flag.String("listenaddr", ":8080", "Address on which to expose the metrics")
	url := flag.String("url", "/metrics", "URL where to expose the metrics")
	configFlag := flag.String("config", "",
		"Config to turn OpenConfig telemetry into Prometheus metrics")
	username, password, subscriptions, addrs, opts := client.ParseFlags()

	if *configFlag == "" {
		glog.Fatal("You need specify a config file using -config flag")
	}
	cfg, err := ioutil.ReadFile(*configFlag)
	if err != nil {
		glog.Fatalf("Can't read config file %q: %v", *configFlag, err)
	}
	config, err := parseConfig(cfg)
	if err != nil {
		glog.Fatal(err)
	}

	// Ignore the default "subscribe-to-everything" subscription of the
	// -subscribe flag.
	if subscriptions[0] == "" {
		subscriptions = subscriptions[1:]
	}
	// Add the subscriptions from the config file.
	subscriptions = append(subscriptions, config.Subscriptions...)

	coll := newCollector(config)
	prometheus.MustRegister(coll)

	wg := new(sync.WaitGroup)
	for _, addr := range addrs {
		wg.Add(1)
		c := client.New(username, password, addr, opts)
		go c.Subscribe(wg, subscriptions, coll.update)
	}

	http.Handle(*url, promhttp.Handler())
	glog.Fatal(http.ListenAndServe(*listenaddr, nil))
}
