package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
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

	ls.HTTPMux.Handle("/text-to-image", oapiReqValidator(ls.TextToImage()))
	ls.HTTPMux.Handle("/image-to-image", oapiReqValidator(ls.ImageToImage()))
	ls.HTTPMux.Handle("/upscale", oapiReqValidator(ls.Upscale()))
	ls.HTTPMux.Handle("/image-to-video", oapiReqValidator(ls.ImageToVideo()))
	ls.HTTPMux.Handle("/image-to-video/result", ls.ImageToVideoResult())
	ls.HTTPMux.Handle("/audio-to-text", oapiReqValidator(ls.AudioToText()))

	return nil
}

func (ls *LivepeerServer) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)
		requestID := string(core.RandomManifestID())
		ctx = clog.AddVal(ctx, "request_id", requestID)

		var req worker.TextToImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(r.Context(), "Received TextToImage request prompt=%v model_id=%v", req.Prompt, *req.ModelId)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,
		}

		start := time.Now()
		resp, err := processTextToImage(ctx, params, req)
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
		clog.Infof(ctx, "Processed TextToImage request prompt=%v model_id=%v took=%v", req.Prompt, *req.ModelId, took)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (ls *LivepeerServer) ImageToImage() http.Handler {
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

		var req worker.ImageToImageMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToImage request imageSize=%v prompt=%v model_id=%v", req.Image.FileSize(), req.Prompt, *req.ModelId)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,
		}

		start := time.Now()
		resp, err := processImageToImage(ctx, params, req)
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
		clog.V(common.VERBOSE).Infof(ctx, "Processed ImageToImage request imageSize=%v prompt=%v model_id=%v took=%v", req.Image.FileSize(), req.Prompt, *req.ModelId, took)

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

		var req worker.ImageToVideoMultipartRequestBody
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
				var e *ServiceUnavailableError
				if errors.As(err, &e) {
					respondJsonError(ctx, w, err, http.StatusServiceUnavailable)
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

func (ls *LivepeerServer) Upscale() http.Handler {
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

		var req worker.UpscaleMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received Upscale request imageSize=%v prompt=%v model_id=%v", req.Image.FileSize(), req.Prompt, *req.ModelId)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(requestID),
			sessManager: ls.AISessionManager,
		}

		start := time.Now()
		resp, err := processUpscale(ctx, params, req)
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
		clog.V(common.VERBOSE).Infof(ctx, "Processed Upscale request imageSize=%v prompt=%v model_id=%v took=%v", req.Image.FileSize(), req.Prompt, *req.ModelId, took)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (ls *LivepeerServer) AudioToText() http.Handler {
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

		var req worker.AudioToTextMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondJsonError(ctx, w, err, http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received AudioToText request model_id=%v", *req.ModelId)

		params := aiRequestParams{
			node:        ls.LivepeerNode,
			os:          drivers.NodeStorage.NewSession(string(core.RandomManifestID())),
			sessManager: ls.AISessionManager,
		}

		start := time.Now()
		resp, err := processAudioToText(ctx, params, req)
		if err != nil {
			var e *ServiceUnavailableError
			var reqError *BadRequestError
			if errors.As(err, &e) {
				respondJsonError(ctx, w, err, http.StatusServiceUnavailable)
				return
			} else if errors.As(err, &reqError) {
				respondJsonError(ctx, w, err, http.StatusBadRequest)
				return
			} else {
				respondJsonError(ctx, w, err, http.StatusInternalServerError)
				return
			}
		}

		took := time.Since(start)
		clog.V(common.VERBOSE).Infof(ctx, "Processed AudioToText request model_id=%v took=%v", *req.ModelId, took)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
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
