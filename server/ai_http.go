package server

// ai_http.go implements the HTTP server for AI-related requests at the Orchestrator.

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	url2 "net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

var MaxAIRequestSize = 3000000000 // 3GB

var TrickleHTTPPath = "/ai/trickle/"

func startAIServer(lp *lphttp) error {
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

	lp.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
		Mux:      lp.transRPC,
		BasePath: TrickleHTTPPath,
	})

	lp.transRPC.Handle("/text-to-image", oapiReqValidator(aiHttpHandle(lp, jsonDecoder[worker.GenTextToImageJSONRequestBody])))
	lp.transRPC.Handle("/image-to-image", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenImageToImageMultipartRequestBody])))
	lp.transRPC.Handle("/image-to-video", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenImageToVideoMultipartRequestBody])))
	lp.transRPC.Handle("/upscale", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenUpscaleMultipartRequestBody])))
	lp.transRPC.Handle("/audio-to-text", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenAudioToTextMultipartRequestBody])))
	lp.transRPC.Handle("/llm", oapiReqValidator(aiHttpHandle(lp, jsonDecoder[worker.GenLLMJSONRequestBody])))
	lp.transRPC.Handle("/segment-anything-2", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenSegmentAnything2MultipartRequestBody])))
	lp.transRPC.Handle("/image-to-text", oapiReqValidator(aiHttpHandle(lp, multipartDecoder[worker.GenImageToTextMultipartRequestBody])))
	lp.transRPC.Handle("/text-to-speech", oapiReqValidator(aiHttpHandle(lp, jsonDecoder[worker.GenTextToSpeechJSONRequestBody])))
	lp.transRPC.Handle("/live-video-to-video", oapiReqValidator(lp.StartLiveVideoToVideo()))
	// Additionally, there is the '/aiResults' endpoint registered in server/rpc.go

	return nil
}

// aiHttpHandle handles AI requests by decoding the request body and processing it.
func aiHttpHandle[I any](h *lphttp, decoderFunc func(*I, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := h.orchestrator
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		var req I
		if err := decoderFunc(&req, r); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		handleAIRequest(ctx, w, r, orch, req)
	})
}

func (h *lphttp) StartLiveVideoToVideo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(context.Background(), clog.ClientIP, remoteAddr)

		streamID := r.Header.Get("streamID")
		gatewayRequestID := r.Header.Get("requestID")
		requestID := string(core.RandomManifestID())

		var req worker.GenLiveVideoToVideoJSONRequestBody
		if err := jsonDecoder(&req, r); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.GatewayRequestId != nil && *req.GatewayRequestId != "" {
			gatewayRequestID = *req.GatewayRequestId
		}
		if req.StreamId != nil && *req.StreamId != "" {
			streamID = *req.StreamId
		}
		ctx = clog.AddVal(ctx, "request_id", gatewayRequestID)
		ctx = clog.AddVal(ctx, "manifest_id", requestID)
		ctx = clog.AddVal(ctx, "stream_id", streamID)

		orch := h.orchestrator
		pipeline := "live-video-to-video"
		cap := core.Capability_LiveVideoToVideo
		modelID := *req.ModelId
		clog.Info(ctx, "Received request", "cap", cap, "modelID", modelID)

		// Create storage for the request (for AI Workers, must run before CheckAICapacity)
		err := orch.CreateStorageForRequest(requestID)
		if err != nil {
			respondWithError(w, "Could not create storage to receive results", http.StatusInternalServerError)
		}

		// Check if there is capacity for the request
		hasCapacity, _ := orch.CheckAICapacity(pipeline, modelID)
		if !hasCapacity {
			clog.Errorf(ctx, "Insufficient capacity for pipeline=%v modelID=%v", pipeline, modelID)
			respondWithError(w, "insufficient capacity", http.StatusServiceUnavailable)
			return
		}

		// Start trickle server for live-video
		var (
			mid        = requestID // Request ID is used for the manifest ID
			pubUrl     = orch.ServiceURI().JoinPath(TrickleHTTPPath, mid).String()
			subUrl     = pubUrl + "-out"
			controlUrl = pubUrl + "-control"
			eventsUrl  = pubUrl + "-events"
		)

		// Handle initial payment, the rest of the payments are done separately from the stream processing
		// Note that this payment is debit from the balance and acts as a buffer for the AI Realtime Video processing
		payment, err := getPayment(r.Header.Get(paymentHeader))
		if err != nil {
			respondWithError(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		sender := getPaymentSender(payment)
		_, ctx, err = verifySegCreds(ctx, h.orchestrator, r.Header.Get(segmentHeader), sender)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusForbidden)
			return
		}
		if err := orch.ProcessPayment(ctx, payment, core.ManifestID(mid)); err != nil {
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, core.ManifestID(mid)) {
			respondWithError(w, "Insufficient balance", http.StatusBadRequest)
			return
		}

		//If successful, then create the trickle channels
		// Precreate the channels to avoid race conditions
		// TODO get the expected mime type from the request
		pubCh := trickle.NewLocalPublisher(h.trickleSrv, mid, "video/MP2T")
		pubCh.CreateChannel()
		subCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-out", "video/MP2T")
		subCh.CreateChannel()
		controlPubCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-control", "application/json")
		controlPubCh.CreateChannel()
		eventsCh := trickle.NewLocalPublisher(h.trickleSrv, mid+"-events", "application/json")
		eventsCh.CreateChannel()

		ctx, cancel := context.WithCancel(ctx)
		closeSession := func() {
			pubCh.Close()
			subCh.Close()
			eventsCh.Close()
			controlPubCh.Close()
			cancel()
		}

		// Start payment receiver which accounts the payments and stops the stream if the payment is insufficient
		priceInfo := payment.GetExpectedPrice()
		var paymentProcessor *LivePaymentProcessor
		if priceInfo != nil && priceInfo.PricePerUnit != 0 {
			paymentReceiver := livePaymentReceiver{orchestrator: h.orchestrator}
			accountPaymentFunc := func(inPixels int64) error {
				err := paymentReceiver.AccountPayment(ctx, &SegmentInfoReceiver{
					sender:    sender,
					inPixels:  inPixels,
					priceInfo: priceInfo,
					sessionID: mid,
				})
				if err != nil {
					clog.Errorf(ctx, "Error accounting payment, stopping stream processing", err)
					closeSession()
				}
				return err
			}
			paymentProcessor = NewLivePaymentProcessor(ctx, h.node.LivePaymentInterval, accountPaymentFunc)
		} else {
			clog.Warningf(ctx, "No price info found for model %v, Orchestrator will not charge for video processing", modelID)
		}

		// Subscribe to the publishUrl for payments monitoring and payment processing
		go func() {
			sub := trickle.NewLocalSubscriber(h.trickleSrv, mid)
			for {
				// Set seq to next segment in case the subscriber is outside
				// the server's retention window
				sub.SetSeq(-1)
				segment, err := sub.Read()
				if err != nil {
					clog.Infof(ctx, "Error getting local trickle segment err=%v", err)
					closeSession()
					return
				}
				reader := segment.Reader
				if paymentProcessor != nil {
					reader = paymentProcessor.process(ctx, segment.Reader)
				}
				io.Copy(io.Discard, reader)
			}
		}()

		// Prepare request to worker
		controlUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, controlUrl)
		eventsUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, eventsUrl)
		subscribeUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, pubUrl)
		publishUrlOverwrite := overwriteHost(h.node.LiveAITrickleHostForRunner, subUrl)

		workerReq := worker.LiveVideoToVideoParams{
			ModelId:          req.ModelId,
			PublishUrl:       publishUrlOverwrite,
			SubscribeUrl:     subscribeUrlOverwrite,
			EventsUrl:        &eventsUrlOverwrite,
			ControlUrl:       &controlUrlOverwrite,
			Params:           req.Params,
			GatewayRequestId: &gatewayRequestID,
			ManifestId:       &mid,
			StreamId:         &streamID,
		}

		// Send request to the worker
		_, err = orch.LiveVideoToVideo(ctx, requestID, workerReq)
		if err != nil {
			if monitor.Enabled {
				monitor.AIProcessingError(err.Error(), pipeline, modelID, ethcommon.Address{}.String())
			}

			closeSession()
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare the response
		jsonData, err := json.Marshal(&worker.LiveVideoToVideoResponse{
			PublishUrl:   pubUrl,
			SubscribeUrl: subUrl,
			ControlUrl:   &controlUrl,
			EventsUrl:    &eventsUrl,
			RequestId:    &requestID,
			ManifestId:   &mid,
		})
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			closeSession()
			return
		}

		took := time.Since(startTime)
		clog.Info(ctx, "Processed request", "cap", cap, "modelID", modelID, "took", took)
		respondJsonOk(w, jsonData)
	})
}

// overwriteHost is used to overwrite the trickle host, because it may be different for runner
// runner may run inside Docker container, in a different network, or even on a different machine
func overwriteHost(hostOverwrite, url string) string {
	if hostOverwrite == "" {
		return url
	}
	u, err := url2.ParseRequestURI(url)
	if err != nil {
		slog.Warn("Couldn't parse url to overwrite for worker, using original url", "url", url, "err", err)
		return url
	}
	u.Host = hostOverwrite
	return u.String()
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
	case worker.GenLLMJSONRequestBody:
		pipeline = "llm"
		cap = core.Capability_LLM
		modelID = *v.Model
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
	case worker.GenImageToTextMultipartRequestBody:
		pipeline = "image-to-text"
		cap = core.Capability_ImageToText
		modelID = *v.ModelId
		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.ImageToText(ctx, requestID, v)
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
	case worker.GenTextToSpeechJSONRequestBody:
		pipeline = "text-to-speech"
		cap = core.Capability_TextToSpeech
		modelID = *v.ModelId

		submitFn = func(ctx context.Context) (interface{}, error) {
			return orch.TextToSpeech(ctx, requestID, v)
		}

		// TTS pricing is typically in characters, including punctuation.
		words := utf8.RuneCountInString(*v.Text)
		outPixels = int64(1000 * words)
	default:
		respondWithError(w, "Unknown request type", http.StatusBadRequest)
		return
	}

	clog.V(common.VERBOSE).Infof(ctx, "Received request id=%v cap=%v modelID=%v", requestID, cap, modelID)

	manifestID := core.ManifestID(strconv.Itoa(int(cap)) + "_" + modelID)

	// Check if there is capacity for the request.
	// Capability capacity is reserved if available and released when response is received
	hasCapacity, releaseCapacity := orch.CheckAICapacity(pipeline, modelID)
	if !hasCapacity {
		clog.Errorf(ctx, "Insufficient capacity for pipeline=%v modelID=%v", pipeline, modelID)
		respondWithError(w, "insufficient capacity", http.StatusServiceUnavailable)
		return
	}
	// Known limitation:
	// This call will set a fixed price for all requests in a session identified by a manifestID.
	// Since all requests for a capability + modelID are treated as "session" with a single manifestID, all
	// requests for a capability + modelID will get the same fixed price for as long as the orch is running
	if err := orch.ProcessPayment(ctx, payment, manifestID); err != nil {
		respondWithError(w, err.Error(), http.StatusBadRequest)
		releaseCapacity <- true
		return
	}

	if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, manifestID) {
		respondWithError(w, "Insufficient balance", http.StatusBadRequest)
		//release capacity, payment issue is not orchestrator's fault
		releaseCapacity <- true
		return
	}

	err = orch.CreateStorageForRequest(requestID)
	if err != nil {
		respondWithError(w, "Could not create storage to receive results", http.StatusInternalServerError)
		//do not release capacity, this is an unrecoverable error
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
		releaseCapacity <- true
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
		case worker.GenImageToTextMultipartRequestBody:
			latencyScore = CalculateImageToTextLatencyScore(took, outPixels)
		case worker.GenTextToSpeechJSONRequestBody:
			latencyScore = CalculateTextToSpeechLatencyScore(took, outPixels)
		}

		var pricePerAIUnit float64
		if priceInfo := payment.GetExpectedPrice(); priceInfo != nil && priceInfo.GetPixelsPerUnit() != 0 {
			pricePerAIUnit = float64(priceInfo.GetPricePerUnit()) / float64(priceInfo.GetPixelsPerUnit())
		}

		monitor.AIJobProcessed(ctx, pipeline, modelID, monitor.AIJobInfo{LatencyScore: latencyScore, PricePerUnit: pricePerAIUnit})
	}

	// Check if the response is a streaming response
	if streamChan, ok := resp.(<-chan *worker.LLMResponse); ok {
		glog.Infof("Streaming response for request id=%v", requestID)

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			releaseCapacity <- true
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

			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
				break
			}
		}
		//release capacity after streaming is done
		releaseCapacity <- true

	} else {
		// Non-streaming response
		releaseCapacity <- true
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
				workerResult.Err = errors.New(string(body))
			}
			glog.Errorf("AI Worker error for taskId=%v err=%q", tid, workerResult.Err)
			orch.AIResults(tid, &workerResult)
			w.Write([]byte("OK"))
			return
		case "text/event-stream":
			resultType = "streaming"
			glog.Infof("Received %s response from remote worker=%s taskId=%d", resultType, r.RemoteAddr, tid)
			resChan := make(chan *worker.LLMResponse, 100)
			workerResult.Results = (<-chan *worker.LLMResponse)(resChan)

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
						var chunk worker.LLMResponse
						if err := json.Unmarshal([]byte(data), &chunk); err != nil {
							clog.Errorf(ctx, "Error unmarshaling stream data: %v", err)
							continue
						}
						resChan <- &chunk
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
			case "audio-to-text", "segment-anything-2", "llm", "image-to-text":
				err := json.Unmarshal(body, &results)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
			case "text-to-speech":
				var parsedResp worker.AudioResponse
				err := json.Unmarshal(body, &parsedResp)
				if err != nil {
					glog.Error("Error getting results json:", err)
					wkrResult.Err = err
					break
				}
				results = parsedResp
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
