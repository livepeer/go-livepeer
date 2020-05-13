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

	"github.com/livepeer/go-livepeer/common"
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

type Orchestrator interface {
	ServiceURI() *url.URL
	Address() ethcommon.Address
	TranscoderSecret() string
	Sign([]byte) ([]byte, error)
	VerifySig(ethcommon.Address, string, []byte) bool
	CurrentBlock() *big.Int
	CheckCapacity(core.ManifestID) error
	TranscodeSeg(*core.SegTranscodingMetadata, *stream.HLSSegment) (*core.TranscodeResult, error)
	ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int)
	TranscoderResults(job int64, res *core.RemoteTranscoderResult)
	ProcessPayment(payment net.Payment, manifestID core.ManifestID) error
	TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error)
	PriceInfo(sender ethcommon.Address) (*net.PriceInfo, error)
	SufficientBalance(addr ethcommon.Address, manifestID core.ManifestID) bool
	DebitFees(addr ethcommon.Address, manifestID core.ManifestID, price *net.PriceInfo, pixels int64)
}

// Balance describes methods for a session's balance maintenance
type Balance interface {
	Credit(amount *big.Rat)
	StageUpdate(minCredit *big.Rat, ev *big.Rat) (int, *big.Rat, *big.Rat)
}

// BalanceUpdateStatus indicates the current status of a balance update
type BalanceUpdateStatus int

const (
	// Staged indicates that the update has been created but the credit
	// has not been spent yet
	Staged = iota
	// CreditSpent indicates that the update's credit has been spent
	// but the debit has not been processed yet
	CreditSpent
	// ReceivedChange indicates that the update's credit has been spent
	// and a debit was processed such that there was "change" (net of credit/debit)
	ReceivedChange
)

// BalanceUpdate describes an update to be performed on the balance of a session
type BalanceUpdate struct {
	// ExistingCredit is the existing credit reserved for the update
	ExistingCredit *big.Rat

	// NewCredit is the new credit for the update provided by a payment
	NewCredit *big.Rat

	// NumTickets is the number of tickets in the payment for the update
	NumTickets int

	// Debit is the amount to debit for the update
	Debit *big.Rat

	// Status is the current status of the update
	Status BalanceUpdateStatus
}

// BroadcastSession - session-specific state for broadcasters
type BroadcastSession struct {
	Broadcaster      common.Broadcaster
	ManifestID       core.ManifestID
	Profiles         []ffmpeg.VideoProfile
	OrchestratorInfo *net.OrchestratorInfo
	OrchestratorOS   drivers.OSSession
	BroadcasterOS    drivers.OSSession
	Sender           pm.Sender
	PMSessionID      string
	Balance          Balance
	LatencyScore     float64
}

// ReceivedTranscodeResult contains received transcode result data and related metadata
type ReceivedTranscodeResult struct {
	*net.TranscodeData
	Info         *net.OrchestratorInfo
	LatencyScore float64
}

type lphttp struct {
	orchestrator Orchestrator
	orchRPC      *grpc.Server
	transRPC     *http.ServeMux
}

// grpc methods
func (h *lphttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if r.ProtoMajor == 2 && strings.HasPrefix(ct, "application/grpc") {
		h.orchRPC.ServeHTTP(w, r)
	} else {
		h.transRPC.ServeHTTP(w, r)
	}
}

func (h *lphttp) GetOrchestrator(context context.Context, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	return getOrchestrator(h.orchestrator, req)
}

func (h *lphttp) Ping(context context.Context, req *net.PingPong) (*net.PingPong, error) {
	return ping(context, req, h.orchestrator)
}

// XXX do something about the implicit start of the http mux? this smells
func StartTranscodeServer(orch Orchestrator, bind string, mux *http.ServeMux, workDir string, acceptRemoteTranscoders bool) {
	s := grpc.NewServer()
	lp := lphttp{
		orchestrator: orch,
		orchRPC:      s,
		transRPC:     mux,
	}
	net.RegisterOrchestratorServer(s, &lp)
	lp.transRPC.HandleFunc("/segment", lp.ServeSegment)
	if acceptRemoteTranscoders {
		net.RegisterTranscoderServer(s, &lp)
		lp.transRPC.HandleFunc("/transcodeResults", lp.TranscodeResults)
	}

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

// CheckOrchestratorAvailability - the broadcaster calls CheckOrchestratorAvailability which invokes Ping on the orchestrator
func CheckOrchestratorAvailability(orch Orchestrator) bool {
	ts := time.Now()
	tsSignature, err := orch.Sign([]byte(fmt.Sprintf("%v", ts)))
	if err != nil {
		return false
	}

	ping := crypto.Keccak256(tsSignature)

	orchClient, conn, err := startOrchestratorClient(orch.ServiceURI())
	if err != nil {
		return false
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	pong, err := orchClient.Ping(ctx, &net.PingPong{Value: ping})
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

// GetOrchestratorInfo - the broadcaster calls GetOrchestratorInfo which invokes GetOrchestrator on the orchestrator
func GetOrchestratorInfo(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
	c, conn, err := startOrchestratorClient(orchestratorServer)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req, err := genOrchestratorReq(bcast)
	r, err := c.GetOrchestrator(ctx, req)
	if err != nil {
		glog.Errorf("Could not get orchestrator orch=%v err=%v", orchestratorServer, err)
		return nil, errors.New("Could not get orchestrator err=" + err.Error())
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
		glog.Errorf("Did not connect to orch=%v err=%v", uri, err)
		return nil, nil, fmt.Errorf("Did not connect to orch=%v err=%v", uri, err)
	}
	c := net.NewOrchestratorClient(conn)

	return c, conn, nil
}

func genOrchestratorReq(b common.Broadcaster) (*net.OrchestratorRequest, error) {
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

	// currently, orchestrator == transcoder
	return orchestratorInfo(orch, addr, orch.ServiceURI().String())
}

func orchestratorInfo(orch Orchestrator, addr ethcommon.Address, serviceURI string) (*net.OrchestratorInfo, error) {
	priceInfo, err := orch.PriceInfo(addr)
	if err != nil {
		return nil, err
	}

	params, err := orch.TicketParams(addr, priceInfo)
	if err != nil {
		return nil, err
	}

	tr := net.OrchestratorInfo{
		Transcoder:   serviceURI,
		TicketParams: params,
		PriceInfo:    priceInfo,
		Address:      orch.Address().Bytes(),
	}

	os := drivers.NodeStorage.NewSession(string(core.RandomManifestID()))

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

func pmTicketParams(params *net.TicketParams) *pm.TicketParams {
	if params == nil {
		return nil
	}

	return &pm.TicketParams{
		Recipient:         ethcommon.BytesToAddress(params.Recipient),
		FaceValue:         new(big.Int).SetBytes(params.FaceValue),
		WinProb:           new(big.Int).SetBytes(params.WinProb),
		RecipientRandHash: ethcommon.BytesToHash(params.RecipientRandHash),
		Seed:              new(big.Int).SetBytes(params.Seed),
		ExpirationBlock:   new(big.Int).SetBytes(params.ExpirationBlock),
		ExpirationParams: &pm.TicketExpirationParams{
			CreationRound:          params.ExpirationParams.GetCreationRound(),
			CreationRoundBlockHash: ethcommon.BytesToHash(params.ExpirationParams.GetCreationRoundBlockHash()),
		},
	}
}
