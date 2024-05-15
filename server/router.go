package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	gonet "net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	"github.com/livepeer/go-livepeer/net"
	probing "github.com/prometheus-community/pro-bing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"

	"github.com/oschwald/geoip2-golang"
)

const getOrchestratorTimeout = 2 * time.Second

var errNoOrchestrators = errors.New("no orchestrators")

type Router struct {
	uris []*url.URL
	srv  *grpc.Server
	net.UnimplementedOrchestratorServer
}

func (r *Router) EndTranscodingSession(ctx context.Context, request *net.EndTranscodingSessionRequest) (*net.EndTranscodingSessionResponse, error) {
	// shouldn't ever be called on Router
	return &net.EndTranscodingSessionResponse{}, nil
}

func NewRouter(uris []*url.URL) *Router {
	return &Router{uris: uris}
}

func (r *Router) Start(uri *url.URL, serviceURI *url.URL, workDir string) error {
	listener, err := gonet.Listen("tcp", uri.Host)
	if err != nil {
		return err
	}
	defer listener.Close()

	certFile, keyFile, err := getCert(serviceURI, workDir)
	if err != nil {
		return err
	}

	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return err
	}

	s := grpc.NewServer(grpc.Creds(creds))
	r.srv = s

	net.RegisterOrchestratorServer(s, r)

	grpc_health_v1.RegisterHealthServer(s, health.NewServer())

	errCh := make(chan error)
	go func() {
		errCh <- s.Serve(listener)
	}()

	time.Sleep(1 * time.Second)

	if err := checkAvailability(context.Background(), serviceURI); err != nil {
		s.Stop()
		return err
	}

	glog.Infof("Started router server at %v", uri)

	return <-errCh
}

func (r *Router) Stop() {
	r.srv.Stop()
}

func (r *Router) GetOrchestrator(ctx context.Context, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	return getOrchestratorInfo(ctx, r.uris, req)
}

func (r *Router) Ping(ctx context.Context, req *net.PingPong) (*net.PingPong, error) {
	return &net.PingPong{Value: []byte{}}, nil
}

func checkAvailability(ctx context.Context, uri *url.URL) error {
	orch_client, conn, err := startOrchestratorClient(ctx, uri)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(clog.Clone(context.Background(), ctx), GRPCConnectTimeout)
	defer cancel()

	_, err = orch_client.Ping(ctx, &net.PingPong{Value: []byte{}})
	if err != nil {
		return err
	}

	return nil
}

func getOrchestratorInfo(ctx context.Context, uris []*url.URL, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	if len(uris) == 0 {
		return nil, errNoOrchestrators
	}

	infoCh := make(chan *net.OrchestratorInfo)
	errCh := make(chan error, len(uris))

	cctx, cancel := context.WithTimeout(ctx, getOrchestratorTimeout)
	defer cancel()

	for _, uri := range uris {
		go func(uri *url.URL) {
			client, conn, err := startOrchestratorClient(ctx, uri)
			if err != nil {
				errCh <- fmt.Errorf("%v err=%q", uri, err)
				return
			}
			defer conn.Close()

			info, err := client.GetOrchestrator(cctx, req)
			if err != nil {
				errCh <- fmt.Errorf("%v err=%q", uri, err)
				return
			}

			select {
			case infoCh <- info:
			default:
			}
		}(uri)
	}

	errCtr := 0
	for {
		select {
		case info := <-infoCh:
			//glog.Infof("Forwarding OrchestratorInfo orch=%v", info.Transcoder)
			return info, nil
		case err := <-errCh:
			glog.Error(err)
			errCtr++
			if errCtr >= len(uris) {
				return nil, errNoOrchestrators
			}
		case <-cctx.Done():
			return nil, errors.New("timed out")
		}
	}
}

var errNoClientIp = errors.New("cannot get client ip")
var errParsingClientIp = errors.New("error parsing client ip")

type ClientInfo struct {
	addr string
	ip   string
	port string
}

type LatencyRouter struct {
	srv                    *grpc.Server
	testBroadcasterIP      string
	workDir                string
	roundRobin             bool
	cacheTime              time.Duration
	searchTimeout          time.Duration
	pingBroadcasterTimeout time.Duration
	secret                 string
	geoRouting             bool
	overrideTranscoder     string
	overridePricePerUnit   int64

	bmu                    sync.RWMutex
	closestOrchestratorToB map[string]LatencyCheckResponse
	orchNodes              map[url.URL]OrchNode
	broadcasterReqeuests   map[ethcommon.Address]*net.OrchestratorRequest

	cmu     sync.RWMutex
	clients map[url.URL]*LatencyCheckClient
}

type OrchNode struct {
	uri       url.URL
	routerUri url.URL
	orchInfo  map[ethcommon.Address]net.OrchestratorInfo
	updatedAt time.Time
}

type LatencyCheckResponse struct {
	RespTime       float64
	Distance       float64
	OrchUri        url.URL
	UpdatedAt      time.Time
	DoNotUpdate    bool
	UseGeoDistance bool
}

type RouterUpdated struct {
	b_ip_addr           string
	orch_router_ip_addr string
	updated             bool
}

type LatencyCheckClient struct {
	client net.LatencyCheckClient
	conn   *grpc.ClientConn
	err    error
}

func CreateOrchNode(uri *url.URL, router_uri *url.URL) OrchNode {
	return OrchNode{uri: *uri, routerUri: *router_uri, orchInfo: make(map[ethcommon.Address]net.OrchestratorInfo)}
}

func NewLatencyRouter(orch_nodes []OrchNode, test_broadcaster_ip string, cache_time time.Duration, search_timeout time.Duration, ping_broadcaster_timeout time.Duration, round_robin bool, geo_routing bool, override_transcoder string, override_ppu int64) *LatencyRouter {
	router := &LatencyRouter{
		orchNodes:              make(map[url.URL]OrchNode),
		testBroadcasterIP:      test_broadcaster_ip,
		cacheTime:              cache_time,
		searchTimeout:          search_timeout,
		pingBroadcasterTimeout: ping_broadcaster_timeout,
		roundRobin:             round_robin,
		geoRouting:             geo_routing,
		overrideTranscoder:     override_transcoder,
		overridePricePerUnit:   override_ppu,
		closestOrchestratorToB: make(map[string]LatencyCheckResponse),
		clients:                make(map[url.URL]*LatencyCheckClient),
		broadcasterReqeuests:   make(map[ethcommon.Address]*net.OrchestratorRequest),
	}

	for _, orch_node := range orch_nodes {
		router.orchNodes[orch_node.uri] = orch_node
	}

	return router

}

func (r *LatencyRouter) EndTranscodingSession(ctx context.Context, request *net.EndTranscodingSessionRequest) (*net.EndTranscodingSessionResponse, error) {
	// shouldn't ever be called on Router
	glog.Errorf("EndTranscodingSession called, this should never happen...")
	return &net.EndTranscodingSessionResponse{}, nil
}

func (r *LatencyRouter) Ping(ctx context.Context, req *net.PingPong) (*net.PingPong, error) {
	return &net.PingPong{Value: []byte{}}, nil
}

func (r *LatencyRouter) Start(uri *url.URL, serviceURI *url.URL, dataPort string, workDir string, secret string, backgroundUpdate bool, maxConcurrentUpdates int) error {
	r.workDir = workDir
	r.secret = secret
	glog.Infof("using working directory " + r.workDir)
	if r.geoRouting == true {
		_, err := os.Stat(r.workDir + "/GeoIP2-City.mmdb")
		if err != nil {
			glog.Infof("could not stat mmdb file")
			r.geoRouting = false
		} else {
			glog.Infof("using geo routing")
		}
	}

	listener, err := gonet.Listen("tcp", uri.Host)
	if err != nil {
		return err
	}
	defer listener.Close()

	certFile, keyFile, err := getCert(serviceURI, workDir)
	if err != nil {
		return err
	}

	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return err
	}

	s := grpc.NewServer(grpc.Creds(creds))
	r.srv = s

	net.RegisterLatencyCheckServer(s, r)
	net.RegisterOrchestratorServer(s, r) //to use the GetOrchestrator rpc client
	//grpc_health_v1.RegisterHealthServer(s, health.NewServer()) //TODO: why is this included?

	errCh := make(chan error)
	go func() {
		errCh <- s.Serve(listener)
	}()

	go func() {
		dataURI := &url.URL{
			Scheme: serviceURI.Scheme,
			Host:   serviceURI.Hostname() + ":" + dataPort,
			Path:   serviceURI.Path,
		}
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12, // Minimum TLS version supported
			// You can further configure TLS settings here if needed
		}

		srv := &http.Server{Addr: dataURI.Host, TLSConfig: tlsConfig}
		mux := http.DefaultServeMux
		http.DefaultServeMux = http.NewServeMux()

		mux.Handle("/routingdata", r.basicAuth(r.handleGetRoutingData))

		mux.Handle("/grafana", r.basicAuth(r.handleGrafanaData))

		mux.Handle("/updateroutingdata", r.basicAuth(r.handleUpdateRoutingData))

		mux.Handle("/api/health", r.basicAuth(r.handleHealth))

		srv.Handler = mux

		glog.Infof("started router data server at %v", dataURI)
		err = srv.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			glog.Fatal(err)
		}

	}()

	time.Sleep(1 * time.Second)

	if err := checkAvailability(context.Background(), serviceURI); err != nil {
		s.Stop()
		return err
	}

	glog.Infof("started router server at %v", uri)

	glog.Infof("loading routing")
	r.LoadRouting()

	glog.Infof("starting router clients")
	r.CreateClients()
	go func() {
		//check if clients are connected every minute, try to connect if not connected
		r.MonitorClients()
	}()

	if backgroundUpdate {

		go func() {
			//check if ping should be updated
			r.MonitorBroadcasters(maxConcurrentUpdates)
		}()
	}

	glog.Infof("starting background process to cache OrchestratorInfo requests for each broadcaster")
	go func() {
		r.CacheOrchestratorInfo()
	}()

	//if test ips are specified, start testing
	if r.testBroadcasterIP != "" {
		for _, broadcaster_ip := range strings.Split(r.testBroadcasterIP, ",") {
			broadcaster_ip = strings.TrimSpace(broadcaster_ip)
			net_addr, err := gonet.ResolveTCPAddr("tcp", broadcaster_ip+":43674")
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
				defer cancel()
				t_ctx := peer.NewContext(ctx, &peer.Peer{Addr: net_addr, AuthInfo: nil})
				r.GetOrchestrator(t_ctx, &net.OrchestratorRequest{})
				<-ctx.Done()
				glog.Infof("test ping completed for ip %s", broadcaster_ip)
			} else {
				glog.Errorf("error testing broadcaster ip: %q", err)
			}
		}
	}

	return <-errCh
}

func (r *LatencyRouter) Stop() {
	for _, client_conn := range r.clients {
		if client_conn.err == nil {
			client_conn.conn.Close()
		}
	}

	r.srv.Stop()
}

func (r *LatencyRouter) SaveRouting() {
	//save routing to load on startup
	file, _ := json.MarshalIndent(r.closestOrchestratorToB, "", " ")
	r.SaveRoutingJson(file)

	return
}

func (r *LatencyRouter) SaveRoutingJson(json_data []byte) {
	r.bmu.Lock()
	defer r.bmu.Unlock()
	err := ioutil.WriteFile(filepath.Join(r.workDir, "routing.json"), json_data, 0644)
	if err != nil {
		glog.Errorf("error saving routing: %v", err.Error())
	}
}

func (r *LatencyRouter) LoadRouting() {
	r.bmu.Lock()
	defer r.bmu.Unlock()
	routing_file := filepath.Join(r.workDir, "routing.json")
	if _, err := os.Stat(routing_file); err == nil {
		json_file, err := ioutil.ReadFile(routing_file)
		if err != nil {
			glog.Errorf("error reading routing file: %v", err.Error())
		}

		err = json.Unmarshal([]byte(json_file), &r.closestOrchestratorToB)
		if err != nil {
			glog.Errorf("error loading routing: %v", err.Error())
		}
	} else {
		glog.Errorf("no routing file exists")
	}
}

func (r *LatencyRouter) GetOrchestrator(ctx context.Context, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	st := time.Now()
	//get broadcaster addr
	client_addr, ok := peer.FromContext(ctx)
	if ok {
		glog.Infof("%v  GetOrchestrator request received", client_addr.Addr.String())
	} else {
		return nil, errNoClientIp
	}

	//verify GetOrchestrator request
	b_addr := ethcommon.BytesToAddress(req.GetAddress())
	if r.VerifySig(b_addr.Hex(), req.GetSig()) == false {
		glog.Infof("%v  GetOrchestrator request failed verification", client_addr.Addr.String())
		return nil, errors.New("GetOrchestrator request failed verification")
	}

	//capture request for background refresh of OrchestratorInfo
	r.SaveBroadcasterRequest(b_addr, req)

	//get the closest orchestrator
	orch_info, err := r.getOrchestratorInfoClosestToB(context.Background(), req, client_addr.Addr.String())
	if orch_info != nil && err == nil {
		glog.Infof("%v  sending closest orchestrator info in %s  addr %v", client_addr.Addr.String(), time.Since(st), b_addr.Hex())
		return orch_info, nil
	} else {
		if errors.Is(err, context.Canceled) == false {
			glog.Errorf("%v  failed to return orchestrator info: %v", client_addr.Addr.String(), err.Error())
			glog.Errorf("%v  failed to get orch info for request  time: %v   ctx err: %v   addr %v", client_addr.Addr.String(), time.Since(st), ctx.Err(), b_addr.Hex())
		} else {
			glog.Errorf("%v  request canceled by Broadcaster", client_addr.Addr.String())
		}

		return nil, err
	}
}

// include GetOrchestrator request verification similar to rpc.go for orchestrator
func (r *LatencyRouter) VerifySig(msg string, sig []byte) bool {
	return lpcrypto.VerifySig(ethcommon.HexToAddress(msg), crypto.Keccak256([]byte(msg)), sig)
}

func (r *LatencyRouter) getOrchestratorInfoClosestToB(ctx context.Context, req *net.OrchestratorRequest, client_addr string) (*net.OrchestratorInfo, error) {
	if len(r.orchNodes) == 0 {
		return nil, errNoOrchestrators
	}

	client_ip, client_port, ip_err := gonet.SplitHostPort(client_addr)
	if ip_err != nil {
		glog.Errorf("%v  error parsing peer address: %q", client_addr, ip_err)
		return nil, ip_err
	}

	client_info := ClientInfo{addr: client_addr, ip: client_ip, port: client_port}

	//check if in cache period
	//skip if no cached orchestrator, cacheTime not set or the OrchestratorRequest is nil
	cachedOrchResp, err := r.GetCachedClosestOrchestrator(client_info)
	if err == nil && r.cacheTime > time.Second && req != nil {
		time_since_cached := time.Now().Sub(cachedOrchResp.UpdatedAt)
		if time_since_cached < r.cacheTime || cachedOrchResp.DoNotUpdate {
			//check if orch info cached (updates every minute in MonitorBroadcasters
			cached_info, err := r.GetCachedOrchInfoForB(ethcommon.BytesToAddress(req.GetAddress()), cachedOrchResp.OrchUri)
			if err != nil {
				//get fresh OrchestratorInfo
				cached_info, err = r.GetOrchestratorInfo(ctx, client_info, req, cachedOrchResp.OrchUri)
			}
			if err == nil {
				expires_in := (cached_info.AuthToken.Expiration - time.Now().Unix()) / 60 
				glog.Infof("%v  returning orchestrator cached %s ago  orch addr: %v priceperunit: %v ticket_params_expiration: %v minutes", client_addr, time_since_cached.Round(time.Second), cached_info.GetTranscoder(), cached_info.PriceInfo.GetPricePerUnit(), expires_in)
				return cached_info, nil
			}
		}
	}

	if r.geoRouting {
		return r.GetClosestOrchByDistance(ctx, req, client_info)
	} else {
		return r.GetClosestOrchByPing(ctx, req, client_info)
	}

}

func (r *LatencyRouter) GetCachedOrchInfoForB(b_addr ethcommon.Address, orch_url url.URL) (*net.OrchestratorInfo, error) {
	if _, ok := r.orchNodes[orch_url]; ok {
		if _, ok := r.orchNodes[orch_url].orchInfo[b_addr]; ok {
			cached_info := r.orchNodes[orch_url].orchInfo[b_addr]
			glog.Infof("sending cached orchestrator info")
			return &cached_info, nil
		}
	}

	return nil, errNoOrchestrators
}

func (r *LatencyRouter) SaveBroadcasterRequest(b_addr ethcommon.Address, req *net.OrchestratorRequest) {
	r.bmu.Lock()
	defer r.bmu.Unlock()

	r.broadcasterReqeuests[b_addr] = req
}

func (r *LatencyRouter) GetCachedClosestOrchestrator(b_ip_addr ClientInfo) (LatencyCheckResponse, error) {
	r.bmu.RLock()
	defer r.bmu.RUnlock()
	closestOrchWithResp, client_ip_exists := r.closestOrchestratorToB[b_ip_addr.ip]
	if client_ip_exists {
		if r.geoRouting {
			if closestOrchWithResp.Distance == 0 {
				return LatencyCheckResponse{}, errNoOrchestrators
			}
		}

		return closestOrchWithResp, nil
	} else {
		return LatencyCheckResponse{}, errNoOrchestrators
	}
}

func (r *LatencyRouter) SetClosestOrchestrator(b_ip_addr ClientInfo, resp *LatencyCheckResponse) {
	if resp.RespTime == 0 && resp.Distance == 0 {
		resp.DoNotUpdate = true
	}

	r.bmu.Lock()
	defer r.bmu.Unlock()
	//cache the fastest O to the B
	r.closestOrchestratorToB[b_ip_addr.ip] = *resp
}

func (r *LatencyRouter) PingOrchestrator(ctx context.Context, broadcaster_ip string, orch_uri url.URL) (bool, error) {
	client, conn, err := startOrchestratorClient(ctx, &orch_uri)
	if err != nil {
		glog.Errorf("%v  could not connect to Orchestrator %v  err: %s", broadcaster_ip, orch_uri.String(), err.Error())
		return false, err
	}
	defer conn.Close()

	cctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	_, ping_err := client.Ping(cctx, &net.PingPong{Value: []byte("are you there")})
	if ping_err == nil {
		return true, nil
	} else {
		return false, ping_err
	}
}

func (r *LatencyRouter) GetOrchestratorInfo(ctx context.Context, client_info ClientInfo, req *net.OrchestratorRequest, orch_uri url.URL) (*net.OrchestratorInfo, error) {
	client, conn, err := startOrchestratorClient(ctx, &orch_uri)
	if err != nil {
		glog.Errorf("%v  could not connect to Orchestrator %v  err: %s", client_info.addr, orch_uri.String(), err.Error())
		return nil, err
	}
	defer conn.Close()

	cctx, cancel := context.WithTimeout(ctx, getOrchestratorTimeout)
	defer cancel()

	info, err := client.GetOrchestrator(cctx, req)
	if err != nil {
		glog.Errorf("%v  could not get OrchestratorInfo from %v, err: %s, addr: %s", client_info.addr, orch_uri.String(), err.Error(), ethcommon.BytesToAddress(req.Address).Hex())
		return nil, err
	}

	return info, nil
}

func (r *LatencyRouter) MonitorBroadcasters(maxConcurrentUpdates int) {
	var wg sync.WaitGroup
	funcs := 0
	glog.Infof("background process started to monitor and update routing to broadcasters, first check is in 1 minute")
	for {
		time.Sleep(1 * time.Minute)
		for broadcaster_ip, lat_chk_resp := range r.closestOrchestratorToB {
			if lat_chk_resp.DoNotUpdate == false && lat_chk_resp.UpdatedAt.Add(r.cacheTime).Before(time.Now().Add(10*time.Minute)) {
				glog.Infof("%v  updating routing for broadcaster ip that will expire in %s", broadcaster_ip, lat_chk_resp.UpdatedAt.Add(r.cacheTime).Sub(time.Now()))
				funcs++
				wg.Add(1)
				go func(b_addr string) {
					r.getOrchestratorInfoClosestToB(context.Background(), nil, b_addr+":80") //add port so can parse ip addr
					wg.Done()
				}(broadcaster_ip)
				if funcs >= maxConcurrentUpdates {
					wg.Wait()
					funcs = 0
				}

			}
		}
	}
}

func (r *LatencyRouter) CacheOrchestratorInfo() {
	glog.Infof("background process started to cache orchestrator info every minute")
	for {
		for b_addr, b_req := range r.broadcasterReqeuests {
			for orch_url, _ := range r.orchNodes {
				orch_info, err := r.GetOrchestratorInfo(context.Background(), ClientInfo{addr: "background update"}, b_req, orch_url)
				if err == nil {
					if r.overridePricePerUnit > -1 {
						orch_info.PriceInfo.PricePerUnit = r.overridePricePerUnit
						if r.overridePricePerUnit == 0 {
							orch_info.TicketParams.FaceValue = big.NewInt(0).Bytes()
						}
					}
					if r.overrideTranscoder != "" {
						orch_info.Transcoder = r.overrideTranscoder
					}

					r.bmu.Lock()
					r.orchNodes[orch_url].orchInfo[b_addr] = *orch_info
					r.bmu.Unlock()
				} else {
					r.bmu.Lock()
					delete(r.orchNodes[orch_url].orchInfo, b_addr)
					r.bmu.Unlock()
				}
			}
		}

		time.Sleep(1 * time.Minute)
	}
}

// grpc methods
func (r *LatencyRouter) GetLatency(ctx context.Context, req *net.LatencyCheckReq) (*net.LatencyCheckRes, error) {
	glog.Infof("%v  received latency check request, pinging to provide latency", req.GetUri())

	pingTime := r.SendPing(ctx, req.GetUri())
	glog.Infof("%v  latency check ping completed in %v microseconds", req.GetUri(), pingTime)
	return &net.LatencyCheckRes{RespTime: pingTime}, nil
}

func (r *LatencyRouter) UpdateRouter(ctx context.Context, req *net.ClosestOrchestratorReq) (*net.ClosestOrchestratorRes, error) {

	b_uri := req.GetBroadcasterUri()
	o_uri, _ := url.Parse(req.GetOrchestratorUri())
	respTime := req.GetRespTime()
	distance := req.GetDistance()

	r.SetClosestOrchestrator(ClientInfo{ip: b_uri}, &LatencyCheckResponse{RespTime: respTime, Distance: distance, OrchUri: *o_uri, UpdatedAt: time.Now(), DoNotUpdate: false})
	glog.Infof("%v  router updated to provide orchestrator %v", b_uri, o_uri)
	return &net.ClosestOrchestratorRes{Updated: true}, nil
}

func (r *LatencyRouter) GetClosestOrchByDistance(ctx context.Context, req *net.OrchestratorRequest, client_info ClientInfo) (*net.OrchestratorInfo, error) {
	var responses []LatencyCheckResponse
	for _, orch_node := range r.orchNodes {
		distance := r.GetDistanceFromB(ctx, orch_node.uri.Hostname(), client_info.ip)
		resp := LatencyCheckResponse{Distance: distance, OrchUri: orch_node.uri, UpdatedAt: time.Now(), DoNotUpdate: false, UseGeoDistance: true}
		glog.Infof("%v   distance to B for orch %v is %v", client_info.addr, orch_node.uri.String(), distance)
		responses = append(responses, resp)
	}

	return r.SendOrchInfo(ctx, client_info, req, responses)
}

func (r *LatencyRouter) GetClosestOrchByPing(ctx context.Context, req *net.OrchestratorRequest, client_info ClientInfo) (*net.OrchestratorInfo, error) {
	//send request to each orch node in separate go routine to concurrently process
	// results come into latencyCh and errors come in errCh
	totOrch := len(r.orchNodes)
	latencyCh := make(chan LatencyCheckResponse, totOrch)
	errCh := make(chan error, totOrch)
	totCh := make(chan int, totOrch)

	errCtr := 0
	respCtr := 0
	totCtr := 0
	var responses []LatencyCheckResponse
	lctx, cancel := context.WithTimeout(ctx, r.searchTimeout)
	defer cancel()

	for _, orch_node := range r.orchNodes {
		go func(b_info ClientInfo, orch_node OrchNode) {
			client := r.GetClient(orch_node.uri)
			if client == nil {
				glog.Infof("%v  rpc client is not connected for %v", b_info.addr, orch_node.routerUri.String())
				errCh <- errors.New("client not available for " + orch_node.uri.String())
				return
			}
			glog.Infof("%v  sending latency check request to orch at %v", b_info.addr, orch_node.routerUri.String())
			latencyCheckRes, err := client.GetLatency(ctx, &net.LatencyCheckReq{Uri: b_info.addr})
			if err != nil {
				errCh <- fmt.Errorf("%v err=%q", orch_node.routerUri, err)
				return
			}

			select {
			case latencyCh <- LatencyCheckResponse{RespTime: latencyCheckRes.GetRespTime() / 1000, OrchUri: orch_node.uri, UpdatedAt: time.Now(), DoNotUpdate: false, UseGeoDistance: false}:
			default:
			}
		}(client_info, orch_node)
	}

	for {
		select {
		case latencyCheckResp := <-latencyCh:
			responses = append(responses, latencyCheckResp)
			respCtr++

			glog.Infof("%v  received latency check from %v with ping time of %vms", client_info.addr, latencyCheckResp.OrchUri.String(), latencyCheckResp.RespTime)
			//if want to early return, do it here
			if !r.roundRobin {
				return r.SendOrchInfo(ctx, client_info, req, responses)
			}
			//update tracking total responses
			totCh <- 1
		case err := <-errCh:
			glog.Error(err.Error())
			errCtr++
			//update tracking total responses
			totCh <- 1
		case <-totCh:
			totCtr++
			if totCtr >= len(r.orchNodes) && r.roundRobin {
				return r.SendOrchInfo(ctx, client_info, req, responses)
			}
		case <-lctx.Done():
			//when the context time limit is complete, return the closest orchestrator so far
			glog.Infof("%v  searchTimeout expired, sending OrchestratorInfo for responses received (%v of %v)", client_info.addr, totCtr, len(r.orchNodes))
			return r.SendOrchInfo(ctx, client_info, req, responses)
		}
	}

}

func (r *LatencyRouter) SendOrchInfo(ctx context.Context, client_info ClientInfo, req *net.OrchestratorRequest, responses []LatencyCheckResponse) (*net.OrchestratorInfo, error) {
	//sort the responses based on ping time
	if r.geoRouting {
		sort.Slice(responses, func(i, j int) bool {
			return responses[i].Distance < responses[j].Distance
		})
	} else {
		sort.Slice(responses, func(i, j int) bool {
			return responses[i].RespTime < responses[j].RespTime
		})
	}
	//if req is nil we are updating ping tests only, no OrchestratorInfo to send back
	if req != nil {
		b_addr := ethcommon.BytesToAddress(req.GetAddress())
		//get orchestrator info for fastest resp time O
		for _, resp := range responses {
			info, err := r.GetCachedOrchInfoForB(b_addr, resp.OrchUri)
			//get the orch info from O if not cached
			if err != nil {
				info, err = r.GetOrchestratorInfo(ctx, client_info, req, resp.OrchUri)
			}

			if err == nil {
				//cache it
				r.SetClosestOrchestrator(client_info, &resp)
				//update the other routers
				go r.updateRouters(client_info.ip, &resp)
				glog.Infof("%v  sending orchestrator info for %v  orch addr: %v  priceperunit: %v", client_info.addr, resp.OrchUri.String(), info.GetTranscoder(), info.PriceInfo.GetPricePerUnit())

				return info, err
			} else {
				glog.Infof("%v  orchestrator failed response for %v  error: %v", client_info.addr, resp.OrchUri.String(), err.Error())
			}
		}

		//none of the Os returns a GetOrchestrator response, return no orchestrators error
		//   when the router cannot connect to an O the searchTimeout will flush to here and return no orchestrators
		//   connections to O waits 3 seconds.
		return nil, errNoOrchestrators
	} else {
		for idx, _ := range responses {
			glog.Infof("%v  pinging Orchestrator to confirm status", client_info.addr)
			//rpc ping orch to check its up
			ping_test, err := r.PingOrchestrator(ctx, client_info.addr, responses[idx].OrchUri)
			if err != nil {
				glog.Infof("%v  rpc ping failed %v", client_info.addr, err.Error())
			}
			if ping_test {
				//cache it
				r.SetClosestOrchestrator(client_info, &responses[idx])
				//update the other routers
				go r.updateRouters(client_info.ip, &responses[idx])
				//return nil because we are just updating ping tests
				glog.Infof("%v  updated cached orchestrator to %v", client_info.addr, responses[idx].OrchUri.String())
				return nil, nil
			}

		}
		//if rpc ping does not work on any of the Orch nodes update latency check response so background process does not run again
		r.SetClosestOrchestrator(client_info, &responses[0])
		return nil, nil
	}
}

func (r *LatencyRouter) SendPing(ctx context.Context, b_ip_addr string) float64 {

	addr_split := strings.Split(b_ip_addr, ":")

	pinger, err := probing.NewPinger(addr_split[0])
	if err != nil {
		panic(err)
	}
	pinger.Interval = 1 * time.Millisecond
	pinger.Timeout = r.pingBroadcasterTimeout
	pinger.Count = 2
	pinger.SetPrivileged(true)

	p_err := pinger.Run() // Blocks until finished.
	if p_err != nil {
		panic(p_err)
	}
	stats := pinger.Statistics() // get send/receive/duplicate/rtt stats
	glog.Infof("%v  ping test results:  %vms  %v%% packet loss", b_ip_addr, stats.AvgRtt.Microseconds(), stats.PacketLoss)
	if stats.PacketLoss != 100 {
		return float64(stats.AvgRtt.Microseconds())
	} else {
		start := time.Now()
		took := int64(0)
		client := http.Client{
			Timeout: r.pingBroadcasterTimeout,
		}
		resp, err := client.Head("https://" + addr_split[0] + ":8935")
		if err != nil {
			took = time.Since(start).Milliseconds()
		} else {
			defer resp.Body.Close()
			took = time.Since(start).Milliseconds()
		}
		glog.Infof("%v  icmp ping failed, using backup ping test results:  %vms  error: %s", b_ip_addr, took, err.Error())
		return float64(time.Since(start).Microseconds())
	}
}

func (r *LatencyRouter) GetDistanceFromB(ctx context.Context, orch_ip string, b_ip string) float64 {
	db, err := geoip2.Open(r.workDir + "/GeoIP2-City.mmdb")
	if err != nil {
		glog.Fatal(err)
	}
	defer db.Close()

	o_ip := gonet.ParseIP(orch_ip)
	o_record, err := db.City(o_ip)
	if err != nil {
		glog.Infof("%v  failed to parse orch ip for geo distance calc  %v", b_ip, orch_ip)
		glog.Error(err)
	}
	b_record, err := db.City(gonet.ParseIP(b_ip))
	if err != nil {
		glog.Infof("%v  failed to parse B ip for geo distance calc  %v", b_ip, orch_ip)
		glog.Error(err)
	}

	// Convert degrees to radians
	lat1Rad := o_record.Location.Latitude * (math.Pi / 180)
	lon1Rad := o_record.Location.Longitude * (math.Pi / 180)
	lat2Rad := b_record.Location.Latitude * (math.Pi / 180)
	lon2Rad := b_record.Location.Longitude * (math.Pi / 180)

	// Radius of the Earth in kilometers
	radius := 6371.0

	// Haversine formula
	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad
	a := math.Pow(math.Sin(deltaLat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(deltaLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := math.Max(radius*c, 1.0)

	return distance
}

func (r *LatencyRouter) updateRouters(b_ip_addr string, resp *LatencyCheckResponse) {
	glog.Infof("%v  Updating routers", b_ip_addr)
	//do not update if RespTime and Distance is 0 because the routing failed
	if resp.RespTime == 0 && resp.Distance == 0 {
		return
	}
	updateCh := make(chan RouterUpdated, len(r.orchNodes))
	errCh := make(chan error, len(r.orchNodes))

	upd_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, orch_node := range r.orchNodes {
		go func(orch_node OrchNode, b_ip_addr string, resp *LatencyCheckResponse) {
			client := r.GetClient(orch_node.uri)
			if client != nil {
				updateRouterRes, err := client.UpdateRouter(upd_ctx, &net.ClosestOrchestratorReq{BroadcasterUri: b_ip_addr, OrchestratorUri: resp.OrchUri.String(), RespTime: resp.RespTime, Distance: resp.Distance})
				if err != nil {
					glog.Errorf("%v  failed to update router %v", b_ip_addr, orch_node.routerUri.String())
				}
				select {
				case updateCh <- RouterUpdated{b_ip_addr: b_ip_addr, orch_router_ip_addr: orch_node.routerUri.String(), updated: updateRouterRes.GetUpdated()}:
				default:
					glog.Error("no response received from updating router")
				}
			} else {
				glog.Errorf("failed to start router client for %v", orch_node.routerUri.String())
			}
		}(orch_node, b_ip_addr, resp)
	}

	errCtr := 0
	updateCtr := 0
	for {
		select {
		case routerUpdated := <-updateCh:
			updateCtr++
			glog.Infof("Router updated b_addr=%s status=%v (%v of %v, %v errors)", b_ip_addr, routerUpdated.updated, updateCtr, len(r.orchNodes), errCtr)
		case err := <-errCh:
			errCtr++
			glog.Infof("Failed router update b_addr=%s status=%v (%v of %v, %v errors)", b_ip_addr, false, updateCtr, len(r.orchNodes), errCtr)
			glog.Error(err)
		case <-upd_ctx.Done():
			glog.Infof("Updating routers completed")
			return
		}
	}
}

// TODO: add checks that connection is open
func (r *LatencyRouter) MonitorClients() {
	for {
		//sleep for 5 miinutes and check again
		time.Sleep(1 * time.Minute)
		for _, orch_node := range r.orchNodes {
			if r.clients[orch_node.uri].err != nil {
				client, conn, err := startLatencyRouterClient(context.Background(), orch_node.routerUri)
				r.SetClient(orch_node.uri, LatencyCheckClient{client: client, conn: conn, err: err})
			}
		}
		//save routing json for backup
		r.SaveRouting()
	}
}

func (r *LatencyRouter) CreateClients() {
	for _, orch_node := range r.orchNodes {
		client, conn, err := startLatencyRouterClient(context.Background(), orch_node.routerUri)
		r.SetClient(orch_node.uri, LatencyCheckClient{client: client, conn: conn, err: err})
	}
}

func (r *LatencyRouter) GetClient(orch_uri url.URL) net.LatencyCheckClient {
	r.cmu.RLock()
	defer r.cmu.RUnlock()

	orch_client, orch_client_exists := r.clients[orch_uri]
	if orch_client_exists {
		if orch_client.client != nil {
			return orch_client.client
		} else {
			//check node exists and create the client
			orch_node, orch_exists := r.orchNodes[orch_uri]
			if orch_exists {
				client, conn, err := startLatencyRouterClient(context.Background(), orch_node.routerUri)
				r.SetClient(orch_uri, LatencyCheckClient{client: client, conn: conn, err: err})
				return client
			} else {
				return nil
			}
		}
	} else {
		return nil
	}
}

func (r *LatencyRouter) SetClient(orch_uri url.URL, client LatencyCheckClient) {
	r.cmu.Lock()
	defer r.cmu.Unlock()
	r.clients[orch_uri] = &client
}

func startLatencyRouterClient(ctx context.Context, router_uri url.URL) (net.LatencyCheckClient, *grpc.ClientConn, error) {
	glog.Infof("Connecting RPC to uri=%v", router_uri.String())
	conn, err := grpc.Dial(router_uri.Host,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithTimeout(GRPCConnectTimeout))
	if err != nil {
		glog.Errorf("Did not connect to orch=%v error=%v", router_uri.String(), err.Error())
		return nil, nil, err

	}
	c := net.NewLatencyCheckClient(conn)

	return c, conn, nil
}

// update and reporting endpoints
// ===========================================================================================
func (r *LatencyRouter) handleGetRoutingData(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=routing.json")
	w.Header().Set("Content-Type", "application/json")

	glog.Info("received routing data request")
	if reader, err := os.Open(filepath.Join(r.workDir, "routing.json")); err == nil {
		defer reader.Close()
		io.Copy(w, reader)
	} else {
		respond500(w, "file not available")
	}
}

func (r *LatencyRouter) handleGrafanaData(w http.ResponseWriter, req *http.Request) {

	glog.Info("received grafana data request")
	w.Header().Set("Content-Type", "application/json")
	type GrafanaData struct {
		Broadcaster  string
		Orchestrator string
		RespTime     float64
		Distance     float64
		UpdatedAt    time.Time
		DoNotUpdate  bool
	}
	grafana_data := []GrafanaData{}
	for b_ip, v := range r.closestOrchestratorToB {
		grafana_data = append(grafana_data, GrafanaData{Broadcaster: b_ip, Orchestrator: v.OrchUri.Host, RespTime: v.RespTime, Distance: v.Distance, UpdatedAt: v.UpdatedAt, DoNotUpdate: v.DoNotUpdate})
	}
	// Create a data object to unmarshal JSON into
	data, err := json.Marshal(grafana_data)

	if err != nil {
		glog.Errorf("Error marshaling JSON: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	_, err = w.Write(data)
	if err != nil {
		glog.Errorf("Error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (r *LatencyRouter) handleUpdateRoutingData(w http.ResponseWriter, req *http.Request) {
	glog.Info("received update routing data request")
	// Check if the request method is POST
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Parse the incoming file from the request
	file, header, err := req.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	glog.Infof("received json data %v : %v", header.Filename, header.Size)
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		respond400(w, "could not read json file")
	}

	//save routing to load on startup
	//json_data, _ := json.MarshalIndent(buf, "", " ")
	r.SaveRoutingJson(buf.Bytes())
	r.LoadRouting()
	glog.Infof("routing data saved from webserver")
	respondOk(w, nil)
}

func (r *LatencyRouter) handleHealth(w http.ResponseWriter, req *http.Request) {
	glog.Info("received health check request")
	w.WriteHeader(200)
}

func (r *LatencyRouter) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		username, password, ok := req.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte("routeradmin"))
			expectedPasswordHash := sha256.Sum256([]byte(r.secret))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, req)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
