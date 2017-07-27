// Copyright (C) 2015  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package monitor provides an embedded HTTP server to expose
// metrics for monitoring
package monitor

import (
	"expvar"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // Go documentation recommended usage

	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/netns"
)

// Server represents a monitoring server
type Server interface {
	Run()
}

// server contains information for the monitoring server
type server struct {
	vrfName string
	// Server name e.g. host[:port]
	serverName string
}

// NewServer creates a new server struct
func NewServer(address string) Server {
	vrfName, addr, err := netns.ParseAddress(address)
	if err != nil {
		glog.Errorf("Failed to parse address: %s", err)
	}
	return &server{
		vrfName:    vrfName,
		serverName: addr,
	}
}

func debugHandler(w http.ResponseWriter, r *http.Request) {
	indexTmpl := `<html>
	<head>
	<title>/debug</title>
	</head>
	<body>
	<p>/debug</p>
	<div><a href="/debug/vars">vars</a></div>
	<div><a href="/debug/pprof">pprof</a></div>
	</body>
	</html>
	`
	fmt.Fprintf(w, indexTmpl)
}

// PrintableHistogram represents a Histogram that can be printed as
// a chart.
type PrintableHistogram interface {
	Print() string
}

// Pretty prints the latency histograms
func histogramHandler(w http.ResponseWriter, r *http.Request) {
	expvar.Do(func(kv expvar.KeyValue) {
		if hist, ok := kv.Value.(PrintableHistogram); ok {
			w.Write([]byte(hist.Print()))
		}
	})
}

// Run sets up the HTTP server and any handlers
func (s *server) Run() {
	http.HandleFunc("/debug", debugHandler)
	http.HandleFunc("/debug/histograms", histogramHandler)

	var listener net.Listener
	err := netns.Do(s.vrfName, func() error {
		var err error
		listener, err = net.Listen("tcp", s.serverName)
		return err
	})
	if err != nil {
		glog.Fatalf("Could not start monitor server in VRF %q: %s", s.vrfName, err)
	}

	err = http.Serve(listener, nil)
	if err != nil {
		glog.Fatal("http serve returned with error:", err)
	}
}
