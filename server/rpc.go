package server

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

const GRPCConnectTimeout = 3 * time.Second
const GRPCTimeout = 8 * time.Second
const HTTPTimeout = 8 * time.Second

type Orchestrator interface {
	ServiceURI() *url.URL
	Address() ethcommon.Address
	TranscoderSecret() string
	Sign([]byte) ([]byte, error)
	VerifySig(ethcommon.Address, string, []byte) bool
	CurrentBlock() *big.Int
	CheckCapacity(core.ManifestID) error
	TranscodeSeg(*core.SegTranscodingMetadata, *stream.HLSSegment) (*core.TranscodeResult, error)
	ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer)
	TranscoderResults(job int64, res *core.RemoteTranscoderResult)
	ProcessPayment(payment net.Payment, manifestID core.ManifestID) error
	TicketParams(sender ethcommon.Address) *net.TicketParams
}

type Broadcaster interface {
	Address() ethcommon.Address
	Sign([]byte) ([]byte, error)
}

// Session-specific state for broadcasters
type BroadcastSession struct {
	Broadcaster      Broadcaster
	ManifestID       core.ManifestID
	Profiles         []ffmpeg.VideoProfile
	OrchestratorInfo *net.OrchestratorInfo
	OrchestratorOS   drivers.OSSession
	BroadcasterOS    drivers.OSSession
	Sender           pm.Sender
	PMSessionID      string
}

type lphttp struct {
	orchestrator Orchestrator
	orchRpc      *grpc.Server
	transRpc     *http.ServeMux
}

// grpc methods
func (h *lphttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if r.ProtoMajor == 2 && strings.HasPrefix(ct, "application/grpc") {
		h.orchRpc.ServeHTTP(w, r)
	} else {
		h.transRpc.ServeHTTP(w, r)
	}
}

func (h *lphttp) GetOrchestrator(context context.Context, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	return getOrchestrator(h.orchestrator, req)
}

func (h *lphttp) Ping(context context.Context, req *net.PingPong) (*net.PingPong, error) {
	return ping(context, req, h.orchestrator)
}

// XXX do something about the implicit start of the http mux? this smells
func StartTranscodeServer(orch Orchestrator, bind string, mux *http.ServeMux, workDir string) {
	s := grpc.NewServer()
	lp := lphttp{
		orchestrator: orch,
		orchRpc:      s,
		transRpc:     mux,
	}
	net.RegisterOrchestratorServer(s, &lp)
	net.RegisterTranscoderServer(s, &lp)
	lp.transRpc.HandleFunc("/segment", lp.ServeSegment)
	lp.transRpc.HandleFunc("/transcodeResults", lp.TranscodeResults)

	cert, key, err := getCert(orch.ServiceURI(), workDir)
	if err != nil {
		return // XXX return error
	}

	glog.Info("Listening for RPC on ", bind)
	srv := http.Server{
		Addr:    bind,
		Handler: &lp,
		// XXX doesn't handle streaming RPC well; split remote transcoder RPC?
		//ReadTimeout:  HTTPTimeout,
		//WriteTimeout: HTTPTimeout,
	}
	srv.ListenAndServeTLS(cert, key)
}

// The broadcaster calls CheckOrchestratorAvailability which invokes Ping on the orchestrator
func CheckOrchestratorAvailability(orch Orchestrator) bool {
	ts := time.Now()
	ts_signature, err := orch.Sign([]byte(fmt.Sprintf("%v", ts)))
	if err != nil {
		return false
	}

	ping := crypto.Keccak256(ts_signature)

	orch_client, conn, err := startOrchestratorClient(orch.ServiceURI())
	if err != nil {
		return false
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	pong, err := orch_client.Ping(ctx, &net.PingPong{Value: ping})
	if err != nil {
		glog.Error("Was not able to submit Ping: ", err)
		return false
	}

	return orch.VerifySig(orch.Address(), string(ping), pong.Value)
}

func ping(context context.Context, req *net.PingPong, orch Orchestrator) (*net.PingPong, error) {
	glog.Info("Received Ping request")
	value, err := orch.Sign(req.Value)
	if err != nil {
		glog.Error("Unable to sign Ping request")
		return nil, err
	}
	return &net.PingPong{Value: value}, nil
}

// The broadcaster calls GetOrchestratorInfo which invokes GetOrchestrator on the orchestrator
func GetOrchestratorInfo(ctx context.Context, bcast Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
	c, conn, err := startOrchestratorClient(orchestratorServer)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req, err := genOrchestratorReq(bcast)
	r, err := c.GetOrchestrator(ctx, req)
	if err != nil {
		glog.Errorf("Could not get orchestrator %v: %v", orchestratorServer, err)
		return nil, errors.New("Could not get orchestrator: " + err.Error())
	}

	return r, nil
}

func startOrchestratorClient(uri *url.URL) (net.OrchestratorClient, *grpc.ClientConn, error) {
	glog.Infof("Connecting RPC to %v", uri)
	conn, err := grpc.Dial(uri.Host,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithTimeout(GRPCConnectTimeout))
	if err != nil {
		glog.Error("Did not connect: ", err)
		return nil, nil, errors.New("Did not connect: " + err.Error())
	}
	c := net.NewOrchestratorClient(conn)

	return c, conn, nil
}

func genOrchestratorReq(b Broadcaster) (*net.OrchestratorRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", b.Address().Hex())))
	if err != nil {
		return nil, err
	}
	return &net.OrchestratorRequest{Address: b.Address().Bytes(), Sig: sig}, nil
}

func getOrchestrator(orch Orchestrator, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	addr := ethcommon.BytesToAddress(req.Address)
	if err := verifyOrchestratorReq(orch, addr, req.Sig); err != nil {
		return nil, fmt.Errorf("Invalid orchestrator request (%v)", err)
	}

	tr := net.OrchestratorInfo{
		Transcoder:   orch.ServiceURI().String(), // currently,  orchestrator == transcoder
		TicketParams: orch.TicketParams(addr),
	}

	storagePrefix := core.RandomManifestID()
	os := drivers.NodeStorage.NewSession(string(storagePrefix))

	if os != nil && os.IsExternal() {
		tr.Storage = []*net.OSInfo{os.GetInfo()}
	}

	return &tr, nil
}

func verifyOrchestratorReq(orch Orchestrator, addr ethcommon.Address, sig []byte) error {
	if !orch.VerifySig(addr, addr.Hex(), sig) {
		glog.Error("orchestrator req sig check failed")
		return fmt.Errorf("orchestrator req sig check failed")
	}
	return orch.CheckCapacity("")
}
