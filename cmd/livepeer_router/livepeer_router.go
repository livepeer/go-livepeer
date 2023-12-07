package main

import (
	"flag"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/server"
)

func defaultAddr(addr, defaultHost, defaultPort string) string {
	if addr == "" {
		return defaultHost + ":" + defaultPort
	}
	if addr[0] == ':' {
		return defaultHost + addr
	}
	// not IPv6 safe
	if !strings.Contains(addr, ":") {
		return addr + ":" + defaultPort
	}
	return addr
}

func main() {
	flag.Set("logtostderr", "true")
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	datadir := flag.String("datadir", "", "Directory that data is stored in")
	httpAddr := flag.String("httpAddr", "", "Address (IP:port) to bind to for HTTP")
	serviceAddr := flag.String("serviceAddr", "", "Publicly accessible URI (IP:port or hostname) to receive requests at")
	orchAddr := flag.String("orchAddr", "", "Comma delimited list of orchestrator URIs (IP:port or hostname) to use")

	flag.Parse()

	usr, err := user.Current()
	if err != nil {
		glog.Exitf("Cannot find current user: %v", err)
	}

	if *datadir == "" {
		homedir := os.Getenv("HOME")
		if homedir == "" {
			homedir = usr.HomeDir
		}
		*datadir = filepath.Join(homedir, ".lpRouterData")
	}

	if _, err := os.Stat(*datadir); os.IsNotExist(err) {
		glog.Infof("Creating datadir: %v", *datadir)
		if err = os.MkdirAll(*datadir, 0755); err != nil {
			glog.Exitf("Error creating datadir: %v", err)
		}
	}

	if *serviceAddr == "" {
		glog.Exit("Missing -serviceAddr")
	}

	serviceURI, err := url.ParseRequestURI("https://" + *serviceAddr)
	if err != nil {
		glog.Exitf("Could not parse -serviceAddr: %v", err)
	}

	*httpAddr = defaultAddr(*httpAddr, "0.0.0.0", serviceURI.Port())
	uri, err := url.ParseRequestURI("https://" + *httpAddr)
	if err != nil {
		glog.Exitf("Could not parse -httpAddr: %v", err)
	}

	var uris []*url.URL
	if len(*orchAddr) > 0 {
		for _, addr := range strings.Split(*orchAddr, ",") {
			addr = strings.TrimSpace(addr)
			if !strings.HasPrefix(addr, "http") {
				addr = "https://" + addr
			}
			uri, err := url.ParseRequestURI(addr)
			if err != nil {
				glog.Exitf("Could not parse orchestrator URI: %v", err)
			}
			uris = append(uris, uri)
		}
	}

	errCh := make(chan error)
	srv := server.NewRouter(uris)
	go func() {
		errCh <- srv.Start(uri, serviceURI, *datadir)
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
		glog.Infof("Shutting down router server")
		srv.Stop()
	case err := <-errCh:
		if err != nil {
			glog.Errorf("Router server error: %v", err)
		}
	}
}
