package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
)

const (
	defaultTextToImageModelID      = "stabilityai/sdxl-turbo"
	defaultImageToImageModelID     = "stabilityai/sdxl-turbo"
	defaultImageToVideoModelID     = "stabilityai/stable-video-diffusion-img2vid-xt"
	defaultUpscaleModelID          = "stabilityai/stable-diffusion-x4-upscaler"
	defaultAudioToTextModelID      = "openai/whisper-large-v3"
	defaultLLMModelID              = "meta-llama/llama-3.1-8B-Instruct"
	defaultSegmentAnything2ModelID = "facebook/sam2-hiera-large"
	defaultImageToTextModelID      = "Salesforce/blip-image-captioning-large"
	defaultLiveVideoToVideoModelID = "noop"
	defaultTextToSpeechModelID     = "parler-tts/parler-tts-large-v1"

	maxTries         = 20
	maxSameSessTries = 3

	// protoVerAIWorker is the protocol version advertised on live AI requests.
	protoVerAIWorker = "Livepeer-AI-Worker-1.0"
)

var errWrongFormat = fmt.Errorf("result not in correct format")

type ServiceUnavailableError struct {
	err error
}

func (e *ServiceUnavailableError) Error() string {
	return e.err.Error()
}

type BadRequestError struct {
	err error
}

func (e *BadRequestError) Error() string {
	return e.err.Error()
}

// pipelineDeprecationSunset is the advertised sunset date (RFC 1123) for
// deprecated AI pipelines, surfaced via the HTTP Sunset header.
const pipelineDeprecationSunset = "Wed, 31 Dec 2025 23:59:59 GMT"

// PipelineDeprecatedError indicates a request targeted a deprecated AI pipeline
// (the batch request/response pipelines and BYOC). It maps to HTTP 410 Gone.
type PipelineDeprecatedError struct {
	pipeline string
}

func (e *PipelineDeprecatedError) Error() string {
	return fmt.Sprintf("pipeline %q is deprecated and no longer supported", e.pipeline)
}

// parseBadRequestError checks if the error is a bad request error and returns a BadRequestError.
func parseBadRequestError(err error) *BadRequestError {
	if err == nil {
		return nil
	}
	if err, ok := err.(*BadRequestError); ok {
		return err
	}

	const errorCode = "returned 400"
	if !strings.Contains(err.Error(), errorCode) {
		return nil
	}

	parts := strings.SplitN(err.Error(), errorCode, 2)
	detail := strings.TrimSpace(parts[1])
	if detail == "" {
		detail = "bad request"
	}

	return &BadRequestError{err: errors.New(detail)}
}

type aiRequestParams struct {
	node        *core.LivepeerNode
	os          drivers.OSSession
	sessManager *AISessionManager

	liveParams *liveRequestParams
}

// For live video pipelines
type liveRequestParams struct {
	segmentReader *media.SwitchableSegmentReader
	stream        string
	requestID     string
	streamID      string
	manifestID    string
	pipelineID    string
	pipeline      string
	orchestrator  string

	paymentSender          LivePaymentSender
	paymentProcessInterval time.Duration
	outSegmentTimeout      time.Duration

	// list of RTMP output destinations
	rtmpOutputs []string
	// prefix to identify local (MediaMTX) RTMP hosts
	localRTMPPrefix string

	// Stops the pipeline with an error. Also kicks the input
	kickInput func(error)
	// Cancels the execution for the given Orchestrator session
	kickOrch context.CancelCauseFunc

	// Report an error event
	sendErrorEvent func(error)

	// State for the stream processing
	// startTime is the time when the first request is sent to the orchestrator
	startTime time.Time
	// sess is passed from the orchestrator selection, ugly hack
	sess *AISession

	// Everything below needs to be protected by `mu` for concurrent modification + access
	mu sync.Mutex

	// when the write for the last segment started
	lastSegmentTime time.Time
}

const initPixelsToPay = 60 * 30 * 720 * 1280 // 60 seconds, 30fps, 1280p

func submitLiveVideoToVideo(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenLiveVideoToVideoJSONRequestBody) (any, error) {
	sess = sess.Clone()

	// Storing sess in the liveParams; it's ugly, but we need to pass it back and don't want to break this function interface
	params.liveParams.sess = sess
	params.liveParams.startTime = time.Now()

	var paymentHeaders worker.RequestEditorFn
	if hasRemoteSigner(params) {
		rpp, ok := params.liveParams.paymentSender.(*remotePaymentSender)
		if !ok {
			return nil, errors.New("remote sender was not the correct type")
		}
		res, err := rpp.RequestPayment(ctx, &SegmentInfoSender{
			sess: sess.BroadcastSession,
		})
		if err != nil {
			return nil, err
		}
		paymentHeaders = func(_ context.Context, req *http.Request) error {
			req.Header.Set(segmentHeader, res.SegCreds)
			req.Header.Set(paymentHeader, res.Payment)
			req.Header.Set("Authorization", protoVerAIWorker)
			return nil
		}
	} else {

		// Live Video should not reuse the existing session balance, because it could lead to not sending the init
		// payment, which in turns may cause "Insufficient Balance" on the Orchestrator's side.
		// It works differently than other AI Jobs, because Live Video is accounted by mid on the Orchestrator's side.
		clearSessionBalance(sess.BroadcastSession, core.RandomManifestID())

		var (
			balUpdate *BalanceUpdate
			err       error
		)
		paymentHeaders, balUpdate, err = prepareAIPayment(ctx, sess, initPixelsToPay)
		if err != nil {
			if monitor.Enabled {
				monitor.AIRequestError(err.Error(), "LiveVideoToVideo", *req.ModelId, sess.OrchestratorInfo)
			}
			return nil, err
		}
		defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)
	}

	// Send request to orchestrator
	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "LiveVideoToVideo", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	reqTimeout := 5 * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	resp, err := client.GenLiveVideoToVideoWithResponse(reqCtx, req, paymentHeaders)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	if resp.JSON200.ControlUrl == nil {
		return nil, errors.New("control URL is missing")
	}

	return resp, nil
}

func processAIRequest(ctx context.Context, params aiRequestParams, req interface{}) (interface{}, error) {
	var cap core.Capability
	var modelID string
	var submitFn func(context.Context, aiRequestParams, *AISession) (interface{}, error)

	switch v := req.(type) {
	case worker.GenLiveVideoToVideoJSONRequestBody:
		cap = core.Capability_LiveVideoToVideo
		modelID = defaultLiveVideoToVideoModelID
		if v.ModelId != nil && *v.ModelId != "" {
			modelID = *v.ModelId
		} else {
			// set default model
			v.ModelId = &modelID
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitLiveVideoToVideo(ctx, params, sess, v)
		}
	default:
		return nil, fmt.Errorf("unsupported request type %T", req)
	}
	capName := cap.String()
	ctx = clog.AddVal(ctx, "model_id", modelID)

	clog.V(common.VERBOSE).Infof(ctx, "Received AI request model_id=%s", modelID)
	start := time.Now()
	defer clog.Infof(ctx, "Processed AI request model_id=%v took=%v", modelID, time.Since(start))

	var resp interface{}

	processingRetryTimeout := params.node.AIProcesssingRetryTimeout
	cctx, cancel := context.WithTimeout(ctx, processingRetryTimeout)
	defer cancel()

	tries := 0
	sessTries := map[string]int{}
	var retryableSessions []*AISession
	for tries < maxTries {
		select {
		case <-cctx.Done():
			err := cctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("no orchestrators available within %v timeout", processingRetryTimeout)
				if monitor.Enabled {
					monitor.AIRequestError(err.Error(), monitor.ToPipeline(capName), modelID, nil)
				}
			}
			return nil, &ServiceUnavailableError{err: err}
		default:
		}

		tries++
		sess, err := params.sessManager.Select(ctx, cap, modelID)
		if err != nil {
			clog.Infof(ctx, "Error selecting session modelID=%v err=%v", modelID, err)
			if cap == core.Capability_LiveVideoToVideo && sess != nil {
				// for live video, remove the session from the pool to avoid retrying it
				params.sessManager.Remove(ctx, sess)
			}
			continue
		}
		if sess == nil {
			break
		}
		sessTries[sess.Transcoder()]++
		if params.liveParams != nil {
			if params.liveParams.orchestrator != "" && !strings.Contains(sess.Transcoder(), params.liveParams.orchestrator) {
				// user requested a specific orchestrator, so ignore all the others
				clog.Infof(ctx, "Skipping orchestrator=%s because user request specific orchestrator=%s", sess.Transcoder(), params.liveParams.orchestrator)
				retryableSessions = append(retryableSessions, sess)
				continue
			}
		}

		resp, err = submitFn(ctx, params, sess)
		if err == nil {
			params.sessManager.Complete(ctx, sess)
			break
		}

		// Don't suspend the session if the error is a transient error.
		if isRetryableError(err) && sessTries[sess.Transcoder()] < maxSameSessTries {
			clog.Infof(ctx, "Error submitting request with retryable error modelID=%v try=%v orch=%v err=%v", modelID, tries, sess.Transcoder(), err)
			params.sessManager.Complete(ctx, sess)
			continue
		}

		// retry some specific errors with another session. re-check for retryable errors in case max retries were hit above
		if isRetryableError(err) || isInvalidTicketSenderNonce(err) || isNoCapacityError(err) {
			clog.Infof(ctx, "Error submitting request with non-retryable error modelID=%v try=%v orch=%v err=%v", modelID, tries, sess.Transcoder(), err)
			if cap == core.Capability_LiveVideoToVideo {
				// for live video, remove the session from the pool to avoid retrying it
				params.sessManager.Remove(ctx, sess)
			} else {
				// for non realtime video, get the session back to the pool as soon as the request completes
				retryableSessions = append(retryableSessions, sess)
			}
			continue
		}

		// Suspend the session on other errors.
		clog.Infof(ctx, "Error submitting request modelID=%v try=%v orch=%v err=%v", modelID, tries, sess.Transcoder(), err)
		params.sessManager.Remove(ctx, sess) //TODO: Improve session selection logic for live-video-to-video

		if errors.Is(err, common.ErrAudioDurationCalculation) {
			return nil, &BadRequestError{err}
		}

		if badRequestErr := parseBadRequestError(err); badRequestErr != nil {
			return nil, badRequestErr
		}
	}

	//add retryable sessions back to selector
	for _, sess := range retryableSessions {
		params.sessManager.Complete(ctx, sess)
	}

	if resp == nil {
		errMsg := "no orchestrators available"
		if monitor.Enabled {
			monitor.AIRequestError(errMsg, monitor.ToPipeline(capName), modelID, nil)
		}
		monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
			"type":        "gateway_no_orchestrators_available",
			"timestamp":   time.Now().UnixMilli(),
			"stream_id":   params.liveParams.streamID,
			"pipeline_id": params.liveParams.pipelineID,
			"request_id":  params.liveParams.requestID,
			"orchestrator_info": map[string]interface{}{
				"address": "",
				"url":     "",
			},
		})
		return nil, &ServiceUnavailableError{err: errors.New(errMsg)}
	}
	return resp, nil
}

// isRetryableError checks if the error is a transient error that can be retried.
func isRetryableError(err error) bool {
	return errContainsMsg(err, "ticketparams expired")
}

func isInvalidTicketSenderNonce(err error) bool {
	return errContainsMsg(err, "invalid ticket sendernonce")
}

func isNoCapacityError(err error) bool {
	return errContainsMsg(err, "insufficient capacity")
}

func errContainsMsg(err error, msgs ...string) bool {
	errMsg := strings.ToLower(err.Error())
	for _, msg := range msgs {
		if strings.Contains(errMsg, msg) {
			return true
		}
	}
	return false
}
func prepareAIPayment(ctx context.Context, sess *AISession, outPixels int64) (worker.RequestEditorFn, *BalanceUpdate, error) {
	// genSegCreds expects a stream.HLSSegment so in order to reuse it here we pass a dummy object
	segCreds, err := genSegCreds(sess.BroadcastSession, &stream.HLSSegment{}, nil, false)
	if err != nil {
		return nil, nil, err
	}

	priceInfo, err := common.RatPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
	if err != nil {
		return nil, nil, err
	}

	// At the moment, outPixels is expected to just be height * width * frames
	// If the # of inference/denoising steps becomes configurable, a possible updated formula could be height * width * frames * steps
	// If additional parameters that influence compute cost become configurable, then the formula should be reconsidered
	fee, err := estimateAIFee(outPixels, priceInfo)
	if err != nil {
		return nil, nil, err
	}

	balUpdate, err := newBalanceUpdate(sess.BroadcastSession, fee)
	if err != nil {
		return nil, nil, err
	}
	balUpdate.Debit = fee

	payment, err := genPayment(ctx, sess.BroadcastSession, balUpdate.NumTickets)
	if err != nil {
		clog.Errorf(ctx, "Could not create payment err=%q", err)

		if monitor.Enabled {
			monitor.PaymentCreateError(ctx)
		}

		return nil, nil, err
	}

	// As soon as the request is sent to the orch consider the balance update's credit as spent
	balUpdate.Status = CreditSpent
	if monitor.Enabled {
		monitor.TicketValueSent(ctx, balUpdate.NewCredit)
		monitor.TicketsSent(ctx, balUpdate.NumTickets)
	}

	setHeaders := func(_ context.Context, req *http.Request) error {
		req.Header.Set(segmentHeader, segCreds)
		req.Header.Set(paymentHeader, payment)
		req.Header.Set("Authorization", protoVerAIWorker)
		return nil
	}

	return setHeaders, balUpdate, nil
}

func estimateAIFee(outPixels int64, priceInfo *big.Rat) (*big.Rat, error) {
	if priceInfo == nil {
		return nil, nil
	}

	fee := new(big.Rat).SetInt64(outPixels)
	fee.Mul(fee, priceInfo)

	return fee, nil
}

// encodeReqMetadata encodes a map of metadata into a JSON string.
func encodeReqMetadata(metadata map[string]string) string {
	metadataBytes, _ := json.Marshal(metadata)
	return string(metadataBytes)
}

func hasRemoteSigner(params aiRequestParams) bool {
	return params.node != nil && params.node.RemoteSignerUrl != nil
}
