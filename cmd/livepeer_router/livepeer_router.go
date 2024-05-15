package main

import (
	"flag"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	datadir := flag.String("datadir", "", "Directory that data is stored in. Default is /[home]/.lpRouterData")
	httpAddr := flag.String("httpAddr", "", "Address (IP:port) to bind to for HTTP")
	serviceAddr := flag.String("serviceAddr", "", "Publicly accessible URI (IP:port or hostname) to receive requests at. All routers need to run on this port. If using Geo Distance, this needs to be an IP address.")
	dataPort := flag.String("dataPort", "", "Port to serve saved routing data")
	orchAddr := flag.String("orchAddr", "", "Comma delimited list of orchestrator URIs (IP:port or hostname) to use. If using Geo Distance, this needs to be an ip address")
	useLatencyToB := flag.Bool("useLatencyToB", false, "Select orchestrator based on latency to broadcaster")
	searchTimeout := flag.Duration("searchTimeout", 375*time.Millisecond, "Time to wait for orchestrators response.  Default is 375 milliseconds. Needs to be under 3 seconds to stay in B discovery loop. (seconds = 1s, milliseconds = 1000ms)")
	pingBroadcasterTimeout := flag.Duration("pingBroadcasterTimeout", 250*time.Millisecond, "Time to wait for orchestrators response.  Default is 250 milliseconds. Response needs to be under 3 seconds to stay in B discovery loop. (seconds = 1s, milliseconds = 1000ms)")
	secret := flag.String("secret", "", "Secret to secure router data endpoint with")
	cacheTime := flag.Duration("cacheTime", 60*time.Minute, "Input time to cache closest orch (minutes = 5m, seconds = 45s, default is 60 minutes)")
	roundRobin := flag.Bool("roundRobin", true, "Ping all orchestrators to get closest orch, returns first orch to respond if set to false")
	testBroadcasterIP := flag.String("testBroadcasterIP", "", "Input known broadcaster IP address for testing (comma delimited)")
	backgroundUpdate := flag.Bool("backgroundUpdate", false, "Run this on only one router.  Update ping tests to broadcasters in background process")
	maxConcurrentUpdates := flag.Int("maxConcurrentUpdates", 5, "number of concurrent background updates to run")
	geoRouting := flag.Bool("geoRouting", false, "use maxmind db to router requests on distance (save db at /[dataDir]/GeoIP2-City.mmdb)")
	overrideTranscoder := flag.String("overrideTranscoder", "", "url to override transcoder with from orchestrator (eg https://127.0.0.1:8935)")
	overridePricePerUnit := flag.Int64("overridePricePerUnit", -1, "pricePerUnit to override OrchestratorInfo PriceInfo returned by orchestrator")

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

	dPort := "8080"
	if *dataPort == "" {
		sPort, err := strconv.Atoi(serviceURI.Port())
		if err == nil {
			dPort = strconv.Itoa(sPort + 1)
		}
	} else {
		dPort = *dataPort
	}

	var uris []*url.URL
	var orch_nodes []server.OrchNode
	if len(*orchAddr) > 0 {
		for _, addr := range strings.Split(*orchAddr, ",") {
			addr = strings.TrimSpace(addr)
			if !strings.HasPrefix(addr, "http") {
				addr = "https://" + addr
			}

			orchUri, err := url.ParseRequestURI(addr)
			if err != nil {
				glog.Exitf("Could not parse orchestrator URI: %v", err)
			}
			uris = append(uris, orchUri)

			if *useLatencyToB {
				routerUri, err := url.ParseRequestURI("https://" + orchUri.Hostname() + ":" + serviceURI.Port())
				if err != nil {
					glog.Fatalf("Could not parse orchestrator router URI: %v", err)
				}
				node := server.CreateOrchNode(orchUri, routerUri)
				orch_nodes = append(orch_nodes, node)
			}

		}
	}

	errCh := make(chan error)
	if *useLatencyToB {
		srv := server.NewLatencyRouter(orch_nodes, *testBroadcasterIP, *cacheTime, *searchTimeout, *pingBroadcasterTimeout, *roundRobin, *geoRouting, *overrideTranscoder, *overridePricePerUnit)
		glog.Info("Starting livepeer latency based router")
		go func() {
			errCh <- srv.Start(uri, serviceURI, dPort, *datadir, *secret, *backgroundUpdate, *maxConcurrentUpdates)
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
	} else {
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
}
