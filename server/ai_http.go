package server

// ai_http.go implements the HTTP server for AI-related requests at the Orchestrator.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	url2 "net/url"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
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

	lp.transRPC.Handle("/live-video-to-video", oapiReqValidator(lp.StartLiveVideoToVideo()))
	// Additionally, there is the '/aiResults' endpoint registered in server/rpc.go

	return nil
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

		// For every segment, check payments
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
				if paymentProcessor != nil {
					paymentProcessor.process(ctx)
				}
				// read the segment so we know when it is complete, otherwise sub.Read()
				// would rapidly request follow-on segments that do not yet exist
				io.Copy(io.Discard, segment.Reader)
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
