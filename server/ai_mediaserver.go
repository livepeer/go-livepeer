package server

import (
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

func startAIMediaServer(ls *LivepeerServer) error {
	swagger, err := worker.GetSwagger()
	if err != nil {
		return err
	}
	swagger.Servers = nil

	oapiReqValidator := middleware.OapiRequestValidator(swagger)

	openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)

	ls.HTTPMux.Handle("/text-to-image", oapiReqValidator(ls.TextToImage()))
	ls.HTTPMux.Handle("/image-to-video", oapiReqValidator(ls.ImageToVideo()))

	return nil
}

func (ls *LivepeerServer) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req worker.TextToImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(r.Context(), "Received TextToImage request prompt=%v model_id=%v", req.Prompt, *req.ModelId)

		params := aiRequestParams{
			node: ls.LivepeerNode,
		}

		start := time.Now()
		resp, err := processTextToImage(ctx, params, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		took := time.Since(start)
		clog.Infof(ctx, "Processed TextToImage request prompt=%v model_id=%v took=%v", req.Prompt, *req.ModelId, took)

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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.ImageToVideoMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToVideo request imageSize=%v model_id=%v", req.Image.FileSize(), *req.ModelId)

		params := aiRequestParams{
			node: ls.LivepeerNode,
			os:   drivers.NodeStorage.NewSession(string(core.RandomManifestID())),
		}

		start := time.Now()
		urls, err := processImageToVideo(ctx, params, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		took := time.Since(start)
		clog.Infof(ctx, "Processed ImageToVideo request imageSize=%v model_id=%v took=%v", req.Image.FileSize(), *req.ModelId, took)

		// HACK: Re-use worker.ImageResponse to return results
		videos := make([]worker.Media, len(urls))
		for i, url := range urls {
			videos[i] = worker.Media{
				Url: url,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(&worker.ImageResponse{Images: videos})
	})
}
