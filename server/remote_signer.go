package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/byoc"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

const HTTPStatusRefreshSession = 480
const HTTPStatusPriceExceeded = 481
const HTTPStatusNoTickets = 482
const RemoteType_LiveVideoToVideo = "lv2v"
const PipelineLiveVideoToVideo = "live-video-to-video"
const RemoteType_BYOC = "byoc"

// minPreloadSecs is the orchestrator's minimum required balance (seconds), used
// as the floor when preloading an initial BYOC ticket batch.
const minPreloadSecs = 60

// SignOrchestratorInfo handles signing GetOrchestratorInfo requests for multiple orchestrators
func (ls *LivepeerServer) SignOrchestratorInfo(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	remoteAddr := getRemoteAddr(r)
	clog.Info(ctx, "Orch info signature request", "ip", remoteAddr)

	// Get the broadcaster (signer)
	// In remote signer mode, we may not have an OrchestratorPool, so create a broadcaster directly
	gw := core.NewBroadcaster(ls.LivepeerNode)

	// Create empty params for signing
	params := GetOrchestratorInfoParams{}

	// Generate the request (this creates the signature)
	req, err := genOrchestratorReq(gw, params)
	if err != nil {
		clog.Errorf(ctx, "Failed to generate request: err=%q", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	// Extract signature and format as hex
	var (
		signature = "0x" + hex.EncodeToString(req.Sig)
		address   = gw.Address().String()
	)

	results := map[string]string{
		"address":   address,
		"signature": signature,
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(results)
}

// SignBYOCJobRequest signs a BYOC job using the V1 binary format (FlattenBYOCJob).
type SignBYOCJobRequestInput struct {
	ID             string `json:"id"`
	Capability     string `json:"capability"`
	Request        string `json:"request"`
	Parameters     string `json:"parameters"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type SignBYOCJobRequestResponse struct {
	Sender    string `json:"sender"`
	Signature string `json:"signature"`
}

func (ls *LivepeerServer) SignBYOCJobRequest(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	remoteAddr := getRemoteAddr(r)
	clog.Info(ctx, "BYOC job signing request", "ip", remoteAddr)

	gw := core.NewBroadcaster(ls.LivepeerNode)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	var req SignBYOCJobRequestInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		clog.Errorf(ctx, "Failed to decode SignBYOCJobRequest err=%q", err)
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Capability == "" {
		err := fmt.Errorf("sign-byoc-job requires non-empty id and capability")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	if req.TimeoutSeconds <= 0 {
		err := fmt.Errorf("sign-byoc-job requires positive timeout_seconds")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	sigPayload := byoc.FlattenBYOCJob(&byoc.BYOCJobSigningInput{
		ID:             req.ID,
		Capability:     req.Capability,
		Request:        req.Request,
		Parameters:     req.Parameters,
		TimeoutSeconds: req.TimeoutSeconds,
	})

	sig, err := gw.Sign(sigPayload)
	if err != nil {
		clog.Errorf(ctx, "Failed to sign BYOC job request err=%q", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	response := SignBYOCJobRequestResponse{
		Sender:    gw.Address().Hex(),
		Signature: "0x" + hex.EncodeToString(sig),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// StartRemoteSignerServer starts the HTTP server for remote signer mode
func StartRemoteSignerServer(ls *LivepeerServer, bind string) error {
	// These endpoints sign caller-supplied payloads and must only be exposed on trusted networks.
	// Register the remote signer endpoints
	ls.HTTPMux.Handle("POST /sign-orchestrator-info", http.HandlerFunc(ls.SignOrchestratorInfo))
	ls.HTTPMux.Handle("POST /generate-live-payment", http.HandlerFunc(ls.GenerateLivePayment))
	ls.HTTPMux.Handle("POST /sign-byoc-job", http.HandlerFunc(ls.SignBYOCJobRequest))
	if ls.LivepeerNode.RemoteDiscovery {
		rdp := RemoteDiscoveryConfig{
			Pool:     ls.LivepeerNode.OrchestratorPool,
			Node:     ls.LivepeerNode,
			Interval: ls.LivepeerNode.LiveAICapReportInterval,
		}.New()
		ls.HTTPMux.Handle("GET /discover-orchestrators", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ls.GetOrchestrators(rdp, w, r)
		}))
	}

	// Start the HTTP server
	glog.Info("Starting Remote Signer server on ", bind)
	gw := core.NewBroadcaster(ls.LivepeerNode)
	sig, err := gw.Sign([]byte(fmt.Sprintf("%v", gw.Address().Hex())))
	if err != nil {
		return err
	}
	ls.LivepeerNode.InfoSig = sig
	srv := http.Server{
		Addr:        bind,
		Handler:     ls.HTTPMux,
		IdleTimeout: HTTPIdleTimeout,
	}
	return srv.ListenAndServe()
}

// HexBytes represents a byte slice that marshals/unmarshals as hex with 0x prefix
type HexBytes []byte

func (h HexBytes) MarshalJSON() ([]byte, error) {
	hexStr := "0x" + hex.EncodeToString(h)
	return json.Marshal(hexStr)
}

func (h *HexBytes) UnmarshalJSON(data []byte) error {
	var hexStr string
	if err := json.Unmarshal(data, &hexStr); err != nil {
		return err
	}

	// Remove 0x prefix if present
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	// Decode hex string to bytes
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Errorf("invalid hex string: %v", err)
	}

	*h = decoded
	return nil
}

// OrchInfoSigResponse represents the response from the remote signer
type OrchInfoSigResponse struct {
	Address   HexBytes `json:"address"`
	Signature HexBytes `json:"signature"`
}

// State required for remote ticket creation.
// Treated as an opaque, signed blob by the gateway.
type RemotePaymentState struct {
	StateID              string
	PMSessionID          string
	LastUpdate           time.Time
	OrchestratorAddress  ethcommon.Address
	SenderNonce          uint32
	Balance              string
	InitialPricePerUnit  int64
	InitialPixelsPerUnit int64
	SequenceNumber       uint64
}

type RemotePaymentStateSig struct {
	State []byte
	Sig   []byte
}

// RemotePaymentRequest is sent by the gateway to the remote signer to request a batch of tickets.
// TODO length limits for string / byte fields
type RemotePaymentRequest struct {
	// State is an opaque, signed blob previously returned by the remote signer.
	State RemotePaymentStateSig `json:"state,omitempty"`

	// protobuf bytes of net.OrchestratorInfo. Required
	Orchestrator []byte `json:"orchestrator"`

	// Set if an ID is needed to tie into orch accounting for a session. Optional
	ManifestID string `json:"manifestID,omitempty"`

	// Number of pixels to generate a ticket for. Required if `type` is not set.
	InPixels int64 `json:"inPixels"`

	// Job type to automatically calculate payments. Valid values: `lv2v`, `byoc`. Optional.
	Type string `json:"type"`

	// Capabilities to include in the ticket. Optional; may be set for the lv2v job type.
	Capabilities []byte `json:"capabilities"`

	// Expected job duration. Used as the initial preload when there is no prior
	// state, so that short BYOC batch jobs don't over-reserve. Floored to the
	// orchestrator's minimum balance requirement (minPreloadSecs). Optional.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

// Returned by the remote signer and includes a new payment plus updated state.
type RemotePaymentResponse struct {
	Payment  string                `json:"payment"`
	SegCreds string                `json:"segCreds,omitempty"`
	State    RemotePaymentStateSig `json:"state"`
}

// Signs the serialized state with the remote signer's Ethereum key.
func signState(ls *LivepeerServer, stateBytes []byte) ([]byte, error) {
	if ls == nil || ls.LivepeerNode == nil || ls.LivepeerNode.Eth == nil {
		return nil, fmt.Errorf("ethereum client not configured for remote signer")
	}
	sig, err := ls.LivepeerNode.Eth.Sign(stateBytes)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// verifyStateSignature verifies that sig is a valid signature over stateBytes produced
// by the remote signer's Ethereum account.
func verifyStateSignature(ls *LivepeerServer, stateBytes []byte, sig []byte) error {
	if ls == nil || ls.LivepeerNode == nil || ls.LivepeerNode.Eth == nil {
		return fmt.Errorf("ethereum client not configured for remote signer")
	}
	addr := ls.LivepeerNode.Eth.Account().Address
	if !lpcrypto.VerifySig(addr, stateBytes, sig) {
		return fmt.Errorf("invalid state signature")
	}
	return nil
}

// resolvePriceInfo returns the effective PriceInfo for a payment request and
// validates that it is usable (non-nil with non-zero price and pixels per unit).
// For BYOC, pricing may only be advertised in CapabilitiesPrices rather than
// the top-level PriceInfo, so we search for the capability-specific entry that
// matches the requested ManifestID.
func resolvePriceInfo(oInfo *net.OrchestratorInfo, reqType string, manifestID string) (*net.PriceInfo, error) {
	if reqType == RemoteType_BYOC {
		if manifestID == "" {
			return nil, errors.New("missing manifestID for BYOC capability")
		}
		byocCap := uint32(core.Capability_BYOC)
		candidates := append([]*net.PriceInfo{oInfo.PriceInfo}, oInfo.CapabilitiesPrices...)
		for _, cp := range candidates {
			if cp == nil || cp.PricePerUnit == 0 || cp.PixelsPerUnit == 0 {
				continue
			}
			if cp.Capability != byocCap || cp.Constraint != manifestID {
				continue
			}
			return cp, nil
		}
		return nil, fmt.Errorf("missing or zero priceInfo for BYOC capability %q; "+
			"ensure the orchestrator advertises capability-specific pricing "+
			"(CapabilitiesPrices with Capability=BYOC and Constraint=<name>)", manifestID)
	}

	p := oInfo.PriceInfo
	if p == nil || p.PricePerUnit == 0 || p.PixelsPerUnit == 0 {
		return nil, errors.New("missing or zero priceInfo")
	}
	return p, nil
}

// GenerateLivePayment handles remote generation of a payment for live streams.
func (ls *LivepeerServer) GenerateLivePayment(w http.ResponseWriter, r *http.Request) {
	requestID := string(core.RandomManifestID())
	ctx := clog.AddVal(r.Context(), "request_id", requestID)
	remoteAddr := getRemoteAddr(r)
	clog.Info(ctx, "Live payment request", "ip", remoteAddr)

	// TODO avoid using the global Balances; keep balance changes request-local
	if ls.LivepeerNode.Balances == nil || ls.LivepeerNode.Sender == nil {
		err := fmt.Errorf("LivepeerNode missing balances or sender")
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}
	balances, sender := ls.LivepeerNode.Balances, ls.LivepeerNode.Sender

	var req RemotePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		clog.Errorf(ctx, "Failed to decode RemotePaymentRequest err=%q", err)
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	if len(req.Orchestrator) == 0 {
		err := fmt.Errorf("missing orchestrator")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	var oInfo net.OrchestratorInfo
	if err := proto.Unmarshal(req.Orchestrator, &oInfo); err != nil {
		clog.Errorf(ctx, "Failed to unmarshal orch info err=%q", err)
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	priceInfo, err := resolvePriceInfo(&oInfo, req.Type, req.ManifestID)
	if err != nil {
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	if oInfo.TicketParams == nil {
		err := fmt.Errorf("missing ticketParams in OrchestratorInfo")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	orchAddr := ethcommon.BytesToAddress(oInfo.Address)
	manifestID := req.ManifestID

	// Load or initialize state
	var state *RemotePaymentState
	reqState, reqSig := req.State.State, req.State.Sig
	hasState := len(reqState) != 0 || len(reqSig) != 0
	if hasState {
		if err := verifyStateSignature(ls, reqState, reqSig); err != nil {
			err = errors.New("invalid sig")
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(reqState, &state); err != nil {
			err = errors.New("invalid state")
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		if state.OrchestratorAddress != orchAddr {
			err := fmt.Errorf("orchestratorAddress mismatch")
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		state.SequenceNumber++
	} else {
		state = &RemotePaymentState{
			StateID:              string(core.RandomManifestID()),
			OrchestratorAddress:  orchAddr,
			InitialPricePerUnit:  priceInfo.PricePerUnit,
			InitialPixelsPerUnit: priceInfo.PixelsPerUnit,
		}
	}

	stateID := core.ManifestID(state.StateID)
	ctx = clog.AddVal(ctx, "state_id", state.StateID)
	ctx = clog.AddVal(ctx, "seqNo", fmt.Sprintf("%d", state.SequenceNumber))

	if manifestID == "" {
		if hasState {
			// Required for lv2v so stateful requests stay tied to the same id.
			err := errors.New("missing manifestID")
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		manifestID = string(core.RandomManifestID())
	}
	ctx = clog.AddVal(ctx, "manifest_id", manifestID)

	streamParams := &core.StreamParameters{
		// Embedded within genSegCreds, may be used by orch for payment accounting
		ManifestID: core.ManifestID(manifestID),
	}
	if len(req.Capabilities) > 0 {
		var caps net.Capabilities
		if err := proto.Unmarshal(req.Capabilities, &caps); err != nil {
			clog.Errorf(ctx, "Failed to unmarshal capabilities err=%q", err)
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		streamParams.Capabilities = core.CapabilitiesFromNetCapabilities(&caps)
	}

	pmParams := pmTicketParams(oInfo.TicketParams)
	if pmParams == nil {
		err := fmt.Errorf("failed to derive ticket params from OrchestratorInfo")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	sessionBalance := core.NewBalance(orchAddr, stateID, balances)

	// Restore balance
	oldBal := &big.Rat{}
	if state.Balance != "" {
		if _, ok := oldBal.SetString(state.Balance); ok {
			// Reset existing balance for this stream and apply saved value
			sessionBalance.Reserve()
			sessionBalance.Credit(oldBal)
		}
	}

	// Reset nonce if session has been refreshed.
	sessionID := pmParams.RecipientRandHash.Hex()
	nonce := state.SenderNonce
	if state.PMSessionID != sessionID {
		nonce = 0
	}

	initialPrice := &net.PriceInfo{
		PricePerUnit:  state.InitialPricePerUnit,
		PixelsPerUnit: state.InitialPixelsPerUnit,
	}

	sess := &BroadcastSession{
		Broadcaster:      core.NewBroadcaster(ls.LivepeerNode),
		Params:           streamParams,
		Sender:           sender,
		Balances:         balances,
		Balance:          sessionBalance,
		lock:             &sync.RWMutex{},
		OrchestratorInfo: &oInfo,
		CleanupSession:   sender.CleanupSession,
		PMSessionID:      sender.StartSessionWithNonce(*pmParams, nonce),
		InitialPrice:     initialPrice,
	}
	defer sess.CleanupSession(sess.PMSessionID)

	if should, err := shouldRefreshSession(ctx, sess); err == nil && should {
		err := errors.New("refresh session for remote signer")
		respondJsonError(ctx, w, err, HTTPStatusRefreshSession)
		return
	} else if err != nil {
		err = fmt.Errorf("remote signer could not check whether to refresh session: %w", err)
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	pixels := req.InPixels
	now := time.Now()
	lastUpdate := state.LastUpdate
	if lastUpdate.IsZero() {
		lastUpdate = now
	}
	billableSecs := now.Sub(lastUpdate).Seconds()
	switch req.Type {
	case RemoteType_LiveVideoToVideo:
		info := defaultSegInfo
		if billableSecs <= 0 {
			// preload with 60 seconds of data for LV2V
			billableSecs = 60
		}
		pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
		pixels = int64(pixelsPerSec * billableSecs) // pixels to charge for
	case RemoteType_BYOC:
		secondsToPrefund := billableSecs
		if secondsToPrefund <= 0 {
			// Preload for expected job duration (floored to the orchestrator's
			// minimum balance requirement) so batch jobs size the initial ticket
			// batch to match the work, instead of using a fixed buffer.
			secondsToPrefund = float64(max(req.TimeoutSeconds, minPreloadSecs))
		}
		if secondsToPrefund < 1 {
			secondsToPrefund = 1
		}
		// BYOC pre-funding is ceil(seconds) of work at the advertised unit scale.
		pixels = int64(math.Ceil(secondsToPrefund)) * priceInfo.PixelsPerUnit
	case "":
		// caller supplied req.InPixels directly
	default:
		err = errors.New("invalid job type")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	if pixels <= 0 {
		err = errors.New("missing pixels or job type")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	// Validate orchestrator price against configured max price
	orchPrice := new(big.Rat).SetFrac64(priceInfo.PricePerUnit, priceInfo.PixelsPerUnit)
	maxPrice := BroadcastCfg.GetCapabilitiesMaxPrice(streamParams.Capabilities)
	if maxPrice != nil && orchPrice.Cmp(maxPrice) > 0 {
		err := fmt.Errorf("orchestrator price %v exceeds maximum price %v", orchPrice.FloatString(3), maxPrice.FloatString(3))
		clog.Warningf(ctx, "Rejecting payment request: %v", err)
		respondJsonError(ctx, w, err, HTTPStatusPriceExceeded)
		return
	}

	// Compute required fee using initial price
	fee := calculateFee(pixels, initialPrice)

	// Create balance update
	balUpdate, err := newBalanceUpdate(sess, fee)
	if err != nil {
		err = fmt.Errorf("Failed to update balance: %w", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}
	if balUpdate.NumTickets <= 0 {
		// No new tickets are needed when reserved balance already covers the
		// required minimum credit (fee with ticket EV as the floor). Caller
		// should retry once balance has been run down further.
		err = errors.New("no tickets")
		clog.Errorf(ctx, "No tickets")
		respondJsonError(ctx, w, err, HTTPStatusNoTickets)
		return
	}
	if balUpdate.NumTickets > 100 {
		// Prevent both draining funds and perf issues
		ev, err := sender.EV(sess.PMSessionID)
		if err != nil {
			clog.Errorf(ctx, "Could not retrieve EV", err)
			ev = new(big.Rat)
		}
		err = fmt.Errorf("numTickets %d exceeds maximum of 100", balUpdate.NumTickets)
		clog.Errorf(ctx, "%v: if legitimate check EV config %s", err, ev.FloatString(3))
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	balUpdate.Debit = fee
	balUpdate.Status = ReceivedChange

	// Generate payment tickets
	payment, err := genPayment(ctx, sess, balUpdate.NumTickets)
	if err != nil {
		clog.Errorf(ctx, "Could not create payment err=%q", err)
		if monitor.Enabled {
			monitor.PaymentCreateError(ctx)
		}
		// Check if error is due to price increase validation (price-related error, not server error)
		// NB: Do not really want this to drift for any length of time.
		// The initial price is used to calculate the number of tickets needed,
		// and if this is lower then the G will run out of credit on the O.
		// The O should keep the price fixed per session anyway
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "Orchestrator price has more than doubled") {
			statusCode = HTTPStatusPriceExceeded
		}
		respondJsonError(ctx, w, err, statusCode)
		return
	}

	// Generate segment credentials with an empty segment
	segCreds, err := genSegCreds(sess, &stream.HLSSegment{}, nil, false)
	if err != nil {
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	// Complete balance update and set state to new balance
	completeBalanceUpdate(sess, balUpdate) // Updates sessionBalance internally
	newBal := sessionBalance.Balance()
	if newBal == nil {
		err = errors.New("zero balance?")
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}
	state.Balance = newBal.RatString()
	state.LastUpdate = time.Now()
	state.PMSessionID = sess.PMSessionID
	state.SenderNonce, err = sender.Nonce(sess.PMSessionID)
	if err != nil {
		err = fmt.Errorf("remote signer failed to retrieve nonce: %w", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	// Encode and sign updated state
	stateBytes, err := json.Marshal(state)
	if err != nil {
		clog.Errorf(ctx, "Failed to encode updated RemotePaymentState err=%q", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	stateSig, err := signState(ls, stateBytes)
	if err != nil {
		clog.Errorf(ctx, "Could not sign state err=%q", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	clog.Info(ctx, "Signed", "numTickets", balUpdate.NumTickets, "nonce", state.SenderNonce, "fee", fee.FloatString(0), "sessionId", oInfo.AuthToken.SessionId, "pmSessionId", sess.PMSessionID, "oldBalance", oldBal.FloatString(0), "newBalance", newBal.FloatString(0))

	if monitor.Enabled {
		sessionStatus := "continuing"
		if state.SequenceNumber == 0 {
			sessionStatus = "new"
		}
		pipeline := ""
		if req.Type == RemoteType_LiveVideoToVideo {
			pipeline = PipelineLiveVideoToVideo
		}
		// NB: This could could drop events if tha Kafka queue is full!
		monitor.SendQueueEventAsync("create_signed_ticket", map[string]interface{}{
			"session_id":         state.StateID,
			"session_status":     sessionStatus,
			"pipeline":           pipeline,
			"request_id":         requestID,
			"orch_address":       orchAddr.Hex(),
			"orch_url":           oInfo.Transcoder,
			"manifest_id":        manifestID,
			"pm_session_id":      sess.PMSessionID,
			"current_time":       now.UTC(),
			"current_time_unix":  now.UTC().UnixMilli(),
			"previous_time":      lastUpdate.UTC(),
			"previous_time_unix": lastUpdate.UTC().UnixMilli(),
			"billable_secs":      billableSecs,
			"pixels":             pixels,
			"session_balance":    newBal.FloatString(0),
			"computed_fee":       fee.FloatString(0),
			"cost_per_pixel":     orchPrice.FloatString(10),
			"sequence_number":    state.SequenceNumber,
			"num_tickets":        balUpdate.NumTickets,
		})
	}

	// Return payment (tickets), creds and signed state
	resp := RemotePaymentResponse{
		Payment:  payment,
		SegCreds: segCreds,
		State:    RemotePaymentStateSig{State: stateBytes, Sig: stateSig},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Gateway helper that calls the remote signer service for the GetOrchestratorInfo signature
func GetOrchInfoSig(remoteSignerHost *url.URL) (*OrchInfoSigResponse, error) {

	url := remoteSignerHost.ResolveReference(&url.URL{Path: "/sign-orchestrator-info"})

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make the request
	resp, err := client.Post(url.String(), "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call remote signer: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote signer returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var signerResp OrchInfoSigResponse
	if err := json.NewDecoder(resp.Body).Decode(&signerResp); err != nil {
		return nil, fmt.Errorf("failed to parse remote signer response: %w", err)
	}

	return &signerResp, nil
}

type discoveryResponse struct {
	Address      string   `json:"address,omitempty"`
	Score        float32  `json:"score,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// GetOrchestrators returns the configured orchestrators in webhook-compatible format
func (ls *LivepeerServer) GetOrchestrators(pool *remoteDiscoveryPool, w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	remoteAddr := getRemoteAddr(r)
	clog.Info(ctx, "Get orchestrators request", "ip", remoteAddr)

	if pool == nil {
		respondJsonError(ctx, w, errors.New("no orchestrator pool configured"), http.StatusServiceUnavailable)
		return
	}

	if pool.Size() == 0 {
		respondJsonError(ctx, w, errors.New("cache empty"), http.StatusServiceUnavailable)
		return
	}

	caps := r.URL.Query()["caps"]
	filteredCaps := make([]string, 0, len(caps))
	for _, capability := range caps {
		if capability != "" {
			filteredCaps = append(filteredCaps, capability)
		}
	}

	infos := pool.Orchestrators(filteredCaps)
	resp := make([]discoveryResponse, 0, len(infos))
	for _, cached := range infos {
		od := cached.OD
		resp = append(resp, discoveryResponse{
			Address:      od.LocalInfo.URL.String(),
			Score:        od.LocalInfo.Score,
			Capabilities: append([]string(nil), cached.Capabilities...),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
