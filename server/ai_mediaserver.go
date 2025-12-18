package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/byoc"
	"github.com/livepeer/go-livepeer/monitor"

	"github.com/cenkalti/backoff"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-tools/drivers"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/oapi-codegen/runtime"
)

type ImageToVideoResponseAsync struct {
	RequestID string `json:"request_id"`
}

type ImageToVideoResultRequest struct {
	RequestID string `json:"request_id"`
}

type ImageToVideoResultResponse struct {
	Result *ImageToVideoResult `json:"result,omitempty"`
	Status ImageToVideoStatus  `json:"status"`
}

type ImageToVideoResult struct {
	*worker.ImageResponse
	Error *APIError `json:"error,omitempty"`
}

type ImageToVideoStatus string

const (
	Processing ImageToVideoStatus = "processing"
	Complete   ImageToVideoStatus = "complete"
)

// @title Live Video-To-Video AI
// @version 0.0.0

func startAIMediaServer(ctx context.Context, ls *LivepeerServer) error {
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

	ls.HTTPMux.Handle("/text-to-image", oapiReqValidator(aiMediaServerHandle(ls, jsonDecoder[worker.GenTextToImageJSONRequestBody], processTextToImage)))
	ls.HTTPMux.Handle("/image-to-image", oapiReqValidator(aiMediaServerHandle(ls, multipartDecoder[worker.GenImageToImageMultipartRequestBody], processImageToImage)))
	ls.HTTPMux.Handle("/upscale", oapiReqValidator(aiMediaServerHandle(ls, multipartDecoder[worker.GenUpscaleMultipartRequestBody], processUpscale)))
	ls.HTTPMux.Handle("/image-to-video", oapiReqValidator(ls.ImageToVideo()))
	ls.HTTPMux.Handle("/image-to-video/result", ls.ImageToVideoResult())
	ls.HTTPMux.Handle("/audio-to-text", oapiReqValidator(aiMediaServerHandle(ls, multipartDecoder[worker.GenAudioToTextMultipartRequestBody], processAudioToText)))
	ls.HTTPMux.Handle("/llm", oapiReqValidator(ls.LLM()))
	ls.HTTPMux.Handle("/segment-anything-2", oapiReqValidator(aiMediaServerHandle(ls, multipartDecoder[worker.GenSegmentAnything2MultipartRequestBody], processSegmentAnything2)))
	ls.HTTPMux.Handle("/image-to-text", oapiReqValidator(aiMediaServerHandle(ls, multipartDecoder[worker.GenImageToTextMultipartRequestBody], processImageToText)))
	ls.HTTPMux.Handle("/text-to-speech", oapiReqValidator(aiMediaServerHandle(ls, jsonDecoder[worker.GenTextToSpeechJSONRequestBody], processTextToSpeech)))

	// This is called by the media server when the stream is ready
	ls.HTTPMux.Handle("POST /live/video-to-video/{stream}/start", ls.StartLiveVideo())
	ls.HTTPMux.Handle("POST /live/video-to-video/{prefix}/{stream}/start", ls.StartLiveVideo())
	ls.HTTPMux.Handle("POST /live/video-to-video/{stream}/update", ls.UpdateLiveVideo())
	ls.HTTPMux.Handle("OPTIONS /live/video-to-video/{stream}/update", ls.WithCode(http.StatusNoContent))
	ls.HTTPMux.Handle("/live/video-to-video/smoketest", ls.SmokeTestLiveVideo())

	// Configure WHIP ingest only if an addr is specified.
	// TODO use a proper cli flag
	var whipServer *media.WHIPServer
	if os.Getenv("LIVE_AI_WHIP_ADDR") != "" {
		whipServer = media.NewWHIPServer()
		ls.HTTPMux.Handle("POST /live/video-to-video/{stream}/whip", ls.CreateWhip(whipServer))
		ls.HTTPMux.Handle("HEAD /live/video-to-video/{stream}/whip", ls.WithCode(http.StatusMethodNotAllowed))
		ls.HTTPMux.Handle("OPTIONS /live/video-to-video/{stream}/whip", ls.WithCode(http.StatusNoContent))
	}

	var whepServer *media.WHEPServer
	if os.Getenv("LIVE_AI_WHEP_ADDR") != "" {
		whepServer = media.NewWHEPServer()
		// path is {stream}-{request}-out but golang router won't match that
		ls.HTTPMux.Handle("POST /live/video-to-video/{path}/whep", ls.CreateWhep(whepServer))
		ls.HTTPMux.Handle("HEAD /live/video-to-video/{path}/whep", ls.WithCode(http.StatusMethodNotAllowed))
		ls.HTTPMux.Handle("OPTIONS /live/video-to-video/{path}/whep", ls.WithCode(http.StatusNoContent))
		ls.HTTPMux.Handle("PATCH /live/video-to-video/{path}/whep", ls.WithCode(http.StatusNotImplemented))
	}

	// Stream status
	ls.HTTPMux.Handle("OPTIONS /live/video-to-video/{streamId}/status", ls.WithCode(http.StatusNoContent))
	ls.HTTPMux.Handle("/live/video-to-video/{streamId}/status", ls.GetLiveVideoToVideoStatus())

	//API for dynamic capabilities
	ls.byocSrv = byoc.NewBYOCGatewayServer(ls.LivepeerNode, &StreamStatusStore, whipServer, whepServer, ls.HTTPMux)

	media.StartFileCleanup(ctx, ls.LivepeerNode.WorkDir)

	startHearbeats(ctx, ls.LivepeerNode)
	return nil
}

func generateGatewayLiveURL(hostname, stream, pathSuffix string) string {
	return fmt.Sprintf("https://%s/live/video-to-video/%s/%s", hostname, stream, pathSuffix)
}

func generateWhepUrl(streamName, requestID string) string {
	whepURL := os.Getenv("LIVE_AI_WHEP_URL")
	if whepURL == "" {
		whepURL = "http://localhost:8889/" // default mediamtx output
	}
	whepURL = fmt.Sprintf("%s%s-%s-out/whep", whepURL, streamName, requestID)
	return whepURL
}

func aiMediaServerHandle[I, O any](ls *LivepeerServer, decoderFunc func(*I, *http.Request) error, processorFunc func(context.Context, aiRequestParams, I) (O, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,
		}

		var req I
		if err := decoderFunc(&req, r); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		resp, err := processorFunc(ctx, params, req)
		if err != nil {
			var serviceUnavailableErr *ServiceUnavailableError
			var badRequestErr *BadRequestError
			if errors.As(err, &serviceUnavailableErr) {
				respondJsonError(ctx, w, err, http.StatusServiceUnavailable)
				return
			}
			if errors.As(err, &badRequestErr) {
				respondJsonError(ctx, w, err, http.StatusBadRequest)
				return
			}
			respondJsonError(ctx, w, err, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (ls *LivepeerServer) ImageToVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		var req worker.GenImageToVideoMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		var async bool
		prefer := r.Header.Get("Prefer")
		if prefer == "respond-async" {
			async = true
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToVideo request imageSize=%v model_id=%v async=%v", req.Image.FileSize(), *req.ModelId, async)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,
		}

		if !async {
			start := time.Now()

			resp, err := processImageToVideo(ctx, params, req)
			if err != nil {
				var serviceUnavailableErr *ServiceUnavailableError
				var badRequestErr *BadRequestError
				if errors.As(err, &serviceUnavailableErr) {
					respondJsonError(ctx, w, err, http.StatusServiceUnavailable)
					return
				}
				if errors.As(err, &badRequestErr) {
					respondJsonError(ctx, w, err, http.StatusBadRequest)
					return
				}
				respondJsonError(ctx, w, err, http.StatusInternalServerError)
				return
			}

			took := time.Since(start)
			clog.Infof(ctx, "Processed ImageToVideo request imageSize=%v model_id=%v took=%v", req.Image.FileSize(), *req.ModelId, took)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		var data bytes.Buffer
		if err := json.NewEncoder(&data).Encode(req); err != nil {
			respondJsonError(ctx, w, err, http.StatusInternalServerError)
			return
		}

		path, err := params.os.SaveData(ctx, "request.json", bytes.NewReader(data.Bytes()), nil, 0)
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusInternalServerError)
			return
		}

		clog.Infof(ctx, "Saved ImageToVideo request path=%v", requestID, path)

		cctx := clog.Clone(context.Background(), ctx)
		go func(ctx context.Context) {
			start := time.Now()

			var data bytes.Buffer
			resp, err := processImageToVideo(ctx, params, req)
			if err != nil {
				clog.Errorf(ctx, "Error processing ImageToVideo request err=%v", err)

				handleAPIError(ctx, &data, err, http.StatusInternalServerError)
			} else {
				took := time.Since(start)
				clog.Infof(ctx, "Processed ImageToVideo request imageSize=%v model_id=%v took=%v", req.Image.FileSize(), *req.ModelId, took)

				if err := json.NewEncoder(&data).Encode(resp); err != nil {
					clog.Errorf(ctx, "Error JSON encoding ImageToVideo response err=%v", err)
					return
				}
			}

			path, err := params.os.SaveData(ctx, "result.json", bytes.NewReader(data.Bytes()), nil, 0)
			if err != nil {
				clog.Errorf(ctx, "Error saving ImageToVideo result to object store err=%v", err)
				return
			}

			clog.Infof(ctx, "Saved ImageToVideo result path=%v", path)
		}(cctx)

		resp := &ImageToVideoResponseAsync{
			RequestID: requestID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (ls *LivepeerServer) LLM() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		var req worker.GenLLMJSONRequestBody
		if err := jsonDecoder(&req, r); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		//check required fields
		if req.Model == nil || req.Messages == nil || req.Stream == nil || req.MaxTokens == nil || len(req.Messages) == 0 {
			respondJsonError(ctx, w, errors.New("missing required fields"), http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received LLM request model_id=%v stream=%v", *req.Model, *req.Stream)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,
		}

		start := time.Now()
		resp, err := processLLM(ctx, params, req)
		if err != nil {
			var e *ServiceUnavailableError
			if errors.As(err, &e) {
				respondJsonError(ctx, w, err, http.StatusServiceUnavailable)
				return
			}
			respondJsonError(ctx, w, err, http.StatusInternalServerError)
			return
		}

		took := time.Since(start)
		clog.V(common.VERBOSE).Infof(ctx, "Processed LLM request model_id=%v took=%v", *req.Model, took)

		if streamChan, ok := resp.(chan *worker.LLMResponse); ok {
			// Handle streaming response (SSE)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			for chunk := range streamChan {
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
				if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
					break
				}
			}
		} else if llmResp, ok := resp.(*worker.LLMResponse); ok {
			// Handle non-streaming response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(llmResp)
		} else {
			http.Error(w, "Unexpected response type", http.StatusInternalServerError)
		}
	})
}

func (ls *LivepeerServer) ImageToVideoResult() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req ImageToVideoResultRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		ctx = clog.AddVal(ctx, "request_id", req.RequestID)

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToVideoResult request request_id=%v", req.RequestID)

		sess := drivers.NodeStorage.NewSession(req.RequestID)

		_, err := sess.ReadData(ctx, "request.json")
		if err != nil {
			respondJsonError(ctx, w, errors.New("invalid request ID"), http.StatusBadRequest)
			return
		}

		resp := ImageToVideoResultResponse{
			Status: Processing,
		}

		reader, err := sess.ReadData(ctx, "result.json")
		if err != nil {
			// TODO: Distinguish between error reading data vs. file DNE
			// Right now we assume that this file will exist when processing is done even
			// if an error was encountered
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp.Status = Complete

		if err := json.NewDecoder(reader.Body).Decode(&resp.Result); err != nil {
			respondJsonError(ctx, w, err, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// @Summary Start Live Video
// @Accept multipart/form-data
// @Param stream path string true "Stream Key"
// @Param source_id formData string true "MediaMTX source ID, used for calls back to MediaMTX"
// @Param source_type formData string true "MediaMTX specific source type (webrtcSession/rtmpConn)"
// @Param query formData string true "Queryparams from the original ingest URL"
// @Success 200
// @Router /live/video-to-video/{stream}/start [get]
func (ls *LivepeerServer) StartLiveVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create fresh context instead of using r.Context() since ctx will outlive the request
		ctx := context.Background()

		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		streamName := r.PathValue("stream")
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
		sourceType = strings.ToLower(sourceType) // mediamtx changed casing between versions
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
		orchestrator := qp.Get("orchestrator")
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
		if ls.liveAIAuthWebhookURL != nil {
			authResp, err := authenticateAIStream(ls.liveAIAuthWebhookURL, ls.liveAIAuthApiKey, AIAuthRequest{
				Stream:      streamName,
				Type:        sourceTypeStr,
				QueryParams: queryParams,
				GatewayHost: ls.LivepeerNode.GatewayHost,
				WhepURL:     whepURL,
				UpdateURL:   generateGatewayLiveURL(ls.LivepeerNode.GatewayHost, streamName, "/update"),
				StatusURL:   generateGatewayLiveURL(ls.LivepeerNode.GatewayHost, streamID, "/status"),
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
		monitor.AILiveVideoAttempt(pipeline) // this `pipeline` is actually modelID

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
				orchestrator:           orchestrator,
			},
		}

		registerControl(ctx, params)

		// Create a special parent context for orchestrator cancellation
		orchCtx, orchCancel := context.WithCancel(ctx)

		// Kick off the RTMP pull and segmentation as soon as possible
		go func() {
			ms := media.MediaSegmenter{Workdir: ls.LivepeerNode.WorkDir, MediaMTXClient: mediaMTXClient}
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
			<-orchSelection // wait for selection to complete
			cleanupControl(ctx, params)
			ssr.Close()
			orchCancel()
		}()

		req := worker.GenLiveVideoToVideoJSONRequestBody{
			ModelId:          &pipeline,
			Params:           &pipelineParams,
			GatewayRequestId: &requestID,
			StreamId:         &streamID,
		}
		processStream(orchCtx, params, req)
		close(orchSelection)
	})
}

func processStream(ctx context.Context, params aiRequestParams, req worker.GenLiveVideoToVideoJSONRequestBody) {
	orchSwapper := NewOrchestratorSwapper(params)
	isFirst, firstProcessed := true, make(chan interface{})
	go func() {
		var err error
		for {
			perOrchCtx, perOrchCancel := context.WithCancelCause(ctx)
			params.liveParams = newParams(params.liveParams, perOrchCancel)
			var resp interface{}
			resp, err = processAIRequest(perOrchCtx, params, req)
			if err != nil {
				clog.Errorf(ctx, "Error processing AI Request: %s", err)
				perOrchCancel(err)
				break
			}

			if err = startProcessing(perOrchCtx, params, resp); err != nil {
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
			// report the swap
			monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
				"type":        "orchestrator_swap",
				"stream_id":   params.liveParams.streamID,
				"request_id":  params.liveParams.requestID,
				"pipeline":    params.liveParams.pipeline,
				"pipeline_id": params.liveParams.pipelineID,
				"message":     err.Error(),
				"orchestrator_info": map[string]interface{}{
					"address": params.liveParams.sess.Address(),
					"url":     params.liveParams.sess.Transcoder(),
				},
			})
		}
		if isFirst {
			// failed before selecting an orchestrator
			firstProcessed <- struct{}{}
		}
		params.liveParams.kickInput(err)
	}()
	<-firstProcessed
}

func newParams(params *liveRequestParams, cancelOrch context.CancelCauseFunc) *liveRequestParams {
	return &liveRequestParams{
		segmentReader:          params.segmentReader,
		rtmpOutputs:            params.rtmpOutputs,
		localRTMPPrefix:        params.localRTMPPrefix,
		stream:                 params.stream,
		paymentProcessInterval: params.paymentProcessInterval,
		outSegmentTimeout:      params.outSegmentTimeout,
		requestID:              params.requestID,
		streamID:               params.streamID,
		pipelineID:             params.pipelineID,
		pipeline:               params.pipeline,
		sendErrorEvent:         params.sendErrorEvent,
		kickInput:              params.kickInput,
		orchestrator:           params.orchestrator,
		startTime:              time.Now(),
		kickOrch:               cancelOrch,
	}

}

func startProcessing(ctx context.Context, params aiRequestParams, res interface{}) error {
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
	if resp.JSON200.ManifestId != nil {
		ctx = clog.AddVal(ctx, "manifest_id", *resp.JSON200.ManifestId)
		params.liveParams.manifestID = *resp.JSON200.ManifestId
	}
	clog.V(common.VERBOSE).Infof(ctx, "pub %s sub %s control %s events %s", pub, sub, control, events)

	startControlPublish(ctx, control, params)
	startTricklePublish(ctx, pub, params, params.liveParams.sess)
	startTrickleSubscribe(ctx, sub, params, params.liveParams.sess)
	startEventsSubscribe(ctx, events, params, params.liveParams.sess)
	return nil
}

func getRemoteHost(remoteAddr string) (string, error) {
	if remoteAddr == "" {
		return "", errors.New("remoteAddr is empty")
	}

	// Handle IPv6 addresses by splitting on last colon
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return "", fmt.Errorf("couldn't parse remote host: %w", err)
	}

	// Clean up IPv6 brackets if present
	host = strings.Trim(host, "[]")

	if host == "::1" {
		return "127.0.0.1", nil
	}

	return host, nil
}

// @Summary Update Live Stream
// @Param stream path string true "Stream Key"
// @Param params body string true "update request"
// @Success 200
// @Router /live/video-to-video/{stream}/update [post]
func (ls *LivepeerServer) UpdateLiveVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Get stream from path param
		stream := r.PathValue("stream")
		if stream == "" {
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}
		ls.LivepeerNode.LiveMu.RLock()
		// NB: LiveMu is a global lock, avoid holding it
		// during blocking network actions ... can't defer
		p, ok := ls.LivepeerNode.LivePipelines[stream]
		ls.LivepeerNode.LiveMu.RUnlock()
		if !ok {
			// Stream not found
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		reader := http.MaxBytesReader(w, r.Body, 1*1024*104) // 1 MB
		defer reader.Close()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		params := string(data)
		ls.LivepeerNode.LiveMu.Lock()
		p.Params = data
		reportUpdate := p.ReportUpdate
		controlPub := p.ControlPub
		ls.LivepeerNode.LiveMu.Unlock()

		if controlPub == nil {
			clog.Info(ctx, "No orchestrator available, caching params", "stream", stream, "params", params)
			return
		}

		clog.V(6).Infof(ctx, "Sending Live Video Update Control API stream=%s, params=%s", stream, params)
		if err := controlPub.Write(bytes.NewReader(data)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reportUpdate(data)

		corsHeaders(w, r.Method)
	})
}

// @Summary Get Live Stream Status
// @Param stream path string true "Stream ID"
// @Success 200
// @Router /live/video-to-video/{stream}/status [get]
func (ls *LivepeerServer) GetLiveVideoToVideoStatus() http.Handler {
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
			} else {
				// Clone so we can safely update the object (shallow)
				status = maps.Clone(status)
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

func (ls *LivepeerServer) CreateWhip(server *media.WHIPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// two main sequential parts here
		//
		// WHIP : stream setup, await tracks
		// Others: auth, selection, job setup
		// Run them in parallel

		streamRequestTime := time.Now().UnixMilli()
		corsHeaders(w, r.Method)

		// NB: deliberately not using r.Context since ctx will outlive the request
		ctx := context.Background()
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)
		streamName := r.PathValue("stream")
		if streamName == "" {
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "stream", streamName)

		ssr := media.NewSwitchableSegmentReader()

		whipConn := media.NewWHIPConnection()
		whepURL := generateWhepUrl(streamName, requestID)

		go func() {
			internalOutputHost := os.Getenv("LIVE_AI_PLAYBACK_HOST") // TODO proper cli arg
			if internalOutputHost == "" {
				internalOutputHost = "rtmp://localhost/"
			}
			mediamtxOutputURL := internalOutputHost + streamName + "-out"
			mediaMTXOutputAlias := fmt.Sprintf("%s%s-%s-out", internalOutputHost, streamName, requestID)
			outputURL := r.URL.Query().Get("rtmpOutput")
			streamID := r.URL.Query().Get("streamId")
			pipelineID := ""
			pipeline := r.URL.Query().Get("pipeline")
			pipelineParams := make(map[string]interface{})
			sourceTypeStr := "livepeer-whip"
			queryParams := r.URL.Query().Encode()
			orchestrator := r.URL.Query().Get("orchestrator")
			rawParams := r.URL.Query().Get("params")
			if rawParams != "" {
				if err := json.Unmarshal([]byte(rawParams), &pipelineParams); err != nil {
					clog.Errorf(ctx, "Invalid pipeline params: %s", err)
					http.Error(w, "Invalid pipeline params", http.StatusBadRequest)
					return
				}
			}
			// collect RTMP outputs
			var rtmpOutputs []string

			ctx = clog.AddVal(ctx, "source_type", sourceTypeStr)

			if ls.liveAIAuthWebhookURL != nil {
				authResp, err := authenticateAIStream(ls.liveAIAuthWebhookURL, ls.liveAIAuthApiKey, AIAuthRequest{
					Stream:      streamName,
					Type:        sourceTypeStr,
					QueryParams: queryParams,
					GatewayHost: ls.LivepeerNode.GatewayHost,
					WhepURL:     whepURL,
					UpdateURL:   generateGatewayLiveURL(ls.LivepeerNode.GatewayHost, streamName, "/update"),
					StatusURL:   generateGatewayLiveURL(ls.LivepeerNode.GatewayHost, streamID, "/status"),
				})
				if err != nil {
					whipConn.Close()
					clog.Errorf(ctx, "Live AI auth failed: %s", err.Error())
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
					ctx = clog.AddVal(ctx, "stream_id", streamID)
				}

				if authResp.PipelineID != "" {
					pipelineID = authResp.PipelineID
				}
			}

			sendErrorEvent := LiveErrorEventSender(ctx, streamID, map[string]string{
				"type":        "error",
				"request_id":  requestID,
				"stream_id":   streamID,
				"pipeline_id": pipelineID,
				"pipeline":    pipeline,
			})
			kickInput := func(err error) {
				if err == nil {
					return
				}
				clog.Errorf(ctx, "Live video pipeline finished with error: %s", err)
				sendErrorEvent(err)
				whipConn.Close()
			}

			clog.Info(ctx, "Received live video AI request", "pipelineParams", pipelineParams)

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

			clog.V(common.VERBOSE).Infof(ctx, "AI Live video attempt")
			monitor.AILiveVideoAttempt(pipeline) // this `pipeline` is actually modelID

			go func() {
				defer func() {
					if r := recover(); r != nil {
						clog.Errorf(ctx, "Panic in stream close event routine: %s", r)
					}
				}()
				err := whipConn.AwaitClose()
				if err == nil {
					// For now, set a "whip disconnected" event"
					err = errors.New("whip disconnected")
				}
				sendErrorEvent(err)
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
			}()

			if outputURL != "" {
				rtmpOutputs = append(rtmpOutputs, outputURL)
			}
			if mediamtxOutputURL != "" {
				rtmpOutputs = append(rtmpOutputs, mediamtxOutputURL, mediaMTXOutputAlias)
			}
			clog.Info(ctx, "RTMP outputs", "destinations", rtmpOutputs)

			params := aiRequestParams{
				node:        ls.LivepeerNode,
				os:          drivers.NodeStorage.NewSession(requestID),
				sessManager: ls.AISessionManager,

				liveParams: &liveRequestParams{
					segmentReader:          ssr,
					rtmpOutputs:            rtmpOutputs,
					localRTMPPrefix:        internalOutputHost,
					stream:                 streamName,
					paymentProcessInterval: ls.livePaymentInterval,
					outSegmentTimeout:      ls.outSegmentTimeout,
					requestID:              requestID,
					streamID:               streamID,
					pipelineID:             pipelineID,
					pipeline:               pipeline,
					kickInput:              kickInput,
					sendErrorEvent:         sendErrorEvent,
					orchestrator:           orchestrator,
				},
			}

			registerControl(ctx, params)

			req := worker.GenLiveVideoToVideoJSONRequestBody{
				ModelId:          &pipeline,
				Params:           &pipelineParams,
				GatewayRequestId: &requestID,
				StreamId:         &streamID,
			}

			// Create a special parent context for orchestrator cancellation
			orchCtx, orchCancel := context.WithCancel(ctx)

			processStream(orchCtx, params, req)

			statsContext, statsCancel := context.WithCancel(ctx)
			defer statsCancel()
			go runStats(statsContext, whipConn, streamID, pipelineID, requestID)

			whipConn.AwaitClose()
			cleanupControl(ctx, params)
			ssr.Close()
			orchCancel()
			clog.Info(ctx, "Live cleaned up")
		}()

		conn := server.CreateWHIP(ctx, ssr, whepURL, w, r)
		whipConn.SetWHIPConnection(conn) // might be nil if theres an error and thats okay
	})
}

func (ls *LivepeerServer) CreateWhep(server *media.WHEPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		parts := strings.Split(path, "-")
		if len(parts) != 3 || parts[2] != "out" {
			if len(parts) != 2 || parts[1] != "out" {
				http.Error(w, "Malformed stream ID", http.StatusNotFound)
				return
			}
			parts[1] = ""
		}
		stream, requestID := parts[0], parts[1]
		ctx := r.Context()
		ctx = clog.AddVal(ctx, "stream", stream)
		outWriter, rid := getOutWriter(stream, ls.LivepeerNode)
		if outWriter == nil || (requestID != rid && requestID != "") {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		ctx = clog.AddVal(ctx, "request_id", rid)
		corsHeaders(w, r.Method)
		server.CreateWHEP(ctx, w, r, outWriter.MakeReader(), stream)
	})
}

func runStats(ctx context.Context, whipConn *media.WHIPConnection, streamID string, pipelineID string, requestID string) {
	// Periodically check whip stats and write logs and metrics
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			media.ClearOutputStats(requestID + "-video")
			media.ClearOutputStats(requestID + "-audio")
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
				outputStats := media.GetOutputStats(requestID + "-" + s.Type.String())
				s.LastOutputTS = float64(outputStats.GetLastOutputTS()) / 90000.0
				s.Latency = s.LastInputTS - s.LastOutputTS
				clog.Info(ctx, "whip InboundRTPStreamStats",
					"kind", s.Type,
					"jitter", fmt.Sprintf("%.3f", s.Jitter),
					"packets_lost", s.PacketsLost,
					"packets_received", s.PacketsReceived,
					"rtt", s.RTT,
					"last_input_ts", fmt.Sprintf("%.3f", s.LastInputTS),
					"last_output_ts", fmt.Sprintf("%.3f", s.LastOutputTS),
					"latency", fmt.Sprintf("%0.3f", s.Latency),
				)
			}
			GatewayStatus.StoreKey(streamID, "ingest_metrics", map[string]interface{}{
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

func (ls *LivepeerServer) WithCode(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, r.Method)
		w.WriteHeader(code)
	})
}

func cleanupControl(ctx context.Context, params aiRequestParams) {
	clog.Infof(ctx, "Live video pipeline finished")
	stream := params.liveParams.stream
	node := params.node
	node.LiveMu.Lock()
	pub, ok := node.LivePipelines[stream]
	if !ok || pub.RequestID != params.liveParams.requestID {
		// already cleaned up
		node.LiveMu.Unlock()
		return
	}
	delete(node.LivePipelines, stream)
	if monitor.Enabled {
		monitorCurrentLiveSessions(node.LivePipelines)
	}
	node.LiveMu.Unlock()

	if pub.ControlPub != nil {
		if err := pub.ControlPub.Close(); err != nil {
			slog.Info("Error closing trickle publisher", "err", err)
		}
		pub.StopControl()
	}
	pub.Closed = true
}

func monitorCurrentLiveSessions(pipelines map[string]*core.LivePipeline) {
	countByPipeline := make(map[string]int)
	var streams []string
	for k, v := range pipelines {
		countByPipeline[v.Pipeline] = countByPipeline[v.Pipeline] + 1
		streams = append(streams, k)
	}
	monitor.AICurrentLiveSessions(countByPipeline)
	clog.V(common.DEBUG).Infof(context.Background(), "Streams currently live (total=%d): %v", len(pipelines), streams)
}

func corsHeaders(w http.ResponseWriter, reqMethod string) {
	if os.Getenv("LIVE_AI_ALLOW_CORS") == "" { // TODO cli arg
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if reqMethod == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST, GET")
		// Allows us send down a preferred STUN server without ICE restart
		// https://datatracker.ietf.org/doc/html/draft-ietf-wish-whip-16#section-4.6
		w.Header()["Link"] = media.GenICELinkHeaders(media.WebrtcConfig.ICEServers)
	}
}

const defaultSmokeTestDuration = 5 * time.Minute
const maxSmokeTestDuration = 60 * time.Minute

type smokeTestRequest struct {
	StreamURL    string `json:"stream_url"`
	DurationSecs int    `json:"duration_secs"`
}

// @Summary Start Smoke Test
// @Param request body smokeTestRequest true "smoke test request"
// @Success 200
// @Router /live/video-to-video/smoketest [put]
func (ls *LivepeerServer) SmokeTestLiveVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req smokeTestRequest
		defer r.Body.Close()
		d := json.NewDecoder(r.Body)
		err := d.Decode(&req)
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}
		if req.StreamURL == "" {
			http.Error(w, "Missing stream url", http.StatusBadRequest)
			return
		}

		ingestURL := req.StreamURL
		duration := defaultSmokeTestDuration
		if req.DurationSecs != 0 {
			if float64(req.DurationSecs) > maxSmokeTestDuration.Seconds() {
				http.Error(w, "Request exceeds max duration "+maxSmokeTestDuration.String(), http.StatusBadRequest)
				return
			}
			duration = time.Duration(req.DurationSecs) * time.Second
		}
		// Use an FFMPEG test card
		var params = []string{
			"-re",
			"-f", "lavfi",
			"-i", "testsrc=size=1920x1080:rate=30,format=yuv420p",
			"-f", "lavfi",
			"-i", "sine",
			"-c:v", "libx264",
			"-b:v", "1000k",
			"-x264-params", "keyint=60",
			"-c:a", "aac",
			"-to", fmt.Sprintf("%f", duration.Seconds()),
			"-f", "flv",
			ingestURL,
		}

		ctx, cancel := context.WithTimeout(context.Background(), duration+time.Minute)
		cmd := exec.CommandContext(ctx, "ffmpeg", params...)
		var outputBuf bytes.Buffer
		cmd.Stdout = &outputBuf
		cmd.Stderr = &outputBuf

		clog.Infof(ctx, "Starting smoke test for %s duration %s", ingestURL, duration)

		if err := cmd.Start(); err != nil {
			cancel()
			clog.Errorf(ctx, "Smoke test failed to start ffmpeg. Error: %s\nCommand: ffmpeg %s", err, strings.Join(params, " "))
			http.Error(w, "Failed to start stream", http.StatusInternalServerError)
			return
		}

		go func() {
			defer cancel()
			_ = backoff.Retry(func() error {
				if state, err := cmd.Process.Wait(); err != nil || state.ExitCode() != 0 {
					clog.Errorf(ctx, "Smoke test failed to run ffmpeg. Exit Code: %d, Error: %s\nCommand: ffmpeg %s\n", state.ExitCode(), err, strings.Join(params, " "))
					clog.Errorf(ctx, "Smoke test ffmpeg output:\n%s\n", outputBuf.String())
					return fmt.Errorf("ffmpeg failed")
				}
				clog.Infof(ctx, "Smoke test finished successfully for %s", ingestURL)
				return nil
			}, backoff.WithMaxRetries(backoff.NewConstantBackOff(30*time.Second), 3))
		}()
	})
}

func startHearbeats(ctx context.Context, node *core.LivepeerNode) {
	if node.LiveAIHeartbeatURL == "" {
		return
	}

	go func() {
		ticker := time.NewTicker(node.LiveAIHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendHeartbeat(ctx, node, node.LiveAIHeartbeatURL, node.LiveAIHeartbeatHeaders)
			}
		}
	}()
}

func getStreamIDs(node *core.LivepeerNode) []string {
	node.LiveMu.Lock()
	defer node.LiveMu.Unlock()
	var streamIDs []string
	for _, pipeline := range node.LivePipelines {
		streamIDs = append(streamIDs, pipeline.StreamID)
	}
	return streamIDs
}

func sendHeartbeat(ctx context.Context, node *core.LivepeerNode, liveAIHeartbeatURL string, liveAIHeartbeatHeaders map[string]string) {
	streamIDs := getStreamIDs(node)

	reqBody, err := json.Marshal(map[string]interface{}{
		"ids": streamIDs,
	})
	if err != nil {
		clog.Errorf(ctx, "heartbeat: failed to marshal request body %s", err)
		return
	}

	request, err := http.NewRequest("POST", liveAIHeartbeatURL, bytes.NewReader(reqBody))
	if err != nil {
		clog.Errorf(ctx, "heartbeat: failed to build heartbeat request %s", err)
		return
	}

	request.Header.Set("Content-Type", "application/json")
	for key, value := range liveAIHeartbeatHeaders {
		request.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		clog.Errorf(ctx, "heartbeat: failed to send heartbeat %s", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		clog.Errorf(ctx, "heartbeat: failed to send heartbeat %s", resp.Status)
		clog.Errorf(ctx, "heartbeat: response body: %s", string(body))
		return
	}
}
