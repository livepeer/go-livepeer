package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
)

var getNewTokenTimeout = 3 * time.Second

// startStreamProcessingFunc is an alias for startStreamProcessing that can be overridden in tests
// to avoid starting up actual stream processing
var startStreamProcessingFunc = startStreamProcessing

func (ls *LivepeerServer) StartStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			corsHeaders(w, r.Method)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Create fresh context instead of using r.Context() since ctx will outlive the request
		ctx := r.Context()

		corsHeaders(w, r.Method)
		//verify request, get orchestrators available and sign request
		gatewayJob, err := ls.setupGatewayJob(ctx, r, false)
		if err != nil {
			clog.Errorf(ctx, "Error setting up job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		//setup body size limit, will error if too large
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		streamUrls, code, err := ls.setupStream(ctx, r, gatewayJob)
		if err != nil {
			clog.Errorf(ctx, "Error setting up stream: %s", err)
			http.Error(w, err.Error(), code)
			return
		}

		go ls.runStream(gatewayJob)

		go ls.monitorStream(gatewayJob.Job.Req.ID)

		if streamUrls != nil {
			// Stream started successfully
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(streamUrls)
		} else {
			//case where we are subscribing to own streams in setupStream
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

func (ls *LivepeerServer) StopStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create fresh context instead of using r.Context() since ctx will outlive the request
		ctx := r.Context()
		streamId := r.PathValue("streamId")

		stream, exists := ls.LivepeerNode.LivePipelines[streamId]
		if !exists {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}

		params, err := getStreamRequestParams(stream)
		if err != nil {
			clog.Errorf(ctx, "Error getting stream request params: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		stream.StopStream(nil)
		delete(ls.LivepeerNode.LivePipelines, streamId)

		stopJob, err := ls.setupGatewayJob(ctx, r, true)
		if err != nil {
			clog.Errorf(ctx, "Error setting up stop job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stopJob.sign() //no changes to make, sign job
		//setup sender
		jobSender, err := getJobSender(ctx, ls.LivepeerNode)
		if err != nil {
			clog.Errorf(ctx, "Error getting job sender: %v", err)
			return
		}

		token, err := sessionToToken(params.liveParams.sess)
		if err != nil {
			clog.Errorf(ctx, "Error converting session to token: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newToken, err := getToken(ctx, getNewTokenTimeout, token.ServiceAddr, stopJob.Job.Req.Capability, jobSender.Addr, jobSender.Sig)
		if err != nil {
			clog.Errorf(ctx, "Error converting session to token: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			clog.Errorf(ctx, "Error reading request body: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		resp, code, err := ls.sendJobToOrch(ctx, r, stopJob.Job.Req, stopJob.SignedJobReq, *newToken, "/ai/stream/stop", body)
		if err != nil {
			clog.Errorf(ctx, "Error sending job to orchestrator: %s", err)
			http.Error(w, err.Error(), code)
			return
		}

		w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
		return
	})
}

func (ls *LivepeerServer) runStream(gatewayJob *gatewayJob) {
	streamID := gatewayJob.Job.Req.ID
	stream, exists := ls.LivepeerNode.LivePipelines[streamID]
	if !exists {
		glog.Errorf("Stream %s not found", streamID)
		return
	}
	// Ensure cleanup happens on ALL exit paths
	var exitErr error
	defer func() {
		// Best-effort cleanup
		if stream, exists := ls.LivepeerNode.LivePipelines[streamID]; exists {
			stream.StopStream(exitErr)
		}
	}()
	//this context passes to all channels that will close when stream is canceled
	ctx := stream.GetContext()
	ctx = clog.AddVal(ctx, "stream_id", streamID)

	params, err := getStreamRequestParams(stream)
	if err != nil {
		clog.Errorf(ctx, "Error getting stream request params: %s", err)
		exitErr = err
		return
	}

	//monitor for lots of fast swaps, likely something wrong with request
	orchSwapper := NewOrchestratorSwapper(params)

	firstProcessed := false
	for _, orch := range gatewayJob.Orchs {
		clog.Infof(ctx, "Starting stream processing")
		//refresh the token if not first Orch to confirm capacity and new ticket params
		if firstProcessed {
			newToken, err := getToken(ctx, getNewTokenTimeout, orch.ServiceAddr, gatewayJob.Job.Req.Capability, gatewayJob.Job.Req.Sender, gatewayJob.Job.Req.Sig)
			if err != nil {
				clog.Errorf(ctx, "Error getting token for orch=%v err=%v", orch.ServiceAddr, err)
				continue
			}
			orch = *newToken
		}

		orchSession, err := tokenToAISession(orch)
		if err != nil {
			clog.Errorf(ctx, "Error converting token to AISession: %v", err)
			continue
		}
		params.liveParams.sess = &orchSession

		ctx = clog.AddVal(ctx, "orch", hexutil.Encode(orch.TicketParams.Recipient))
		ctx = clog.AddVal(ctx, "orch_url", orch.ServiceAddr)

		//set request ID to persist from Gateway to Worker
		gatewayJob.Job.Req.ID = params.liveParams.streamID
		err = gatewayJob.sign()
		if err != nil {
			clog.Errorf(ctx, "Error signing job, exiting stream processing request: %v", err)
			exitErr = err
			return
		}
		orchResp, _, err := ls.sendJobToOrch(ctx, nil, gatewayJob.Job.Req, gatewayJob.SignedJobReq, orch, "/ai/stream/start", stream.StreamRequest())
		if err != nil {
			clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orch.ServiceAddr, err.Error())
			continue
		}
		defer orchResp.Body.Close()
		io.Copy(io.Discard, orchResp.Body)

		GatewayStatus.StoreKey(streamID, "orchestrator", orch.ServiceAddr)

		params.liveParams.orchPublishUrl = orchResp.Header.Get("X-Publish-Url")
		params.liveParams.orchSubscribeUrl = orchResp.Header.Get("X-Subscribe-Url")
		params.liveParams.orchControlUrl = orchResp.Header.Get("X-Control-Url")
		params.liveParams.orchEventsUrl = orchResp.Header.Get("X-Events-Url")
		params.liveParams.orchDataUrl = orchResp.Header.Get("X-Data-Url")

		perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
		params.liveParams.kickOrch = perOrchCancel
		stream.UpdateStreamParams(params) //update params used to kickOrch (perOrchCancel) and urls
		if err = startStreamProcessingFunc(perOrchCtx, stream, params); err != nil {
			clog.Errorf(ctx, "Error starting processing: %s", err)
			perOrchCancel(err)
			break
		}
		//something caused the Orch to stop performing, try to get the error and move to next Orchestrator
		<-perOrchCtx.Done()
		err = context.Cause(perOrchCtx)
		if errors.Is(err, context.Canceled) {
			// this happens if parent ctx was cancelled without a CancelCause
			// or if passing `nil` as a CancelCause
			err = nil
		}
		if !params.inputStreamExists() {
			clog.Info(ctx, "No stream exists, skipping orchestrator swap")
			break
		}

		//if swapping too fast, stop trying since likely a bad request
		if swapErr := orchSwapper.checkSwap(ctx); swapErr != nil {
			if err != nil {
				err = fmt.Errorf("%w: %w", swapErr, err)
			} else {
				err = swapErr
			}
			break
		}
		firstProcessed = true
		// will swap, but first notify with the reason for the swap
		if err == nil {
			err = errors.New("unknown swap reason")
		}

		clog.Infof(ctx, "Retrying stream with a different orchestrator err=%v", err.Error())

		params.liveParams.sendErrorEvent(err)

		//if there is ingress input then force off
		if params.liveParams.kickInput != nil {
			params.liveParams.kickInput(err)
		}

	}

	//all orchestrators tried or stream ended, stop the stream
	// stream stop called in defer above
	exitErr = errors.New("All Orchestrators exhausted, restart the stream")
}

func (ls *LivepeerServer) monitorStream(streamId string) {
	ctx := context.Background()
	ctx = clog.AddVal(ctx, "stream_id", streamId)

	stream, exists := ls.LivepeerNode.LivePipelines[streamId]
	if !exists {
		clog.Errorf(ctx, "Stream %s not found", streamId)
		return
	}
	params, err := getStreamRequestParams(stream)
	if err != nil {
		clog.Errorf(ctx, "Error getting stream request params: %v", err)
		return
	}

	ctx = clog.AddVal(ctx, "request_id", params.liveParams.requestID)

	// Create a ticker that runs every minute for payments with buffer to ensure payment is completed
	dur := 50 * time.Second
	pmtTicker := time.NewTicker(dur)
	defer pmtTicker.Stop()
	//setup sender
	jobSender, err := getJobSender(ctx, ls.LivepeerNode)
	if err != nil {
		clog.Errorf(ctx, "Error getting job sender: %v", err)
		return
	}

	//ensure live pipeline is cleaned up if monitoring ends
	defer ls.LivepeerNode.RemoveLivePipeline(streamId)
	//start monitoring loop
	streamCtx := stream.GetContext()
	for {
		select {
		case <-streamCtx.Done():
			clog.Infof(ctx, "Stream %s stopped, ending monitoring", streamId)
			return
		case <-pmtTicker.C:
			if !params.inputStreamExists() {
				clog.Infof(ctx, "Input stream does not exist for stream %s, ending monitoring", streamId)
				return
			}

			err := ls.sendPaymentForStream(ctx, stream, jobSender)
			if err != nil {
				clog.Errorf(ctx, "Error sending payment for stream %s: %v", streamId, err)
			}
		}
	}
}

func (ls *LivepeerServer) sendPaymentForStream(ctx context.Context, stream *core.LivePipeline, jobSender *core.JobSender) error {
	params, err := getStreamRequestParams(stream)
	if err != nil {
		clog.Errorf(ctx, "Error getting stream request params: %v", err)
		return err
	}
	token, err := sessionToToken(params.liveParams.sess)
	if err != nil {
		clog.Errorf(ctx, "Error getting token for session: %v", err)
		return err
	}

	// fetch new JobToken with each payment
	// update the session for the LivePipeline with new token
	newToken, err := getToken(ctx, getNewTokenTimeout, token.ServiceAddr, stream.Pipeline, jobSender.Addr, jobSender.Sig)
	if err != nil {
		clog.Errorf(ctx, "Error getting new token for %s: %v", token.ServiceAddr, err)
		return err
	}
	newSess, err := tokenToAISession(*newToken)
	if err != nil {
		clog.Errorf(ctx, "Error converting token to AI session: %v", err)
		return err
	}
	params.liveParams.sess = &newSess
	stream.UpdateStreamParams(params)

	// send the payment
	streamID := params.liveParams.streamID
	jobDetails := JobRequestDetails{StreamId: streamID}
	jobDetailsStr, err := json.Marshal(jobDetails)
	if err != nil {
		clog.Errorf(ctx, "Error marshalling job details: %v", err)
		return err
	}
	req := &JobRequest{Request: string(jobDetailsStr), Parameters: "{}", Capability: stream.Pipeline,
		Sender:  jobSender.Addr,
		Timeout: 70,
	}
	//sign the request
	job := gatewayJob{Job: &orchJob{Req: req}, node: ls.LivepeerNode}
	err = job.sign()
	if err != nil {
		clog.Errorf(ctx, "Error signing job, continuing monitoring: %v", err)
		return err
	}

	if newSess.OrchestratorInfo.PriceInfo.PricePerUnit > 0 {
		pmtHdr, err := createPayment(ctx, req, newToken, ls.LivepeerNode)
		if err != nil {
			clog.Errorf(ctx, "Error processing stream payment for %s: %v", streamID, err)
			// Continue monitoring even if payment fails
		}
		if pmtHdr == "" {
			// This is no payment required, error logged above
			return nil
		}

		//send the payment, update the stream with the refreshed token
		clog.Infof(ctx, "Sending stream payment for %s", streamID)
		statusCode, err := ls.sendPayment(ctx, token.ServiceAddr+"/ai/stream/payment", stream.Pipeline, job.SignedJobReq, pmtHdr)
		if err != nil {
			clog.Errorf(ctx, "Error sending stream payment for %s: %v", streamID, err)
			return err
		}
		if statusCode != http.StatusOK {
			clog.Errorf(ctx, "Unexpected status code %d received for %s", statusCode, streamID)
			return errors.New("unexpected status code")
		}
	}

	return nil
}

type StartRequest struct {
	Stream     string `json:"stream_name"`
	RtmpOutput string `json:"rtmp_output"`
	StreamId   string `json:"stream_id"`
	Params     string `json:"params"`
}

type StreamUrls struct {
	StreamId      string `json:"stream_id"`
	WhipUrl       string `json:"whip_url"`
	WhepUrl       string `json:"whep_url"`
	RtmpUrl       string `json:"rtmp_url"`
	RtmpOutputUrl string `json:"rtmp_output_url"`
	UpdateUrl     string `json:"update_url"`
	StatusUrl     string `json:"status_url"`
	DataUrl       string `json:"data_url"`
}

func (ls *LivepeerServer) setupStream(ctx context.Context, r *http.Request, job *gatewayJob) (*StreamUrls, int, error) {
	if job == nil {
		return nil, http.StatusBadRequest, errors.New("invalid job")
	}

	requestID := string(core.RandomManifestID())
	ctx = clog.AddVal(ctx, "request_id", requestID)

	// Setup request body to be able to preserve for retries
	// Read the entire body first with 10MB limit
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		if maxErr, ok := err.(*http.MaxBytesError); ok {
			clog.Warningf(ctx, "Request body too large (over 10MB)")
			return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("request body too large (max %d bytes)", maxErr.Limit)
		} else {
			clog.Errorf(ctx, "Error reading request body: %v", err)
			return nil, http.StatusBadRequest, fmt.Errorf("error reading request body: %w", err)
		}
	}
	r.Body.Close()

	// Decode the StartRequest from JSON body
	var startReq StartRequest
	if err := json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(&startReq); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid JSON request body: %w", err)
	}

	//live-video-to-video uses path value for this
	streamName := startReq.Stream

	streamRequestTime := time.Now().UnixMilli()

	ctx = clog.AddVal(ctx, "stream", streamName)

	// If auth webhook is set and returns an output URL, this will be replaced
	outputURL := startReq.RtmpOutput

	// convention to avoid re-subscribing to our own streams
	// in case we want to push outputs back into mediamtx -
	// use an `-out` suffix for the stream name.
	if strings.HasSuffix(streamName, "-out") {
		// skip for now; we don't want to re-publish our own outputs
		return nil, 0, nil
	}

	// if auth webhook returns pipeline config these will be replaced
	pipeline := job.Job.Req.Capability
	rawParams := startReq.Params
	streamID := startReq.StreamId

	var pipelineID string
	var pipelineParams map[string]interface{}
	if rawParams != "" {
		if err := json.Unmarshal([]byte(rawParams), &pipelineParams); err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid model params")
		}
	}

	//ensure a streamid exists and includes the streamName if provided
	if streamID == "" {
		streamID = string(core.RandomManifestID())
	}
	if streamName != "" {
		streamID = fmt.Sprintf("%s-%s", streamName, streamID)
	}
	// BYOC uses Livepeer native WHIP
	// Currently for webrtc we need to add a path prefix due to the ingress setup
	//mediaMTXStreamPrefix := r.PathValue("prefix")
	//if mediaMTXStreamPrefix != "" {
	//	mediaMTXStreamPrefix = mediaMTXStreamPrefix + "/"
	//}
	mediaMtxHost := os.Getenv("LIVE_AI_PLAYBACK_HOST")
	if mediaMtxHost == "" {
		mediaMtxHost = "rtmp://localhost:1935"
	}
	mediaMTXInputURL := fmt.Sprintf("%s/%s%s", mediaMtxHost, "", streamID)
	mediaMTXOutputURL := mediaMTXInputURL + "-out"
	mediaMTXOutputAlias := fmt.Sprintf("%s-%s-out", mediaMTXInputURL, requestID)

	var (
		whipURL string
		rtmpURL string
		whepURL string
		dataURL string
	)

	updateURL := fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "update")
	statusURL := fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "status")

	if job.Job.Params.EnableVideoIngress {
		whipURL = fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "whip")
		rtmpURL = mediaMTXInputURL
	}
	if job.Job.Params.EnableVideoEgress {
		whepURL = generateWhepUrl(streamID, requestID)
	}
	if job.Job.Params.EnableDataOutput {
		dataURL = fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "data")
	}

	//if set this will overwrite settings above
	if LiveAIAuthWebhookURL != nil {
		authResp, err := authenticateAIStream(LiveAIAuthWebhookURL, ls.liveAIAuthApiKey, AIAuthRequest{
			Stream:      streamName,
			Type:        "", //sourceTypeStr
			QueryParams: rawParams,
			GatewayHost: ls.LivepeerNode.GatewayHost,
			WhepURL:     whepURL,
			UpdateURL:   updateURL,
			StatusURL:   statusURL,
		})
		if err != nil {
			return nil, http.StatusForbidden, fmt.Errorf("live ai auth failed: %w", err)
		}

		if authResp.RTMPOutputURL != "" {
			outputURL = authResp.RTMPOutputURL
		}

		if authResp.Pipeline != "" {
			pipeline = authResp.Pipeline
		}

		if len(authResp.paramsMap) > 0 {
			if _, ok := authResp.paramsMap["prompt"]; !ok && pipeline == "comfyui" {
				pipelineParams = map[string]interface{}{"prompt": authResp.paramsMap}
			} else {
				pipelineParams = authResp.paramsMap
			}
		}

		if authResp.StreamID != "" {
			streamID = authResp.StreamID
		}

		if authResp.PipelineID != "" {
			pipelineID = authResp.PipelineID
		}
	}

	ctx = clog.AddVal(ctx, "stream_id", streamID)
	clog.Infof(ctx, "Received live video AI request")

	// collect all RTMP outputs
	var rtmpOutputs []string
	if job.Job.Params.EnableVideoEgress {
		if outputURL != "" {
			rtmpOutputs = append(rtmpOutputs, outputURL)
		}
		if mediaMTXOutputURL != "" {
			rtmpOutputs = append(rtmpOutputs, mediaMTXOutputURL, mediaMTXOutputAlias)
		}
	}

	clog.Info(ctx, "RTMP outputs", "destinations", rtmpOutputs)

	// Clear any previous gateway status
	GatewayStatus.Clear(streamID)
	GatewayStatus.StoreKey(streamID, "whep_url", whepURL)

	monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
		"type":        "gateway_receive_stream_request",
		"timestamp":   streamRequestTime,
		"stream_id":   streamID,
		"pipeline_id": pipelineID,
		"request_id":  requestID,
		"orchestrator_info": map[string]interface{}{
			"address": "",
			"url":     "",
		},
	})

	// Count `ai_live_attempts` after successful parameters validation
	clog.V(common.VERBOSE).Infof(ctx, "AI Live video attempt")
	if monitor.Enabled {
		monitor.AILiveVideoAttempt(job.Job.Req.Capability)
	}

	sendErrorEvent := LiveErrorEventSender(ctx, streamID, map[string]string{
		"type":        "error",
		"request_id":  requestID,
		"stream_id":   streamID,
		"pipeline_id": pipelineID,
		"pipeline":    pipeline,
	})

	//params set with ingest types:
	// RTMP
	//   kickInput will kick the input from MediaMTX to force a reconnect
	//   localRTMPPrefix mediaMTXInputURL matches to get the ingest from MediaMTX
	// WHIP
	//   kickInput will close the whip connection
	//   localRTMPPrefix set by ENV variable LIVE_AI_PLAYBACK_HOST
	ssr := media.NewSwitchableSegmentReader() //this converts ingest to segments to send to Orchestrator
	params := aiRequestParams{
		node:        ls.LivepeerNode,
		os:          drivers.NodeStorage.NewSession(requestID),
		sessManager: nil,

		liveParams: &liveRequestParams{
			segmentReader:          ssr,
			startTime:              time.Now(),
			rtmpOutputs:            rtmpOutputs,
			stream:                 streamID, //live video to video uses stream name, byoc combines to one id
			paymentProcessInterval: ls.livePaymentInterval,
			outSegmentTimeout:      ls.outSegmentTimeout,
			requestID:              requestID,
			streamID:               streamID,
			pipelineID:             pipelineID,
			pipeline:               pipeline,
			sendErrorEvent:         sendErrorEvent,
			manifestID:             pipeline, //byoc uses one balance per capability name
		},
	}

	//create a dataWriter for data channel if enabled
	if job.Job.Params.EnableDataOutput {
		params.liveParams.dataWriter = media.NewSegmentWriter(5)
	}

	//check if stream exists
	if params.inputStreamExists() {
		return nil, http.StatusBadRequest, fmt.Errorf("stream already exists: %s", streamID)
	}

	clog.Infof(ctx, "stream setup videoIngress=%v videoEgress=%v dataOutput=%v", job.Job.Params.EnableVideoIngress, job.Job.Params.EnableVideoEgress, job.Job.Params.EnableDataOutput)

	//save the stream setup
	paramsReq := map[string]interface{}{
		"params": pipelineParams,
	}
	paramsReqBytes, _ := json.Marshal(paramsReq)
	ls.LivepeerNode.NewLivePipeline(requestID, streamID, pipeline, params, paramsReqBytes) //track the pipeline for cancellation

	job.Job.Req.ID = streamID
	streamUrls := StreamUrls{
		StreamId:      streamID,
		WhipUrl:       whipURL,
		WhepUrl:       whepURL,
		RtmpUrl:       rtmpURL,
		RtmpOutputUrl: strings.Join(rtmpOutputs, ","),
		UpdateUrl:     updateURL,
		StatusUrl:     statusURL,
		DataUrl:       dataURL,
	}

	return &streamUrls, http.StatusOK, nil
}

// mediamtx sends this request to go-livepeer when rtmp stream received
func (ls *LivepeerServer) StartStreamRTMPIngest() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		streamId := r.PathValue("streamId")
		ctx = clog.AddVal(ctx, "stream_id", streamId)

		stream, ok := ls.LivepeerNode.LivePipelines[streamId]
		if !ok {
			respondJsonError(ctx, w, fmt.Errorf("stream not found: %s", streamId), http.StatusNotFound)
			return
		}

		params, err := getStreamRequestParams(stream)
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		//set source ID and source type needed for mediamtx client control api
		sourceID := r.FormValue("source_id")
		if sourceID == "" {
			http.Error(w, "missing source_id", http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "source_id", sourceID)
		sourceType := r.FormValue("source_type")
		sourceType = strings.ToLower(sourceType) //normalize the source type so rtmpConn matches to rtmpconn
		if sourceType == "" {
			http.Error(w, "missing source_type", http.StatusBadRequest)
			return
		}

		clog.Infof(ctx, "RTMP ingest from MediaMTX connected sourceID=%s sourceType=%s", sourceID, sourceType)
		//note that mediaMtxHost is the ip address of media mtx
		// mediamtx sends a post request in the runOnReady event setup in mediamtx.yml
		// StartLiveVideo calls this remoteHost
		mediaMtxHost, err := getRemoteHost(r.RemoteAddr)
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		mediaMTXInputURL := fmt.Sprintf("rtmp://%s/%s%s", mediaMtxHost, "", streamId)
		mediaMTXClient := media.NewMediaMTXClient(mediaMtxHost, ls.mediaMTXApiPassword, sourceID, sourceType)
		segmenterCtx, cancelSegmenter := context.WithCancel(clog.Clone(context.Background(), ctx))

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		kickInput := func(err error) {
			defer cancelSegmenter()
			if err == nil {
				return
			}
			clog.Errorf(ctx, "Live video pipeline finished with error: %s", err)

			params.liveParams.sendErrorEvent(err)

			err = mediaMTXClient.KickInputConnection(ctx)
			if err != nil {
				clog.Errorf(ctx, "Failed to kick input connection: %s", err)
			}
		}

		params.liveParams.localRTMPPrefix = mediaMTXInputURL
		params.liveParams.kickInput = kickInput
		stream.UpdateStreamParams(params) //add kickInput to stream params

		// Kick off the RTMP pull and segmentation
		clog.Infof(ctx, "Starting RTMP ingest from MediaMTX")
		go func() {
			ms := media.MediaSegmenter{Workdir: ls.LivepeerNode.WorkDir, MediaMTXClient: mediaMTXClient}
			//segmenter blocks until done
			ms.RunSegmentation(segmenterCtx, params.liveParams.localRTMPPrefix, params.liveParams.segmentReader.Read)

			params.liveParams.sendErrorEvent(errors.New("mediamtx ingest disconnected"))
			monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
				"type":        "gateway_ingest_stream_closed",
				"timestamp":   time.Now().UnixMilli(),
				"stream_id":   params.liveParams.streamID,
				"pipeline_id": params.liveParams.pipelineID,
				"request_id":  params.liveParams.requestID,
				"orchestrator_info": map[string]interface{}{
					"address": "",
					"url":     "",
				},
			})
			params.liveParams.segmentReader.Close()

			stream.StopStream(nil)
		}()

		//write response
		w.WriteHeader(http.StatusOK)
	})
}

func (ls *LivepeerServer) StartStreamWhipIngest(whipServer *media.WHIPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		streamId := r.PathValue("streamId")
		ctx = clog.AddVal(ctx, "stream_id", streamId)

		stream, ok := ls.LivepeerNode.LivePipelines[streamId]
		if !ok {
			respondJsonError(ctx, w, fmt.Errorf("stream not found: %s", streamId), http.StatusNotFound)
			return
		}

		params, err := getStreamRequestParams(stream)
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		whipConn := media.NewWHIPConnection()
		whepURL := generateWhepUrl(streamId, params.liveParams.requestID)

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		kickInput := func(err error) {
			if err == nil {
				return
			}
			clog.Errorf(ctx, "Live video pipeline finished with error: %s", err)
			params.liveParams.sendErrorEvent(err)
			whipConn.Close()
		}
		params.liveParams.kickInput = kickInput
		stream.UpdateStreamParams(params) //add kickInput to stream params

		//wait for the WHIP connection to close and then cleanup
		go func() {
			statsContext, statsCancel := context.WithCancel(ctx)
			defer statsCancel()
			go runStats(statsContext, whipConn, streamId, stream.Pipeline, params.liveParams.requestID)

			whipConn.AwaitClose()
			params.liveParams.segmentReader.Close()
			if params.liveParams.kickOrch != nil {
				params.liveParams.kickOrch(errors.New("whip connection closed"))
			}
			stream.StopStream(nil)
			clog.Info(ctx, "Live cleaned up")
		}()

		if whipServer == nil {
			respondJsonError(ctx, w, fmt.Errorf("whip server not configured"), http.StatusInternalServerError)
			whipConn.Close()
			return
		}

		conn := whipServer.CreateWHIP(ctx, params.liveParams.segmentReader, whepURL, w, r)
		whipConn.SetWHIPConnection(conn) // might be nil if theres an error and thats okay
	})
}

func startStreamProcessing(ctx context.Context, stream *core.LivePipeline, params aiRequestParams) error {

	//Optional channels
	if params.liveParams.orchPublishUrl != "" {
		clog.Infof(ctx, "Starting video ingress publisher")
		pub, err := common.AppendHostname(params.liveParams.orchPublishUrl, params.liveParams.sess.BroadcastSession.Transcoder())
		if err != nil {
			return fmt.Errorf("invalid publish URL: %w", err)
		}
		startTricklePublish(ctx, pub, params, params.liveParams.sess)
	}

	if params.liveParams.orchSubscribeUrl != "" {
		clog.Infof(ctx, "Starting video egress subscriber")
		sub, err := common.AppendHostname(params.liveParams.orchSubscribeUrl, params.liveParams.sess.BroadcastSession.Transcoder())
		if err != nil {
			return fmt.Errorf("invalid subscribe URL: %w", err)
		}
		startTrickleSubscribe(ctx, sub, params, params.liveParams.sess)
	}

	if params.liveParams.orchDataUrl != "" {
		clog.Infof(ctx, "Starting data channel subscriber")
		data, err := common.AppendHostname(params.liveParams.orchDataUrl, params.liveParams.sess.BroadcastSession.Transcoder())
		if err != nil {
			return fmt.Errorf("invalid data URL: %w", err)
		}
		params.liveParams.manifestID = stream.Pipeline

		startDataSubscribe(ctx, data, params, params.liveParams.sess)
	}

	//required channels
	control, err := common.AppendHostname(params.liveParams.orchControlUrl, params.liveParams.sess.BroadcastSession.Transcoder())
	if err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}
	events, err := common.AppendHostname(params.liveParams.orchEventsUrl, params.liveParams.sess.BroadcastSession.Transcoder())
	if err != nil {
		return fmt.Errorf("invalid events URL: %w", err)
	}

	startControlPublish(ctx, control, params)
	startEventsSubscribe(ctx, events, params, params.liveParams.sess)

	return nil
}

func (ls *LivepeerServer) GetStreamData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamId := r.PathValue("streamId")
		if streamId == "" {
			http.Error(w, "stream name is required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		ctx = clog.AddVal(ctx, "stream", streamId)

		// Get the live pipeline for this stream
		stream, exists := ls.LivepeerNode.LivePipelines[streamId]
		if !exists {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		params, err := getStreamRequestParams(stream)
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		// Get the data reading buffer
		if params.liveParams.dataWriter == nil {
			http.Error(w, "Stream data not available", http.StatusServiceUnavailable)
			return
		}
		dataReader := params.liveParams.dataWriter.MakeReader(media.SegmentReaderConfig{})

		// Set up SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		clog.Infof(ctx, "Starting SSE data stream for stream=%s", streamId)

		// Listen for broadcast signals from ring buffer writes
		// dataReader.Read() blocks on rb.cond.Wait() until startDataSubscribe broadcasts
		for {
			select {
			case <-ctx.Done():
				clog.Info(ctx, "SSE data stream client disconnected")
				return
			default:
				reader, err := dataReader.Next()
				if err != nil {
					if err == io.EOF {
						// Stream ended
						fmt.Fprintf(w, `event: end\ndata: {"type":"stream_ended"}\n\n`)
						flusher.Flush()
						return
					}
					clog.Errorf(ctx, "Error reading from ring buffer: %v", err)
					return
				}
				start := time.Now()
				data, err := io.ReadAll(reader)
				clog.V(6).Infof(ctx, "SSE data read took %v", time.Since(start))
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})
}

func (ls *LivepeerServer) UpdateStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		corsHeaders(w, r.Method)

		// Get stream from path param
		streamId := r.PathValue("streamId")
		if streamId == "" {
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}
		stream, ok := ls.LivepeerNode.LivePipelines[streamId]
		if !ok {
			// Stream not found
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}

		params, err := getStreamRequestParams(stream)
		if err != nil {
			clog.Errorf(ctx, "Error getting stream request params: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		updateJob, err := ls.setupGatewayJob(ctx, r, true)
		if err != nil {
			clog.Errorf(ctx, "Error setting up update job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updateJob.sign()
		//setup sender
		jobSender, err := getJobSender(ctx, ls.LivepeerNode)
		if err != nil {
			clog.Errorf(ctx, "Error getting job sender: %v", err)
			return
		}
		token, err := sessionToToken(params.liveParams.sess)
		if err != nil {
			clog.Errorf(ctx, "Error converting session to token: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newToken, err := getToken(ctx, getNewTokenTimeout, token.ServiceAddr, updateJob.Job.Req.Capability, jobSender.Addr, jobSender.Sig)
		if err != nil {
			clog.Errorf(ctx, "Error converting session to token: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		//had issues with control publisher not sending down full data when including base64 encoded binary data
		// switched to using regular post request like /stream/start and /stream/stop
		//controlPub := stream.ControlPub

		reader := http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
		defer reader.Close()

		reportUpdate := stream.ReportUpdate

		data, err := io.ReadAll(reader)
		if err != nil {
			if maxErr, ok := err.(*http.MaxBytesError); ok {
				clog.Warningf(ctx, "Request body too large (over 10MB)")
				http.Error(w, fmt.Sprintf("request body too large (max %d bytes)", maxErr.Limit), http.StatusRequestEntityTooLarge)
				return
			} else {
				clog.Errorf(ctx, "Error reading request body: %v", err)
				http.Error(w, "Error reading request body", http.StatusBadRequest)
				return
			}
		}
		stream.Params = data

		resp, code, err := ls.sendJobToOrch(ctx, r, updateJob.Job.Req, updateJob.SignedJobReq, *newToken, "/ai/stream/update", data)
		if err != nil {
			clog.Errorf(ctx, "Error sending job to orchestrator: %s", err)
			http.Error(w, err.Error(), code)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Call reportUpdate callback if available
			if reportUpdate != nil {
				reportUpdate(data)
			}
		}

		clog.Infof(ctx, "stream params updated for stream=%s, but orchestrator returned status %d", streamId, resp.StatusCode)

		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})
}

func (ls *LivepeerServer) GetStreamStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		corsHeaders(w, r.Method)

		streamId := r.PathValue("streamId")
		if streamId == "" {
			http.Error(w, "stream id is required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		ctx = clog.AddVal(ctx, "stream", streamId)

		// Get status for specific stream
		status, exists := StreamStatusStore.Get(streamId)
		gatewayStatus, gatewayExists := GatewayStatus.Get(streamId)
		if !exists && !gatewayExists {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		if gatewayExists {
			if status == nil {
				status = make(map[string]any)
			}
			status["gateway_status"] = gatewayStatus
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			clog.Errorf(ctx, "Failed to encode stream status err=%v", err)
			http.Error(w, "Failed to encode status", http.StatusInternalServerError)
			return
		}
	})
}

// StartStream handles the POST /stream/start endpoint for the Orchestrator
func (h *lphttp) StartStream(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator
	remoteAddr := getRemoteAddr(r)
	ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

	orchJob, err := h.setupOrchJob(ctx, r, false)
	if err != nil {
		code := http.StatusBadRequest
		if err == errInsufficientBalance {
			code = http.StatusPaymentRequired
		}
		respondWithError(w, err.Error(), code)
		return
	}
	ctx = clog.AddVal(ctx, "stream_id", orchJob.Req.ID)

	workerRoute := orchJob.Req.CapabilityUrl + "/stream/start"

	// Read the original body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	var jobParams JobParameters
	err = json.Unmarshal([]byte(orchJob.Req.Parameters), &jobParams)
	if err != nil {
		clog.Errorf(ctx, "unable to parse parameters err=%v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clog.Infof(ctx, "Processing stream start request videoIngress=%v videoEgress=%v dataOutput=%v", jobParams.EnableVideoIngress, jobParams.EnableVideoEgress, jobParams.EnableDataOutput)
	// Start trickle server for live-video
	var (
		mid          = orchJob.Req.ID // Request ID is used for the manifest ID
		pubUrl       = h.orchestrator.ServiceURI().JoinPath(TrickleHTTPPath, mid).String()
		subUrl       = pubUrl + "-out"
		controlUrl   = pubUrl + "-control"
		eventsUrl    = pubUrl + "-events"
		dataUrl      = pubUrl + "-data"
		pubCh        *trickle.TrickleLocalPublisher
		subCh        *trickle.TrickleLocalPublisher
		controlPubCh *trickle.TrickleLocalPublisher
		eventsCh     *trickle.TrickleLocalPublisher
		dataCh       *trickle.TrickleLocalPublisher
	)

	reqBodyForRunner := make(map[string]interface{})
	reqBodyForRunner["gateway_request_id"] = mid
	//required channels
	controlPubCh = trickle.NewLocalPublisher(h.trickleSrv, mid+"-control", "application/json")
	controlPubCh.CreateChannel()
	controlUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, controlUrl)
	reqBodyForRunner["control_url"] = controlUrl
	w.Header().Set("X-Control-Url", controlUrl)

	eventsCh = trickle.NewLocalPublisher(h.trickleSrv, mid+"-events", "application/json")
	eventsCh.CreateChannel()
	eventsUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, eventsUrl)
	reqBodyForRunner["events_url"] = eventsUrl
	w.Header().Set("X-Events-Url", eventsUrl)

	//Optional channels
	if jobParams.EnableVideoIngress {
		pubCh = trickle.NewLocalPublisher(h.trickleSrv, mid, "video/MP2T")
		pubCh.CreateChannel()
		pubUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, pubUrl)
		reqBodyForRunner["subscribe_url"] = pubUrl //runner needs to subscribe to input
		w.Header().Set("X-Publish-Url", pubUrl)    //gateway will connect to pubUrl to send ingress video
	}

	if jobParams.EnableVideoEgress {
		subCh = trickle.NewLocalPublisher(h.trickleSrv, mid+"-out", "video/MP2T")
		subCh.CreateChannel()
		subUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, subUrl)
		reqBodyForRunner["publish_url"] = subUrl  //runner needs to send results -out
		w.Header().Set("X-Subscribe-Url", subUrl) //gateway will connect to subUrl to receive results
	}

	if jobParams.EnableDataOutput {
		dataCh = trickle.NewLocalPublisher(h.trickleSrv, mid+"-data", "application/jsonl")
		dataCh.CreateChannel()
		dataUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, dataUrl)
		reqBodyForRunner["data_url"] = dataUrl
		w.Header().Set("X-Data-Url", dataUrl)
	}
	//parse the request body json to add to the request to the runner
	var bodyJSON map[string]interface{}
	if err := json.Unmarshal(body, &bodyJSON); err != nil {
		clog.Errorf(ctx, "Failed to parse body as JSON, using as string: %v", err)
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	for key, value := range bodyJSON {
		reqBodyForRunner[key] = value
	}

	reqBodyBytes, err := json.Marshal(reqBodyForRunner)
	if err != nil {
		clog.Errorf(ctx, "Failed to marshal request body err=%v", err)
		http.Error(w, "Failed to marshal request body", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(reqBodyBytes))
	// set the headers
	req.Header.Add("Content-Length", r.Header.Get("Content-Length"))
	req.Header.Add("Content-Type", r.Header.Get("Content-Type"))

	start := time.Now()
	resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
	if err != nil {
		clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
		respondWithError(w, "Error sending request to worker", http.StatusInternalServerError)
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Error reading response body: %v", err)
		respondWithError(w, "Error reading response body", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	//error response from worker but assume can retry and pass along error response and status code
	if resp.StatusCode > 399 {
		clog.Errorf(ctx, "error processing stream start request statusCode=%d", resp.StatusCode)

		chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
		w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
		//return error response from the worker
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
	w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))

	clog.V(common.SHORT).Infof(ctx, "stream start processed successfully took=%v balance=%v", time.Since(start), getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))

	//setup the stream
	stream, err := h.node.ExternalCapabilities.AddStream(orchJob.Req.ID, orchJob.Req.Capability, reqBodyBytes)
	if err != nil {
		clog.Errorf(ctx, "Error adding stream to external capabilities: %v", err)
		respondWithError(w, "Error adding stream to external capabilities", http.StatusInternalServerError)
		return
	}

	stream.SetChannels(pubCh, subCh, controlPubCh, eventsCh, dataCh)

	//start payment monitoring
	go func() {
		stream, exists := h.node.ExternalCapabilities.GetStream(orchJob.Req.ID)
		if !exists {
			clog.Infof(ctx, "Stream not found for payment monitoring, exiting monitoring stream_id=%s", orchJob.Req.ID)
			return
		}

		ctx := context.Background()
		ctx = clog.AddVal(ctx, "stream_id", orchJob.Req.ID)
		ctx = clog.AddVal(ctx, "capability", orchJob.Req.Capability)

		pmtCheckDur := 23 * time.Second //run slightly faster than gateway so can return updated balance
		pmtTicker := time.NewTicker(pmtCheckDur)
		defer pmtTicker.Stop()
		shouldStopStreamNextRound := false
		for {
			select {
			case <-stream.StreamCtx.Done():
				h.orchestrator.FreeExternalCapabilityCapacity(orchJob.Req.Capability)
				clog.Infof(ctx, "Stream ended, stopping payment monitoring and released capacity")
				return
			case <-pmtTicker.C:
				// Check payment status
				extCap, ok := h.node.ExternalCapabilities.Capabilities[orchJob.Req.Capability]
				if !ok {
					clog.Errorf(ctx, "Capability not found for payment monitoring, exiting monitoring capability=%s", orchJob.Req.Capability)
					return
				}
				jobPriceRat := big.NewRat(orchJob.JobPrice.PricePerUnit, orchJob.JobPrice.PixelsPerUnit)
				if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
					//lock during balance update to complete balance update
					extCap.Mu.Lock()
					h.orchestrator.DebitFees(orchJob.Sender, core.ManifestID(orchJob.Req.Capability), orchJob.JobPrice, int64(pmtCheckDur.Seconds()))
					senderBalance := getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability)
					extCap.Mu.Unlock()
					if senderBalance != nil {
						if senderBalance.Cmp(big.NewRat(0, 1)) < 0 {
							if !shouldStopStreamNextRound {
								//warn once
								clog.Warningf(ctx, "Insufficient balance for stream capability, will stop stream next round if not replenished sender=%s capability=%s balance=%s", orchJob.Sender, orchJob.Req.Capability, senderBalance.FloatString(0))
								shouldStopStreamNextRound = true
								continue
							}

							clog.Infof(ctx, "Insufficient balance, stopping stream %s for sender %s", orchJob.Req.ID, orchJob.Sender)
							_, exists := h.node.ExternalCapabilities.GetStream(orchJob.Req.ID)
							if exists {
								h.node.ExternalCapabilities.RemoveStream(orchJob.Req.ID)
							}

							return
						}

						clog.V(8).Infof(ctx, "Payment balance for stream capability is good balance=%v", senderBalance.FloatString(0))
					}
				}

				//check if stream still exists
				// if not, send stop to worker and exit monitoring
				stream, exists := h.node.ExternalCapabilities.GetStream(orchJob.Req.ID)
				if !exists {
					req, err := http.NewRequestWithContext(ctx, "POST", orchJob.Req.CapabilityUrl+"/stream/stop", nil)
					// set the headers
					resp, err = sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
					if err != nil {
						clog.Errorf(ctx, "Error sending request to worker %v: %v", orchJob.Req.CapabilityUrl, err)
						respondWithError(w, "Error sending request to worker", http.StatusInternalServerError)
						return
					}
					defer resp.Body.Close()
					io.Copy(io.Discard, resp.Body)

					//end monitoring of stream
					return
				}

				//check if control channel is still open, end if not
				if !stream.IsActive() {
					// Stop the stream and free capacity
					h.node.ExternalCapabilities.RemoveStream(orchJob.Req.ID)
					return
				}
			}
		}
	}()

	//send back the trickle urls set in header
	w.WriteHeader(http.StatusOK)
	return
}

func (h *lphttp) StopStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orchJob, err := h.setupOrchJob(ctx, r, false)
	if err != nil {
		respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid err=%v", err), http.StatusBadRequest)
		return
	}

	var jobDetails JobRequestDetails
	err = json.Unmarshal([]byte(orchJob.Req.Request), &jobDetails)
	if err != nil {
		respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid, failed to parse stream id err=%v", err), http.StatusBadRequest)
		return
	}
	clog.Infof(ctx, "Stopping stream %s", jobDetails.StreamId)

	// Read the original body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	workerRoute := orchJob.Req.CapabilityUrl + "/stream/stop"
	req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(body))
	if err != nil {
		clog.Errorf(ctx, "failed to create /stream/stop request to worker err=%v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
	if err != nil {
		clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Error reading response body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		clog.Errorf(ctx, "error processing stream stop request statusCode=%d", resp.StatusCode)
	}

	// Stop the stream and free capacity
	h.node.ExternalCapabilities.RemoveStream(jobDetails.StreamId)

	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (h *lphttp) UpdateStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orchJob, err := h.setupOrchJob(ctx, r, false)
	if err != nil {
		respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid err=%v", err), http.StatusBadRequest)
		return
	}

	var jobDetails JobRequestDetails
	err = json.Unmarshal([]byte(orchJob.Req.Request), &jobDetails)
	if err != nil {
		respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid, failed to parse stream id err=%v", err), http.StatusBadRequest)
		return
	}
	clog.Infof(ctx, "Stopping stream %s", jobDetails.StreamId)

	// Read the original body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	workerRoute := orchJob.Req.CapabilityUrl + "/stream/params"
	req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(body))
	if err != nil {
		clog.Errorf(ctx, "failed to create /stream/params request to worker err=%v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
	if err != nil {
		clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
		respondWithError(w, "Error sending request to worker", http.StatusInternalServerError)
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Error reading response body: %v", err)
		respondWithError(w, "Error reading response body", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		clog.Errorf(ctx, "error processing stream update request statusCode=%d", resp.StatusCode)
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (h *lphttp) ProcessStreamPayment(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator
	ctx := r.Context()

	//this will validate the request and process the payment
	orchJob, err := h.setupOrchJob(ctx, r, false)
	if err != nil {
		respondWithError(w, fmt.Sprintf("Failed to process payment, request not valid err=%v", err), http.StatusBadRequest)
		return
	}
	ctx = clog.AddVal(ctx, "stream_id", orchJob.Details.StreamId)
	ctx = clog.AddVal(ctx, "capability", orchJob.Req.Capability)
	ctx = clog.AddVal(ctx, "sender", orchJob.Req.Sender)

	senderAddr := ethcommon.HexToAddress(orchJob.Req.Sender)

	capBal := orch.Balance(senderAddr, core.ManifestID(orchJob.Req.Capability))
	if capBal != nil {
		capBal, err = common.PriceToInt64(capBal)
		if err != nil {
			clog.Errorf(ctx, "could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), orchJob.Req.Capability, err.Error())
			capBal = big.NewRat(0, 1)
		}
	} else {
		capBal = big.NewRat(0, 1)
	}

	w.Header().Set(jobPaymentBalanceHdr, capBal.FloatString(0))
	w.WriteHeader(http.StatusOK)
}

func tokenToAISession(token core.JobToken) (AISession, error) {
	var session BroadcastSession

	// Initialize the lock to avoid nil pointer dereference in methods
	// like (*BroadcastSession).Transcoder() which acquire RLock()
	session.lock = &sync.RWMutex{}

	//default to zero price if its nil, Orchestrator will reject stream if charging a price above zero
	if token.Price == nil {
		token.Price = &net.PriceInfo{}
	}

	orchInfo := net.OrchestratorInfo{Transcoder: token.ServiceAddr, TicketParams: token.TicketParams, PriceInfo: token.Price}
	orchInfo.Transcoder = token.ServiceAddr
	if token.SenderAddress != nil {
		orchInfo.Address = ethcommon.Hex2Bytes(token.SenderAddress.Addr)
	}
	session.OrchestratorInfo = &orchInfo

	return AISession{BroadcastSession: &session}, nil
}

func sessionToToken(session *AISession) (core.JobToken, error) {
	var token core.JobToken

	token.ServiceAddr = session.OrchestratorInfo.Transcoder
	token.TicketParams = session.OrchestratorInfo.TicketParams
	token.Price = session.OrchestratorInfo.PriceInfo
	return token, nil
}

func getStreamRequestParams(stream *core.LivePipeline) (aiRequestParams, error) {
	if stream == nil {
		return aiRequestParams{}, fmt.Errorf("stream is nil")
	}

	streamParams := stream.StreamParams()
	params, ok := streamParams.(aiRequestParams)
	if !ok {
		return aiRequestParams{}, fmt.Errorf("failed to cast stream params to aiRequestParams")
	}
	return params, nil
}
