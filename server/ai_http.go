package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/oapi-codegen/runtime"
)

func startAIServer(lp lphttp) error {
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

	lp.transRPC.Handle("/text-to-image", oapiReqValidator(lp.TextToImage()))
	lp.transRPC.Handle("/image-to-image", oapiReqValidator(lp.ImageToImage()))
	lp.transRPC.Handle("/image-to-video", oapiReqValidator(lp.ImageToVideo()))

	return nil
}

func (h *lphttp) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		payment, err := getPayment(r.Header.Get(paymentHeader))
		if err != nil {
			respondWithError(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		sender := getPaymentSender(payment)

		_, ctx, err = verifySegCreds(ctx, orch, r.Header.Get(segmentHeader), sender)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusForbidden)
			return
		}

		var req worker.TextToImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received TextToImage request prompt=%v model_id=%v", req.Prompt, *req.ModelId)

		manifestID := core.ManifestID(strconv.Itoa(int(core.Capability_TextToImage)) + "_" + *req.ModelId)

		// Known limitation:
		// This call will set a fixed price for all requests in a session identified by a manifestID.
		// Since all requests for a capability + modelID are treated as "session" with a single manifestID, all
		// requests for a capability + modelID will get the same fixed price for as long as the orch is running
		if err := h.orchestrator.ProcessPayment(ctx, payment, manifestID); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, manifestID) {
			respondWithError(w, "Insufficient balance", http.StatusBadRequest)
			return
		}

		// At the moment, this method does not return a new OrchestratorInfo with updated ticket params + price with
		// extended expiry because the response format does not include such a field. As a result, the broadcaster
		// might encounter an expiration error for ticket params + price when it is using an old OrchestratorInfo returned
		// by the orch during discovery. In that scenario, the broadcaster can use a GetOrchestrator() RPC call to get a
		// a new OrchestratorInfo before submitting a request.

		start := time.Now()
		resp, err := h.orchestrator.TextToImage(r.Context(), req)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		took := time.Since(start)
		clog.Infof(ctx, "Processed TextToImage request prompt=%v model_id=%v took=%v", req.Prompt, *req.ModelId, took)

		// TODO: The orchestrator should require the broadcaster to always specify a height and width
		height := int64(512)
		if req.Height != nil {
			height = int64(*req.Height)
		}
		width := int64(512)
		if req.Width != nil {
			width = int64(*req.Width)
		}

		// If the # of inference/denoising steps becomes configurable, a possible updated formula could be height * width * steps
		// If additional parameters that influence compute cost become configurable, then the formula should be reconsidered
		outPixels := height * width
		h.orchestrator.DebitFees(sender, manifestID, payment.GetExpectedPrice(), outPixels)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func (h *lphttp) ImageToImage() http.Handler {
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

		clog.V(common.VERBOSE).Infof(r.Context(), "Received ImageToImage request imageSize=%v prompt=%v model_id=%v", req.Image.FileSize(), req.Prompt, *req.ModelId)

		start := time.Now()
		resp, err := h.orchestrator.ImageToImage(r.Context(), req)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		took := time.Since(start)
		clog.Infof(ctx, "Processed ImageToImage request imageSize=%v prompt=%v model_id=%v took=%v", req.Image.FileSize(), req.Prompt, *req.ModelId, took)

		w.Header().Set("Content-Type", "application/json")
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
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.ImageToVideoMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		clog.V(common.VERBOSE).Infof(ctx, "Received ImageToVideo request imageSize=%v model_id=%v", req.Image.FileSize(), *req.ModelId)

		start := time.Now()
		resp, err := h.orchestrator.ImageToVideo(ctx, req)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		took := time.Since(start)
		clog.Infof(ctx, "Processed ImageToVideo request imageSize=%v model_id=%v took=%v", req.Image.FileSize(), *req.ModelId, took)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
