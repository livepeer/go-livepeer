package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
)

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
		gatewayJob, err := ls.setupGatewayJob(ctx, r)
		if err != nil {
			clog.Errorf(ctx, "Error setting up job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp, code, err := ls.setupStream(ctx, r, gatewayJob.Job.Req)
		if err != nil {
			clog.Errorf(ctx, "Error setting up stream: %s", err)
			http.Error(w, err.Error(), code)
			return
		}

		go ls.runStream(gatewayJob)

		go ls.monitorStream(gatewayJob.Job.Req.ID)

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

func (ls *LivepeerServer) StopStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create fresh context instead of using r.Context() since ctx will outlive the request
		ctx := r.Context()
		streamId := r.PathValue("streamId")

		if streamInfo, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]; exists {
			// Copy streamInfo before deletion
			streamInfoCopy := *streamInfo
			//remove the stream
			ls.LivepeerNode.ExternalCapabilities.RemoveStream(streamId)

			stopJob, err := ls.setupGatewayJob(ctx, r)
			if err != nil {
				clog.Errorf(ctx, "Error setting up stop job: %s", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			stopJob.sign()
			resp, code, err := ls.sendJobToOrch(ctx, r, stopJob.Job.Req, stopJob.SignedJobReq, streamInfoCopy.OrchToken.(JobToken), "/ai/stream/stop", streamInfoCopy.StreamRequest)
			if err != nil {
				clog.Errorf(ctx, "Error sending job to orchestrator: %s", err)
				http.Error(w, err.Error(), code)
				return
			}

			w.WriteHeader(http.StatusOK)
			io.Copy(w, resp.Body)
			return
		}

		//no stream exists
		w.WriteHeader(http.StatusNoContent)
		return

	})
}

func (ls *LivepeerServer) runStream(gatewayJob *gatewayJob) {
	streamID := gatewayJob.Job.Req.ID
	stream, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamID]
	if !exists {
		glog.Errorf("Stream %s not found", streamID)
		return
	}
	params := stream.Params.(aiRequestParams)
	//this context passes to all channels that will close when stream is canceled
	ctx := stream.StreamCtx
	ctx = clog.AddVal(ctx, "stream_id", streamID)

	start := time.Now()
	for _, orch := range gatewayJob.Orchs {
		stream.OrchToken = orch
		stream.OrchUrl = orch.ServiceAddr
		ctx = clog.AddVal(ctx, "orch", ethcommon.Bytes2Hex(orch.TicketParams.Recipient))
		ctx = clog.AddVal(ctx, "orch_url", orch.ServiceAddr)
		clog.Infof(ctx, "Starting stream processing")
		//refresh the token if after 5 minutes from start. TicketParams expire in 40 blocks (about 8 minutes).
		if time.Since(start) > 5*time.Minute {
			orch, err := getToken(ctx, 3*time.Second, orch.ServiceAddr, gatewayJob.Job.Req.Capability, gatewayJob.Job.Req.Sender, gatewayJob.Job.Req.Sig)
			if err != nil {
				clog.Errorf(ctx, "Error getting token for orch=%v err=%v", orch.ServiceAddr, err)
				continue
			}
		}

		//set request ID to persist from Gateway to Worker
		gatewayJob.Job.Req.ID = stream.StreamID
		err := gatewayJob.sign()
		if err != nil {
			clog.Errorf(ctx, "Error signing job, exiting stream processing request: %v", err)
			stream.CancelStream()
			return
		}
		orchResp, _, err := ls.sendJobToOrch(ctx, nil, gatewayJob.Job.Req, gatewayJob.SignedJobReq, orch, "/ai/stream/start", stream.StreamRequest)
		if err != nil {
			clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orch.ServiceAddr, err.Error())
			continue
		}

		stream.OrchPublishUrl = orchResp.Header.Get("X-Publish-Url")
		stream.OrchSubscribeUrl = orchResp.Header.Get("X-Subscribe-Url")
		stream.OrchControlUrl = orchResp.Header.Get("X-Control-Url")
		stream.OrchEventsUrl = orchResp.Header.Get("X-Events-Url")
		stream.OrchDataUrl = orchResp.Header.Get("X-Data-Url")

		perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
		params.liveParams.kickOrch = perOrchCancel
		stream.Params = params //update params used to kickOrch (perOrchCancel)
		if err = startStreamProcessing(perOrchCtx, stream); err != nil {
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
		if !ls.LivepeerNode.ExternalCapabilities.StreamExists(streamID) {
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

		//if there is ingress input then force off
		if params.liveParams.kickInput != nil {
			params.liveParams.kickInput(err)
		}
	}

	//exhausted all Orchestrators, end stream
	ls.LivepeerNode.ExternalCapabilities.RemoveStream(streamID)
}

func (ls *LivepeerServer) monitorStream(streamId string) {
	ctx := context.Background()
	ctx = clog.AddVal(ctx, "stream_id", streamId)

	stream, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]
	if !exists {
		clog.Errorf(ctx, "Stream %s not found", streamId)
		return
	}
	params := stream.Params.(aiRequestParams)

	ctx = clog.AddVal(ctx, "request_id", params.liveParams.requestID)

	// Create a ticker that runs every minute for payments
	pmtTicker := time.NewTicker(45 * time.Second)
	defer pmtTicker.Stop()

	for {
		select {
		case <-stream.StreamCtx.Done():
			clog.Infof(ctx, "Stream %s stopped, ending monitoring", streamId)
			ls.LivepeerNode.ExternalCapabilities.RemoveStream(streamId)
			return
		case <-pmtTicker.C:
			// Send payment and fetch new JobToken every minute
			// TicketParams expire in about 8 minutes so have multiple chances to get new token
			// Each payment is for 60 seconds of compute
			jobDetails := JobRequestDetails{StreamId: streamId}
			jobDetailsStr, err := json.Marshal(jobDetails)
			if err != nil {
				clog.Errorf(ctx, "Error marshalling job details: %v", err)
				continue
			}
			req := &JobRequest{Request: string(jobDetailsStr), Parameters: "{}", Capability: stream.Capability,
				Sender:  ls.LivepeerNode.OrchestratorPool.Broadcaster().Address().Hex(),
				Timeout: 60,
			}
			//sign the request
			job := gatewayJob{Job: &orchJob{Req: req}, node: ls.LivepeerNode}
			err = job.sign()
			if err != nil {
				clog.Errorf(ctx, "Error signing job, continuing monitoring: %v", err)
				continue
			}

			pmtHdr, err := createPayment(ctx, req, stream.OrchToken.(JobToken), ls.LivepeerNode)
			if err != nil {
				clog.Errorf(ctx, "Error processing stream payment for %s: %v", streamId, err)
				// Continue monitoring even if payment fails
			}

			//send the payment, update the stream with the refreshed token
			clog.Infof(ctx, "Sending stream payment for %s", streamId)
			newToken, err := ls.sendPayment(ctx, stream.OrchUrl+"/ai/stream/payment", stream.Capability, job.SignedJobReq, pmtHdr)
			if err != nil {
				clog.Errorf(ctx, "Error sending stream payment for %s: %v", streamId, err)
				continue
			}
			if newToken == nil {
				clog.Errorf(ctx, "Updated token not received for %s", streamId)
				continue
			}
			streamToken := stream.OrchToken.(JobToken)
			streamToken.TicketParams = newToken.TicketParams
			streamToken.Balance = newToken.Balance
			streamToken.Price = newToken.Price
			stream.OrchToken = streamToken
		}
	}
}

func (ls *LivepeerServer) setupStream(ctx context.Context, r *http.Request, req *JobRequest) (map[string]string, int, error) {
	requestID := string(core.RandomManifestID())
	ctx = clog.AddVal(ctx, "request_id", requestID)

	// Setup request body to be able to preserve for retries
	// Read the entire body first
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	// Create a clone of the request with a new body for parsing form values
	bodyForForm := bytes.NewBuffer(bodyBytes)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset the original body

	// Create a temporary request for parsing form data
	formReq := r.Clone(ctx)
	formReq.Body = io.NopCloser(bodyForForm)
	defer r.Body.Close()
	defer formReq.Body.Close()

	// Parse the form (10MB max)
	if err := formReq.ParseMultipartForm(10 << 20); err != nil {
		return nil, http.StatusBadRequest, err
	}
	//live-video-to-video uses path value for this
	streamName := formReq.FormValue("stream")

	streamRequestTime := time.Now().UnixMilli()

	ctx = clog.AddVal(ctx, "stream", streamName)
	//sourceID := formReq.FormValue("source_id")
	//if sourceID == "" {
	//	return nil, http.StatusBadRequest, errors.New("missing source_id")
	//}
	//ctx = clog.AddVal(ctx, "source_id", sourceID)
	//sourceType := formReq.FormValue("source_type")
	//if sourceType == "" {
	//	return nil, http.StatusBadRequest, errors.New("missing source_type")
	//}
	//sourceTypeStr, err := media.MediamtxSourceTypeToString(sourceType)
	//if err != nil {
	//	return nil, http.StatusBadRequest, errors.New("invalid source type")
	//}
	//ctx = clog.AddVal(ctx, "source_type", sourceType)

	// If auth webhook is set and returns an output URL, this will be replaced
	outputURL := formReq.FormValue("rtmpOutput")

	// convention to avoid re-subscribing to our own streams
	// in case we want to push outputs back into mediamtx -
	// use an `-out` suffix for the stream name.
	if strings.HasSuffix(streamName, "-out") {
		// skip for now; we don't want to re-publish our own outputs
		return nil, 0, nil
	}

	// if auth webhook returns pipeline config these will be replaced
	pipeline := req.Capability //streamParamsJson["pipeline"].(string)
	rawParams := formReq.FormValue("params")
	streamID := formReq.FormValue("streamId")

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

	whepURL := generateWhepUrl(streamID, requestID)
	whipURL := fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "whip")
	rtmpURL := mediaMTXInputURL
	updateURL := fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "update")
	statusURL := fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "status")
	dataURL := fmt.Sprintf("https://%s/ai/stream/%s/%s", ls.LivepeerNode.GatewayHost, streamID, "data")

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

		//don't want to update the streamid or pipeline id with byoc
		//if authResp.StreamID != "" {
		//	streamID = authResp.StreamID
		//}

		//if authResp.PipelineID != "" {
		//	pipelineID = authResp.PipelineID
		//}
	}

	ctx = clog.AddVal(ctx, "stream_id", streamID)
	clog.Infof(ctx, "Received live video AI request pipelineParams=%v", streamID, pipelineParams)

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
	var jobParams JobParameters
	if err = json.Unmarshal([]byte(req.Parameters), &jobParams); err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid job parameters")
	}
	if jobParams.EnableDataOutput {
		params.liveParams.dataWriter = media.NewSegmentWriter(5)
	}

	//check if stream exists
	_, exists := ls.LivepeerNode.ExternalCapabilities.Streams[streamID]
	if exists {
		return nil, http.StatusBadRequest, fmt.Errorf("stream already exists: %s", streamID)
	}

	clog.Infof(ctx, "stream setup videoIngress=%v videoEgress=%v dataOutput=%v", jobParams.EnableVideoIngress, jobParams.EnableVideoEgress, jobParams.EnableDataOutput)

	// Close the body, done with all form variables
	r.Body.Close()

	//save the stream setup
	if err := ls.LivepeerNode.ExternalCapabilities.AddStream(streamID, pipeline, params, bodyBytes); err != nil {
		return nil, http.StatusBadRequest, err
	}

	req.ID = streamID
	resp := make(map[string]string)
	resp["stream_id"] = streamID
	resp["whip_url"] = whipURL
	resp["whep_url"] = whepURL
	resp["rtmp_url"] = rtmpURL
	resp["rtmp_output_url"] = strings.Join(rtmpOutputs, ",")
	resp["update_url"] = updateURL
	resp["status_url"] = statusURL
	resp["data_url"] = dataURL

	return resp, http.StatusOK, nil
}

// mediamtx sends this request to go-livepeer when rtmp stream received
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

		params.liveParams.kickInput = kickInput
		stream.Params = params //update params used to kickInput

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

			stream.CancelStream() //cleanupControl(ctx, params)
		}()
	})
}

func (ls *LivepeerServer) StartStreamWhipIngest(whipServer *media.WHIPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		streamId := r.PathValue("streamId")
		ctx = clog.AddVal(ctx, "stream_id", streamId)

		stream, ok := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]
		if !ok {
			respondJsonError(ctx, w, fmt.Errorf("stream not found: %s", streamId), http.StatusNotFound)
			return
		}

		params := stream.Params.(aiRequestParams)

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
		stream.Params = params //update params used to kickInput

		//wait for the WHIP connection to close and then cleanup
		go func() {
			statsContext, statsCancel := context.WithCancel(ctx)
			defer statsCancel()
			go runStats(statsContext, whipConn, streamId, stream.Capability, params.liveParams.requestID)

			whipConn.AwaitClose()
			stream.CancelStream() //cleanupControl(ctx, params)
			params.liveParams.segmentReader.Close()
			params.liveParams.kickOrch(errors.New("whip ingest disconnected"))
			clog.Info(ctx, "Live cleaned up")
		}()

		conn := whipServer.CreateWHIP(ctx, params.liveParams.segmentReader, whepURL, w, r)
		whipConn.SetWHIPConnection(conn) // might be nil if theres an error and thats okay
	})

}

func startStreamProcessing(ctx context.Context, streamInfo *core.StreamInfo) error {
	var channels []string

	//required channels
	control, err := common.AppendHostname(streamInfo.OrchControlUrl, streamInfo.OrchUrl)
	if err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}
	events, err := common.AppendHostname(streamInfo.OrchEventsUrl, streamInfo.OrchUrl)
	if err != nil {
		return fmt.Errorf("invalid events URL: %w", err)
	}

	channels = append(channels, control.String())
	channels = append(channels, events.String())
	startStreamControlPublish(ctx, control, streamInfo)
	startStreamEventsSubscribe(ctx, events, streamInfo)

	//Optional channels
	if streamInfo.OrchPublishUrl != "" {
		clog.Infof(ctx, "Starting video ingress publisher")
		pub, err := common.AppendHostname(streamInfo.OrchPublishUrl, streamInfo.OrchUrl)
		if err != nil {
			return fmt.Errorf("invalid publish URL: %w", err)
		}
		channels = append(channels, pub.String())
		startStreamTricklePublish(ctx, pub, streamInfo)
	}

	if streamInfo.OrchSubscribeUrl != "" {
		clog.Infof(ctx, "Starting video egress subscriber")
		sub, err := common.AppendHostname(streamInfo.OrchSubscribeUrl, streamInfo.OrchUrl)
		if err != nil {
			return fmt.Errorf("invalid subscribe URL: %w", err)
		}
		channels = append(channels, sub.String())
		startStreamTrickleSubscribe(ctx, sub, streamInfo)
	}

	if streamInfo.OrchDataUrl != "" {
		clog.Infof(ctx, "Starting data channel subscriber")
		data, err := common.AppendHostname(streamInfo.OrchDataUrl, streamInfo.OrchUrl)
		if err != nil {
			return fmt.Errorf("invalid data URL: %w", err)
		}
		streamInfo.Params.(aiRequestParams).liveParams.manifestID = streamInfo.Capability

		startStreamDataSubscribe(ctx, data, streamInfo)
	}

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
		stream, ok := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]
		if !ok {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		params := stream.Params.(aiRequestParams)
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

		clog.Infof(ctx, "Starting SSE data stream for stream=%s", stream)

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

				data, err := io.ReadAll(reader)
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
		// Get stream from path param
		streamId := r.PathValue("streamId")
		if streamId == "" {
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}
		stream, ok := ls.LivepeerNode.ExternalCapabilities.Streams[streamId]
		if !ok {
			// Stream not found
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}

		reader := http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10 MB
		defer reader.Close()
		data, err := io.ReadAll(reader)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		params := string(data)
		stream.JobParams = params
		controlPub := stream.ControlPub

		if controlPub == nil {
			clog.Info(ctx, "No orchestrator available, caching params", "stream", stream, "params", params)
			return
		}

		clog.V(6).Infof(ctx, "Sending Live Video Update Control API stream=%s, params=%s", stream, params)
		if err := controlPub.Write(strings.NewReader(params)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		corsHeaders(w, r.Method)
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
		respondWithError(w, err.Error(), http.StatusBadRequest)
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
		mid        = orchJob.Req.ID // Request ID is used for the manifest ID
		pubUrl     = h.orchestrator.ServiceURI().JoinPath(TrickleHTTPPath, mid).String()
		subUrl     = pubUrl + "-out"
		controlUrl = pubUrl + "-control"
		eventsUrl  = pubUrl + "-events"
		dataUrl    = pubUrl + "-data"
	)

	reqBodyForRunner := make(map[string]interface{})
	reqBodyForRunner["gateway_request_id"] = mid
	//required channels
	controlPubCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-control", "application/json")
	controlPubCh.CreateChannel()
	controlUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, controlUrl)
	reqBodyForRunner["control_url"] = controlUrl
	w.Header().Set("X-Control-Url", controlUrl)

	eventsCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-events", "application/json")
	eventsCh.CreateChannel()
	eventsUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, eventsUrl)
	reqBodyForRunner["events_url"] = eventsUrl
	w.Header().Set("X-Events-Url", eventsUrl)

	//Optional channels
	if jobParams.EnableVideoIngress {
		pubCh := trickle.NewLocalPublisher(h.trickleSrv, mid, "video/MP2T")
		pubCh.CreateChannel()
		pubUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, pubUrl)
		reqBodyForRunner["subscribe_url"] = pubUrl //runner needs to subscribe to input
		w.Header().Set("X-Publish-Url", pubUrl)    //gateway will connect to pubUrl to send ingress video
	}

	if jobParams.EnableVideoEgress {
		subCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-out", "video/MP2T")
		subCh.CreateChannel()
		subUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, subUrl)
		reqBodyForRunner["publish_url"] = subUrl  //runner needs to send results -out
		w.Header().Set("X-Subscribe-Url", subUrl) //gateway will connect to subUrl to receive results
	}

	if jobParams.EnableDataOutput {
		dataCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-data", "application/jsonl")
		dataCh.CreateChannel()
		dataUrl = overwriteHost(h.node.LiveAITrickleHostForRunner, dataUrl)
		reqBodyForRunner["data_url"] = dataUrl
		w.Header().Set("X-Data-Url", dataUrl)
	}

	reqBodyForRunner["request"] = base64.StdEncoding.EncodeToString(body)
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
	err = h.node.ExternalCapabilities.AddStream(orchJob.Req.ID, orchJob.Req.Capability, orchJob.Req, respBody)
	if err != nil {
		clog.Errorf(ctx, "Error adding stream to external capabilities: %v", err)
		respondWithError(w, "Error adding stream to external capabilities", http.StatusInternalServerError)
		return
	}

	//start payment monitoring
	go func() {
		stream, _ := h.node.ExternalCapabilities.Streams[orchJob.Req.ID]
		ctx := context.Background()
		ctx = clog.AddVal(ctx, "stream_id", orchJob.Req.ID)

		pmtCheckDur := 30 * time.Second
		pmtTicker := time.NewTicker(pmtCheckDur)
		defer pmtTicker.Stop()
		for {
			select {
			case <-stream.StreamCtx.Done():
				h.orchestrator.FreeExternalCapabilityCapacity(orchJob.Req.Capability)
				return
			case <-pmtTicker.C:
				// Check payment status

				jobPriceRat := big.NewRat(orchJob.JobPrice.PricePerUnit, orchJob.JobPrice.PixelsPerUnit)
				if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
					h.orchestrator.DebitFees(orchJob.Sender, core.ManifestID(orchJob.Req.Capability), orchJob.JobPrice, int64(pmtCheckDur.Seconds()))
					senderBalance := getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability)
					if senderBalance != nil {
						if senderBalance.Cmp(big.NewRat(0, 1)) < 0 {
							clog.Infof(ctx, "Insufficient balance, stopping stream %s for sender %s", orchJob.Req.ID, orchJob.Sender)
							_, exists := h.node.ExternalCapabilities.Streams[orchJob.Req.ID]
							if exists {
								h.node.ExternalCapabilities.RemoveStream(orchJob.Req.ID)
							}
						}

						clog.V(8).Infof(ctx, "Payment balance for stream capability is good balance=%v", senderBalance.FloatString(0))
					}
				}

				//check if stream still exists
				// if not, send stop to worker and exit monitoring
				_, exists := h.node.ExternalCapabilities.Streams[orchJob.Req.ID]
				if !exists {
					req, err := http.NewRequestWithContext(ctx, "POST", orchJob.Req.CapabilityUrl+"/stream/stop", nil)
					// set the headers
					_, err = sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
					if err != nil {
						clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
						respondWithError(w, "Error sending request to worker", http.StatusInternalServerError)
						return
					}
					//end monitoring of stream
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
		clog.Errorf(ctx, "failed to create /stop/stream request to worker err=%v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
		clog.Errorf(ctx, "error processing stream stop request statusCode=%d", resp.StatusCode)
	}

	// Stop the stream and free capacity
	h.node.ExternalCapabilities.RemoveStream(jobDetails.StreamId)

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
	clog.Infof(ctx, "Processing payment request")

	jobPrice, err := orch.JobPriceInfo(senderAddr, orchJob.Req.Capability)
	if err != nil {
		statusCode := http.StatusBadRequest
		if err.Error() == "insufficient sender reserve" {
			statusCode = http.StatusServiceUnavailable
		}
		glog.Errorf("could not get price err=%v", err.Error())
		http.Error(w, fmt.Sprintf("Could not get price err=%v", err.Error()), statusCode)
		return
	}
	ticketParams, err := orch.TicketParams(senderAddr, jobPrice)
	if err != nil {
		glog.Errorf("could not get ticket params err=%v", err.Error())
		http.Error(w, fmt.Sprintf("Could not get ticket params err=%v", err.Error()), http.StatusBadRequest)
		return
	}

	capBal := orch.Balance(senderAddr, core.ManifestID(orchJob.Req.Capability))
	if capBal != nil {
		capBal, err = common.PriceToInt64(capBal)
		if err != nil {
			clog.Errorf(context.TODO(), "could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), orchJob.Req.Capability, err.Error())
			capBal = big.NewRat(0, 1)
		}
	} else {
		capBal = big.NewRat(0, 1)
	}
	//convert to int64. Note: returns with 000 more digits to allow for precision of 3 decimal places.
	capBalInt, err := common.PriceToFixed(capBal)
	if err != nil {
		glog.Errorf("could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), orchJob.Req.Capability, err.Error())
		capBalInt = 0
	} else {
		// Remove the last three digits from capBalInt
		capBalInt = capBalInt / 1000
	}

	jobToken := JobToken{TicketParams: ticketParams, Balance: capBalInt, Price: jobPrice}
	clog.Infof(ctx, "Processed payment request stream_id=%s capability=%s balance=%v pricePerUnit=%v pixelsPerUnit=%v", orchJob.Details.StreamId, orchJob.Req.Capability, jobToken.Balance, jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)

	json.NewEncoder(w).Encode(jobToken)
}
