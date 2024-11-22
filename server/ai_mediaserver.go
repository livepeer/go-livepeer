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
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/livepeer/ai-worker/worker"
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

func startAIMediaServer(ls *LivepeerServer) error {
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
	ls.HTTPMux.Handle("/live/video-to-video/{stream}/start", ls.StartLiveVideo())
	ls.HTTPMux.Handle("/live/video-to-video/{stream}/update", ls.UpdateLiveVideo())

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

		var req worker.GenLLMFormdataRequestBody

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received LLM request prompt=%v model_id=%v stream=%v", req.Prompt, *req.ModelId, *req.Stream)

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
		clog.V(common.VERBOSE).Infof(ctx, "Processed LLM request prompt=%v model_id=%v took=%v", req.Prompt, *req.ModelId, took)

		if streamChan, ok := resp.(chan worker.LlmStreamChunk); ok {
			// Handle streaming response (SSE)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			for chunk := range streamChan {
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
				if chunk.Done {
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

func (ls *LivepeerServer) StartLiveVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		streamName := r.PathValue("stream")
		if streamName == "" {
			clog.Errorf(ctx, "Missing stream name")
			http.Error(w, "Missing stream name", http.StatusBadRequest)
			return
		}
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
		sourceTypeStr, err := mediamtxSourceTypeToString(sourceType)
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
		if outputURL == "" {
			// re-publish to ourselves for now
			// Not sure if we want this to be permanent
			outputURL = fmt.Sprintf("rtmp://%s/%s-out", remoteHost, streamName)
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
		var pipelineParams map[string]interface{}
		if rawParams != "" {
			if err := json.Unmarshal([]byte(rawParams), &pipelineParams); err != nil {
				clog.Errorf(ctx, "Invalid pipeline params: %s", err)
				http.Error(w, "Invalid model params", http.StatusBadRequest)
				return
			}
		}

		if LiveAIAuthWebhookURL != nil {
			authResp, err := authenticateAIStream(LiveAIAuthWebhookURL, AIAuthRequest{
				Stream:      streamName,
				Type:        sourceTypeStr,
				QueryParams: queryParams,
			})
			if err != nil {
				kickErr := ls.kickInputConnection(remoteHost, sourceID, sourceType)
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
				pipelineParams = authResp.paramsMap
			}
		}

		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)
		clog.Infof(ctx, "Received live video AI request for %s", streamName)

		// Kick off the RTMP pull and segmentation as soon as possible
		ssr := media.NewSwitchableSegmentReader()
		go func() {
			ms := media.MediaSegmenter{Workdir: ls.LivepeerNode.WorkDir}
			ms.RunSegmentation(fmt.Sprintf("rtmp://%s/%s", remoteHost, streamName), ssr.Read)
			ssr.Close()
			ls.cleanupLive(streamName)
		}()

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,

			liveParams: liveRequestParams{
				segmentReader: ssr,
				outputRTMPURL: outputURL,
				stream:        streamName,
			},
		}

		req := worker.GenLiveVideoToVideoJSONRequestBody{
			ModelId: &pipeline,
			Params:  &pipelineParams,
		}
		processAIRequest(ctx, params, req)
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

func (ls *LivepeerServer) cleanupLive(stream string) {
	ls.LivepeerNode.LiveMu.Lock()
	pub, ok := ls.LivepeerNode.LivePipelines[stream]
	delete(ls.LivepeerNode.LivePipelines, stream)
	ls.LivepeerNode.LiveMu.Unlock()

	if ok && pub != nil && pub.ControlPub != nil {
		if err := pub.ControlPub.Close(); err != nil {
			slog.Info("Error closing trickle publisher", "err", err)
		}
	}
}
