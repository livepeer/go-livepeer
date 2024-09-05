package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang/glog"
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
	// /aiResults endpoint registered in server/rpc.go

	return nil
}

func (h *lphttp) TextToImage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req worker.TextToImageJSONRequestBody
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

		var req worker.ImageToImageMultipartRequestBody
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

		var req worker.ImageToVideoMultipartRequestBody
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

		var req worker.UpscaleMultipartRequestBody
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

		var req worker.AudioToTextMultipartRequestBody
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

		var req worker.SegmentAnything2MultipartRequestBody
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

	requestID := string(core.RandomManifestID())

	var cap core.Capability
	var pipeline string
	var modelID string
	var submitFn func(context.Context) (interface{}, error)
	var outPixels int64

	switch v := req.(type) {
	case worker.TextToImageJSONRequestBody:
		pipeline = "text-to-image"
		cap = core.Capability_TextToImage
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.TextToImage(ctx, requestID, v)
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
	case worker.ImageToImageMultipartRequestBody:
		pipeline = "image-to-image"
		cap = core.Capability_ImageToImage
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToImage(ctx, requestID, v)
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
	case worker.UpscaleMultipartRequestBody:
		pipeline = "upscale"
		cap = core.Capability_Upscale
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.Upscale(ctx, requestID, v)
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
	case worker.ImageToVideoMultipartRequestBody:
		pipeline = "image-to-video"
		cap = core.Capability_ImageToVideo
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToVideo(ctx, requestID, v)
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
	case worker.AudioToTextMultipartRequestBody:
		pipeline = "audio-to-text"
		cap = core.Capability_AudioToText
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.AudioToText(ctx, requestID, v)
		}

		outPixels, err = common.CalculateAudioDuration(v.Audio)
		if err != nil {
			respondWithError(w, "Unable to calculate duration", http.StatusBadRequest)
			return
		}
		outPixels *= 1000 // Convert to milliseconds
	case worker.SegmentAnything2MultipartRequestBody:
		pipeline = "segment-anything-2"
		cap = core.Capability_SegmentAnything2
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.SegmentAnything2(ctx, requestID, v)
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
	default:
		respondWithError(w, "Unknown request type", http.StatusBadRequest)
		return
	}

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

	err = orch.CreateStorageForRequest(requestID)
	if err != nil {
		respondWithError(w, "Could not create storage to receive results", http.StatusInternalServerError)
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

	//backwards compatibility to old gateway api
	if pipeline == "text-to-image" || pipeline == "image-to-image" || pipeline == "upscale" {
		if r.Header.Get("Authorization") != protoVerAIWorker {
			imgResp := resp.(*worker.ImageResponse)
			prefix := "data:image/png;base64," //https://github.com/livepeer/ai-worker/blob/78b58131f12867ce5a4d0f6e2b9038e70de5c8e3/runner/app/routes/util.py#L56
			storage, exists := orch.GetStorageForRequest(requestID)
			if exists {
				for i, image := range imgResp.Images {
					fileData, err := storage.ReadData(ctx, image.Url)
					if err == nil {
						clog.V(common.VERBOSE).Infof(ctx, "replacing response with base64 for gateway on older api gateway_api=%v", r.Header.Get("Authorization"))
						data, _ := io.ReadAll(fileData.Body)
						imgResp.Images[i].Url = prefix + base64.StdEncoding.EncodeToString(data)
					} else {
						glog.Error(err)
					}
				}
			}
			//return the modified response
			resp = imgResp
		}
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
		case worker.TextToImageJSONRequestBody:
			latencyScore = CalculateTextToImageLatencyScore(took, v, outPixels)
		case worker.ImageToImageMultipartRequestBody:
			latencyScore = CalculateImageToImageLatencyScore(took, v, outPixels)
		case worker.ImageToVideoMultipartRequestBody:
			latencyScore = CalculateImageToVideoLatencyScore(took, v, outPixels)
		case worker.UpscaleMultipartRequestBody:
			latencyScore = CalculateUpscaleLatencyScore(took, v, outPixels)
		case worker.AudioToTextMultipartRequestBody:
			durationSeconds, err := common.CalculateAudioDuration(v.Audio)
			if err == nil {
				latencyScore = CalculateAudioToTextLatencyScore(took, durationSeconds)
			}
		case worker.SegmentAnything2MultipartRequestBody:
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

//
// Orchestrator receiving results from the remote AI worker
//

func (h *lphttp) AIResults() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		authType := r.Header.Get("Authorization")
		if protoVerAIWorker != authType {
			glog.Error("Invalid auth type ", authType)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		creds := r.Header.Get("Credentials")

		if creds != orch.TranscoderSecret() {
			glog.Error("Invalid shared secret")
			respondWithError(w, errSecret.Error(), http.StatusUnauthorized)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			glog.Error("Error getting mime type ", err)
			http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
			return
		}

		tid, err := strconv.ParseInt(r.Header.Get("TaskId"), 10, 64)
		if err != nil {
			glog.Error("Could not parse task ID ", err)
			http.Error(w, "Invalid Task ID", http.StatusBadRequest)
			return
		}

		var workerResult core.RemoteAIWorkerResult
		workerResult.Files = make(map[string][]byte)

		if aiWorkerErrorMimeType == mediaType {
			w.Write([]byte("OK"))
			body, err := io.ReadAll(r.Body)
			if err != nil {
				glog.Errorf("Unable to read ai worker error body taskId=%v err=%q", tid, err)
				workerResult.Err = err
			} else {
				workerResult.Err = fmt.Errorf(string(body))
			}
			glog.Errorf("AI Worker error for taskId=%v err=%q", tid, workerResult.Err)
			orch.AIResults(tid, &workerResult)
			return
		}

		if mediaType == "multipart/mixed" {
			start := time.Now()
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					glog.Error("Could not process multipart part ", err)
					workerResult.Err = err
					break
				}
				body, err := common.ReadAtMost(p, MaxAIRequestSize)
				if err != nil {
					glog.Error("Error reading body ", err)
					workerResult.Err = err
					break
				}

				//this is where we would include metadata on each result if want to separate
				//instead the multipart response includes the json and the files separately with the json "url" field matching to part names
				cDisp := p.Header.Get("Content-Disposition")
				if p.Header.Get("Content-Type") == "application/json" {
					var results worker.ImageResponse
					err := json.Unmarshal(body, &results)
					if err != nil {
						glog.Error("Error getting results json:", err)
						workerResult.Err = err
						break
					}

					workerResult.Results = &results
				} else if cDisp != "" {
					//these are the result files binary data
					resultName := p.FileName()
					workerResult.Files[resultName] = body
				}
			}

			dlDur := time.Since(start)
			glog.V(common.VERBOSE).Infof("Downloaded results from remote worker=%s taskId=%d dur=%s", r.RemoteAddr, tid, dlDur)
			workerResult.DownloadTime = dlDur

			orch.AIResults(tid, &workerResult)
		}

		if workerResult.Err != nil {
			http.Error(w, workerResult.Err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("OK"))
	})
}
