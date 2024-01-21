package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/oapi-codegen/runtime"
)

func startAIServer(lp lphttp) error {
	swagger, err := worker.GetSwagger()
	if err != nil {
		return err
	}
	swagger.Servers = nil

	oapiReqValidator := middleware.OapiRequestValidator(swagger)

	openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)

	lp.transRPC.Handle("/text-to-image", oapiReqValidator(lp.TextToImage()))
	lp.transRPC.Handle("/image-to-image", oapiReqValidator(lp.ImageToImage()))
	lp.transRPC.Handle("/image-to-video", oapiReqValidator(lp.ImageToVideo()))

	return nil
}

func (h *lphttp) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req worker.TextToImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(r.Context(), "Received TextToImage request prompt=%v model_id=%v", req.Prompt, *req.ModelId)

		resp, err := h.orchestrator.TextToImage(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (h *lphttp) ImageToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		multiRdr, err := r.MultipartReader()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.ImageToImageMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		clog.V(common.VERBOSE).Infof(r.Context(), "Received ImageToImage request imageSize=%v prompt=%v model_id=%v", req.Image.FileSize(), req.Prompt, *req.ModelId)

		resp, err := h.orchestrator.ImageToImage(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (h *lphttp) ImageToVideo() http.Handler {
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

		results, err := h.orchestrator.ImageToVideo(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: Handle more than one video
		if len(results) != 1 {
			http.Error(w, "failed to return results", http.StatusInternalServerError)
			return
		}

		res := results[0]

		// Assume only single rendition right now
		seg := res.TranscodeData.Segments[0]
		name := fmt.Sprintf("%v.mp4", core.RandomManifestID())
		segData := bytes.NewReader(seg.Data)
		uri, err := res.OS.SaveData(ctx, name, segData, nil, 0)
		if err != nil {
			clog.Errorf(ctx, "Could not upload segment err=%q", err)
		}

		var result net.TranscodeResult
		if err != nil {
			clog.Errorf(ctx, "Could not transcode err=%q", err)
			result = net.TranscodeResult{Result: &net.TranscodeResult_Error{Error: err.Error()}}
		} else {
			result = net.TranscodeResult{
				Result: &net.TranscodeResult_Data{
					Data: &net.TranscodeData{
						Segments: []*net.TranscodedSegmentData{
							{Url: uri, Pixels: seg.Pixels},
						},
						Sig: res.Sig,
					},
				},
			}
		}

		tr := &net.TranscodeResult{
			Result: result.Result,
			// TODO: Add other fields
		}

		buf, err := proto.Marshal(tr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
}
