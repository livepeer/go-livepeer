package byoc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-tools/drivers"
)

func (bsg *BYOCGatewayServer) StartStream() http.Handler {
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
		gatewayJob, err := bsg.setupGatewayJob(ctx, r.Header.Get(jobRequestHdr), "", "", false)
		if err != nil {
			clog.Errorf(ctx, "Error setting up job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		//setup body size limit, will error if too large
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		streamUrls, code, err := bsg.setupStream(ctx, r, gatewayJob)
		if err != nil {
			clog.Errorf(ctx, "Error setting up stream: %s", err)
			http.Error(w, err.Error(), code)
			return
		}

		go bsg.runStream(gatewayJob)

		go bsg.monitorStream(gatewayJob.Job.Req.ID)

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

func (bsg *BYOCGatewayServer) StopStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create fresh context instead of using r.Context() since ctx will outlive the request
		ctx := r.Context()
		streamId := r.PathValue("streamId")
		glog.Infof("Received stop request for stream %s", streamId)
		stream, err := bsg.streamPipeline(streamId)
		if err != nil {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		glog.Infof("Pipeline found, stopping %s", streamId)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			clog.Errorf(ctx, "Error reading request body: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		//stop on orchestrator first
		orch := stream.streamParams.liveParams.orchToken
		respBody, orchErr := bsg.sendStopStreamToOrch(ctx, streamId, stream.streamParams.liveParams.pipeline, body, orch)
		if orchErr != nil {
			clog.Errorf(ctx, "Error sending stop request to orchestrator: %s", orchErr)
		}

		//stop and remove on gateway
		bsg.stopStreamPipeline(streamId, nil)
		bsg.removeStreamPipeline(streamId)

		w.WriteHeader(http.StatusOK)
		w.Write(respBody)
	})
}

func (bsg *BYOCGatewayServer) sendStopStreamToOrch(ctx context.Context, streamID string, capability string, body []byte, orch JobToken) ([]byte, error) {
	glog.Infof("Sending stop request to Orchestrator %s", orch.ServiceAddr)
	jobReqDet := JobRequestDetails{StreamId: streamID}
	jobReqDetStr, err := json.Marshal(jobReqDet)
	if err != nil {
		return nil, fmt.Errorf("error marshalling job request details: %w", err)
	}
	stopJob := &gatewayJob{
		Job: &orchJob{
			Req: &JobRequest{
				Capability: capability,
				Request:    string(jobReqDetStr),
				Timeout:    10,
			},
		},
		node: bsg.node,
	}
	stopJob.sign()

	jobSender, err := getJobSender(ctx, bsg.node)
	if err != nil {
		return nil, fmt.Errorf("error getting job sender: %w", err)
	}

	newToken, err := getToken(ctx, getNewTokenTimeout, orch.ServiceAddr, stopJob.Job.Req.Capability, jobSender.Addr, jobSender.Sig)
	if err != nil {
		return nil, fmt.Errorf("error converting session to token: %w", err)
	}

	// Send the stop request
	resp, code, err := bsg.sendJobToOrch(ctx, nil, stopJob.Job.Req, stopJob.SignedJobReq, *newToken, "/ai/stream/stop", body)
	if err != nil {
		return nil, fmt.Errorf("error sending stop job to orchestrator: %w", err)
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading stop response body: %w", err)
	}
	resp.Body.Close()

	if code != http.StatusOK {
		return nil, fmt.Errorf("orchestrator returned status %d", code)
	}

	clog.Infof(ctx, "Sent stop request to Orchestrator %s for stream %s", orch.ServiceAddr, streamID)
	return body, nil
}

func (bsg *BYOCGatewayServer) runStream(gatewayJob *gatewayJob) {
	streamID := gatewayJob.Job.Req.ID
	stream, err := bsg.streamPipeline(streamID)
	if err != nil {
		glog.Errorf("Stream %s not found", streamID)
		return
	}
	// Ensure cleanup happens on ALL exit paths
	var exitErr error
	defer func() {
		// Best-effort cleanup
		bsg.stopStreamPipeline(streamID, exitErr)
	}()
	//this context passes to all channels that will close when stream is canceled
	ctx := bsg.streamPipelineContext(streamID)
	ctx = clog.AddVal(ctx, "stream_id", streamID)

	params, err := bsg.streamPipelineParams(streamID)
	if err != nil {
		clog.Errorf(ctx, "Error getting stream request params: %s", err)
		exitErr = err
		return
	}

	firstProcessed := false
	lastSwap := time.Now()
	swapCnt := 0
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

		ctx = clog.AddVal(ctx, "orch", hexutil.Encode(orch.TicketParams.Recipient))
		ctx = clog.AddVal(ctx, "orch_url", orch.ServiceAddr)

		err = gatewayJob.sign()
		if err != nil {
			clog.Errorf(ctx, "Error signing job, exiting stream processing request: %v", err)
			exitErr = err
			return
		}
		// add Orchestrator specific info to liveParams and save with stream
		perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
		params.liveParams.mu.Lock()
		params.liveParams.kickOrch = perOrchCancel
		params.liveParams.orchToken = orch
		params.liveParams.mu.Unlock()
		// Create new params instance for this orchestrator to avoid race conditions
		bsg.updateStreamPipelineParams(streamID, params) //update params used to kickOrch (perOrchCancel) and urls
		streamReq := bsg.streamPipelineRequest(streamID)

		orchResp, _, err := bsg.sendJobToOrch(ctx, nil, gatewayJob.Job.Req, gatewayJob.SignedJobReq, orch, "/ai/stream/start", streamReq)
		if err != nil {
			clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orch.ServiceAddr, err.Error())
			continue
		}
		defer orchResp.Body.Close()
		io.Copy(io.Discard, orchResp.Body)

		bsg.statusStore.StoreKey(streamID, "orchestrator", orch.ServiceAddr)
		orchUrls := orchTrickleUrls{
			orchPublishUrl:   orchResp.Header.Get("X-Publish-Url"),
			orchSubscribeUrl: orchResp.Header.Get("X-Subscribe-Url"),
			orchControlUrl:   orchResp.Header.Get("X-Control-Url"),
			orchEventsUrl:    orchResp.Header.Get("X-Events-Url"),
			orchDataUrl:      orchResp.Header.Get("X-Data-Url"),
		}

		if err = bsg.startStreamProcessing(perOrchCtx, stream, params, orchUrls); err != nil {
			clog.Errorf(ctx, "Error starting processing: %s", err)
			perOrchCancel(err)
			break
		}
		//something caused the Orch to stop performing, try to get the error and move to next Orchestrator
		<-perOrchCtx.Done()
		err = context.Cause(perOrchCtx)

		_, err = bsg.sendStopStreamToOrch(ctx, streamID, params.liveParams.pipeline, []byte(err.Error()), params.liveParams.orchToken)
		if err != nil {
			clog.Errorf(ctx, "Error sending stop to orchestrator: %s", err)
		}
		params.liveParams.orchToken = JobToken{} //clear out orch token because stream is stopped
		bsg.updateStreamPipelineParams(streamID, params)

		if errors.Is(err, context.Canceled) {
			// this happens if parent ctx was cancelled without a CancelCause
			// or if passing `nil` as a CancelCause
			err = nil
		}
		if _, err := bsg.streamPipeline(streamID); err != nil {
			clog.Info(ctx, "No stream exists, skipping orchestrator swap")
			break
		}

		//if swapping too fast, stop trying since likely a bad request
		if firstProcessed && swapCnt > 2 && time.Since(lastSwap) < (1*time.Minute) {
			clog.Info(ctx, "Orchestrator swaps happening too quickly, stopping stream")
			break
		}
		if firstProcessed {
			swapCnt++
		} else {
			firstProcessed = true
		}

		// will swap, but first notify with the reason for the swap
		if err == nil {
			err = errors.New("unknown swap reason")
		}

		clog.Infof(ctx, "Retrying stream with a different orchestrator err=%v", err.Error())

		params.liveParams.sendErrorEvent(err)
	}

	//if there is ingress input then force off
	if params.liveParams.kickInput != nil {
		exitErr = errors.New("no orchestrators available, ending stream")
		clog.Infof(ctx, "Kicking input stream err=%v", exitErr)
		params.liveParams.kickInput(exitErr)
	} else {
		clog.Infof(ctx, "No input stream to kick")
	}

	//all orchestrators tried or stream ended, stop the stream
	// stream stop called in defer above
	exitErr = errors.New("All Orchestrators exhausted, restart the stream")
}

func (bsg *BYOCGatewayServer) monitorStream(streamId string) {
	ctx := context.Background()
	ctx = clog.AddVal(ctx, "stream_id", streamId)

	stream, err := bsg.streamPipeline(streamId)
	if err != nil {
		clog.Errorf(ctx, "Stream %s not found: %v", streamId, err)
		return
	}

	ctx = clog.AddVal(ctx, "request_id", stream.streamParams.liveParams.requestID)

	// Create a ticker that runs every minute for payments with buffer to ensure payment is completed
	dur := 50 * time.Second
	pmtTicker := time.NewTicker(dur)
	defer pmtTicker.Stop()
	//setup sender
	jobSender, err := getJobSender(ctx, bsg.node)
	if err != nil {
		clog.Errorf(ctx, "Error getting job sender: %v", err)
		return
	}

	//ensure live pipeline is cleaned up if monitoring ends
	defer bsg.removeStreamPipeline(streamId)
	//start monitoring loop
	streamCtx := bsg.streamPipelineContext(streamId)
	for {
		select {
		case <-streamCtx.Done():
			clog.Infof(ctx, "Stream %s stopped, ending monitoring", streamId)
			//send stop to orchestrator because stream is done
			if stream.streamParams.liveParams.orchToken.ServiceAddr != "" {
				bsg.sendStopStreamToOrch(context.Background(), streamId, stream.streamParams.liveParams.pipeline, []byte("stream ended"), stream.streamParams.liveParams.orchToken)
			}
			return
		case <-pmtTicker.C:
			if _, err := bsg.streamPipeline(streamId); err != nil {
				clog.Infof(ctx, "Input stream does not exist for stream %s, ending monitoring", streamId)
				return
			}

			err := bsg.sendPaymentForStream(ctx, streamId, jobSender)
			if err != nil {
				clog.Errorf(ctx, "Error sending payment for stream %s: %v", streamId, err)
			}
		}
	}
}

func (bsg *BYOCGatewayServer) sendPaymentForStream(ctx context.Context, streamID string, jobSender *JobSender) error {
	stream, err := bsg.streamPipeline(streamID)
	if err != nil {
		clog.Errorf(ctx, "Error getting stream pipeline for stream %s: %v", streamID, err)
		return err
	}
	stream.streamParams.liveParams.mu.Lock()
	orch := stream.streamParams.liveParams.orchToken
	stream.streamParams.liveParams.mu.Unlock()

	// fetch new JobToken with each payment
	// update the session for the LivePipeline with new token
	newToken, err := getToken(ctx, getNewTokenTimeout, orch.ServiceAddr, stream.Pipeline, jobSender.Addr, jobSender.Sig)
	if err != nil {
		clog.Errorf(ctx, "Error getting new token for %s: %v", orch.ServiceAddr, err)
		return err
	}
	stream.streamParams.liveParams.mu.Lock()
	stream.streamParams.liveParams.orchToken = *newToken
	stream.streamParams.liveParams.mu.Unlock()
	bsg.updateStreamPipelineParams(streamID, stream.streamParams)

	// send the payment
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
	job := gatewayJob{Job: &orchJob{Req: req}, node: bsg.node}
	err = job.sign()
	if err != nil {
		clog.Errorf(ctx, "Error signing job, continuing monitoring: %v", err)
		return err
	}

	if orch.Price.PricePerUnit > 0 {
		pmtHdr, err := bsg.createPayment(ctx, req, newToken)
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
		statusCode, err := bsg.sendPayment(ctx, orch.ServiceAddr+"/ai/stream/payment", stream.Pipeline, job.SignedJobReq, pmtHdr)
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

func (bsg *BYOCGatewayServer) setupStream(ctx context.Context, r *http.Request, job *gatewayJob) (*StreamUrls, int, error) {
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

	updateURL := fmt.Sprintf("https://%s/process/stream/%s/%s", bsg.node.GatewayHost, streamID, "update")
	statusURL := fmt.Sprintf("https://%s/process/stream/%s/%s", bsg.node.GatewayHost, streamID, "status")
	stopURL := fmt.Sprintf("https://%s/process/stream/%s/%s", bsg.node.GatewayHost, streamID, "stop")

	if job.Job.Params.EnableVideoIngress {
		whipURL = fmt.Sprintf("https://%s/process/stream/%s/%s", bsg.node.GatewayHost, streamID, "whip")
		rtmpURL = mediaMTXInputURL
	}
	if job.Job.Params.EnableVideoEgress {
		whepURL = generateWhepUrl(streamID, requestID)
	}
	if job.Job.Params.EnableDataOutput {
		dataURL = fmt.Sprintf("https://%s/process/stream/%s/%s", bsg.node.GatewayHost, streamID, "data")
	}

	//if set this will overwrite settings above
	if LiveAIAuthWebhookURL != nil {
		authResp, err := authenticateAIStream(LiveAIAuthWebhookURL, bsg.node.LiveAIAuthApiKey, AIAuthRequest{
			Stream:      streamName,
			Type:        "", //sourceTypeStr
			QueryParams: rawParams,
			GatewayHost: bsg.node.GatewayHost,
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
	bsg.statusStore.Clear(streamID)
	bsg.statusStore.StoreKey(streamID, "whep_url", whepURL)

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

	sendErrorEvent := bsg.LiveErrorEventSender(ctx, streamID, map[string]string{
		"type":        "error",
		"request_id":  requestID,
		"stream_id":   streamID,
		"pipeline_id": pipelineID,
		"pipeline":    pipeline,
	})

	inputStreamExists := func(streamId string) bool {
		return bsg.streamPipelineExists(streamId)
	}

	//params set with ingest types:
	// RTMP
	//   kickInput will kick the input from MediaMTX to force a reconnect
	//   localRTMPPrefix mediaMTXInputURL matches to get the ingest from MediaMTX
	// WHIP
	//   kickInput will close the whip connection
	//   localRTMPPrefix set by ENV variable LIVE_AI_PLAYBACK_HOST
	ssr := media.NewSwitchableSegmentReader() //this converts ingest to segments to send to Orchestrator
	params := byocAIRequestParams{
		node:              bsg.node,
		os:                drivers.NodeStorage.NewSession(requestID),
		inputStreamExists: inputStreamExists,
		liveParams: &byocLiveRequestParams{
			segmentReader:          ssr,
			startTime:              time.Now(),
			rtmpOutputs:            rtmpOutputs,
			stream:                 streamID, //live video to video uses stream name, byoc combines to one id
			paymentProcessInterval: bsg.node.LivePaymentInterval,
			outSegmentTimeout:      bsg.node.LiveOutSegmentTimeout,
			requestID:              requestID,
			streamID:               streamID,
			pipelineID:             pipelineID,
			pipeline:               pipeline,
			sendErrorEvent:         sendErrorEvent,

			manifestID: pipeline, //byoc uses one balance per capability name
		},
	}

	//create a dataWriter for data channel if enabled
	if job.Job.Params.EnableDataOutput {
		params.liveParams.dataWriter = media.NewSegmentWriter(5)
	}

	//check if stream exists
	if params.inputStreamExists(streamID) {
		return nil, http.StatusBadRequest, fmt.Errorf("stream already exists: %s", streamID)
	}

	clog.Infof(ctx, "stream setup videoIngress=%v videoEgress=%v dataOutput=%v", job.Job.Params.EnableVideoIngress, job.Job.Params.EnableVideoEgress, job.Job.Params.EnableDataOutput)

	//save the stream setup
	paramsReq := map[string]interface{}{
		"params": pipelineParams,
	}
	paramsReqBytes, _ := json.Marshal(paramsReq)
	bsg.newStreamPipeline(requestID, streamID, pipeline, params, paramsReqBytes) //track the pipeline for cancellation

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
		StopUrl:       stopURL,
	}

	return &streamUrls, http.StatusOK, nil
}

// mediamtx sends this request to go-livepeer when rtmp stream received
func (bsg *BYOCGatewayServer) StartStreamRTMPIngest() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		streamId := r.PathValue("streamId")
		ctx = clog.AddVal(ctx, "stream_id", streamId)

		stream, err := bsg.streamPipeline(streamId)
		if err != nil {
			respondJsonError(ctx, w, fmt.Errorf("stream not found: %s", streamId), http.StatusNotFound)
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
		mediaMTXClient := media.NewMediaMTXClient(mediaMtxHost, bsg.node.MediaMTXApiPassword, sourceID, sourceType)
		segmenterCtx, cancelSegmenter := context.WithCancel(clog.Clone(context.Background(), ctx))

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		kickInput := func(streamErr error) {
			defer cancelSegmenter()
			if streamErr == nil {
				return
			}
			clog.Errorf(ctx, "Live video pipeline finished with error: %s", streamErr)

			err = mediaMTXClient.KickInputConnection(ctx)
			if err != nil {
				clog.Errorf(ctx, "Failed to kick input connection: %s", err)
			}
			stream, err := bsg.streamPipeline(streamId)
			if err != nil {
				clog.Errorf(ctx, "Error getting stream pipeline for stream %s: %s", streamId, err)
				return
			}
			stream.streamParams.liveParams.sendErrorEvent(streamErr)
		}

		stream.streamParams.liveParams.localRTMPPrefix = mediaMTXInputURL
		stream.streamParams.liveParams.kickInput = kickInput
		bsg.updateStreamPipelineParams(streamId, stream.streamParams) //add kickInput to stream params

		// Kick off the RTMP pull and segmentation
		clog.Infof(ctx, "Starting RTMP ingest from MediaMTX")
		go func() {
			ms := media.MediaSegmenter{Workdir: bsg.node.WorkDir, MediaMTXClient: mediaMTXClient}
			//segmenter blocks until done
			ms.RunSegmentation(segmenterCtx, stream.streamParams.liveParams.localRTMPPrefix, stream.streamParams.liveParams.segmentReader.Read)

			stream.streamParams.liveParams.sendErrorEvent(errors.New("mediamtx ingest disconnected"))
			monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
				"type":        "gateway_ingest_stream_closed",
				"timestamp":   time.Now().UnixMilli(),
				"stream_id":   stream.streamParams.liveParams.streamID,
				"pipeline_id": stream.streamParams.liveParams.pipelineID,
				"request_id":  stream.streamParams.liveParams.requestID,
				"orchestrator_info": map[string]interface{}{
					"address": "",
					"url":     "",
				},
			})
			stream.streamParams.liveParams.segmentReader.Close()

			bsg.stopStreamPipeline(streamId, nil)
		}()

		//write response
		w.WriteHeader(http.StatusOK)
	})
}

func (bsg *BYOCGatewayServer) StartStreamWhipIngest(whipServer *media.WHIPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		streamId := r.PathValue("streamId")
		ctx = clog.AddVal(ctx, "stream_id", streamId)

		stream, err := bsg.streamPipeline(streamId)
		if err != nil {
			respondJsonError(ctx, w, fmt.Errorf("stream not found: %s", streamId), http.StatusNotFound)
			return
		}

		whipConn := media.NewWHIPConnection()
		whepURL := generateWhepUrl(streamId, stream.streamParams.liveParams.requestID)

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		kickInput := func(err error) {
			if err == nil {
				return
			}
			clog.Errorf(ctx, "Live video pipeline finished with error: %s", err)
			stream.streamParams.liveParams.sendErrorEvent(err)
			whipConn.Close()
		}
		stream.streamParams.liveParams.kickInput = kickInput
		bsg.updateStreamPipelineParams(streamId, stream.streamParams) //add kickInput to stream params

		//wait for the WHIP connection to close and then cleanup
		go func() {
			statsContext, statsCancel := context.WithCancel(ctx)
			defer statsCancel()
			go bsg.runStats(statsContext, whipConn, streamId, stream.Pipeline, stream.streamParams.liveParams.requestID)

			whipConn.AwaitClose()
			stream, err := bsg.streamPipeline(streamId)
			if err != nil {
				clog.Errorf(ctx, "Error getting stream pipeline for stream %s: %s", streamId, err)
				return
			}
			stream.streamParams.liveParams.segmentReader.Close()
			bsg.stopStreamPipeline(streamId, nil)
			clog.Info(ctx, "Live cleaned up")
		}()

		if whipServer == nil {
			respondJsonError(ctx, w, fmt.Errorf("whip server not configured"), http.StatusInternalServerError)
			whipConn.Close()
			return
		}

		conn := whipServer.CreateWHIP(ctx, stream.streamParams.liveParams.segmentReader, whepURL, w, r)
		whipConn.SetWHIPConnection(conn) // might be nil if theres an error and thats okay
	})
}

func (bsg *BYOCGatewayServer) startStreamProcessing(ctx context.Context, stream *BYOCStreamPipeline, params byocAIRequestParams, orchUrls orchTrickleUrls) error {
	// Lock once and copy all needed fields
	params.liveParams.mu.Lock()
	orch := params.liveParams.orchToken
	params.liveParams.mu.Unlock()

	//Optional channels
	if orchUrls.orchPublishUrl != "" {
		clog.Infof(ctx, "Starting video ingress publisher")
		pub, err := common.AppendHostname(orchUrls.orchPublishUrl, orch.URL())
		if err != nil {
			return fmt.Errorf("invalid publish URL: %w", err)
		}
		bsg.startTricklePublish(ctx, pub, params, orch.Address(), orch.URL())
	}

	if orchUrls.orchSubscribeUrl != "" {
		clog.Infof(ctx, "Starting video egress subscriber")
		sub, err := common.AppendHostname(orchUrls.orchSubscribeUrl, orch.URL())
		if err != nil {
			return fmt.Errorf("invalid subscribe URL: %w", err)
		}
		bsg.startTrickleSubscribe(ctx, sub, params, orch.Address(), orch.URL())
	}

	if orchUrls.orchDataUrl != "" {
		clog.Infof(ctx, "Starting data channel subscriber")
		data, err := common.AppendHostname(orchUrls.orchDataUrl, orch.URL())
		if err != nil {
			return fmt.Errorf("invalid data URL: %w", err)
		}

		bsg.startDataSubscribe(ctx, data, params, orch.Address(), orch.URL())
	}

	//required channels
	control, err := common.AppendHostname(orchUrls.orchControlUrl, orch.URL())
	if err != nil {
		return fmt.Errorf("invalid control URL: %w", err)
	}
	events, err := common.AppendHostname(orchUrls.orchEventsUrl, orch.URL())
	if err != nil {
		return fmt.Errorf("invalid events URL: %w", err)
	}

	bsg.startControlPublish(ctx, control, params, orch.Address(), orch.URL())
	bsg.startEventsSubscribe(ctx, events, params, orch.Address(), orch.URL())

	return nil
}

func (bsg *BYOCGatewayServer) StreamData() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamId := r.PathValue("streamId")
		if streamId == "" {
			http.Error(w, "stream name is required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		ctx = clog.AddVal(ctx, "stream", streamId)

		// Get the live pipeline for this stream
		stream, err := bsg.streamPipeline(streamId)
		if err != nil {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		// Get the data reading buffer
		if stream.streamParams.liveParams.dataWriter == nil {
			http.Error(w, "Stream data not available", http.StatusServiceUnavailable)
			return
		}
		dataReader := stream.streamParams.liveParams.dataWriter.MakeReader(media.SegmentReaderConfig{})

		// Set up SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

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

func (bsg *BYOCGatewayServer) UpdateStream() http.Handler {
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
		stream, err := bsg.streamPipeline(streamId)
		if err != nil {
			// Stream not found
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}

		params, err := bsg.streamPipelineParams(streamId)
		if err != nil {
			clog.Errorf(ctx, "Error getting stream request params: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		updateJob, err := bsg.setupGatewayJob(ctx, r.Header.Get(jobRequestHdr), "", "", true)
		if err != nil {
			clog.Errorf(ctx, "Error setting up update job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updateJob.sign()
		//setup sender
		jobSender, err := getJobSender(ctx, bsg.node)
		if err != nil {
			clog.Errorf(ctx, "Error getting job sender: %v", err)
			return
		}

		params.liveParams.mu.Lock()
		orch := params.liveParams.orchToken
		params.liveParams.mu.Unlock()

		newToken, err := getToken(ctx, getNewTokenTimeout, orch.ServiceAddr, updateJob.Job.Req.Capability, jobSender.Addr, jobSender.Sig)
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

		resp, code, err := bsg.sendJobToOrch(ctx, r, updateJob.Job.Req, updateJob.SignedJobReq, *newToken, "/ai/stream/update", data)
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

func (bsg *BYOCGatewayServer) StreamStatus() http.Handler {
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
		status, exists := bsg.statusStore.Get(streamId)
		if !exists {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			clog.Errorf(ctx, "Failed to encode stream status err=%v", err)
			http.Error(w, "Failed to encode status", http.StatusInternalServerError)
			return
		}
	})
}

func (bsg *BYOCGatewayServer) monitorCurrentLiveSessions() {
	countByPipeline := make(map[string]int)
	var streams []string
	for k, v := range bsg.StreamPipelines {
		countByPipeline[v.Pipeline] = countByPipeline[v.Pipeline] + 1
		streams = append(streams, k)
	}
	monitor.AICurrentLiveSessions(countByPipeline)
	clog.V(common.DEBUG).Infof(context.Background(), "Streams currently live (total=%d): %v", len(bsg.StreamPipelines), streams)
}

func (bsg *BYOCGatewayServer) runStats(ctx context.Context, whipConn *media.WHIPConnection, streamID string, pipelineID string, requestID string) {
	// Periodically check whip stats and write logs and metrics
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := whipConn.Stats()
			if err != nil {
				clog.Errorf(ctx, "WHIP stats returned error: %s", err)
				continue
			}
			if monitor.Enabled {
				monitor.AIWhipTransportBytesReceived(int64(stats.PeerConnStats.BytesReceived))
				monitor.AIWhipTransportBytesSent(int64(stats.PeerConnStats.BytesSent))
			}
			clog.Info(ctx, "whip TransportStats", "ID", stats.PeerConnStats.ID, "bytes_received", stats.PeerConnStats.BytesReceived, "bytes_sent", stats.PeerConnStats.BytesSent)
			for _, s := range stats.TrackStats {
				clog.Info(ctx, "whip InboundRTPStreamStats", "kind", s.Type, "jitter", fmt.Sprintf("%.3f", s.Jitter), "packets_lost", s.PacketsLost, "packets_received", s.PacketsReceived, "rtt", s.RTT)
			}
			bsg.statusStore.StoreKey(streamID, "ingest_metrics", map[string]interface{}{
				"stats": stats,
			})

			monitor.SendQueueEventAsync("stream_ingest_metrics", map[string]interface{}{
				"timestamp":   time.Now().UnixMilli(),
				"stream_id":   streamID,
				"pipeline_id": pipelineID,
				"request_id":  requestID,
				"stats":       stats,
			})
		}
	}
}

func stopProcessing(ctx context.Context, params byocAIRequestParams, err error) {
	clog.InfofErr(ctx, "Stopping processing", err)
	params.liveParams.kickOrch(err)
}
