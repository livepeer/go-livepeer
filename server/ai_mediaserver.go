package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

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
			slog.ErrorContext(context.Background(), "oapi validation error", "statusCode", statusCode, "message", message)
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
	ls.HTTPMux.Handle("/live/video-to-video/{stream}/start", ls.StartLiveVideo())
	ls.HTTPMux.Handle("/live/video-to-video/{prefix}/{stream}/start", ls.StartLiveVideo())
	ls.HTTPMux.Handle("/live/video-to-video/{stream}/update", ls.UpdateLiveVideo())
	ls.HTTPMux.Handle("/live/video-to-video/smoketest", ls.SmokeTestLiveVideo())

	// Stream status
	ls.HTTPMux.Handle("/live/video-to-video/{streamId}/status", ls.GetLiveVideoToVideoStatus())

	media.StartFileCleanup(ctx, ls.LivepeerNode.WorkDir)
	return nil
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

		slog.DebugContext(ctx, "Received ImageToVideo request", "imageSize", req.Image.FileSize(), "model_id", *req.ModelId, "async", async)

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
			slog.InfoContext(ctx, "Processed ImageToVideo request", "imageSize", req.Image.FileSize(), "model_id", *req.ModelId, "took", took)

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

		slog.InfoContext(ctx, "Saved ImageToVideo request", "path", requestID, "path", path)

		cctx := clog.Clone(context.Background(), ctx)
		go func(ctx context.Context) {
			start := time.Now()

			var data bytes.Buffer
			resp, err := processImageToVideo(ctx, params, req)
			if err != nil {
				slog.ErrorContext(ctx, "Error processing ImageToVideo request", "err", err)

				handleAPIError(ctx, &data, err, http.StatusInternalServerError)
			} else {
				took := time.Since(start)
				slog.InfoContext(ctx, "Processed ImageToVideo request", "imageSize", req.Image.FileSize(), "model_id", *req.ModelId, "took", took)

				if err := json.NewEncoder(&data).Encode(resp); err != nil {
					slog.ErrorContext(ctx, "Error JSON encoding ImageToVideo response", "err", err)
					return
				}
			}

			path, err := params.os.SaveData(ctx, "result.json", bytes.NewReader(data.Bytes()), nil, 0)
			if err != nil {
				slog.ErrorContext(ctx, "Error saving ImageToVideo result to object store", "err", err)
				return
			}

			slog.InfoContext(ctx, "Saved ImageToVideo result", "path", path)
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

		slog.DebugContext(ctx, "Received LLM request", "model_id", *req.Model, "stream", *req.Stream)

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
		slog.DebugContext(ctx, "Processed LLM request", "model_id", *req.Model, "took", took)

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
		ctx := r.Context()
		streamName := r.PathValue("stream")
		if streamName == "" {
			slog.ErrorContext(ctx, "Missing stream name")
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}

		ctx = clog.AddVal(ctx, "stream", streamName)
		sourceID := r.FormValue("source_id")
		if sourceID == "" {
			slog.ErrorContext(ctx, "Missing source_id")
			http.Error(w, "Missing source_id", http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "source_id", sourceID)
		sourceType := r.FormValue("source_type")
		if sourceType == "" {
			slog.ErrorContext(ctx, "Missing source_type")
			http.Error(w, "Missing source_type", http.StatusBadRequest)
			return
		}
		sourceType = strings.ToLower(sourceType) // mediamtx changed casing between versions
		sourceTypeStr, err := media.MediamtxSourceTypeToString(sourceType)
		if err != nil {
			slog.ErrorContext(ctx, "Invalid source type", "source_type", sourceType)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "source_type", sourceType)

		remoteHost, err := getRemoteHost(r.RemoteAddr)
		if err != nil {
			slog.ErrorContext(ctx, "Could not find callback host", "err", err.Error())
			http.Error(w, "Could not find callback host", http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "remote_addr", remoteHost)

		queryParams := r.FormValue("query")
		qp, err := url.ParseQuery(queryParams)
		if err != nil {
			slog.ErrorContext(ctx, "invalid query params", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// If auth webhook is set and returns an output URL, this will be replaced
		outputURL := qp.Get("rtmpOutput")

		mediaMTXOutputURL := fmt.Sprintf("rtmp://%s/aiWebrtc/%s-out", remoteHost, streamName)
		if outputURL == "" {
			// re-publish to ourselves for now
			// Not sure if we want this to be permanent
			outputURL = mediaMTXOutputURL
		}

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
		var streamID, pipelineID string
		var pipelineParams map[string]interface{}
		if rawParams != "" {
			if err := json.Unmarshal([]byte(rawParams), &pipelineParams); err != nil {
				slog.ErrorContext(ctx, "Invalid pipeline params", "err", err)
				http.Error(w, "Invalid model params", http.StatusBadRequest)
				return
			}
		}

		mediaMTXClient := media.NewMediaMTXClient(remoteHost, ls.mediaMTXApiPassword, sourceID, sourceType)

		if LiveAIAuthWebhookURL != nil {
			authResp, err := authenticateAIStream(LiveAIAuthWebhookURL, ls.liveAIAuthApiKey, AIAuthRequest{
				Stream:      streamName,
				Type:        sourceTypeStr,
				QueryParams: queryParams,
				GatewayHost: ls.LivepeerNode.GatewayHost,
			})
			if err != nil {
				kickErr := mediaMTXClient.KickInputConnection(ctx)
				if kickErr != nil {
					slog.ErrorContext(ctx, "failed to kick input connection", "err", kickErr.Error())
				}
				slog.ErrorContext(ctx, "Live AI auth failed", "err", err.Error())
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

		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)
		ctx = clog.AddVal(ctx, "stream_id", streamID)
		slog.InfoContext(ctx, "Received live video AI request", "stream", streamName, "pipelineParams", pipelineParams)

		// Count `ai_live_attempts` after successful parameters validation
		clog.V(common.VERBOSE).Infof(ctx, "AI Live video attempt")
		if monitor.Enabled {
			monitor.AILiveVideoAttempt()
		}

		// Kick off the RTMP pull and segmentation as soon as possible
		ssr := media.NewSwitchableSegmentReader()
		go func() {
			// Currently for webrtc we need to add a path prefix due to the ingress setup
			mediaMTXStreamPrefix := r.PathValue("prefix")
			if mediaMTXStreamPrefix != "" {
				mediaMTXStreamPrefix = mediaMTXStreamPrefix + "/"
			}
			ms := media.MediaSegmenter{Workdir: ls.LivepeerNode.WorkDir, MediaMTXClient: mediaMTXClient}
			ms.RunSegmentation(ctx, fmt.Sprintf("rtmp://%s/%s%s", remoteHost, mediaMTXStreamPrefix, streamName), ssr.Read)
			ssr.Close()
			ls.cleanupLive(streamName)
		}()

		sendErrorEvent := LiveErrorEventSender(ctx, streamID, map[string]string{
			"type":        "error",
			"request_id":  requestID,
			"stream_id":   streamID,
			"pipeline_id": pipelineID,
			"pipeline":    pipeline,
		})

		// this function is called when the pipeline hits a fatal error, we kick the input connection to allow
		// the client to reconnect and restart the pipeline
		stopPipeline := func(err error) {
			if err == nil {
				return
			}
			slog.ErrorContext(ctx, "Live video pipeline stopping", "err", err)

			sendErrorEvent(err)

			err = mediaMTXClient.KickInputConnection(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to kick input connection", "err", err)
			}
		}

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,

			liveParams: liveRequestParams{
				segmentReader:          ssr,
				outputRTMPURL:          outputURL,
				mediaMTXOutputRTMPURL:  mediaMTXOutputURL,
				stream:                 streamName,
				paymentProcessInterval: ls.livePaymentInterval,
				requestID:              requestID,
				streamID:               streamID,
				pipelineID:             pipelineID,
				stopPipeline:           stopPipeline,
				sendErrorEvent:         sendErrorEvent,
			},
		}

		req := worker.GenLiveVideoToVideoJSONRequestBody{
			ModelId: &pipeline,
			Params:  &pipelineParams,
		}
		_, err = processAIRequest(ctx, params, req)
		if err != nil {
			stopPipeline(err)
		}
	})
}

func getRemoteHost(remoteAddr string) (string, error) {
	if remoteAddr == "" {
		return "", errors.New("remoteAddr is empty")
	}
	split := strings.Split(remoteAddr, ":")
	if len(split) < 1 {
		return "", fmt.Errorf("couldn't find remote host: %s", remoteAddr)
	}
	return split[0], nil
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
		defer ls.LivepeerNode.LiveMu.RUnlock()
		p, ok := ls.LivepeerNode.LivePipelines[stream]
		if !ok {
			// Stream not found
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		defer r.Body.Close()
		params, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.V(6).Infof(ctx, "Sending Live Video Update Control API stream=%s, params=%s", stream, string(params))
		if err := p.ControlPub.Write(strings.NewReader(string(params))); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// @Summary Get Live Stream Status
// @Param stream path string true "Stream ID"
// @Success 200
// @Router /live/video-to-video/{stream}/status [get]
func (ls *LivepeerServer) GetLiveVideoToVideoStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamId := r.PathValue("streamId")
		if streamId == "" {
			http.Error(w, "stream id is required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		ctx = clog.AddVal(ctx, "stream", streamId)

		// Get status for specific stream
		status, exists := StreamStatusStore.Get(streamId)
		if !exists {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}
		gatewayStatus, exists := GatewayStatus.Get(streamId)
		if exists {
			for k, v := range gatewayStatus {
				status["gateway_"+k] = v
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			slog.ErrorContext(ctx, "Failed to encode stream status", "err", err)
			http.Error(w, "Failed to encode status", http.StatusInternalServerError)
			return
		}
	})
}

func (ls *LivepeerServer) cleanupLive(stream string) {
	ls.LivepeerNode.LiveMu.Lock()
	pub, ok := ls.LivepeerNode.LivePipelines[stream]
	delete(ls.LivepeerNode.LivePipelines, stream)
	ls.LivepeerNode.LiveMu.Unlock()
	if monitor.Enabled {
		monitor.AICurrentLiveSessions(len(ls.LivepeerNode.LivePipelines))
	}

	if ok && pub != nil && pub.ControlPub != nil {
		if err := pub.ControlPub.Close(); err != nil {
			slog.Info("Error closing trickle publisher", "err", err)
		}
		pub.StopControl()
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

		slog.InfoContext(ctx, "Starting smoke test", "ingestURL", ingestURL, "duration", duration)

		if err := cmd.Start(); err != nil {
			cancel()
			slog.ErrorContext(ctx, "Smoke test failed to start ffmpeg", "err", err, "command", "\nffmpeg "+strings.Join(params, " "))
			http.Error(w, "Failed to start stream", http.StatusInternalServerError)
			return
		}

		go func() {
			defer cancel()
			_ = backoff.Retry(func() error {
				if state, err := cmd.Process.Wait(); err != nil || state.ExitCode() != 0 {
					slog.ErrorContext(ctx, "Smoke test failed to run ffmpeg", "err", err, "exit_code", state.ExitCode(), "command", "\nffmpeg "+strings.Join(params, " "))
					slog.ErrorContext(ctx, "Smoke test ffmpeg output", "output", outputBuf.String())
					return fmt.Errorf("ffmpeg failed")
				}
				slog.InfoContext(ctx, "Smoke test finished successfully", "ingestURL", ingestURL)
				return nil
			}, backoff.WithMaxRetries(backoff.NewConstantBackOff(30*time.Second), 3))
		}()
	})
}
