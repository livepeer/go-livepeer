package server

// ai_http.go implements the HTTP server for AI-related requests at the Orchestrator.

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	url2 "net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	lpnet "github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/trickle"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

var MaxAIRequestSize = 3000000000 // 3GB

var TrickleHTTPPath = "/ai/trickle/"

const maxLiveRunnerTrickleChannelsPerRequest = 25
const liveRunnerSenderHeader = "Livepeer-Payer-Address"
const maxScopeRequestBodySize = 1 << 20 // 1 MB

func startAIServer(lp *lphttp) error {
	swagger, err := worker.GetSwagger()
	if err != nil {
		return err
	}
	swagger.Servers = nil

	opts := &middleware.Options{
		Options: openapi3filter.Options{
			ExcludeRequestBody: true,
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			clog.Errorf(context.Background(), "oapi validation error statusCode=%v message=%v", statusCode, message)
		},
	}
	oapiReqValidator := middleware.OapiRequestValidatorWithOptions(swagger, opts)

	openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)

	lp.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
		Mux:      lp.transRPC,
		BasePath: TrickleHTTPPath,
	})
	if sw, ok := lp.node.AIWorker.(*worker.ServerlessWorker); ok {
		sw.SetTrickleServer(lp.trickleSrv)
	}
	if manager, ok := lp.liveRunnerManager(); ok {
		publicTrickleBaseURL := lp.orchestrator.ServiceURI().JoinPath(TrickleHTTPPath).String()
		internalTrickleBaseURL := liveRunnerURI(lp.node, lp.orchestrator).JoinPath(TrickleHTTPPath).String()
		manager.SetTrickleServer(lp.trickleSrv, publicTrickleBaseURL, internalTrickleBaseURL)
	}

	lp.transRPC.Handle("/text-to-image", oapiReqValidator(aiHttpHandle(lp, jsonDecoder[worker.GenTextToImageJSONRequestBody])))
	lp.transRPC.Handle("/image-to-image", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenImageToImageMultipartRequestBody])))
	lp.transRPC.Handle("/image-to-video", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenImageToVideoMultipartRequestBody])))
	lp.transRPC.Handle("/upscale", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenUpscaleMultipartRequestBody])))
	lp.transRPC.Handle("/audio-to-text", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenAudioToTextMultipartRequestBody])))
	lp.transRPC.Handle("/llm", oapiReqValidator(aiHttpHandle(lp, jsonDecoder[worker.GenLLMJSONRequestBody])))
	lp.transRPC.Handle("/segment-anything-2", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenSegmentAnything2MultipartRequestBody])))
	lp.transRPC.Handle("/image-to-text", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenImageToTextMultipartRequestBody])))
	lp.transRPC.Handle("/text-to-speech", oapiReqValidator(aiHttpHandle(lp, jsonDecoder[worker.GenTextToSpeechJSONRequestBody])))
	lp.transRPC.Handle("/scope", lp.StartScope())
	lp.transRPC.Handle("/live-video-to-video", oapiReqValidator(lp.StartLiveVideoToVideo()))
	// Internal runner endpoints
	lp.transRPC.HandleFunc("POST /runners/heartbeat", lp.LiveRunnerHeartbeat)
	lp.transRPC.HandleFunc("POST /runners/{runner_id}/unregister", lp.UnregisterLiveRunner)
	lp.transRPC.HandleFunc("POST /runner/{runner_id}/session/{session_id}/channels", lp.CreateLiveRunnerTrickleChannel)
	lp.transRPC.HandleFunc("DELETE /runner/{runner_id}/session/{session_id}/channels", lp.DeleteLiveRunnerTrickleChannels)
	lp.transRPC.HandleFunc("POST /runner/{runner_id}/session/{session_id}/proxy", lp.CreateLiveRunnerSessionProxy)
	lp.transRPC.HandleFunc("POST /runner/{runner_id}/session/{session_id}/stop", lp.StopLiveRunnerSessionInternal)
	// Public client endpoints
	lp.transRPC.HandleFunc("GET /discovery", lp.DiscoverLiveRunners)
	lp.transRPC.HandleFunc("POST /apps/{runner_id}/session", lp.ReserveLiveRunnerSession)
	lp.transRPC.HandleFunc("POST /apps/{runner_id}/session/{session_id}/stop", lp.StopLiveRunnerSession)
	lp.transRPC.HandleFunc("POST /apps/{runner_id}/session/{session_id}/payment", lp.PaymentForLiveRunnerSession)
	lp.transRPC.HandleFunc("/apps/{runner_id}/session/{session_id}/app/{app_path...}", lp.ProxyLiveRunnerSession)
	lp.transRPC.HandleFunc("/apps/{runner_id}/app/{app_path...}", lp.ProxyLiveRunnerSingleShot)

	// Additionally, there is the '/aiResults' endpoint registered in server/rpc.go

	return nil
}

type liveRunnerManager interface {
	Heartbeat(req runner.LiveRunnerHeartbeatRequest, auth string) (*runner.LiveRunnerHeartbeatResponse, error)
	Unregister(runnerID, auth string) error
	Runners() []runner.LiveRunnerDiscoveryRunner
	PaymentInfo(runnerID string) (*runner.LiveRunnerPriceInfo, error)
	ReserveSession(runnerID string, sessionID ...string) (string, string, error)
	ReleaseSession(runnerID, sessionID string) error
	RunnerMode(runnerID string) (string, error)
	RunnerEndpointForSession(runnerID, sessionID string) (string, error)
	SessionTokenForSession(runnerID, sessionID string) (string, error)
	ValidSessionToken(runnerID, sessionID, token string) error
	SetTrickleServer(srv *trickle.Server, publicBaseURL, internalBaseURL string)
	CreateTrickleChannel(runnerID, sessionID, name, mimeType string) (runner.LiveRunnerTrickleChannel, error)
	DeleteTrickleChannel(runnerID, sessionID, name string) error
	CreateSessionProxy(runnerID, sessionID, targetURL string) (runner.LiveRunnerProxy, error)
	ResolveSessionProxy(host, path string) (runner.LiveRunnerProxyRoute, bool, error)
}

func (h *lphttp) liveRunnerManager() (liveRunnerManager, bool) {
	if h.node == nil {
		return nil, false
	}
	manager, ok := h.node.LiveRunnerManager.(liveRunnerManager)
	return manager, ok
}

func (h *lphttp) LiveRunnerHeartbeat(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	var req runner.LiveRunnerHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := manager.Heartbeat(req, r.Header.Get("Authorization"))
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJsonOk(w, data)
}

func (h *lphttp) UnregisterLiveRunner(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	if err := manager.Unregister(runnerID, r.Header.Get("Authorization")); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type liveRunnerSessionResponse struct {
	SessionID  string `json:"session_id"`
	AppURL     string `json:"app_url"`
	ControlURL string `json:"control_url"`
}

type liveRunnerPaymentChallengeResponse struct {
	PaymentParams string `json:"payment_params"`
	// Keep the URL in top-level JSON so clients do not need to parse the protobuf just to route payment.
	Orchestrator string `json:"orchestrator"`
	ManifestID   string `json:"manifest_id"`
}

type liveRunnerTrickleChannelRequest struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
}

type liveRunnerTrickleChannelsRequest struct {
	Channels []liveRunnerTrickleChannelRequest `json:"channels"`
}

type liveRunnerTrickleChannelsResponse struct {
	Channels []runner.LiveRunnerTrickleChannel `json:"channels"`
}

type deleteLiveRunnerTrickleChannelsRequest struct {
	Channels []string `json:"channels"`
}

type deleteLiveRunnerTrickleChannelsResponse struct {
	Deleted []string `json:"deleted"`
}

type liveRunnerProxyRequest struct {
	TargetURL string `json:"target_url"`
}

func (h *lphttp) ReserveLiveRunnerSession(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	priceInfo, err := manager.PaymentInfo(runnerID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	paymentRequired := priceInfo != nil
	if paymentRequired && r.Header.Get(paymentHeader) == "" && r.Header.Get(segmentHeader) == "" {
		h.runnerChallenge(w, r, priceInfo)
		return
	}
	var (
		payment   lpnet.Payment
		segData   *core.SegTranscodingMetadata
		ctx       = r.Context()
		sessionID string
	)
	if paymentRequired {
		var err error
		payment, segData, ctx, err = h.processPaymentAndSegmentHeaders(w, r)
		if err != nil {
			return
		}
		if string(segData.ManifestID) != segData.AuthToken.SessionId {
			respondWithError(w, "mismatched manifest and auth token", http.StatusForbidden)
			return
		}
		// for easier correlation across orch, gw, signer
		sessionID = string(segData.ManifestID)
	}
	sessionID, _, err = manager.ReserveSession(runnerID, sessionID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	ctx = clog.AddVal(ctx, "runner_id", runnerID)
	ctx = clog.AddVal(ctx, "session_id", sessionID)
	if paymentRequired {
		if err := h.orchestrator.ProcessPayment(ctx, payment, segData.ManifestID); err != nil {
			if releaseErr := manager.ReleaseSession(runnerID, sessionID); releaseErr != nil {
				clog.Errorf(ctx, "Error releasing live runner session after payment failure err=%v", releaseErr)
			}
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		monitorCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		paymentReceiver := livePaymentReceiver{orchestrator: h.orchestrator}
		accountPaymentFunc := func(units int64) error {
			err := paymentReceiver.AccountPayment(monitorCtx, &SegmentInfoReceiver{
				sender:    getPaymentSender(payment),
				units:     units,
				priceInfo: payment.GetExpectedPrice(),
				sessionID: string(segData.ManifestID),
			})
			if err != nil {
				clog.Errorf(monitorCtx, "Error accounting live runner payment, releasing session err=%v", err)
				if releaseErr := manager.ReleaseSession(runnerID, sessionID); releaseErr != nil {
					clog.Errorf(monitorCtx, "Error releasing live runner session after payment failure err=%v", releaseErr)
				}
				// Stop both the ticker loop below and the LivePaymentProcessor goroutine.
				cancel()
			}
			return err
		}
		paymentProcessor, err := newLiveRunnerPaymentProcessor(monitorCtx, h.node.LivePaymentInterval, priceInfo.Unit, accountPaymentFunc)
		if err != nil {
			cancel()
			if releaseErr := manager.ReleaseSession(runnerID, sessionID); releaseErr != nil {
				clog.Errorf(ctx, "Error releasing live runner session after payment setup failure err=%v", releaseErr)
			}
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		go func() {
			ticker := time.NewTicker(h.node.LivePaymentInterval)
			defer ticker.Stop()
			defer cancel()
			for {
				select {
				case <-ticker.C:
					// Stop monitoring once the live runner session has been released
					// by an explicit stop, runner cleanup, expiry, or payment failure.
					if _, err := manager.RunnerEndpointForSession(runnerID, sessionID); err != nil {
						return
					}
					paymentProcessor.process(monitorCtx)
				case <-monitorCtx.Done():
					return
				}
			}
		}()
	}
	controlURL := h.orchestrator.ServiceURI().JoinPath("apps", runnerID, "session", sessionID).String()
	appURL := h.orchestrator.ServiceURI().JoinPath("apps", runnerID, "session", sessionID, "app").String()
	data, err := json.Marshal(liveRunnerSessionResponse{SessionID: sessionID, AppURL: appURL, ControlURL: controlURL})
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJsonOk(w, data)
}

func newLiveRunnerPaymentProcessor(ctx context.Context, processInterval time.Duration, unit string, processUnitsFunc func(int64) error) (*LivePaymentProcessor, error) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "seconds":
		return NewLivePaymentProcessor(ctx, processInterval, processUnitsFunc), nil
	case "720p-pixel-seconds":
		return NewLV2VPaymentProcessor(ctx, processInterval, processUnitsFunc), nil
	default:
		return nil, fmt.Errorf("unsupported live runner payment unit %q", unit)
	}
}

func (h *lphttp) runnerChallenge(w http.ResponseWriter, r *http.Request, priceInfo *runner.LiveRunnerPriceInfo) {
	sender, err := h.runnerSender(r)
	if err != nil {
		respondJsonError(r.Context(), w, err, http.StatusPaymentRequired)
		return
	}
	oInfo, err := h.runnerOrchInfo(sender, priceInfo)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := marshalLivePaymentChallengeResponse(oInfo)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_, _ = w.Write(data)
}

func (h *lphttp) scopePaymentChallenge(w http.ResponseWriter, r *http.Request) (bool, error) {
	sender, err := h.runnerSender(r)
	if err != nil {
		respondJsonError(r.Context(), w, err, http.StatusPaymentRequired)
		return true, nil
	}
	constraints := core.NewCapabilities(nil, nil)
	if h.node != nil && h.node.Capabilities != nil {
		constraints = h.node.Capabilities
	}
	caps := newAICapabilities(core.Capability_LiveVideoToVideo, "scope", true, constraints)
	oInfo, err := orchestratorInfoWithCaps(h.orchestrator, sender, h.orchestrator.ServiceURI().String(), "", caps.ToNetCapabilities())
	if err != nil {
		return false, err
	}
	data, err := marshalLivePaymentChallengeResponse(oInfo)
	if err != nil {
		return false, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_, _ = w.Write(data)
	return true, nil
}

func marshalLivePaymentChallengeResponse(oInfo *lpnet.OrchestratorInfo) ([]byte, error) {
	buf, err := proto.Marshal(oInfo)
	if err != nil {
		return nil, err
	}
	return json.Marshal(liveRunnerPaymentChallengeResponse{
		PaymentParams: base64.StdEncoding.EncodeToString(buf),
		Orchestrator:  oInfo.GetTranscoder(),
		ManifestID:    oInfo.GetAuthToken().GetSessionId(),
	})
}

func (h *lphttp) processScopePayment(ctx context.Context, w http.ResponseWriter, r *http.Request) (ethcommon.Address, *lpnet.PriceInfo, string, bool, *runner.RunnerError) {
	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil || r.Header.Get(paymentHeader) == "" {
		handled, challengeErr := h.scopePaymentChallenge(w, r)
		if handled {
			return ethcommon.Address{}, nil, "", true, nil
		}
		if challengeErr != nil {
			return ethcommon.Address{}, nil, "", false, &runner.RunnerError{Message: challengeErr.Error(), StatusCode: http.StatusInternalServerError}
		}
		return ethcommon.Address{}, nil, "", false, &runner.RunnerError{Message: err.Error(), StatusCode: http.StatusPaymentRequired}
	}
	sender := getPaymentSender(payment)
	segData, ctx, err := verifySegCreds(ctx, h.orchestrator, r.Header.Get(segmentHeader), sender)
	if err != nil {
		return ethcommon.Address{}, nil, "", false, &runner.RunnerError{Message: err.Error(), StatusCode: http.StatusForbidden}
	}
	if string(segData.ManifestID) != segData.AuthToken.SessionId {
		clog.Info(ctx, "Legacy Scope payment manifest mismatch", "manifest_id", segData.ManifestID, "auth_session_id", segData.AuthToken.SessionId)
	}
	manifestID := string(segData.ManifestID)
	if err := h.orchestrator.ProcessPayment(ctx, payment, segData.ManifestID); err != nil {
		return ethcommon.Address{}, nil, "", false, &runner.RunnerError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}
	if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !h.orchestrator.SufficientBalance(sender, segData.ManifestID) {
		return ethcommon.Address{}, nil, "", false, &runner.RunnerError{Message: "Insufficient balance", StatusCode: http.StatusBadRequest}
	}
	return sender, payment.GetExpectedPrice(), manifestID, false, nil
}

func (h *lphttp) runnerSender(r *http.Request) (ethcommon.Address, error) {
	addr := r.Header.Get(liveRunnerSenderHeader)
	if !ethcommon.IsHexAddress(addr) {
		return ethcommon.Address{}, fmt.Errorf("invalid live runner payment signer address")
	}
	return ethcommon.HexToAddress(addr), nil
}

func (h *lphttp) runnerOrchInfo(sender ethcommon.Address, priceInfo *runner.LiveRunnerPriceInfo) (*lpnet.OrchestratorInfo, error) {
	if priceInfo == nil {
		return nil, fmt.Errorf("missing live runner price info")
	}
	pricePerUnit, err := priceInfo.Price.Int64()
	if err != nil || pricePerUnit <= 0 {
		return nil, fmt.Errorf("invalid live runner price info")
	}
	netPriceInfo := &lpnet.PriceInfo{
		PricePerUnit:  pricePerUnit,
		PixelsPerUnit: 1,
	}
	params, err := h.orchestrator.TicketParams(sender, netPriceInfo)
	if err != nil {
		return nil, err
	}
	manifestID := string(core.RandomManifestID())
	expiration := time.Now().Add(authTokenValidPeriod).Unix()
	authToken := h.orchestrator.AuthToken(manifestID, expiration)
	return &lpnet.OrchestratorInfo{
		Transcoder:   h.orchestrator.ServiceURI().String(),
		TicketParams: params,
		PriceInfo:    netPriceInfo,
		Address:      h.orchestrator.Address().Bytes(),
		AuthToken:    authToken,
	}, nil
}

func (h *lphttp) StopLiveRunnerSession(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	if err := manager.ReleaseSession(r.PathValue("runner_id"), r.PathValue("session_id")); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PaymentForLiveRunnerSession receives payment for a live runner session and
// rejects payment once the session is no longer live.
func (h *lphttp) PaymentForLiveRunnerSession(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	sessionID := r.PathValue("session_id")

	if _, err := manager.RunnerEndpointForSession(runnerID, sessionID); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}

	payment, segData, ctx, err := h.processPaymentAndSegmentHeaders(w, r)
	if err != nil {
		return
	}
	if string(segData.ManifestID) != sessionID {
		respondWithError(w, "mismatched session and payment manifest", http.StatusForbidden)
		return
	}

	var netCaps *lpnet.Capabilities
	if segData.Caps != nil {
		netCaps = segData.Caps.ToNetCapabilities()
	}
	oInfo, err := orchestratorInfoWithCaps(h.orchestrator, getPaymentSender(payment), h.orchestrator.ServiceURI().String(), core.ManifestID(segData.AuthToken.SessionId), netCaps)
	if err != nil {
		clog.Errorf(ctx, "Error updating orchestrator info - err=%q", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	oInfo.AuthToken = segData.AuthToken

	if err := h.orchestrator.ProcessPayment(ctx, payment, segData.ManifestID); err != nil {
		clog.Errorf(ctx, "error processing payment: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	buf, err := proto.Marshal(&lpnet.PaymentResult{Info: oInfo})
	if err != nil {
		clog.Errorf(ctx, "Unable to marshal payment result err=%q", err)
		return
	}
	clog.V(common.DEBUG).Infof(ctx, "Live runner session payment processed, current balance=%s", currentBalanceLog(h, payment, segData))

	w.Write(buf)
}

func (h *lphttp) StopLiveRunnerSessionInternal(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	sessionID := r.PathValue("session_id")
	if err := manager.ValidSessionToken(runnerID, sessionID, r.Header.Get("Livepeer-Session-Token")); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	if err := manager.ReleaseSession(runnerID, sessionID); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *lphttp) CreateLiveRunnerTrickleChannel(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	sessionID := r.PathValue("session_id")
	if err := manager.ValidSessionToken(runnerID, sessionID, r.Header.Get("Livepeer-Session-Token")); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}

	var req liveRunnerTrickleChannelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Channels) == 0 {
		respondWithError(w, "channels is required", http.StatusBadRequest)
		return
	}
	if len(req.Channels) > maxLiveRunnerTrickleChannelsPerRequest {
		respondWithError(w, fmt.Sprintf("channels cannot contain more than %d entries", maxLiveRunnerTrickleChannelsPerRequest), http.StatusBadRequest)
		return
	}

	channels := make([]runner.LiveRunnerTrickleChannel, 0, len(req.Channels))
	for _, channelReq := range req.Channels {
		channel, err := manager.CreateTrickleChannel(
			runnerID,
			sessionID,
			channelReq.Name,
			channelReq.MimeType,
		)
		if err != nil {
			respondWithLiveRunnerError(w, err)
			return
		}
		channels = append(channels, channel)
	}
	data, err := json.Marshal(liveRunnerTrickleChannelsResponse{Channels: channels})
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJsonOk(w, data)
}

func (h *lphttp) DeleteLiveRunnerTrickleChannels(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	sessionID := r.PathValue("session_id")
	if err := manager.ValidSessionToken(runnerID, sessionID, r.Header.Get("Livepeer-Session-Token")); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}

	var req deleteLiveRunnerTrickleChannelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Channels) == 0 {
		respondWithError(w, "channels is required", http.StatusBadRequest)
		return
	}
	if len(req.Channels) > maxLiveRunnerTrickleChannelsPerRequest {
		respondWithError(w, fmt.Sprintf("channels cannot contain more than %d entries", maxLiveRunnerTrickleChannelsPerRequest), http.StatusBadRequest)
		return
	}
	for _, channelName := range req.Channels {
		if err := manager.DeleteTrickleChannel(runnerID, sessionID, channelName); err != nil {
			respondWithLiveRunnerError(w, err)
			return
		}
	}
	data, err := json.Marshal(deleteLiveRunnerTrickleChannelsResponse{Deleted: req.Channels})
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJsonOk(w, data)
}

func (h *lphttp) CreateLiveRunnerSessionProxy(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}
	runnerID := r.PathValue("runner_id")
	sessionID := r.PathValue("session_id")
	if err := manager.ValidSessionToken(runnerID, sessionID, r.Header.Get("Livepeer-Session-Token")); err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}

	var req liveRunnerProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}
	proxy, err := manager.CreateSessionProxy(runnerID, sessionID, req.TargetURL)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	data, err := json.Marshal(proxy)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJsonOk(w, data)
}

func (h *lphttp) ProxyLiveRunnerSession(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}

	runnerID := r.PathValue("runner_id")
	sessionID := r.PathValue("session_id")
	endpoint, err := manager.RunnerEndpointForSession(runnerID, sessionID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	sessionToken, err := manager.SessionTokenForSession(runnerID, sessionID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}

	h.proxyLiveRunner(w, r, runnerID, sessionID, sessionToken, endpoint, r.PathValue("app_path"))
}

func (h *lphttp) ProxyLiveRunnerSingleShot(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	if !ok {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}

	runnerID := r.PathValue("runner_id")
	mode, err := manager.RunnerMode(runnerID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	if mode != runner.LiveRunnerModeSingleShot {
		respondWithError(w, "runner is not single-shot", http.StatusBadRequest)
		return
	}

	sessionID, endpoint, err := manager.ReserveSession(runnerID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	defer func() {
		if err := manager.ReleaseSession(runnerID, sessionID); err != nil {
			slog.Error("error releasing single-shot session", "runner_id", runnerID, "session_id", sessionID, "err", err)
		}
	}()

	sessionToken, err := manager.SessionTokenForSession(runnerID, sessionID)
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return
	}
	h.proxyLiveRunner(w, r, runnerID, sessionID, sessionToken, endpoint, r.PathValue("app_path"))
}

func (h *lphttp) tryLiveRunnerProxy(w http.ResponseWriter, r *http.Request) bool {
	manager, ok := h.liveRunnerManager()
	if !ok {
		return false
	}
	route, matched, err := manager.ResolveSessionProxy(r.Host, r.URL.Path)
	if !matched {
		return false
	}
	if err != nil {
		respondWithLiveRunnerError(w, err)
		return true
	}
	h.proxyLiveRunner(w, r, route.RunnerID, route.SessionID, route.SessionToken, route.TargetURL, route.AppPath)
	return true
}

func (h *lphttp) proxyLiveRunner(w http.ResponseWriter, r *http.Request, runnerID, sessionID, sessionToken, endpoint, appPath string) {
	target, err := url2.Parse(endpoint)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusBadGateway)
		return
	}
	proxyPath, err := url2.JoinPath("/", target.Path, appPath)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusBadGateway)
		return
	}
	sessionControlURL := liveRunnerURI(h.node, h.orchestrator).JoinPath("runner", runnerID, "session", sessionID).String()
	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			req.Out.URL.Scheme = target.Scheme
			req.Out.URL.Host = target.Host
			req.Out.URL.Path = proxyPath
			req.Out.URL.RawPath = ""
			req.Out.URL.RawQuery = req.In.URL.RawQuery
			req.Out.Host = target.Host
			req.Out.Header.Set("Livepeer-Runner-Route", runnerID)
			req.Out.Header.Set("Livepeer-Session-Id", sessionID)
			req.Out.Header.Set("Livepeer-Session-Token", sessionToken)
			req.Out.Header.Set("Livepeer-Session-Control", sessionControlURL)
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			respondWithError(w, err.Error(), http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func respondWithLiveRunnerError(w http.ResponseWriter, err error) {
	var runnerErr *runner.RunnerError
	if errors.As(err, &runnerErr) {
		respondWithError(w, runnerErr.Error(), runnerErr.StatusCode)
		return
	}
	respondWithError(w, err.Error(), http.StatusInternalServerError)
}

type liveRunnerDiscoveryEntry struct {
	Address string                             `json:"address,omitempty"`
	Runners []runner.LiveRunnerDiscoveryRunner `json:"runners,omitempty"`
}

func (h *lphttp) DiscoverLiveRunners(w http.ResponseWriter, r *http.Request) {
	manager, ok := h.liveRunnerManager()
	var hasServerlessWorker bool
	if h.node != nil {
		_, hasServerlessWorker = h.node.AIWorker.(*worker.ServerlessWorker)
	}
	if !ok && !hasServerlessWorker {
		respondWithError(w, "live runners are not supported", http.StatusNotFound)
		return
	}

	suri := h.orchestrator.ServiceURI()

	var runners []runner.LiveRunnerDiscoveryRunner
	if ok {
		runners = append(runners, manager.Runners()...)
	}
	if hasServerlessWorker {
		pipeline := "live-video-to-video"
		capability := core.Capability_LiveVideoToVideo
		var capConstraints *core.CapabilityConstraints
		if h.node.Capabilities != nil {
			capConstraints = h.node.Capabilities.PerCapability()[capability]
		}

		versionFallback := ""
		versions := h.node.AIWorker.Version()
		for _, version := range versions {
			if versionFallback == "" && version.Version != "" {
				versionFallback = version.Version
			}
		}

		if capConstraints != nil {
			for modelID := range capConstraints.Models {
				versionString := versionFallback
				for _, version := range versions {
					if version.Pipeline == pipeline && version.ModelId == modelID && version.Version != "" {
						versionString = version.Version
						break
					}
				}

				var priceInfo *runner.LiveRunnerPriceInfo
				price := h.node.GetBasePriceForCap("default", capability, modelID)
				if price != nil {
					priceInt64, err := common.PriceToInt64(price)
					if err != nil {
						glog.Errorf("error converting discovery price for capability %v modelID=%v err=%v", core.CapabilityNameLookup[capability], modelID, err)
					} else {
						priceInfo = &runner.LiveRunnerPriceInfo{
							Price:    json.Number(strconv.FormatInt(priceInt64.Num().Int64(), 10)),
							Currency: "wei",
							Unit:     "720p-pixel-seconds",
						}
					}
				}

				capacity := h.node.AIWorker.GetLiveAICapacity(pipeline, modelID)
				runners = append(runners, runner.LiveRunnerDiscoveryRunner{
					URL:               suri.JoinPath("scope").String(),
					GPU:               &runner.LiveRunnerGPU{Name: "H100"},
					App:               pipeline + "/" + modelID,
					Version:           versionString,
					Capacity:          capacity.ContainersIdle + capacity.ContainersInUse,
					CapacityUsed:      capacity.ContainersInUse,
					CapacityAvailable: capacity.ContainersIdle,
					PriceInfo:         priceInfo,
				})
			}
		}
	}

	resp := []liveRunnerDiscoveryEntry{{
		Address: suri.String(),
		Runners: runners,
	}}
	data, err := json.Marshal(resp)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJsonOk(w, data)
}

// aiHttpHandle handles AI requests by decoding the request body and processing it.
func aiHttpHandle[I any](h *lphttp, decoderFunc func(*I, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req I
		if err := decoderFunc(&req, r); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func serverlessHandshakeHTTPStatus(err error) (int, bool) {
	var hsErr *worker.ServerlessHandshakeError
	if !errors.As(err, &hsErr) {
		return 0, false
	}
	switch hsErr.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return hsErr.StatusCode, true
	default:
		return 0, false
	}
}

func (h *lphttp) StartLiveVideoToVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		streamID := r.Header.Get("streamID")
		gatewayRequestID := r.Header.Get("requestID")
		requestID := string(core.RandomManifestID())

		var req worker.GenLiveVideoToVideoJSONRequestBody
		if err := jsonDecoder(&req, r); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.GatewayRequestId != nil && *req.GatewayRequestId != "" {
			gatewayRequestID = *req.GatewayRequestId
		}
		if req.StreamId != nil && *req.StreamId != "" {
			streamID = *req.StreamId
		}
		ctx = clog.AddVal(ctx, "request_id", gatewayRequestID)
		ctx = clog.AddVal(ctx, "manifest_id", requestID)
		ctx = clog.AddVal(ctx, "stream_id", streamID)

		orch := h.orchestrator
		pipeline := "live-video-to-video"
		cap := core.Capability_LiveVideoToVideo
		modelID := *req.ModelId
		clog.Info(ctx, "Received request", "cap", cap, "modelID", modelID)

		// Create storage for the request (for AI Workers, must run before CheckAICapacity)
		err := orch.CreateStorageForRequest(requestID)
		if err != nil {
			respondWithError(w, "Could not create storage to receive results", http.StatusInternalServerError)
		}

		// Check if there is capacity for the request
		hasCapacity, _ := orch.CheckAICapacity(pipeline, modelID)
		if !hasCapacity {
			clog.Errorf(ctx, "Insufficient capacity for pipeline=%v modelID=%v", pipeline, modelID)
			respondWithError(w, "insufficient capacity", http.StatusServiceUnavailable)
			return
		}

		// Start trickle server for live-video
		var (
			mid        = requestID // Request ID is used for the manifest ID
			pubUrl     = orch.ServiceURI().JoinPath(TrickleHTTPPath, mid).String()
			subUrl     = pubUrl + "-out"
			controlUrl = pubUrl + "-control"
			eventsUrl  = pubUrl + "-events"
		)

		// Handle initial payment, the rest of the payments are done separately from the stream processing
		// Note that this payment is debit from the balance and acts as a buffer for the AI Realtime Video processing
		payment, err := getPayment(r.Header.Get(paymentHeader))
		if err != nil {
			respondWithError(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		sender := getPaymentSender(payment)
		_, ctx, err = verifySegCreds(ctx, h.orchestrator, r.Header.Get(segmentHeader), sender)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusForbidden)
			return
		}
		if err := orch.ProcessPayment(ctx, payment, core.ManifestID(mid)); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, core.ManifestID(mid)) {
			respondWithError(w, "Insufficient balance", http.StatusBadRequest)
			return
		}

		//If successful, then create the trickle channels
		// Precreate the channels to avoid race conditions
		// TODO get the expected mime type from the request
		pubCh := trickle.NewLocalPublisher(h.trickleSrv, mid, "video/MP2T")
		pubCh.CreateChannel()
		subCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-out", "video/MP2T")
		subCh.CreateChannel()
		controlPubCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-control", "application/json")
		controlPubCh.CreateChannel()
		eventsCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-events", "application/json")
		eventsCh.CreateChannel()

		ctx, cancel := context.WithCancel(ctx)
		closeSession := func() {
			pubCh.Close()
			subCh.Close()
			eventsCh.Close()
			controlPubCh.Close()
			cancel()
		}

		// Start payment receiver which accounts the payments and stops the stream if the payment is insufficient
		priceInfo := payment.GetExpectedPrice()
		var paymentProcessor *LivePaymentProcessor
		if priceInfo != nil && priceInfo.PricePerUnit != 0 {
			paymentReceiver := livePaymentReceiver{orchestrator: h.orchestrator}
			accountPaymentFunc := func(inPixels int64) error {
				err := paymentReceiver.AccountPayment(ctx, &SegmentInfoReceiver{
					sender:    sender,
					units:     inPixels,
					priceInfo: priceInfo,
					sessionID: mid,
				})
				if err != nil {
					clog.Errorf(ctx, "Error accounting payment, stopping stream processing", err)
					closeSession()
				}
				return err
			}
			paymentProcessor = NewLV2VPaymentProcessor(ctx, h.node.LivePaymentInterval, accountPaymentFunc)
		} else {
			clog.Warningf(ctx, "No price info found for model %v, Orchestrator will not charge for video processing", modelID)
		}

		// For every segment, check payments
		go func() {
			sub := trickle.NewLocalSubscriber(h.trickleSrv, mid)
			for {
				// Set seq to next segment in case the subscriber is outside
				// the server's retention window
				sub.SetSeq(-1)
				segment, err := sub.Read()
				if err != nil {
					clog.Infof(ctx, "Error getting local trickle segment err=%v", err)
					closeSession()
					return
				}
				if paymentProcessor != nil {
					paymentProcessor.process(ctx)
				}
				// read the segment so we know when it is complete, otherwise sub.Read()
				// would rapidly request follow-on segments that do not yet exist
				io.Copy(io.Discard, segment.Reader)
			}
		}()

		// Prepare request to worker
		controlUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, controlUrl)
		eventsUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, eventsUrl)
		subscribeUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, pubUrl)
		publishUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, subUrl)

		workerReq := worker.LiveVideoToVideoParams{
			ModelId:          req.ModelId,
			PublishUrl:       publishUrlOverwrite,
			SubscribeUrl:     subscribeUrlOverwrite,
			EventsUrl:        &eventsUrlOverwrite,
			ControlUrl:       &controlUrlOverwrite,
			Params:           req.Params,
			GatewayRequestId: &gatewayRequestID,
			ManifestId:       &mid,
			StreamId:         &streamID,
		}

		// Send request to the worker
		_, err = orch.LiveVideoToVideo(ctx, requestID, workerReq)
		if err != nil {
			if monitor.Enabled {
				monitor.AIProcessingError(err.Error(), pipeline, modelID, ethcommon.Address{}.String())
			}

			closeSession()
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare the response
		jsonData, err := json.Marshal(&worker.LiveVideoToVideoResponse{
			PublishUrl:   pubUrl,
			SubscribeUrl: subUrl,
			ControlUrl:   &controlUrl,
			EventsUrl:    &eventsUrl,
			ManifestId:   &mid,
		})
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			closeSession()
			return
		}

		took := time.Since(startTime)
		clog.Info(ctx, "Processed request", "cap", cap, "modelID", modelID, "took", took)
		respondJsonOk(w, jsonData)
	})
}

func (h *lphttp) StartScope() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		gatewayRequestID := r.Header.Get("requestID")

		var req worker.GenLiveVideoToVideoJSONRequestBody
		r.Body = http.MaxBytesReader(w, r.Body, maxScopeRequestBodySize)
		if err := jsonDecoder(&req, r); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.GatewayRequestId != nil && *req.GatewayRequestId != "" {
			gatewayRequestID = *req.GatewayRequestId
		}
		ctx = clog.AddVal(ctx, "request_id", gatewayRequestID)
		ctx = clog.AddVal(ctx, "app", "scope")
		orch := h.orchestrator
		pipeline := "live-video-to-video"
		modelID := "scope"

		var (
			manifestID string
			sender     ethcommon.Address
			priceInfo  *lpnet.PriceInfo
		)
		if h.node != nil && h.node.Eth != nil {
			payer, price, paidManifestID, handled, httpErr := h.processScopePayment(ctx, w, r)
			if handled {
				return
			}
			if httpErr != nil {
				respondWithError(w, httpErr.Error(), httpErr.StatusCode)
				return
			}
			manifestID = paidManifestID
			sender = payer
			priceInfo = price
		} else {
			manifestID = string(core.RandomManifestID())
		}

		ctx = clog.AddVal(ctx, "manifest_id", manifestID)
		clog.Info(ctx, "Received Scope request")

		// Create storage for the request (for AI Workers, must run before CheckAICapacity)
		err := orch.CreateStorageForRequest(manifestID)
		if err != nil {
			respondWithError(w, "Could not create storage to receive results", http.StatusInternalServerError)
			return
		}

		// Check if there is capacity for the request
		hasCapacity, _ := orch.CheckAICapacity(pipeline, modelID)
		if !hasCapacity {
			clog.Errorf(ctx, "Insufficient capacity for pipeline=%v modelID=%v", pipeline, modelID)
			respondWithError(w, "insufficient capacity", http.StatusServiceUnavailable)
			return
		}

		// Start trickle server for scope
		baseURL := orch.ServiceURI().JoinPath(TrickleHTTPPath, manifestID).String()
		controlURL := baseURL + "-control"
		eventsURL := baseURL + "-events"

		// Scope only pre-creates control + events channels.
		controlPubCh := trickle.NewLocalPublisher(h.trickleSrv, manifestID+"-control", "application/json")
		controlPubCh.CreateChannel()
		eventsCh := trickle.NewLocalPublisher(h.trickleSrv, manifestID+"-events", "application/json")
		eventsCh.CreateChannel()

		ctx, cancel := context.WithCancel(ctx)
		closeSession := func() {
			eventsCh.Close()
			controlPubCh.Close()
			cancel()
		}

		// Start payment receiver which accounts the payments and stops the stream if the payment is insufficient
		var paymentProcessor *LivePaymentProcessor
		if priceInfo != nil && priceInfo.PricePerUnit != 0 {
			paymentReceiver := livePaymentReceiver{orchestrator: h.orchestrator}
			accountPaymentFunc := func(inPixels int64) error {
				err := paymentReceiver.AccountPayment(ctx, &SegmentInfoReceiver{
					sender:    sender,
					units:     inPixels,
					priceInfo: priceInfo,
					sessionID: manifestID,
				})
				if err != nil {
					clog.Errorf(ctx, "Error accounting payment, stopping stream processing", err)
					closeSession()
				}
				return err
			}
			paymentProcessor = NewLV2VPaymentProcessor(ctx, h.node.LivePaymentInterval, accountPaymentFunc)
		} else {
			clog.Warningf(ctx, "No price info found for model %v, Orchestrator will not charge for video processing", modelID)
		}

		// For every event segment, check payments
		go func() {
			sub := trickle.NewLocalSubscriber(h.trickleSrv, manifestID+"-events")
			for {
				// Set seq to next segment in case the subscriber is outside
				// the server's retention window
				sub.SetSeq(-1)
				segment, err := sub.Read()
				if err != nil {
					clog.Infof(ctx, "Error getting local trickle segment err=%v", err)
					closeSession()
					return
				}
				if paymentProcessor != nil {
					paymentProcessor.process(ctx)
				}
				// read the segment so we know when it is complete, otherwise sub.Read()
				// would rapidly request follow-on segments that do not yet exist
				io.Copy(io.Discard, segment.Reader)
			}
		}()

		// Prepare request to worker
		workerControlURL := overwriteHost(h.node.LiveAITrickleHostForRunner, controlURL)
		workerEventsURL := overwriteHost(h.node.LiveAITrickleHostForRunner, eventsURL)

		workerReq := worker.LiveVideoToVideoParams{
			EventsUrl:        &workerEventsURL,
			ControlUrl:       &workerControlURL,
			Params:           req.Params,
			GatewayRequestId: &gatewayRequestID,
			ManifestId:       &manifestID,
		}

		_, err = orch.LiveVideoToVideo(ctx, manifestID, workerReq)
		if err != nil {
			if monitor.Enabled {
				monitor.AIProcessingError(err.Error(), pipeline, modelID, ethcommon.Address{}.String())
			}

			closeSession()
			if statusCode, ok := serverlessHandshakeHTTPStatus(err); ok {
				respondWithError(w, err.Error(), statusCode)
			} else {
				respondWithError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		jsonData, err := json.Marshal(&worker.LiveVideoToVideoResponse{
			ControlUrl: &controlURL,
			EventsUrl:  &eventsURL,
			ManifestId: &manifestID,
		})
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			closeSession()
			return
		}

		took := time.Since(startTime)
		clog.Info(ctx, "Processed Scope request", "took", took)
		respondJsonOk(w, jsonData)
	})
}

func liveRunnerURI(node *core.LivepeerNode, orch Orchestrator) *url2.URL {
	if node != nil && node.LiveRunnerAddr != nil {
		v := *node.LiveRunnerAddr
		return &v
	}
	return orch.ServiceURI()
}

// overwriteHost is used to overwrite the trickle host, because it may be different for runner
// runner may run inside Docker container, in a different network, or even on a different machine
func overwriteHost(hostOverwrite, url string) string {
	if hostOverwrite == "" {
		return url
	}
	u, err := url2.ParseRequestURI(url)
	if err != nil {
		slog.Warn("Couldn't parse url to overwrite for worker, using original url", "url", url, "err", err)
		return url
	}
	u.Host = hostOverwrite
	return u.String()
}

func handleAIRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, orch Orchestrator, req interface{}) {
	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil {
		respondWithError(w, err.Error(), http.StatusPaymentRequired)
		return
	}
	sender := getPaymentSender(payment)

	_, ctx, err = verifySegCreds(ctx, orch, r.Header.Get(segmentHeader), sender)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusForbidden)
		return
	}

	requestID := string(core.RandomManifestID())

	var cap core.Capability
	var pipeline string
	var modelID string
	var submitFn func(context.Context) (interface{}, error)
	var outPixels int64

	switch v := req.(type) {
	case worker.GenTextToImageJSONRequestBody:
		pipeline = "text-to-image"
		cap = core.Capability_TextToImage
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.TextToImage(ctx, requestID, v)
		}

		// TODO: The orchestrator should require the broadcaster to always specify a height and width
		height := int64(512)
		if v.Height != nil {
			height = int64(*v.Height)
		}
		width := int64(512)
		if v.Width != nil {
			width = int64(*v.Width)
		}
		// NOTE: Should be enforced by the gateway, added for backwards compatibility.
		numImages := int64(1)
		if v.NumImagesPerPrompt != nil {
			numImages = int64(*v.NumImagesPerPrompt)
		}

		outPixels = height * width * numImages
	case worker.GenImageToImageMultipartRequestBody:
		pipeline = "image-to-image"
		cap = core.Capability_ImageToImage
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToImage(ctx, requestID, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		// NOTE: Should be enforced by the gateway, added for backwards compatibility.
		numImages := int64(1)
		if v.NumImagesPerPrompt != nil {
			numImages = int64(*v.NumImagesPerPrompt)
		}

		outPixels = int64(config.Height) * int64(config.Width) * numImages
	case worker.GenUpscaleMultipartRequestBody:
		pipeline = "upscale"
		cap = core.Capability_Upscale
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.Upscale(ctx, requestID, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		outPixels = int64(config.Height) * int64(config.Width)
	case worker.GenImageToVideoMultipartRequestBody:
		pipeline = "image-to-video"
		cap = core.Capability_ImageToVideo
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToVideo(ctx, requestID, v)
		}

		// TODO: The orchestrator should require the broadcaster to always specify a height and width
		height := int64(576)
		if v.Height != nil {
			height = int64(*v.Height)
		}
		width := int64(1024)
		if v.Width != nil {
			width = int64(*v.Width)
		}
		// The # of frames outputted by stable-video-diffusion-img2vid-xt models
		frames := int64(25)

		outPixels = height * width * int64(frames)
	case worker.GenAudioToTextMultipartRequestBody:
		pipeline = "audio-to-text"
		cap = core.Capability_AudioToText
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.AudioToText(ctx, requestID, v)
		}

		outPixels, err = common.CalculateAudioDuration(v.Audio)
		if err != nil {
			respondWithError(w, "Unable to calculate duration", http.StatusBadRequest)
			return
		}
		outPixels *= 1000 // Convert to milliseconds
	case worker.GenLLMJSONRequestBody:
		pipeline = "llm"
		cap = core.Capability_LLM
		modelID = *v.Model
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.LLM(ctx, requestID, v)
		}

		if v.MaxTokens == nil {
			respondWithError(w, "MaxTokens not specified", http.StatusBadRequest)
			return
		}

		// TODO: Improve pricing
		outPixels = int64(*v.MaxTokens)
	case worker.GenSegmentAnything2MultipartRequestBody:
		pipeline = "segment-anything-2"
		cap = core.Capability_SegmentAnything2
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.SegmentAnything2(ctx, requestID, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		outPixels = int64(config.Height) * int64(config.Width)
	case worker.GenImageToTextMultipartRequestBody:
		pipeline = "image-to-text"
		cap = core.Capability_ImageToText
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToText(ctx, requestID, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		outPixels = int64(config.Height) * int64(config.Width)
	case worker.GenTextToSpeechJSONRequestBody:
		pipeline = "text-to-speech"
		cap = core.Capability_TextToSpeech
		modelID = *v.ModelId

		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.TextToSpeech(ctx, requestID, v)
		}

		// TTS pricing is typically in characters, including punctuation.
		words := utf8.RuneCountInString(*v.Text)
		outPixels = int64(1000 * words)
	default:
		respondWithError(w, "Unknown request type", http.StatusBadRequest)
		return
	}

	clog.V(common.VERBOSE).Infof(ctx, "Received request id=%v cap=%v modelID=%v", requestID, cap, modelID)

	manifestID := core.ManifestID(strconv.Itoa(int(cap)) + "_" + modelID)

	// Check if there is capacity for the request.
	// Capability capacity is reserved if available and released when response is received
	hasCapacity, releaseCapacity := orch.CheckAICapacity(pipeline, modelID)
	if !hasCapacity {
		clog.Errorf(ctx, "Insufficient capacity for pipeline=%v modelID=%v", pipeline, modelID)
		respondWithError(w, "insufficient capacity", http.StatusServiceUnavailable)
		return
	}
	// Known limitation:
	// This call will set a fixed price for all requests in a session identified by a manifestID.
	// Since all requests for a capability + modelID are treated as "session" with a single manifestID, all
	// requests for a capability + modelID will get the same fixed price for as long as the orch is running
	if err := orch.ProcessPayment(ctx, payment, manifestID); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		releaseCapacity <- true
		return
	}

	if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, manifestID) {
		respondWithError(w, "Insufficient balance", http.StatusBadRequest)
		//release capacity, payment issue is not orchestrator's fault
		releaseCapacity <- true
		return
	}

	err = orch.CreateStorageForRequest(requestID)
	if err != nil {
		respondWithError(w, "Could not create storage to receive results", http.StatusInternalServerError)
		//do not release capacity, this is an unrecoverable error
		return
	}
	// Note: At the moment, we do not return a new OrchestratorInfo with updated ticket params + price with
	// extended expiry because the response format does not include such a field. As a result, the broadcaster
	// might encounter an expiration error for ticket params + price when it is using an old OrchestratorInfo returned
	// by the orch during discovery. In that scenario, the broadcaster can use a GetOrchestrator() RPC call to get a
	// a new OrchestratorInfo before submitting a request.

	start := time.Now()
	resp, err := submitFn(ctx)

	if err != nil {
		if monitor.Enabled {
			monitor.AIProcessingError(err.Error(), pipeline, modelID, sender.Hex())
		}
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		releaseCapacity <- true
		return
	}

	//backwards compatibility to old gateway api
	//Gateway version through v0.7.9-ai.3 expects to receive base64 encoded images as results for text-to-image, image-to-image, and upscale pipelines
	//The gateway now adds the protoVerAIWorker header to the request to indicate what version of the gateway is making the request
	//UPDATE this logic as the communication protocol between the gateway and orchestrator is updated
	if pipeline == "text-to-image" || pipeline == "image-to-image" || pipeline == "upscale" {
		if r.Header.Get("Authorization") != protoVerAIWorker {
			imgResp := resp.(worker.ImageResponse)
			prefix := "data:image/png;base64," //https://github.com/livepeer/ai-worker/blob/78b58131f12867ce5a4d0f6e2b9038e70de5c8e3/runner/app/routes/util.py#L56
			storage, exists := orch.GetStorageForRequest(requestID)
			if exists {
				for i, image := range imgResp.Images {
					fileData, err := storage.ReadData(ctx, image.Url)
					if err == nil {
						clog.V(common.VERBOSE).Infof(ctx, "replacing response with base64 for gateway on older api gateway_api=%v", r.Header.Get("Authorization"))
						data, _ := io.ReadAll(fileData.Body)
						imgResp.Images[i].Url = prefix + base64.StdEncoding.EncodeToString(data)
					} else {
						glog.Error(err)
					}
				}
			}
			//return the modified response
			resp = imgResp
		}
	}

	took := time.Since(start)
	clog.Infof(ctx, "Processed request id=%v cap=%v modelID=%v took=%v", requestID, cap, modelID, took)

	// At the moment, outPixels is expected to just be height * width * frames
	// If the # of inference/denoising steps becomes configurable, a possible updated formula could be height * width * frames * steps
	// If additional parameters that influence compute cost become configurable, then the formula should be reconsidered
	orch.DebitFees(sender, manifestID, payment.GetExpectedPrice(), outPixels)

	if monitor.Enabled {
		var latencyScore float64
		switch v := req.(type) {
		case worker.GenTextToImageJSONRequestBody:
			latencyScore = CalculateTextToImageLatencyScore(took, v, outPixels)
		case worker.GenImageToImageMultipartRequestBody:
			latencyScore = CalculateImageToImageLatencyScore(took, v, outPixels)
		case worker.GenImageToVideoMultipartRequestBody:
			latencyScore = CalculateImageToVideoLatencyScore(took, v, outPixels)
		case worker.GenUpscaleMultipartRequestBody:
			latencyScore = CalculateUpscaleLatencyScore(took, v, outPixels)
		case worker.GenAudioToTextMultipartRequestBody:
			durationSeconds, err := common.CalculateAudioDuration(v.Audio)
			if err == nil {
				latencyScore = CalculateAudioToTextLatencyScore(took, durationSeconds)
			}
		case worker.GenSegmentAnything2MultipartRequestBody:
			latencyScore = CalculateSegmentAnything2LatencyScore(took, outPixels)
		case worker.GenImageToTextMultipartRequestBody:
			latencyScore = CalculateImageToTextLatencyScore(took, outPixels)
		case worker.GenTextToSpeechJSONRequestBody:
			latencyScore = CalculateTextToSpeechLatencyScore(took, outPixels)
		}

		var pricePerAIUnit float64
		if priceInfo := payment.GetExpectedPrice(); priceInfo != nil && priceInfo.GetPixelsPerUnit() != 0 {
			pricePerAIUnit = float64(priceInfo.GetPricePerUnit()) / float64(priceInfo.GetPixelsPerUnit())
		}

		monitor.AIJobProcessed(ctx, pipeline, modelID, monitor.AIJobInfo{LatencyScore: latencyScore, PricePerUnit: pricePerAIUnit})
	}

	// Check if the response is a streaming response
	if streamChan, ok := resp.(<-chan *worker.LLMResponse); ok {
		glog.Infof("Streaming response for request id=%v", requestID)

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			releaseCapacity <- true
			return
		}

		for chunk := range streamChan {
			data, err := json.Marshal(chunk)
			if err != nil {
				clog.Errorf(ctx, "Error marshaling stream chunk: %v", err)
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
				break
			}
		}
		//release capacity after streaming is done
		releaseCapacity <- true

	} else {
		// Non-streaming response
		releaseCapacity <- true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}

}

//
// Orchestrator receiving results from the remote AI worker
//

func (h *lphttp) AIResults() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		authType := r.Header.Get("Authorization")
		if protoVerAIWorker != authType {
			glog.Error("Invalid auth type ", authType)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		creds := r.Header.Get("Credentials")

		if creds != orch.TranscoderSecret() {
			glog.Error("Invalid shared secret")
			respondWithError(w, errSecret.Error(), http.StatusUnauthorized)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			glog.Error("Error getting mime type ", err)
			http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
			return
		}

		tid, err := strconv.ParseInt(r.Header.Get("TaskId"), 10, 64)
		if err != nil {
			glog.Error("Could not parse task ID ", err)
			http.Error(w, "Invalid Task ID", http.StatusBadRequest)
			return
		}

		pipeline := r.Header.Get("Pipeline")

		var workerResult core.RemoteAIWorkerResult
		workerResult.Files = make(map[string][]byte)

		start := time.Now()
		dlDur := time.Duration(0) // default to 0 in case of early return
		resultType := ""
		switch mediaType {
		case aiWorkerErrorMimeType:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				glog.Errorf("Unable to read ai worker error body taskId=%v err=%q", tid, err)
				workerResult.Err = err
			} else {
				workerResult.Err = errors.New(string(body))
			}
			glog.Errorf("AI Worker error for taskId=%v err=%q", tid, workerResult.Err)
			orch.AIResults(tid, &workerResult)
			w.Write([]byte("OK"))
			return
		case "text/event-stream":
			resultType = "streaming"
			glog.Infof("Received %s response from remote worker=%s taskId=%d", resultType, r.RemoteAddr, tid)
			resChan := make(chan *worker.LLMResponse, 100)
			workerResult.Results = (<-chan *worker.LLMResponse)(resChan)

			defer r.Body.Close()
			defer close(resChan)
			//set a reasonable timeout to stop waiting for results
			ctx, _ := context.WithTimeout(r.Context(), HTTPIdleTimeout)

			//pass results and receive from channel as the results are streamed
			go orch.AIResults(tid, &workerResult)
			// Read the streamed results from the request body
			scanner := bufio.NewScanner(r.Body)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
					line := scanner.Text()
					if strings.HasPrefix(line, "data: ") {
						data := strings.TrimPrefix(line, "data: ")
						var chunk worker.LLMResponse
						if err := json.Unmarshal([]byte(data), &chunk); err != nil {
							clog.Errorf(ctx, "Error unmarshaling stream data: %v", err)
							continue
						}
						resChan <- &chunk
					}
				}
			}
			if err := scanner.Err(); err != nil {
				workerResult.Err = scanner.Err()
			}

			dlDur = time.Since(start)
		case "multipart/mixed":
			resultType = "uploaded"
			glog.Infof("Received %s response from remote worker=%s taskId=%d", resultType, r.RemoteAddr, tid)
			workerResult := parseMultiPartResult(r.Body, params["boundary"], pipeline)

			//return results
			dlDur = time.Since(start)
			workerResult.DownloadTime = dlDur
			orch.AIResults(tid, &workerResult)
		}

		glog.V(common.VERBOSE).Infof("Processed %s results from remote worker=%s taskId=%d dur=%s", resultType, r.RemoteAddr, tid, dlDur)

		if workerResult.Err != nil {
			http.Error(w, workerResult.Err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("OK"))
	})
}

func parseMultiPartResult(body io.Reader, boundary string, pipeline string) core.RemoteAIWorkerResult {
	wkrResult := core.RemoteAIWorkerResult{}
	wkrResult.Files = make(map[string][]byte)

	mr := multipart.NewReader(body, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			glog.Error("Could not process multipart part ", err)
			wkrResult.Err = err
			break
		}
		body, err := common.ReadAtMost(p, MaxAIRequestSize)
		if err != nil {
			glog.Error("Error reading body ", err)
			wkrResult.Err = err
			break
		}

		// this is where we would include metadata on each result if want to separate
		// instead the multipart response includes the json and the files separately with the json "url" field matching to part names
		cDisp := p.Header.Get("Content-Disposition")
		if p.Header.Get("Content-Type") == "application/json" {
			var results interface{}
			switch pipeline {
			case "text-to-image", "image-to-image", "upscale", "image-to-video":
				var parsedResp worker.ImageResponse

				err := json.Unmarshal(body, &parsedResp)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
				results = parsedResp
			case "audio-to-text", "segment-anything-2", "llm", "image-to-text":
				err := json.Unmarshal(body, &results)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
			case "text-to-speech":
				var parsedResp worker.AudioResponse
				err := json.Unmarshal(body, &parsedResp)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
				results = parsedResp
			}

			wkrResult.Results = results
		} else if cDisp != "" {
			//these are the result files binary data
			resultName := p.FileName()
			wkrResult.Files[resultName] = body
		}
	}

	return wkrResult
}
