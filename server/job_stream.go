package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
)

// StartStream handles the POST /stream/start endpoint for the Media Server
func (ls *LivepeerServer) StartStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create fresh context instead of using r.Context() since ctx will outlive the request
		ctx := r.Context()

		orchJob, err := ls.setupJob(ctx, r)
		if err != nil {
			clog.Errorf(ctx, "Error setting up job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp, code, err := ls.setupStream(ctx, r, orchJob.Req)
		if err != nil {
			clog.Errorf(ctx, "Error setting up stream: %s", err)
			http.Error(w, err.Error(), code)
			return
		}

		go ls.runStream(orchJob, resp["stream_id"])

		if resp != nil {
			// Stream started successfully
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			//case where we are subscribing to own streams in setupStream
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

func (ls *LivepeerServer) runStream(job *orchJob, streamID string) {
	ctx := context.Background()
	stream, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamID]
	params := stream.Params.(aiRequestParams)
	if !exists {
		clog.Errorf(ctx, "Stream %s not found", streamID)
		return
	}

	for _, orch := range job.Orchs {
		orchResp, _, err := ls.sendJobToOrch(ctx, nil, job.Req, job.JobReqHdr, orch, "/stream/start", stream.StreamRequest)
		if err != nil {
			clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orch.ServiceAddr, err.Error())
			continue
		}
		stream.OrchPublishUrl = orchResp.Header.Get("Publish-Url")
		stream.OrchSubscribeUrl = orchResp.Header.Get("Subscribe-Url")
		stream.OrchControlUrl = orchResp.Header.Get("Control-Url")
		stream.OrchEventsUrl = orchResp.Header.Get("Events-Url")
		stream.OrchDataUrl = orchResp.Header.Get("Data-Url")

		perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
		params.liveParams = newParams(params.liveParams, perOrchCancel)
		if err = startStreamProcessing(perOrchCtx, stream); err != nil {
			clog.Errorf(ctx, "Error starting processing: %s", err)
			perOrchCancel(err)
			break
		}
		<-perOrchCtx.Done()
		err = context.Cause(perOrchCtx)
		if errors.Is(err, context.Canceled) {
			// this happens if parent ctx was cancelled without a CancelCause
			// or if passing `nil` as a CancelCause
			err = nil
		}
		if !params.inputStreamExists() {
			clog.Info(ctx, "No input stream, skipping orchestrator swap")
			break
		}
		//if swapErr := orchSwapper.checkSwap(ctx); swapErr != nil {
		//	if err != nil {
		//		err = fmt.Errorf("%w: %w", swapErr, err)
		//	} else {
		//		err = swapErr
		//	}
		//	break
		//}
		clog.Infof(ctx, "Retrying stream with a different orchestrator")

		// will swap, but first notify with the reason for the swap
		if err == nil {
			err = errors.New("unknown swap reason")
		}
		params.liveParams.sendErrorEvent(err)

		//if isFirst {
		//	// failed before selecting an orchestrator
		//	firstProcessed <- struct{}{}
		//}
		params.liveParams.kickInput(err)
	}
}

func (ls *LivepeerServer) monitorStream(ctx context.Context, streamId string) {
	stream, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]
	if !exists {
		clog.Errorf(ctx, "Stream %s not found", streamId)
		return
	}

	// Create a ticker that runs every minute for payments
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.StreamCtx.Done():
			clog.Infof(ctx, "Stream %s stopped, ending monitoring", streamId)
			return
		case <-ticker.C:
			// Run payment and fetch new JobToken every minute
			req := &JobRequest{Capability: stream.Capability,
				Sender:  ls.LivepeerNode.OrchestratorPool.Broadcaster().Address().Hex(),
				Timeout: 60,
			}
			pmtHdr, err := createPayment(ctx, req, stream.OrchToken.(JobToken), ls.LivepeerNode)
			if err != nil {
				clog.Errorf(ctx, "Error processing stream payment for %s: %v", streamId, err)
				// Continue monitoring even if payment fails
			}

		}
	}

}

func (ls *LivepeerServer) setupStream(ctx context.Context, r *http.Request, req *JobRequest) (map[string]string, int, error) {
	requestID := string(core.RandomManifestID())
	ctx = clog.AddVal(ctx, "request_id", requestID)

	//live-video-to-video uses path value for this
	streamName := r.FormValue("stream")

	streamRequestTime := time.Now().UnixMilli()

	ctx = clog.AddVal(ctx, "stream", streamName)
	sourceID := r.FormValue("source_id")
	if sourceID == "" {
		return nil, http.StatusBadRequest, errors.New("missing source_id")
	}
	ctx = clog.AddVal(ctx, "source_id", sourceID)
	sourceType := r.FormValue("source_type")
	if sourceType == "" {
		return nil, http.StatusBadRequest, errors.New("missing source_type")
	}
	sourceTypeStr, err := media.MediamtxSourceTypeToString(sourceType)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid source type")
	}
	ctx = clog.AddVal(ctx, "source_type", sourceType)

	//TODO: change the params in query to separate form fields since we
	// are setting things up with a POST request
	queryParams := r.FormValue("query")
	qp, err := url.ParseQuery(queryParams)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid query params")
	}
	// If auth webhook is set and returns an output URL, this will be replaced
	outputURL := qp.Get("rtmpOutput")

	// convention to avoid re-subscribing to our own streams
	// in case we want to push outputs back into mediamtx -
	// use an `-out` suffix for the stream name.
	if strings.HasSuffix(streamName, "-out") {
		// skip for now; we don't want to re-publish our own outputs
		return nil, 0, nil
	}

	// if auth webhook returns pipeline config these will be replaced
	pipeline := qp.Get("pipeline")
	if pipeline == "" {
		pipeline = req.Capability
	}
	rawParams := qp.Get("params")
	streamID := qp.Get("streamId")

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
	// Currently for webrtc we need to add a path prefix due to the ingress setup
	//mediaMTXStreamPrefix := r.PathValue("prefix")
	//if mediaMTXStreamPrefix != "" {
	//	mediaMTXStreamPrefix = mediaMTXStreamPrefix + "/"
	//}
	mediaMtxHost := os.Getenv("LIVE_AI_PLAYBACK_HOST")
	if mediaMtxHost == "" {
		mediaMtxHost = "localhost:1935"
	}
	mediaMTXInputURL := fmt.Sprintf("rtmp://%s/%s%s", mediaMtxHost, "", streamID)
	mediaMTXOutputURL := mediaMTXInputURL + "-out"
	mediaMTXOutputAlias := fmt.Sprintf("%s-%s-out", mediaMTXInputURL, requestID)

	whepURL := generateWhepUrl(streamID, requestID)
	whipURL := fmt.Sprintf("https://%s/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "/whip")
	rtmpURL := mediaMTXInputURL
	updateURL := fmt.Sprintf("https://%s/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "/update")
	statusURL := fmt.Sprintf("https://%s/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "/status")

	//if set this will overwrite settings above
	if LiveAIAuthWebhookURL != nil {
		authResp, err := authenticateAIStream(LiveAIAuthWebhookURL, ls.liveAIAuthApiKey, AIAuthRequest{
			Stream:      streamName,
			Type:        sourceTypeStr,
			QueryParams: queryParams,
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

		//don't want to update the streamid or pipeline id with byoc
		//if authResp.StreamID != "" {
		//	streamID = authResp.StreamID
		//}

		//if authResp.PipelineID != "" {
		//	pipelineID = authResp.PipelineID
		//}
	}

	ctx = clog.AddVal(ctx, "stream_id", streamID)
	clog.Infof(ctx, "Received live video AI request for %s. pipelineParams=%v", streamName, pipelineParams)

	// collect all RTMP outputs
	var rtmpOutputs []string
	if outputURL != "" {
		rtmpOutputs = append(rtmpOutputs, outputURL)
	}
	if mediaMTXOutputURL != "" {
		rtmpOutputs = append(rtmpOutputs, mediaMTXOutputURL, mediaMTXOutputAlias)
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
		monitor.AILiveVideoAttempt(req.Capability)
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
	//kickInput is set with RTMP ingest
	ssr := media.NewSwitchableSegmentReader() //this converts ingest to segments to send to Orchestrator
	params := aiRequestParams{
		node:        ls.LivepeerNode,
		os:          drivers.NodeStorage.NewSession(requestID),
		sessManager: ls.AISessionManager,

		liveParams: &liveRequestParams{
			segmentReader:          ssr,
			rtmpOutputs:            rtmpOutputs,
			stream:                 streamName,
			paymentProcessInterval: ls.livePaymentInterval,
			outSegmentTimeout:      ls.outSegmentTimeout,
			requestID:              requestID,
			streamID:               streamID,
			pipelineID:             pipelineID,
			pipeline:               pipeline,
			sendErrorEvent:         sendErrorEvent,
		},
	}

	//create a dataWriter for data channel if enabled
	if enableData, ok := pipelineParams["enableData"]; ok {
		if enableData == true || enableData == "true" {
			params.liveParams.dataWriter = media.NewSegmentWriter(5)
			pipelineParams["enableData"] = true
			clog.Infof(ctx, "Data channel enabled for stream %s", streamName)
		}
	}

	if _, ok := pipelineParams["enableVideoIngress"]; !ok {
		pipelineParams["enableVideoIngress"] = true
	}

	if _, ok := pipelineParams["enableVideoEgress"]; !ok {
		pipelineParams["enableVideoEgress"] = true
	}

	//send start request to Orchestrator
	_, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamID]
	if exists {
		return nil, http.StatusBadRequest, fmt.Errorf("stream already exists: %s", streamID)
	}

	// read entire body to ensure valid and send to orchestrator
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("Error reading request body")
	}
	r.Body.Close()

	//save the stream setup
	if err := ls.LivepeerNode.ExternalCapabilities.AddStream(streamID, params, body); err != nil {
		return nil, http.StatusBadRequest, err
	}

	resp := make(map[string]string)
	resp["stream_id"] = streamID
	resp["whip_url"] = whipURL
	resp["whep_url"] = whepURL
	resp["rtmp_url"] = rtmpURL
	resp["rtmp_output_url"] = strings.Join(rtmpOutputs, ",")
	resp["update_url"] = updateURL
	resp["status_url"] = statusURL

	return resp, http.StatusOK, nil
}

// mediamtx sends this request to go-livepeer when rtmp strem received
func (ls *LivepeerServer) StartStreamRTMPIngest() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		streamId := r.PathValue("streamId")
		ctx = clog.AddVal(ctx, "stream_id", streamId)

		stream, ok := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]
		if !ok {
			respondJsonError(ctx, w, fmt.Errorf("stream not found: %s", streamId), http.StatusNotFound)
			return
		}

		params := stream.Params.(aiRequestParams)

		//note that mediaMtxHost is the ip address of media mtx
		// media sends a post request in the runOnReady event setup in mediamtx.yml
		// StartLiveVideo calls this remoteHost
		mediaMtxHost, err := getRemoteHost(r.RemoteAddr)
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
		mediaMTXClient := media.NewMediaMTXClient(mediaMtxHost, ls.mediaMTXApiPassword, "rtmp_ingest", "rtmp")

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		segmenterCtx, cancelSegmenter := context.WithCancel(clog.Clone(context.Background(), ctx))
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

		params.liveParams.kickInput = kickInput

		// Create a special parent context for orchestrator cancellation
		//orchCtx, orchCancel := context.WithCancel(ctx)

		// Kick off the RTMP pull and segmentation
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
			cleanupControl(ctx, params)
		}()

		var req worker.GenLiveVideoToVideoJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}
	})
}

func startStream(ctx context.Context, streamData *core.StreamData, req worker.GenLiveVideoToVideoJSONRequestBody) {
	params := streamData.Params.(aiRequestParams)
	orchSwapper := NewOrchestratorSwapper(params)
	isFirst, firstProcessed := true, make(chan interface{})
	go func() {
		var err error
		for {
			perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
			params.liveParams = newParams(params.liveParams, perOrchCancel)

			if err != nil {
				clog.Errorf(ctx, "Error processing AI Request: %s", err)
				perOrchCancel(err)
				break
			}

			if err = startStreamProcessing(perOrchCtx, streamData); err != nil {
				clog.Errorf(ctx, "Error starting processing: %s", err)
				perOrchCancel(err)
				break
			}
			if isFirst {
				isFirst = false
				firstProcessed <- struct{}{}
			}
			<-perOrchCtx.Done()
			err = context.Cause(perOrchCtx)
			if errors.Is(err, context.Canceled) {
				// this happens if parent ctx was cancelled without a CancelCause
				// or if passing `nil` as a CancelCause
				err = nil
			}
			if !params.inputStreamExists() {
				clog.Info(ctx, "No input stream, skipping orchestrator swap")
				break
			}
			if swapErr := orchSwapper.checkSwap(ctx); swapErr != nil {
				if err != nil {
					err = fmt.Errorf("%w: %w", swapErr, err)
				} else {
					err = swapErr
				}
				break
			}
			clog.Infof(ctx, "Retrying stream with a different orchestrator")

			// will swap, but first notify with the reason for the swap
			if err == nil {
				err = errors.New("unknown swap reason")
			}
			params.liveParams.sendErrorEvent(err)
		}
		if isFirst {
			// failed before selecting an orchestrator, exit the stream, something is wrong
			firstProcessed <- struct{}{}
		}
		params.liveParams.kickInput(err)
	}()
	<-firstProcessed
}

func startStreamProcessing(ctx context.Context, streamData *core.StreamData) error {
	params := streamData.Params.(aiRequestParams)
	host := streamData.OrchUrl
	var channels []string

	//this adds the stream to LivePipelines which the Control Publisher and Data Writer
	//are accessible for reading data and sending updates
	registerControl(ctx, params)

	//required channels
	control, err := common.AppendHostname(streamData.OrchControlUrl, host)
	if err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}
	events, err := common.AppendHostname(streamData.OrchEventsUrl, host)
	if err != nil {
		return fmt.Errorf("invalid events URL: %w", err)
	}

	channels = append(channels, control.String())
	channels = append(channels, events.String())
	startControlPublish(ctx, control, params)
	startEventsSubscribe(ctx, events, params, params.liveParams.sess)

	//Optional channels
	if streamData.OrchPublishUrl == "" {
		pub, err := common.AppendHostname(streamData.OrchPublishUrl, host)
		if err != nil {
			return fmt.Errorf("invalid publish URL: %w", err)
		}
		channels = append(channels, pub.String())
		startTricklePublish(ctx, pub, params, params.liveParams.sess)
	}

	if streamData.OrchSubscribeUrl == "" {
		sub, err := common.AppendHostname(streamData.OrchSubscribeUrl, host)
		if err != nil {
			return fmt.Errorf("invalid subscribe URL: %w", err)
		}
		channels = append(channels, sub.String())
		startTrickleSubscribe(ctx, sub, params, params.liveParams.sess)
	}

	if streamData.OrchDataUrl == "" {
		data, err := common.AppendHostname(streamData.OrchDataUrl, host)
		if err != nil {
			return fmt.Errorf("invalid data URL: %w", err)
		}
		streamData.Params.(aiRequestParams).liveParams.manifestID = streamData.Capability

		startDataSubscribe(ctx, data, params, params.liveParams.sess)
	}

	return nil
}

// StartStreamOrchestrator handles the POST /stream/start endpoint for the Orchestrator
func (h *lphttp) StartStreamOrchestrator() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		streamID := r.Header.Get("streamID")
		gatewayRequestID := r.Header.Get("requestID")
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "orch_request_id", requestID)
		ctx = clog.AddVal(ctx, "gateway_request_id", gatewayRequestID)
		ctx = clog.AddVal(ctx, "manifest_id", requestID)
		ctx = clog.AddVal(ctx, "stream_id", streamID)

		var req worker.GenLiveVideoToVideoJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		orch := h.orchestrator
		pipeline := "byoc-stream"
		cap := core.Capability_LiveVideoToVideo
		modelID := *req.ModelId
		clog.V(common.VERBOSE).Infof(ctx, "Received request id=%v cap=%v modelID=%v", requestID, cap, modelID)

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
			dataUrl    = pubUrl + "-data"
		)

		//if data is not enabled remove the url and do not start the data channel
		if enableData, ok := (*req.Params)["enableData"]; ok {
			if val, ok := enableData.(bool); ok {
				//turn off data channel if request sets to false
				if !val {
					dataUrl = ""
				} else {
					clog.Infof(ctx, "data channel is enabled")
				}
			} else {
				clog.Warningf(ctx, "enableData is not a bool, got type %T", enableData)
			}

			//delete the param used for go-livepeer signaling
			delete((*req.Params), "enableData")
		} else {
			//default to no data channel
			dataUrl = ""
		}

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

		// If successful, then create the trickle channels
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
		//optional channels
		var dataCh *trickle.TrickleLocalPublisher
		if dataUrl != "" {
			dataCh = trickle.NewLocalPublisher(h.trickleSrv, mid+"-data", "application/jsonl")
			dataCh.CreateChannel()
		}

		// Start payment receiver which accounts the payments and stops the stream if the payment is insufficient
		priceInfo := payment.GetExpectedPrice()
		var paymentProcessor *LivePaymentProcessor
		ctx, cancel := context.WithCancel(context.Background())
		if priceInfo != nil && priceInfo.PricePerUnit != 0 {
			paymentReceiver := livePaymentReceiver{orchestrator: h.orchestrator}
			accountPaymentFunc := func(inPixels int64) error {
				err := paymentReceiver.AccountPayment(context.Background(), &SegmentInfoReceiver{
					sender:    sender,
					inPixels:  inPixels,
					priceInfo: priceInfo,
					sessionID: mid,
				})
				if err != nil {
					slog.Warn("Error accounting payment, stopping stream processing", "err", err)
					pubCh.Close()
					subCh.Close()
					eventsCh.Close()
					controlPubCh.Close()
					cancel()
				}
				return err
			}
			paymentProcessor = NewLivePaymentProcessor(ctx, h.node.LivePaymentInterval, accountPaymentFunc)
		} else {
			clog.Warningf(ctx, "No price info found for model %v, Orchestrator will not charge for video processing", modelID)
		}

		// Subscribe to the publishUrl for payments monitoring and payment processing
		go func() {
			sub := trickle.NewLocalSubscriber(h.trickleSrv, mid)
			for {
				segment, err := sub.Read()
				if err != nil {
					clog.Infof(ctx, "Error getting local trickle segment err=%v", err)
					return
				}
				reader := segment.Reader
				if paymentProcessor != nil {
					reader = paymentProcessor.process(ctx, segment.Reader)
				}
				io.Copy(io.Discard, reader)
			}
		}()

		// Prepare request to worker
		controlUrlOverwrite := overwriteHostInStream(h.node.LiveAITrickleHostForRunner, controlUrl)
		eventsUrlOverwrite := overwriteHostInStream(h.node.LiveAITrickleHostForRunner, eventsUrl)
		subscribeUrlOverwrite := overwriteHostInStream(h.node.LiveAITrickleHostForRunner, pubUrl)
		publishUrlOverwrite := overwriteHostInStream(h.node.LiveAITrickleHostForRunner, subUrl)
		// optional channels
		var dataUrlOverwrite string
		if dataCh != nil {
			dataUrlOverwrite = overwriteHost(h.node.LiveAITrickleHostForRunner, dataUrl)
		}

		workerReq := worker.LiveVideoToVideoParams{
			ModelId:          req.ModelId,
			PublishUrl:       publishUrlOverwrite,
			SubscribeUrl:     subscribeUrlOverwrite,
			EventsUrl:        &eventsUrlOverwrite,
			ControlUrl:       &controlUrlOverwrite,
			DataUrl:          &dataUrlOverwrite,
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

			pubCh.Close()
			subCh.Close()
			controlPubCh.Close()
			eventsCh.Close()
			dataCh.Close()
			cancel()
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare the response
		jsonData, err := json.Marshal(&worker.LiveVideoToVideoResponse{
			PublishUrl:   pubUrl,
			SubscribeUrl: subUrl,
			ControlUrl:   &controlUrl,
			EventsUrl:    &eventsUrl,
			RequestId:    &requestID,
			ManifestId:   &mid,
		})
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		clog.Infof(ctx, "Processed request id=%v cap=%v modelID=%v took=%v", requestID, cap, modelID)
		respondJsonOk(w, jsonData)
	})
}
