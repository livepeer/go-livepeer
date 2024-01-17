package server

import (
	"encoding/json"
	"net/http"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

func startAIServer(lp lphttp) error {
	swagger, err := worker.GetSwagger()
	if err != nil {
		return err
	}
	swagger.Servers = nil

	oapiReqValidator := middleware.OapiRequestValidator(swagger)

	aiHandler := worker.Handler(worker.Unimplemented{})
	lp.transRPC.Handle("/text-to-image", oapiReqValidator(lp.TextToImage()))
	lp.transRPC.Handle("/image-to-image", aiHandler)
	lp.transRPC.Handle("/image-to-video", aiHandler)

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

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
