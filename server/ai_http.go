package server

import (
	"bufio"
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
	"strings"
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

var MaxAIRequestSize = 3000000000 // 3GB

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
	lp.transRPC.Handle("/llm", oapiReqValidator(lp.LLM()))
	lp.transRPC.Handle("/segment-anything-2", oapiReqValidator(lp.SegmentAnything2()))
	// Additionally, there is the '/aiResults' endpoint registered in server/rpc.go

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

func (h *lphttp) LLM() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator

		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		multiRdr, err := r.MultipartReader()
		if err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req worker.GenLLMFormdataRequestBody
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
	case worker.GenTextToImageJSONRequestBody:
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
	case worker.GenImageToImageMultipartRequestBody:
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
	case worker.GenUpscaleMultipartRequestBody:
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
	case worker.GenImageToVideoMultipartRequestBody:
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
	case worker.GenAudioToTextMultipartRequestBody:
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
	case worker.GenLLMFormdataRequestBody:
		pipeline = "llm"
		cap = core.Capability_LLM
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.LLM(ctx, requestID, v)
		}

		if v.MaxTokens == nil {
			respondWithError(w, "MaxTokens not specified", http.StatusBadRequest)
			return
		}

		// TODO: Improve pricing
		outPixels = int64(*v.MaxTokens)
	case worker.GenSegmentAnything2MultipartRequestBody:
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
	//Gateway version through v0.7.9-ai.3 expects to receive base64 encoded images as results for text-to-image, image-to-image, and upscale pipelines
	//The gateway now adds the protoVerAIWorker header to the request to indicate what version of the gateway is making the request
	//UPDATE this logic as the communication protocol between the gateway and orchestrator is updated
	if pipeline == "text-to-image" || pipeline == "image-to-image" || pipeline == "upscale" {
		if r.Header.Get("Authorization") != protoVerAIWorker {
			imgResp := resp.(worker.ImageResponse)
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

	// Check if the response is a streaming response
	if streamChan, ok := resp.(<-chan worker.LlmStreamChunk); ok {
		glog.Infof("Streaming response for request id=%v", requestID)

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		for chunk := range streamChan {
			data, err := json.Marshal(chunk)
			if err != nil {
				clog.Errorf(ctx, "Error marshaling stream chunk: %v", err)
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if chunk.Done {
				break
			}
		}
	} else {
		// Non-streaming response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}

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

		pipeline := r.Header.Get("Pipeline")

		var workerResult core.RemoteAIWorkerResult
		workerResult.Files = make(map[string][]byte)

		start := time.Now()
		dlDur := time.Duration(0) // default to 0 in case of early return
		resultType := ""
		switch mediaType {
		case aiWorkerErrorMimeType:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				glog.Errorf("Unable to read ai worker error body taskId=%v err=%q", tid, err)
				workerResult.Err = err
			} else {
				workerResult.Err = fmt.Errorf(string(body))
			}
			glog.Errorf("AI Worker error for taskId=%v err=%q", tid, workerResult.Err)
			orch.AIResults(tid, &workerResult)
			w.Write([]byte("OK"))
			return
		case "text/event-stream":
			resultType = "streaming"
			glog.Infof("Received %s response from remote worker=%s taskId=%d", resultType, r.RemoteAddr, tid)
			resChan := make(chan worker.LlmStreamChunk, 100)
			workerResult.Results = (<-chan worker.LlmStreamChunk)(resChan)

			defer r.Body.Close()
			defer close(resChan)
			//set a reasonable timeout to stop waiting for results
			ctx, _ := context.WithTimeout(r.Context(), HTTPIdleTimeout)

			//pass results and receive from channel as the results are streamed
			go orch.AIResults(tid, &workerResult)
			// Read the streamed results from the request body
			scanner := bufio.NewScanner(r.Body)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
					line := scanner.Text()
					if strings.HasPrefix(line, "data: ") {
						data := strings.TrimPrefix(line, "data: ")
						var chunk worker.LlmStreamChunk
						if err := json.Unmarshal([]byte(data), &chunk); err != nil {
							clog.Errorf(ctx, "Error unmarshaling stream data: %v", err)
							continue
						}
						resChan <- chunk
					}
				}
			}
			if err := scanner.Err(); err != nil {
				workerResult.Err = scanner.Err()
			}

			dlDur = time.Since(start)
		case "multipart/mixed":
			resultType = "uploaded"
			glog.Infof("Received %s response from remote worker=%s taskId=%d", resultType, r.RemoteAddr, tid)
			workerResult := parseMultiPartResult(r.Body, params["boundary"], pipeline)

			//return results
			dlDur = time.Since(start)
			workerResult.DownloadTime = dlDur
			orch.AIResults(tid, &workerResult)
		}

		glog.V(common.VERBOSE).Infof("Processed %s results from remote worker=%s taskId=%d dur=%s", resultType, r.RemoteAddr, tid, dlDur)

		if workerResult.Err != nil {
			http.Error(w, workerResult.Err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("OK"))
	})
}

func parseMultiPartResult(body io.Reader, boundary string, pipeline string) core.RemoteAIWorkerResult {
	wkrResult := core.RemoteAIWorkerResult{}
	wkrResult.Files = make(map[string][]byte)

	mr := multipart.NewReader(body, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			glog.Error("Could not process multipart part ", err)
			wkrResult.Err = err
			break
		}
		body, err := common.ReadAtMost(p, MaxAIRequestSize)
		if err != nil {
			glog.Error("Error reading body ", err)
			wkrResult.Err = err
			break
		}

		// this is where we would include metadata on each result if want to separate
		// instead the multipart response includes the json and the files separately with the json "url" field matching to part names
		cDisp := p.Header.Get("Content-Disposition")
		if p.Header.Get("Content-Type") == "application/json" {
			var results interface{}
			switch pipeline {
			case "text-to-image", "image-to-image", "upscale", "image-to-video":
				var parsedResp worker.ImageResponse

				err := json.Unmarshal(body, &parsedResp)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
				results = parsedResp
			case "audio-to-text", "segment-anything-2", "llm":
				err := json.Unmarshal(body, &results)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
			}

			wkrResult.Results = results
		} else if cDisp != "" {
			//these are the result files binary data
			resultName := p.FileName()
			wkrResult.Files[resultName] = body
		}
	}

	return wkrResult
}
