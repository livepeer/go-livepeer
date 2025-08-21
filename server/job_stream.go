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
	url2 "net/url"
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
		ctx := context.Background()

		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		streamName := r.FormValue("stream")
		if streamName == "" {
			clog.Errorf(ctx, "Missing stream name")
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}

		streamRequestTime := time.Now().UnixMilli()

		ctx = clog.AddVal(ctx, "stream", streamName)
		sourceID := r.FormValue("source_id")
		if sourceID == "" {
			clog.Errorf(ctx, "Missing source_id")
			http.Error(w, "Missing source_id", http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "source_id", sourceID)
		sourceType := r.FormValue("source_type")
		if sourceType == "" {
			clog.Errorf(ctx, "Missing source_type")
			http.Error(w, "Missing source_type", http.StatusBadRequest)
			return
		}
		sourceTypeStr, err := media.MediamtxSourceTypeToString(sourceType)
		if err != nil {
			clog.Errorf(ctx, "Invalid source type %s", sourceType)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "source_type", sourceType)

		remoteHost, err := getRemoteHost(r.RemoteAddr)
		if err != nil {
			clog.Errorf(ctx, "Could not find callback host: %s", err.Error())
			http.Error(w, "Could not find callback host", http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "remote_addr", remoteHost)

		queryParams := r.FormValue("query")
		qp, err := url.ParseQuery(queryParams)
		if err != nil {
			clog.Errorf(ctx, "invalid query params, err=%w", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// If auth webhook is set and returns an output URL, this will be replaced
		outputURL := qp.Get("rtmpOutput")
		// Currently for webrtc we need to add a path prefix due to the ingress setup
		mediaMTXStreamPrefix := r.PathValue("prefix")
		if mediaMTXStreamPrefix != "" {
			mediaMTXStreamPrefix = mediaMTXStreamPrefix + "/"
		}
		mediaMTXInputURL := fmt.Sprintf("rtmp://%s/%s%s", remoteHost, mediaMTXStreamPrefix, streamName)
		mediaMTXRtmpURL := r.FormValue("rtmp_url")
		if mediaMTXRtmpURL != "" {
			mediaMTXInputURL = mediaMTXRtmpURL
		}
		mediaMTXOutputURL := mediaMTXInputURL + "-out"
		mediaMTXOutputAlias := fmt.Sprintf("%s-%s-out", mediaMTXInputURL, requestID)

		// convention to avoid re-subscribing to our own streams
		// in case we want to push outputs back into mediamtx -
		// use an `-out` suffix for the stream name.
		if strings.HasSuffix(streamName, "-out") {
			// skip for now; we don't want to re-publish our own outputs
			return
		}

		// if auth webhook returns pipeline config these will be replaced
		pipeline := qp.Get("pipeline")
		rawParams := qp.Get("params")
		streamID := qp.Get("streamId")
		var pipelineID string
		var pipelineParams map[string]interface{}
		if rawParams != "" {
			if err := json.Unmarshal([]byte(rawParams), &pipelineParams); err != nil {
				clog.Errorf(ctx, "Invalid pipeline params: %s", err)
				http.Error(w, "Invalid model params", http.StatusBadRequest)
				return
			}
		}

		mediaMTXClient := media.NewMediaMTXClient(remoteHost, ls.mediaMTXApiPassword, sourceID, sourceType)

		whepURL := generateWhepUrl(streamName, requestID)
		whipURL := fmt.Sprintf("https://%s/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamName, "/whip")
		updateURL := fmt.Sprintf("https://%s/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamName, "/update")
		statusURL := fmt.Sprintf("https://%s/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "/status")

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
				kickErr := mediaMTXClient.KickInputConnection(ctx)
				if kickErr != nil {
					clog.Errorf(ctx, "failed to kick input connection: %s", kickErr.Error())
				}
				clog.Errorf(ctx, "Live AI auth failed: %s", err.Error())
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
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

		// channel that blocks until after orch selection is complete
		// avoids a race condition with closing the control channel
		orchSelection := make(chan bool)

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
			monitor.AILiveVideoAttempt()
		}

		sendErrorEvent := LiveErrorEventSender(ctx, streamID, map[string]string{
			"type":        "error",
			"request_id":  requestID,
			"stream_id":   streamID,
			"pipeline_id": pipelineID,
			"pipeline":    pipeline,
		})

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		segmenterCtx, cancelSegmenter := context.WithCancel(clog.Clone(context.Background(), ctx))
		kickInput := func(err error) {
			defer cancelSegmenter()
			if err == nil {
				return
			}
			clog.Errorf(ctx, "Live video pipeline finished with error: %s", err)

			sendErrorEvent(err)

			err = mediaMTXClient.KickInputConnection(ctx)
			if err != nil {
				clog.Errorf(ctx, "Failed to kick input connection: %s", err)
			}
		}

		ssr := media.NewSwitchableSegmentReader()
		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,

			liveParams: &liveRequestParams{
				segmentReader:          ssr,
				rtmpOutputs:            rtmpOutputs,
				localRTMPPrefix:        mediaMTXInputURL,
				stream:                 streamName,
				paymentProcessInterval: ls.livePaymentInterval,
				outSegmentTimeout:      ls.outSegmentTimeout,
				requestID:              requestID,
				streamID:               streamID,
				pipelineID:             pipelineID,
				pipeline:               pipeline,
				kickInput:              kickInput,
				sendErrorEvent:         sendErrorEvent,
			},
		}

		registerControl(ctx, params)

		// Create a special parent context for orchestrator cancellation
		orchCtx, orchCancel := context.WithCancel(ctx)

		// Kick off the RTMP pull and segmentation as soon as possible
		go func() {
			ms := media.MediaSegmenter{Workdir: ls.LivepeerNode.WorkDir, MediaMTXClient: mediaMTXClient}

			// Wait for stream to exist before starting segmentation
			//   in the case of no input stream but only generating an output stream from instructions
			//   the segmenter will never start
			for {
				streamExists, err := mediaMTXClient.StreamExists()
				if err != nil {
					clog.Errorf(ctx, "Error checking if stream exists: %v", err)
				} else if streamExists {
					break
				}
				select {
				case <-segmenterCtx.Done():
					return
				case <-time.After(200 * time.Millisecond):
					// Continue waiting
				}
			}

			//blocks until error or stream ends
			ms.RunSegmentation(segmenterCtx, mediaMTXInputURL, ssr.Read)
			sendErrorEvent(errors.New("mediamtx ingest disconnected"))
			monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
				"type":        "gateway_ingest_stream_closed",
				"timestamp":   time.Now().UnixMilli(),
				"stream_id":   streamID,
				"pipeline_id": pipelineID,
				"request_id":  requestID,
				"orchestrator_info": map[string]interface{}{
					"address": "",
					"url":     "",
				},
			})
			ssr.Close()
			<-orchSelection // wait for selection to complete
			cleanupControl(ctx, params)
			orchCancel()
		}()

		req := worker.GenLiveVideoToVideoJSONRequestBody{
			ModelId:          &pipeline,
			Params:           &pipelineParams,
			GatewayRequestId: &requestID,
			StreamId:         &streamID,
		}

		startStream(orchCtx, params, req)
		if err != nil {
			clog.Errorf(ctx, "Error starting stream: %s", err)
		}
		close(orchSelection)

		resp := map[string]string{}
		resp["whip_url"] = whipURL
		resp["update_url"] = updateURL
		resp["status_url"] = statusURL

		respJson, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJson)
	})
}

func startStream(ctx context.Context, params aiRequestParams, req worker.GenLiveVideoToVideoJSONRequestBody) {
	orchSwapper := NewOrchestratorSwapper(params)
	isFirst, firstProcessed := true, make(chan interface{})
	go func() {
		var err error
		for {
			perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
			params.liveParams = newParams(params.liveParams, perOrchCancel)

			//need to update this to do a standard BYOC request
			var resp interface{}
			resp, err = processAIRequest(perOrchCtx, params, req)

			if err != nil {
				clog.Errorf(ctx, "Error processing AI Request: %s", err)
				perOrchCancel(err)
				break
			}

			if err = startStreamProcessing(perOrchCtx, params, resp); err != nil {
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
			// failed before selecting an orchestrator
			firstProcessed <- struct{}{}
		}
		params.liveParams.kickInput(err)
	}()
	<-firstProcessed
}

func startStreamProcessing(ctx context.Context, params aiRequestParams, res interface{}) error {
	resp := res.(*worker.GenLiveVideoToVideoResponse)

	host := params.liveParams.sess.Transcoder()
	pub, err := common.AppendHostname(resp.JSON200.PublishUrl, host)
	if err != nil {
		return fmt.Errorf("invalid publish URL: %w", err)
	}
	sub, err := common.AppendHostname(resp.JSON200.SubscribeUrl, host)
	if err != nil {
		return fmt.Errorf("invalid subscribe URL: %w", err)
	}
	control, err := common.AppendHostname(*resp.JSON200.ControlUrl, host)
	if err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}
	events, err := common.AppendHostname(*resp.JSON200.EventsUrl, host)
	if err != nil {
		return fmt.Errorf("invalid events URL: %w", err)
	}
	data, err := common.AppendHostname(strings.Replace(*resp.JSON200.EventsUrl, "-events", "-data", 1), host)
	if err != nil {
		return fmt.Errorf("invalid data URL: %w", err)
	}
	if resp.JSON200.ManifestId != nil {
		ctx = clog.AddVal(ctx, "manifest_id", *resp.JSON200.ManifestId)
		params.liveParams.manifestID = *resp.JSON200.ManifestId
	}
	clog.V(common.VERBOSE).Infof(ctx, "pub %s sub %s control %s events %s data %s", pub, sub, control, events, data)

	//TODO make this configurable
	startControlPublish(ctx, control, params)
	startTricklePublish(ctx, pub, params, params.liveParams.sess)
	startTrickleSubscribe(ctx, sub, params, params.liveParams.sess)
	startEventsSubscribe(ctx, events, params, params.liveParams.sess)
	startDataSubscribe(ctx, data, params, params.liveParams.sess)
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
		dataCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-data", "application/json")
		dataCh.CreateChannel()

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
		dataUrlOverwrite := overwriteHostInStream(h.node.LiveAITrickleHostForRunner, dataUrl)

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

// This function is copied from ai_http.go to avoid import cycle issues
func overwriteHostInStream(hostOverwrite, url string) string {
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
