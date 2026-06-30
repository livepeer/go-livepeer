package server

import (
	"bytes"
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
	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/byoc"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

const HTTPStatusRefreshSession = 480
const HTTPStatusPriceExceeded = 481
const HTTPStatusNoTickets = 482
const RefreshSessionOrchestratorURLHeader = "Livepeer-Orchestrator-URL"
const RemoteType_LiveVideoToVideo = "lv2v"
const PipelineLiveVideoToVideo = "live-video-to-video"
const remoteSignerAuthIDHeader = "Signer-Auth-Id"

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
	AuthExpiry           int64
	SenderNonce          uint32
	Balance              string
	InitialPricePerUnit  int64
	InitialPixelsPerUnit int64
	SequenceNumber       uint64
	AuthID               string
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
	ManifestID string

	// Number of pixels to generate a ticket for. Required if `type` is not set.
	InPixels int64 `json:"inPixels"`

	// Job type to automatically calculate payments. Valid values: `lv2v`. Optional.
	Type string `json:"type"`

	// Capabilities to include in the ticket. Optional; may be set for the lv2v job type.
	Capabilities []byte `json:"capabilities"`

	// Capability is the real BYOC capability name (e.g. "nano-banana"). When
	// set, it overrides the metered `pipeline` label for the emitted
	// create_signed_ticket usage event, decoupling usage attribution from
	// `Type` (which stays "lv2v" for fee/pixel routing). Optional; empty
	// preserves the previous behavior (pipeline falls back to the lv2v
	// constant when Type=="lv2v").
	Capability string `json:"capability,omitempty"`

	// ModelID is the specific provider model from the request
	// payload/parameters. When set, it overrides the metered `model_id`
	// label. Optional; empty preserves the previous behavior (the
	// capabilities-derived model id, which defaults to "unknown" downstream).
	ModelID string `json:"model_id,omitempty"`
}

// Returned by the remote signer and includes a new payment plus updated state.
type RemotePaymentResponse struct {
	Payment  string                `json:"payment"`
	SegCreds string                `json:"segCreds,omitempty"`
	State    RemotePaymentStateSig `json:"state"`
}

type generateLivePaymentWebhookBody struct {
	Headers map[string][]string `json:"headers"`
	State   *RemotePaymentState `json:"state,omitempty"`
}

type authResponse struct {
	// HTTP status that GenerateLivePayment should return to the caller.
	Status *int `json:"status,omitempty"`
	// Optional error message when Status is non-200.
	Reason string `json:"reason,omitempty"`
	// Unix timestamp (seconds) until which auth is considered valid.
	// Allows for skipping webhook callbacks until this time is exceeded.
	Expiry int64 `json:"expiry,omitempty"`
	// Optional opaque identifier.
	AuthID string `json:"auth_id,omitempty"`
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

func (ls *LivepeerServer) authLivePayment(r *http.Request, state *RemotePaymentState) (int, *authResponse, error) {
	if ls == nil || ls.LivepeerNode == nil {
		return http.StatusOK, nil, nil
	}
	callbackURL := ls.LivepeerNode.RemoteSignerWebhookURL
	callbackHeaders := ls.LivepeerNode.RemoteSignerWebhookHeaders
	if callbackURL == nil {
		return http.StatusOK, nil, nil
	}
	if state != nil && state.AuthExpiry != 0 && time.Now().Unix() <= state.AuthExpiry {
		return http.StatusOK, nil, nil
	}

	body, err := json.Marshal(generateLivePaymentWebhookBody{Headers: r.Header, State: state})
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("failed to encode signer auth payload: %v", err)
	}
	webhookReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, callbackURL.String(), bytes.NewReader(body))
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("failed to build signer auth request: %v", err)
	}
	webhookReq.Header.Set("Content-Type", "application/json")
	for key, value := range callbackHeaders {
		webhookReq.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(webhookReq)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("failed to call remote signer webhook: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("failed to read signer auth response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Error with webhook service or signer misconfiguration, so treat as internal
		return http.StatusInternalServerError, nil, fmt.Errorf("signer auth error status %d", resp.StatusCode)
	}

	var webhookResp authResponse
	if err := json.Unmarshal(respBody, &webhookResp); err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("signer auth invalid response: %v", err)
	}
	if webhookResp.Status == nil || *webhookResp.Status <= 0 {
		return http.StatusInternalServerError, nil, errors.New("signer auth invalid status")
	}
	if *webhookResp.Status != http.StatusOK && webhookResp.Reason == "" {
		webhookResp.Reason = fmt.Sprintf("signer auth rejected request with status %d", *webhookResp.Status)
	}

	return *webhookResp.Status, &webhookResp, errors.New(webhookResp.Reason)
}

// maxUsageLabelLen bounds the length of gateway-supplied usage labels
// (pipeline/model_id) before they are emitted to the metering pipeline.
// req.Capability / req.ModelID come straight from the request body, so a buggy
// or malicious caller could otherwise send very long / high-cardinality values
// that inflate Kafka payload size and downstream label cardinality (the
// pipeline label was previously a small fixed set). 128 runes is generous for
// real capability/model identifiers.
const maxUsageLabelLen = 128

// sanitizeUsageLabel trims surrounding whitespace and caps the length (in
// runes, to never emit invalid UTF-8) of a gateway-supplied metering label.
func sanitizeUsageLabel(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxUsageLabelLen {
		return string(r[:maxUsageLabelLen])
	}
	return s
}

// resolveUsageLabels derives the metering `pipeline` and `model_id` labels for
// the emitted create_signed_ticket usage event.
//
// The real BYOC capability/model supplied by the gateway (req.Capability /
// req.ModelID) take precedence and override the labels whenever they are set,
// regardless of `Type` — this decouples usage attribution from `Type`, which
// still drives fee/pixel routing. The gateway-supplied values are sanitized
// (trimmed + length-capped) to bound payload size and label cardinality.
//
// When those additive fields are empty, the labels fall back to the previous
// behavior: for lv2v the pipeline is the live-video-to-video constant and the
// model id is derived from the request capabilities; for any other request the
// untouched label(s) remain empty (the collector then defaults them to
// "unknown"). This makes non-BYOC (lv2v) callers with no override
// byte-identical to before.
func resolveUsageLabels(req *RemotePaymentRequest, caps *core.Capabilities) (pipeline, modelID string) {
	if req.Type == RemoteType_LiveVideoToVideo {
		pipeline = PipelineLiveVideoToVideo
		modelID = caps.ModelIDForCapability(core.Capability_LiveVideoToVideo)
	}
	if c := sanitizeUsageLabel(req.Capability); c != "" {
		pipeline = c
	}
	if m := sanitizeUsageLabel(req.ModelID); m != "" {
		modelID = m
	}
	return pipeline, modelID
}

// resolveByocPrice resolves the per-capability BYOC price advertised in the
// orchestrator's OrchestratorInfo.CapabilitiesPrices, keyed on the request
// capability name (req.Capability). The orchestrator appends external BYOC
// capability prices to CapabilitiesPrices as
// {Capability: Capability_BYOC, Constraint: <capability name>} (see
// core/orchestrator.go GetCapabilitiesPrices), so a match is a BYOC entry whose
// Constraint equals req.Capability.
//
// Granularity is per-capability only: req.ModelID is intentionally NOT used for
// price selection (model_id still flows through for metering attribution).
//
// Returns nil when there is no usable per-capability price (no capability set,
// or no matching entry with a positive rate). A matching entry with a
// non-positive rate is skipped rather than aborting the scan, so a later valid
// duplicate entry for the same constraint (e.g. due to misconfiguration) is
// still honored. Callers fall back to the base oInfo.PriceInfo when nil is
// returned, preserving today's behavior for unconfigured or misconfigured
// capabilities. This is a pure function (no flag, no IO) so it is hermetically
// testable without the CGO/ffmpeg toolchain.
func resolveByocPrice(req *RemotePaymentRequest, oInfo *net.OrchestratorInfo) *net.PriceInfo {
	if req == nil || oInfo == nil || req.Capability == "" {
		return nil
	}
	for _, p := range oInfo.CapabilitiesPrices {
		if p == nil {
			continue
		}
		if p.Capability != uint32(core.Capability_BYOC) || p.Constraint != req.Capability {
			continue
		}
		// Only honor a valid positive rate. Skip invalid/zero entries and keep
		// scanning so a later valid duplicate for the same constraint isn't
		// shadowed; if none is found we fall back to base (never zeroing the fee).
		if p.PricePerUnit <= 0 || p.PixelsPerUnit <= 0 {
			continue
		}
		return &net.PriceInfo{PricePerUnit: p.PricePerUnit, PixelsPerUnit: p.PixelsPerUnit}
	}
	return nil
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
	// BYOC per-capability pricing (flag-gated, default OFF). When enabled and the
	// gateway supplied a real capability, resolve the per-capability price from
	// the orchestrator's advertised CapabilitiesPrices and use it instead of the
	// flat base price. The resolved price is written back to oInfo.PriceInfo so it
	// is the single source for: state init, initialPrice, the max-price ceiling,
	// the payment's ExpectedPrice (which the orchestrator uses to set its fixed
	// per-session price), and the price-doubling guard in validatePrice (which
	// reads sess.OrchestratorInfo.PriceInfo) — keeping all of them cap-vs-cap
	// consistent. When the flag is OFF or no usable cap price matches, behavior is
	// byte-identical to the base-price path.
	priceInfo := oInfo.PriceInfo
	useByocPricing := false
	// Additionally gate on Type=="lv2v": enabling BYOC pricing changes the
	// billing basis (it overrides req.InPixels and bypasses lv2v pixel
	// synthesis), so it must only apply to the live (lv2v) job type that BYOC
	// jobs always use — never to a non-lv2v request that happens to carry a
	// capability.
	if ls.LivepeerNode.ByocPerCapPricing && req.Capability != "" && req.Type == RemoteType_LiveVideoToVideo {
		if capPrice := resolveByocPrice(&req, &oInfo); capPrice != nil {
			priceInfo = capPrice
			oInfo.PriceInfo = capPrice
			useByocPricing = true
		}
	}
	if priceInfo == nil || priceInfo.PricePerUnit == 0 || priceInfo.PixelsPerUnit == 0 {
		err := fmt.Errorf("missing or zero priceInfo")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	if oInfo.TicketParams == nil {
		err := fmt.Errorf("missing ticketParams in OrchestratorInfo")
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}

	orchAddr := ethcommon.BytesToAddress(oInfo.Address)

	// Load or initialize state
	var (
		state *RemotePaymentState
		err   error
	)
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

	manifestID := req.ManifestID
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
		w.Header().Set(RefreshSessionOrchestratorURLHeader, oInfo.Transcoder)
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
	if useByocPricing {
		// BYOC per-capability tariff is denominated per compute-second (wei/sec),
		// so charge on seconds directly rather than synthesizing lv2v pixels. With
		// pixels = ceil(billableSecs) and the resolved per-cap price kept as its
		// reduced wei/sec rational, calculateFee yields capPriceWeiPerSec * seconds.
		if billableSecs <= 0 {
			// preload with 60 seconds, mirroring the lv2v first-call behavior
			billableSecs = (60 * time.Second).Seconds()
		}
		pixels = int64(math.Ceil(billableSecs))
	} else if req.Type == RemoteType_LiveVideoToVideo {
		info := defaultSegInfo
		if billableSecs <= 0 {
			// preload with 60 seconds of data for LV2V
			billableSecs = (60 * time.Second).Seconds()
		}
		pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
		pixels = int64(pixelsPerSec * billableSecs) // pixels to charge for
	} else if req.Type != "" {
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

	callbackStatus, callbackResp, callbackErr := ls.authLivePayment(r, state)
	if callbackStatus != http.StatusOK {
		respondJsonError(ctx, w, callbackErr, callbackStatus)
		return
	}
	if callbackResp != nil {
		state.AuthExpiry = callbackResp.Expiry
	}
	authID := r.Header.Get(remoteSignerAuthIDHeader)
	if callbackResp != nil && callbackResp.AuthID != "" {
		authID = callbackResp.AuthID
	}
	if authID != "" && state.AuthID != authID {
		if state.AuthID != "" {
			clog.Warningf(ctx, "Remote signer auth ID changed oldAuthID=%s newAuthID=%s", state.AuthID, authID)
			respondJsonError(ctx, w, errors.New("remote signer auth ID changed"), http.StatusInternalServerError)
			return
		}
		state.AuthID = authID
	}
	ctx = clog.AddVal(ctx, "auth_id", state.AuthID)

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
		pipeline, modelID := resolveUsageLabels(&req, streamParams.Capabilities)
		// NB: This could could drop events if tha Kafka queue is full!
		monitor.SendQueueEventAsync("create_signed_ticket", map[string]interface{}{
			"session_id":         state.StateID,
			"session_status":     sessionStatus,
			"pipeline":           pipeline,
			"model_id":           modelID,
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
			"auth_id":            state.AuthID,
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
func GetOrchInfoSig(remoteSignerHost *url.URL, headers map[string]string) (*OrchInfoSigResponse, error) {

	url := remoteSignerHost.ResolveReference(&url.URL{Path: "/sign-orchestrator-info"})

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Make the request
	resp, err := client.Do(req)
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

// discoveryResponse is intentionally typed. Do NOT add raw json.RawMessage blobs
// here or pass through arbitrary orchestrator /discovery fields; every exposed
// response field must be reviewed and modeled explicitly.
type discoveryResponse struct {
	Address      string                             `json:"address,omitempty"`
	Score        float32                            `json:"score,omitempty"`
	Capabilities []string                           `json:"capabilities,omitempty"`
	Runners      []runner.LiveRunnerDiscoveryRunner `json:"runners,omitempty"`
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
		resp = append(resp, discoveryResponse{
			Address:      cached.URL.String(),
			Score:        common.Score_Trusted, // Legacy go-livepeer webhook field.
			Capabilities: append([]string(nil), cached.Capabilities...),
			Runners:      append([]runner.LiveRunnerDiscoveryRunner(nil), cached.Runners...),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
