package server

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"strconv"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
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
	lp.transRPC.Handle("/upscale", oapiReqValidator(lp.Upscale()))
	lp.transRPC.Handle("/audio-to-text", oapiReqValidator(lp.AudioToText()))
	lp.transRPC.Handle("/segment-anything-2", oapiReqValidator(lp.SegmentAnything2()))
	lp.transRPC.Handle("/lipsync", oapiReqValidator(lp.Lipsync()))

	return nil
}

func (h *lphttp) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req worker.GenTextToImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) ImageToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenImageToImageMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) ImageToVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenImageToVideoMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) Upscale() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenUpscaleMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) AudioToText() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenAudioToTextMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) SegmentAnything2() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenSegmentAnything2MultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) Lipsync() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenLipsyncMultipartRequestBody
		if err := runtime.BindMultipart(&req, *multiRdr); err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func handleAIRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, orch Orchestrator, req interface{}) {
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

	var cap core.Capability
	var pipeline string
	var modelID string
	var submitFn func(context.Context) (interface{}, error)
	var outPixels int64

	switch v := req.(type) {
	case worker.GenTextToImageJSONRequestBody:
		pipeline = "text-to-image"
		cap = core.Capability_TextToImage
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.TextToImage(ctx, v)
		}

		// TODO: The orchestrator should require the broadcaster to always specify a height and width
		height := int64(512)
		if v.Height != nil {
			height = int64(*v.Height)
		}
		width := int64(512)
		if v.Width != nil {
			width = int64(*v.Width)
		}
		// NOTE: Should be enforced by the gateway, added for backwards compatibility.
		numImages := int64(1)
		if v.NumImagesPerPrompt != nil {
			numImages = int64(*v.NumImagesPerPrompt)
		}

		outPixels = height * width * numImages
	case worker.GenImageToImageMultipartRequestBody:
		pipeline = "image-to-image"
		cap = core.Capability_ImageToImage
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToImage(ctx, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		// NOTE: Should be enforced by the gateway, added for backwards compatibility.
		numImages := int64(1)
		if v.NumImagesPerPrompt != nil {
			numImages = int64(*v.NumImagesPerPrompt)
		}

		outPixels = int64(config.Height) * int64(config.Width) * numImages
	case worker.GenUpscaleMultipartRequestBody:
		pipeline = "upscale"
		cap = core.Capability_Upscale
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.Upscale(ctx, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		outPixels = int64(config.Height) * int64(config.Width)
	case worker.GenImageToVideoMultipartRequestBody:
		pipeline = "image-to-video"
		cap = core.Capability_ImageToVideo
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToVideo(ctx, v)
		}

		// TODO: The orchestrator should require the broadcaster to always specify a height and width
		height := int64(576)
		if v.Height != nil {
			height = int64(*v.Height)
		}
		width := int64(1024)
		if v.Width != nil {
			width = int64(*v.Width)
		}
		// The # of frames outputted by stable-video-diffusion-img2vid-xt models
		frames := int64(25)

		outPixels = height * width * int64(frames)
	case worker.GenAudioToTextMultipartRequestBody:
		pipeline = "audio-to-text"
		cap = core.Capability_AudioToText
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.AudioToText(ctx, v)
		}

		outPixels, err = common.CalculateAudioDuration(v.Audio)
		if err != nil {
			respondWithError(w, "Unable to calculate duration", http.StatusBadRequest)
			return
		}
		outPixels *= 1000 // Convert to milliseconds
	case worker.GenSegmentAnything2MultipartRequestBody:
		pipeline = "segment-anything-2"
		cap = core.Capability_SegmentAnything2
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.SegmentAnything2(ctx, v)
		}

		imageRdr, err := v.Image.Reader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		config, _, err := image.DecodeConfig(imageRdr)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		outPixels = int64(config.Height) * int64(config.Width)
	case worker.GenLipsyncMultipartRequestBody:
		pipeline = "lipsync"
		cap = core.Capability_Lipsync
		modelID = v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.Lipsync(ctx, v)
		}

		// TODO(pschroedl): Infer length of video based on tokenizing text input or length of audio input file
		outPixels = int64(1000)
	default:
		respondWithError(w, "Unknown request type", http.StatusBadRequest)
		return
	}

	requestID := string(core.RandomManifestID())

	clog.V(common.VERBOSE).Infof(ctx, "Received request id=%v cap=%v modelID=%v", requestID, cap, modelID)

	manifestID := core.ManifestID(strconv.Itoa(int(cap)) + "_" + modelID)

	// Check if there is capacity for the request.
	if !orch.CheckAICapacity(pipeline, modelID) {
		respondWithError(w, fmt.Sprintf("Insufficient capacity for pipeline=%v modelID=%v", pipeline, modelID), http.StatusServiceUnavailable)
		return
	}

	// Known limitation:
	// This call will set a fixed price for all requests in a session identified by a manifestID.
	// Since all requests for a capability + modelID are treated as "session" with a single manifestID, all
	// requests for a capability + modelID will get the same fixed price for as long as the orch is running
	if err := orch.ProcessPayment(ctx, payment, manifestID); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, manifestID) {
		respondWithError(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Note: At the moment, we do not return a new OrchestratorInfo with updated ticket params + price with
	// extended expiry because the response format does not include such a field. As a result, the broadcaster
	// might encounter an expiration error for ticket params + price when it is using an old OrchestratorInfo returned
	// by the orch during discovery. In that scenario, the broadcaster can use a GetOrchestrator() RPC call to get a
	// a new OrchestratorInfo before submitting a request.

	start := time.Now()
	resp, err := submitFn(ctx)
	if err != nil {
		if monitor.Enabled {
			monitor.AIProcessingError(err.Error(), pipeline, modelID, sender.Hex())
		}
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	took := time.Since(start)
	clog.Infof(ctx, "Processed request id=%v cap=%v modelID=%v took=%v", requestID, cap, modelID, took)

	// At the moment, outPixels is expected to just be height * width * frames
	// If the # of inference/denoising steps becomes configurable, a possible updated formula could be height * width * frames * steps
	// If additional parameters that influence compute cost become configurable, then the formula should be reconsidered
	orch.DebitFees(sender, manifestID, payment.GetExpectedPrice(), outPixels)

	if monitor.Enabled {
		var latencyScore float64
		switch v := req.(type) {
		case worker.GenTextToImageJSONRequestBody:
			latencyScore = CalculateTextToImageLatencyScore(took, v, outPixels)
		case worker.GenImageToImageMultipartRequestBody:
			latencyScore = CalculateImageToImageLatencyScore(took, v, outPixels)
		case worker.GenImageToVideoMultipartRequestBody:
			latencyScore = CalculateImageToVideoLatencyScore(took, v, outPixels)
		case worker.GenUpscaleMultipartRequestBody:
			latencyScore = CalculateUpscaleLatencyScore(took, v, outPixels)
		case worker.GenAudioToTextMultipartRequestBody:
			durationSeconds, err := common.CalculateAudioDuration(v.Audio)
			if err == nil {
				latencyScore = CalculateAudioToTextLatencyScore(took, durationSeconds)
			}
		case worker.GenSegmentAnything2MultipartRequestBody:
			latencyScore = CalculateSegmentAnything2LatencyScore(took, outPixels)
		}

		var pricePerAIUnit float64
		if priceInfo := payment.GetExpectedPrice(); priceInfo != nil && priceInfo.GetPixelsPerUnit() != 0 {
			pricePerAIUnit = float64(priceInfo.GetPricePerUnit()) / float64(priceInfo.GetPixelsPerUnit())
		}

		monitor.AIJobProcessed(ctx, pipeline, modelID, monitor.AIJobInfo{LatencyScore: latencyScore, PricePerUnit: pricePerAIUnit})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
