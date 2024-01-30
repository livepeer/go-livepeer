package server

import (
	"bytes"
	"context"
	"encoding/json"
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
	Error error `json:"error,omitempty"`
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
		},
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			clog.Errorf(context.Background(), "oapi validation error statusCode=%v message=%v", statusCode, message)
		},
	}
	oapiReqValidator := middleware.OapiRequestValidatorWithOptions(swagger, opts)

	openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)

	ls.HTTPMux.Handle("/text-to-image", oapiReqValidator(ls.TextToImage()))
	ls.HTTPMux.Handle("/image-to-image", oapiReqValidator(ls.ImageToImage()))
	ls.HTTPMux.Handle("/image-to-video", oapiReqValidator(ls.ImageToVideo()))
	ls.HTTPMux.Handle("/image-to-video/result", ls.ImageToVideoResult())

	return nil
}

func (ls *LivepeerServer) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req worker.TextToImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(r.Context(), "Received TextToImage request prompt=%v model_id=%v", req.Prompt, *req.ModelId)

		params := aiRequestParams{
			node: ls.LivepeerNode,
			os:   drivers.NodeStorage.NewSession(string(core.RandomManifestID())),
		}

		start := time.Now()
		resp, err := processTextToImage(ctx, params, req)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
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

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.ImageToImageMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToImage request imageSize=%v prompt=%v model_id=%v", req.Image.FileSize(), req.Prompt, *req.ModelId)

		params := aiRequestParams{
			node: ls.LivepeerNode,
			os:   drivers.NodeStorage.NewSession(string(core.RandomManifestID())),
		}

		start := time.Now()
		resp, err := processImageToImage(ctx, params, req)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
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

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.ImageToVideoMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var async bool
		prefer := r.Header.Get("Prefer")
		if prefer == "respond-async" {
			async = true
		}

		requestID := string(core.RandomManifestID())

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToVideo request request_id=%v imageSize=%v model_id=%v async=%v", requestID, req.Image.FileSize(), *req.ModelId, async)

		params := aiRequestParams{
			node: ls.LivepeerNode,
			os:   drivers.NodeStorage.NewSession(requestID),
		}

		resultCh := make(chan ImageToVideoResult)
		go func(ctx context.Context) {
			start := time.Now()

			if async {
				// Use new context instead of request context for background processing
				ctx = context.Background()

				var data bytes.Buffer
				if err := json.NewEncoder(&data).Encode(req); err != nil {
					clog.Errorf(ctx, "Error JSON encoding ImageToVideo request request_id=%v err=%v", requestID, err)
					return
				}

				path, err := params.os.SaveData(ctx, "request.json", bytes.NewReader(data.Bytes()), nil, 0)
				if err != nil {
					clog.Errorf(ctx, "Error saving ImageToVideo request to object store request_id=%v err=%v", requestID, err)
					return
				}

				clog.Infof(ctx, "Saved ImageToVideo request request_id=%v path=%v", requestID, path)
			}

			resp, err := processImageToVideo(ctx, params, req)
			if err != nil {
				clog.Errorf(ctx, "Error processing ImageToVideo request request_id=%v err=%v", requestID, err)
			} else {
				took := time.Since(start)
				clog.Infof(ctx, "Processed ImageToVideo request request_id=%v imageSize=%v model_id=%v took=%v", requestID, req.Image.FileSize(), *req.ModelId, took)
			}

			result := ImageToVideoResult{
				ImageResponse: resp,
				Error:         err,
			}

			// If async = false, then we expect a receiver on the channel and return early
			// If async = true, then there is no receiver so we move on to save the result in the object store
			select {
			case resultCh <- result:
				return
			default:
			}

			var data bytes.Buffer
			if err := json.NewEncoder(&data).Encode(result); err != nil {
				clog.Errorf(ctx, "Error JSON encoding ImageToVideo result request_id=%v err=%v", requestID, err)
				return
			}

			path, err := params.os.SaveData(ctx, "result.json", bytes.NewReader(data.Bytes()), nil, 0)
			if err != nil {
				clog.Errorf(ctx, "Error saving ImageToVideo result to object store request_id=%v err=%v", requestID, err)
				return
			}

			clog.Infof(ctx, "Saved ImageToVideo result request_id=%v path=%v", requestID, path)
		}(ctx)

		w.Header().Set("Content-Type", "application/json")

		if async {
			w.WriteHeader(http.StatusAccepted)
			resp := &ImageToVideoResponseAsync{
				RequestID: requestID,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		result := <-resultCh
		if result.Error != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result.ImageResponse)
	})
}

func (ls *LivepeerServer) ImageToVideoResult() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req ImageToVideoResultRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToVideoResult request request_id=%v", req.RequestID)

		sess := drivers.NodeStorage.NewSession(req.RequestID)

		_, err := sess.ReadData(ctx, "request.json")
		if err != nil {
			clog.Errorf(ctx, "Error fetching ImageToVideo request err=%v", err)
			respondWithError(w, "invalid request", http.StatusBadRequest)
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
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
