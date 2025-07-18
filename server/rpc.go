package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/patrickmn/go-cache"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

const GRPCConnectTimeout = 3 * time.Second
const GRPCTimeout = 8 * time.Second
const HTTPIdleTimeout = 10 * time.Minute

var authTokenValidPeriod = 30 * time.Minute
var discoveryAuthWebhookCacheCleanup = 5 * time.Minute

var discoveryAuthWebhookCache = cache.New(authTokenValidPeriod, discoveryAuthWebhookCacheCleanup)

type Orchestrator interface {
	ServiceURI() *url.URL
	Address() ethcommon.Address
	TranscoderSecret() string
	Sign([]byte) ([]byte, error)
	VerifySig(ethcommon.Address, string, []byte) bool
	CheckCapacity(core.ManifestID) error
	CheckAICapacity(pipeline, modelID string) (bool, chan<- bool)
	GetLiveAICapacity() worker.Capacity
	TranscodeSeg(context.Context, *core.SegTranscodingMetadata, *stream.HLSSegment) (*core.TranscodeResult, error)
	ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int, capabilities *net.Capabilities)
	TranscoderResults(job int64, res *core.RemoteTranscoderResult)
	ServeAIWorker(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities, hardware []*net.HardwareInformation)
	AIResults(job int64, res *core.RemoteAIWorkerResult)
	ProcessPayment(ctx context.Context, payment net.Payment, manifestID core.ManifestID) error
	TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error)
	PriceInfo(sender ethcommon.Address, manifestID core.ManifestID) (*net.PriceInfo, error)
	PriceInfoForCaps(sender ethcommon.Address, manifestID core.ManifestID, caps *net.Capabilities) (*net.PriceInfo, error)
	GetCapabilitiesPrices(sender ethcommon.Address) ([]*net.PriceInfo, error)
	SufficientBalance(addr ethcommon.Address, manifestID core.ManifestID) bool
	DebitFees(addr ethcommon.Address, manifestID core.ManifestID, price *net.PriceInfo, pixels int64)
	Balance(addr ethcommon.Address, manifestID core.ManifestID) *big.Rat
	Capabilities() *net.Capabilities
	AuthToken(sessionID string, expiration int64) *net.AuthToken
	CreateStorageForRequest(requestID string) error
	GetStorageForRequest(requestID string) (drivers.OSSession, bool)
	WorkerHardware() []worker.HardwareInformation
	TextToImage(ctx context.Context, requestID string, req worker.GenTextToImageJSONRequestBody) (interface{}, error)
	ImageToImage(ctx context.Context, requestID string, req worker.GenImageToImageMultipartRequestBody) (interface{}, error)
	ImageToVideo(ctx context.Context, requestID string, req worker.GenImageToVideoMultipartRequestBody) (interface{}, error)
	Upscale(ctx context.Context, requestID string, req worker.GenUpscaleMultipartRequestBody) (interface{}, error)
	AudioToText(ctx context.Context, requestID string, req worker.GenAudioToTextMultipartRequestBody) (interface{}, error)
	LLM(ctx context.Context, requestID string, req worker.GenLLMJSONRequestBody) (interface{}, error)
	SegmentAnything2(ctx context.Context, requestID string, req worker.GenSegmentAnything2MultipartRequestBody) (interface{}, error)
	ImageToText(ctx context.Context, requestID string, req worker.GenImageToTextMultipartRequestBody) (interface{}, error)
	TextToSpeech(ctx context.Context, requestID string, req worker.GenTextToSpeechJSONRequestBody) (interface{}, error)
	LiveVideoToVideo(ctx context.Context, requestID string, req worker.GenLiveVideoToVideoJSONRequestBody) (interface{}, error)
	RegisterExternalCapability(extCapability string) (*core.ExternalCapability, error)
	RemoveExternalCapability(extCapability string) error
	GetUrlForCapability(extCapability string) string
	CheckExternalCapabilityCapacity(extCapability string) bool
	ReserveExternalCapabilityCapacity(extCapability string) error
	FreeExternalCapabilityCapacity(extCapability string) error
	JobPriceInfo(sender ethcommon.Address, jobCapabiliy string) (*net.PriceInfo, error)
}

// Balance describes methods for a session's balance maintenance
type Balance interface {
	Credit(amount *big.Rat)
	StageUpdate(minCredit *big.Rat, ev *big.Rat) (int, *big.Rat, *big.Rat)
	Balance() *big.Rat
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
	Broadcaster              common.Broadcaster
	Params                   *core.StreamParameters
	BroadcasterOS            drivers.OSSession
	Sender                   pm.Sender
	Balances                 *core.AddressBalances
	OrchestratorScore        float32
	VerifiedByPerceptualHash bool
	lock                     *sync.RWMutex
	// access these fields under the lock
	SegsInFlight     []SegFlightMetadata
	LatencyScore     float64
	OrchestratorInfo *net.OrchestratorInfo
	OrchestratorOS   drivers.OSSession
	PMSessionID      string
	CleanupSession   sessionsCleanup
	Balance          Balance
	InitialPrice     *net.PriceInfo

	InitialLatency time.Duration
}

type GetOrchestratorInfoParams struct {
	Caps                *net.Capabilities
	IgnoreCapacityCheck bool
}

func (bs *BroadcastSession) Transcoder() string {
	bs.lock.RLock()
	defer bs.lock.RUnlock()
	return bs.OrchestratorInfo.Transcoder
}

func (bs *BroadcastSession) Address() string {
	bs.lock.RLock()
	defer bs.lock.RUnlock()
	return hexutil.Encode(bs.OrchestratorInfo.Address)
}

func (bs *BroadcastSession) Clone() *BroadcastSession {
	bs.lock.RLock()
	newSess := *bs
	newSess.lock = &sync.RWMutex{}
	bs.lock.RUnlock()
	return &newSess
}

func (bs *BroadcastSession) IsTrusted() bool {
	return bs.OrchestratorScore == common.Score_Trusted
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
	trickleSrv   *trickle.Server
	node         *core.LivepeerNode
	net.UnimplementedOrchestratorServer
	net.UnimplementedTranscoderServer
	net.UnimplementedAIWorkerServer
}

func (h *lphttp) EndTranscodingSession(ctx context.Context, request *net.EndTranscodingSessionRequest) (*net.EndTranscodingSessionResponse, error) {
	return endTranscodingSession(h.node, h.orchestrator, request)
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
func StartTranscodeServer(orch Orchestrator, bind string, mux *http.ServeMux, workDir string, acceptRemoteTranscoders bool, acceptRemoteAIWorkers bool, n *core.LivepeerNode) error {
	s := grpc.NewServer()
	lp := lphttp{
		orchestrator: orch,
		orchRPC:      s,
		transRPC:     mux,
		node:         n,
	}
	net.RegisterOrchestratorServer(s, &lp)
	lp.transRPC.HandleFunc("/segment", lp.ServeSegment)
	lp.transRPC.HandleFunc("/payment", lp.Payment)
	if acceptRemoteTranscoders {
		net.RegisterTranscoderServer(s, &lp)
		lp.transRPC.HandleFunc("/transcodeResults", lp.TranscodeResults)
	}

	err := startAIServer(&lp)
	if err != nil {
		return err
	}
	if acceptRemoteAIWorkers {
		net.RegisterAIWorkerServer(s, &lp)
		lp.transRPC.Handle("/aiResults", lp.AIResults())
	}
	//API for dynamic capabilities
	lp.transRPC.HandleFunc("/process/request/", lp.ProcessJob)
	lp.transRPC.HandleFunc("/process/token", lp.GetJobToken)
	lp.transRPC.HandleFunc("/capability/register", lp.RegisterCapability)
	lp.transRPC.HandleFunc("/capability/unregister", lp.UnregisterCapability)

	cert, key, err := getCert(orch.ServiceURI(), workDir)
	if err != nil {
		return err
	}

	stopTrickle := lp.trickleSrv.Start()
	defer stopTrickle()

	glog.Info("Listening for RPC on ", bind)
	srv := http.Server{
		Addr:        bind,
		Handler:     &lp,
		IdleTimeout: HTTPIdleTimeout,
	}
	return srv.ListenAndServeTLS(cert, key)
}

// CheckOrchestratorAvailability - the broadcaster calls CheckOrchestratorAvailability which invokes Ping on the orchestrator
func CheckOrchestratorAvailability(orch Orchestrator) bool {
	ts := time.Now()
	tsSignature, err := orch.Sign([]byte(fmt.Sprintf("%v", ts)))
	if err != nil {
		return false
	}

	ping := crypto.Keccak256(tsSignature)

	orchClient, conn, err := startOrchestratorClient(context.Background(), orch.ServiceURI())
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
func GetOrchestratorInfo(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL, params GetOrchestratorInfoParams) (*net.OrchestratorInfo, error) {
	c, conn, err := startOrchestratorClient(ctx, orchestratorServer)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req, err := genOrchestratorReq(bcast, params)
	r, err := c.GetOrchestrator(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not get orchestrator orch=%v", orchestratorServer)
	}

	return r, nil
}

// EndTranscodingSession - the broadcaster calls EndTranscodingSession to tear down sessions used for verification only once
func EndTranscodingSession(ctx context.Context, sess *BroadcastSession) error {
	uri, err := url.Parse(sess.Transcoder())
	if err != nil {
		return err
	}
	c, conn, err := startOrchestratorClient(ctx, uri)
	if err != nil {
		return err
	}
	defer conn.Close()

	req, err := genEndSessionRequest(sess)
	_, err = c.EndTranscodingSession(context.Background(), req)
	if err != nil {
		return errors.Wrapf(err, "Could not end orchestrator session orch=%v", sess.Transcoder())
	}
	return nil
}

func startOrchestratorClient(ctx context.Context, uri *url.URL) (net.OrchestratorClient, *grpc.ClientConn, error) {
	clog.V(common.DEBUG).Infof(ctx, "Connecting RPC to uri=%v", uri)
	conn, err := grpc.Dial(uri.Host,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithTimeout(GRPCConnectTimeout))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Did not connect to orch=%v", uri)

	}
	c := net.NewOrchestratorClient(conn)

	return c, conn, nil
}

func genOrchestratorReq(b common.Broadcaster, params GetOrchestratorInfoParams) (*net.OrchestratorRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", b.Address().Hex())))
	if err != nil {
		return nil, err
	}
	return &net.OrchestratorRequest{Address: b.Address().Bytes(), Sig: sig, Capabilities: params.Caps, IgnoreCapacityCheck: params.IgnoreCapacityCheck}, nil
}

func genEndSessionRequest(sess *BroadcastSession) (*net.EndTranscodingSessionRequest, error) {
	return &net.EndTranscodingSessionRequest{AuthToken: sess.OrchestratorInfo.AuthToken}, nil
}

func getOrchestrator(orch Orchestrator, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	addr := ethcommon.BytesToAddress(req.Address)
	if err := verifyOrchestratorReq(orch, addr, req.Sig); err != nil {
		return nil, fmt.Errorf("Invalid orchestrator request: %v", err)
	}

	if _, err := authenticateBroadcaster(addr.Hex()); err != nil {
		return nil, fmt.Errorf("authentication failed: %v", err)
	}

	// currently, orchestrator == transcoder
	if req.Capabilities == nil {
		return orchestratorInfo(orch, addr, orch.ServiceURI().String(), "")
	}

	if err := checkLiveVideoToVideoCapacity(orch, req); err != nil {
		return nil, fmt.Errorf("Invalid orchestrator request: %v", err)
	}
	return orchestratorInfoWithCaps(orch, addr, orch.ServiceURI().String(), "", req.Capabilities)
}

func checkLiveVideoToVideoCapacity(orch Orchestrator, req *net.OrchestratorRequest) interface{} {
	caps := req.Capabilities
	if req.IgnoreCapacityCheck || caps.Constraints == nil || caps.Constraints.PerCapability == nil {
		return nil
	}

	if liveCap, ok := caps.Constraints.PerCapability[uint32(core.Capability_LiveVideoToVideo)]; ok {
		pipeline := "live-video-to-video"
		for modelID := range liveCap.GetModels() {
			hasCapacity, _ := orch.CheckAICapacity(pipeline, modelID)
			if hasCapacity {
				// It has capacity for at least one of the requested models
				return nil
			}
		}
		// No capacity for any requested model
		return core.ErrOrchCap
	}
	// For no constraints or AI Jobs (non live-video-to-video), we don't want to check capacity
	return nil
}

func endTranscodingSession(node *core.LivepeerNode, orch Orchestrator, req *net.EndTranscodingSessionRequest) (*net.EndTranscodingSessionResponse, error) {
	verifyToken := orch.AuthToken(req.AuthToken.SessionId, req.AuthToken.Expiration)
	if !bytes.Equal(verifyToken.Token, req.AuthToken.Token) {
		return nil, fmt.Errorf("Invalid auth token")
	}
	node.EndTranscodingSession(req.AuthToken.SessionId)
	return &net.EndTranscodingSessionResponse{}, nil
}

func getPriceInfo(orch Orchestrator, addr ethcommon.Address, manifestID core.ManifestID) (*net.PriceInfo, error) {
	if AuthWebhookURL != nil {
		webhookRes := getFromDiscoveryAuthWebhookCache(addr.Hex())
		if webhookRes != nil && webhookRes.PriceInfo != nil {
			return webhookRes.PriceInfo, nil
		}
	}
	return orch.PriceInfo(addr, manifestID)
}

func orchestratorInfo(orch Orchestrator, addr ethcommon.Address, serviceURI string, manifestID core.ManifestID) (*net.OrchestratorInfo, error) {
	return orchestratorInfoWithCaps(orch, addr, serviceURI, manifestID, nil)
}

func orchestratorInfoWithCaps(orch Orchestrator, addr ethcommon.Address, serviceURI string, manifestID core.ManifestID, caps *net.Capabilities) (*net.OrchestratorInfo, error) {
	var priceInfo *net.PriceInfo
	var capsPrices []*net.PriceInfo
	var err error
	if caps == nil {
		//get capability prices
		capsPrices, err = orch.GetCapabilitiesPrices(addr)

		//get base price
		priceInfo, err = getPriceInfo(orch, addr, manifestID)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		priceInfo, err = orch.PriceInfoForCaps(addr, manifestID, caps)
		if err != nil {
			return nil, err
		}
	}

	params, err := orch.TicketParams(addr, priceInfo)
	if err != nil {
		return nil, err
	}

	// Generate auth token
	sessionID := string(core.RandomManifestID())
	expiration := time.Now().Add(authTokenValidPeriod).Unix()
	authToken := orch.AuthToken(sessionID, expiration)

	var workerHardware []*net.HardwareInformation
	if caps == nil {
		workerHardware = workerHardwareToNetWorkerHardware(orch.WorkerHardware())
	}

	capabilities := orch.Capabilities()
	setLiveAICapacity(orch, capabilities)

	tr := net.OrchestratorInfo{
		Transcoder:         serviceURI,
		TicketParams:       params,
		PriceInfo:          priceInfo,
		Address:            orch.Address().Bytes(),
		Capabilities:       capabilities,
		AuthToken:          authToken,
		Hardware:           workerHardware,
		CapabilitiesPrices: capsPrices,
	}

	os := drivers.NodeStorage.NewSession(authToken.SessionId)

	if os != nil {
		if os.IsExternal() {
			tr.Storage = []*net.OSInfo{core.ToNetOSInfo(os.GetInfo())}
		} else {
			os.EndSession()
		}
	}

	return &tr, nil
}

func setLiveAICapacity(orch Orchestrator, capabilities *net.Capabilities) {
	if capabilities == nil || capabilities.Constraints == nil || capabilities.Constraints.PerCapability == nil {
		return
	}
	liveAI, ok := capabilities.Constraints.PerCapability[uint32(core.Capability_LiveVideoToVideo)]
	if !ok {
		return
	}
	if len(liveAI.Models) > 1 {
		// Live AI capacity is calculated based on the number of warm containers and assumes all containers serving the same model
		glog.Warning("Setting Live AI capacity is only supported in a single model setup")
		return
	}
	aiCapacity := orch.GetLiveAICapacity()

	for _, model := range liveAI.Models {
		if model == nil {
			glog.Warning("Model was nil when setting Live AI capacity")
			continue
		}
		model.Capacity = uint32(aiCapacity.ContainersIdle)
		model.CapacityInUse = uint32(aiCapacity.ContainersInUse)
	}
}

func verifyOrchestratorReq(orch Orchestrator, addr ethcommon.Address, sig []byte) error {
	if !orch.VerifySig(addr, addr.Hex(), sig) {
		glog.Error("orchestrator req sig check failed")
		return fmt.Errorf("orchestrator req sig check failed")
	}
	return orch.CheckCapacity("")
}

type discoveryAuthWebhookRes struct {
	PriceInfo *net.PriceInfo `json:"priceInfo,omitempty"`
}

// authenticateBroadcaster returns an error if authentication fails
// on success it caches the webhook response
func authenticateBroadcaster(id string) (*discoveryAuthWebhookRes, error) {
	if AuthWebhookURL == nil {
		return nil, nil
	}

	values := map[string]string{"id": id}
	jsonValues, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	res, err := http.Post(AuthWebhookURL.String(), "application/json", bytes.NewBuffer(jsonValues))
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, errors.New(string(body))
	}

	webhookRes := &discoveryAuthWebhookRes{}
	if err := json.Unmarshal(body, webhookRes); err != nil {
		return nil, err
	}

	addToDiscoveryAuthWebhookCache(id, webhookRes, authTokenValidPeriod)

	return webhookRes, nil
}

func addToDiscoveryAuthWebhookCache(id string, webhookRes *discoveryAuthWebhookRes, expiration time.Duration) {
	_, ok := discoveryAuthWebhookCache.Get(id)
	if ok {
		discoveryAuthWebhookCache.Replace(id, webhookRes, authTokenValidPeriod)
	} else {
		discoveryAuthWebhookCache.Add(id, webhookRes, authTokenValidPeriod)
	}
}

func getFromDiscoveryAuthWebhookCache(id string) *discoveryAuthWebhookRes {
	c, ok := discoveryAuthWebhookCache.Get(id)
	if !ok {
		return nil
	}
	webhookRes, ok := c.(*discoveryAuthWebhookRes)
	if !ok {
		return nil
	}
	return webhookRes
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

func coreSegMetadata(segData *net.SegData) (*core.SegTranscodingMetadata, error) {
	if segData == nil {
		glog.Error("Empty seg data")
		return nil, errors.New("empty seg data")
	}
	var err error
	profiles := []ffmpeg.VideoProfile{}
	if len(segData.FullProfiles3) > 0 {
		profiles, err = makeFfmpegVideoProfiles(segData.FullProfiles3)
	} else if len(segData.FullProfiles2) > 0 {
		profiles, err = makeFfmpegVideoProfiles(segData.FullProfiles2)
	} else if len(segData.FullProfiles) > 0 {
		profiles, err = makeFfmpegVideoProfiles(segData.FullProfiles)
	}
	if err != nil {
		glog.Error("Unable to deserialize profiles ", err)
		return nil, err
	}

	var os *net.OSInfo
	if len(segData.Storage) > 0 {
		os = segData.Storage[0]
	}

	dur := time.Duration(segData.Duration) * time.Millisecond
	if dur < 0 || dur > common.MaxDuration {
		glog.Error("Invalid duration")
		return nil, errDuration
	}
	if dur == 0 {
		dur = 2 * time.Second // assume 2sec default duration
	}

	caps := core.CapabilitiesFromNetCapabilities(segData.Capabilities)
	if caps == nil {
		// For older broadcasters. Note if there are any orchestrator
		// mandatory capabilities, seg creds verification will fail.
		caps = core.NewCapabilities(nil, nil)
	}

	var segPar core.SegmentParameters
	segPar.ForceSessionReinit = segData.ForceSessionReinit
	if segData.SegmentParameters != nil {
		segPar.Clip = &core.SegmentClip{
			From: time.Duration(segData.SegmentParameters.From) * time.Millisecond,
			To:   time.Duration(segData.SegmentParameters.To) * time.Millisecond,
		}
	}

	return &core.SegTranscodingMetadata{
		ManifestID:         core.ManifestID(segData.ManifestId),
		Seq:                segData.Seq,
		Hash:               ethcommon.BytesToHash(segData.Hash),
		Profiles:           profiles,
		OS:                 os,
		Duration:           dur,
		Caps:               caps,
		AuthToken:          segData.AuthToken,
		CalcPerceptualHash: segData.CalcPerceptualHash,
		SegmentParameters:  &segPar,
	}, nil
}

func workerHardwareToNetWorkerHardware(orchHdw []worker.HardwareInformation) []*net.HardwareInformation {
	var workerHardware []byte
	workerHardware, err := json.Marshal(orchHdw)
	if err != nil {
		glog.Errorf("Error converting hardware information to json: %v", err)
		return []*net.HardwareInformation{}
	}
	var netWorkerHardware []*net.HardwareInformation
	err = json.Unmarshal(workerHardware, &netWorkerHardware)
	if err != nil {
		glog.Errorf("Error converting hardware information: %v", err)
		return []*net.HardwareInformation{}
	}

	return netWorkerHardware
}
