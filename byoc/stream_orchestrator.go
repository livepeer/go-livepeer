package byoc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/trickle"
)

var getNewTokenTimeout = 3 * time.Second

// StartStream handles the POST /stream/start endpoint for the Orchestrator
func (bso *BYOCOrchestratorServer) StartStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := bso.orch
		remoteAddr := getRemoteAddr(r)
		ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

		orchJob, err := bso.setupOrchJob(ctx, r, false)
		if err != nil {
			code := http.StatusBadRequest
			if err == errInsufficientBalance {
				code = http.StatusPaymentRequired
			}
			respondWithError(w, err.Error(), code)
			return
		}
		ctx = clog.AddVal(ctx, "stream_id", orchJob.Req.ID)

		workerRoute := orchJob.Req.CapabilityUrl + "/stream/start"

		// Read the original body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		var jobParams JobParameters
		err = json.Unmarshal([]byte(orchJob.Req.Parameters), &jobParams)
		if err != nil {
			clog.Errorf(ctx, "unable to parse parameters err=%v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clog.Infof(ctx, "Processing stream start request videoIngress=%v videoEgress=%v dataOutput=%v", jobParams.EnableVideoIngress, jobParams.EnableVideoEgress, jobParams.EnableDataOutput)
		// Start trickle server for live-video
		var (
			mid          = orchJob.Req.ID // Request ID is used for the manifest ID
			pubUrl       = orch.ServiceURI().JoinPath(bso.trickleBasePath, mid).String()
			subUrl       = pubUrl + "-out"
			controlUrl   = pubUrl + "-control"
			eventsUrl    = pubUrl + "-events"
			dataUrl      = pubUrl + "-data"
			pubCh        *trickle.TrickleLocalPublisher
			subCh        *trickle.TrickleLocalPublisher
			controlPubCh *trickle.TrickleLocalPublisher
			eventsCh     *trickle.TrickleLocalPublisher
			dataCh       *trickle.TrickleLocalPublisher
		)
		failedToStartStream := false

		// reset trickle channels and release capacity on failure
		defer func() {
			if failedToStartStream {
				bso.orch.FreeExternalCapabilityCapacity(orchJob.Req.Capability)
				//close the trickle channels
				if pubCh != nil {
					pubCh.Close()
				}
				if subCh != nil {
					subCh.Close()
				}
				if dataCh != nil {
					dataCh.Close()
				}
				controlPubCh.Close()
				eventsCh.Close()
			}
		}()

		reqBodyForRunner := make(map[string]interface{})
		reqBodyForRunner["gateway_request_id"] = mid
		//required channels
		controlPubCh = trickle.NewLocalPublisher(bso.trickleSrv, mid+"-control", "application/json")
		controlPubCh.CreateChannel()
		controlUrl = overwriteHost(bso.node.LiveAITrickleHostForRunner, controlUrl)
		reqBodyForRunner["control_url"] = controlUrl
		w.Header().Set("X-Control-Url", controlUrl)

		eventsCh = trickle.NewLocalPublisher(bso.trickleSrv, mid+"-events", "application/json")
		eventsCh.CreateChannel()
		eventsUrl = overwriteHost(bso.node.LiveAITrickleHostForRunner, eventsUrl)
		reqBodyForRunner["events_url"] = eventsUrl
		w.Header().Set("X-Events-Url", eventsUrl)

		//Optional channels
		if jobParams.EnableVideoIngress {
			pubCh = trickle.NewLocalPublisher(bso.trickleSrv, mid, "video/MP2T")
			pubCh.CreateChannel()
			pubUrl = overwriteHost(bso.node.LiveAITrickleHostForRunner, pubUrl)
			reqBodyForRunner["subscribe_url"] = pubUrl //runner needs to subscribe to input
			w.Header().Set("X-Publish-Url", pubUrl)    //gateway will connect to pubUrl to send ingress video
		}

		if jobParams.EnableVideoEgress {
			subCh = trickle.NewLocalPublisher(bso.trickleSrv, mid+"-out", "video/MP2T")
			subCh.CreateChannel()
			subUrl = overwriteHost(bso.node.LiveAITrickleHostForRunner, subUrl)
			reqBodyForRunner["publish_url"] = subUrl  //runner needs to send results -out
			w.Header().Set("X-Subscribe-Url", subUrl) //gateway will connect to subUrl to receive results
		}

		if jobParams.EnableDataOutput {
			dataCh = trickle.NewLocalPublisher(bso.trickleSrv, mid+"-data", "application/jsonl")
			dataCh.CreateChannel()
			dataUrl = overwriteHost(bso.node.LiveAITrickleHostForRunner, dataUrl)
			reqBodyForRunner["data_url"] = dataUrl
			w.Header().Set("X-Data-Url", dataUrl)
		}
		//parse the request body json to add to the request to the runner
		var bodyJSON map[string]interface{}
		if err := json.Unmarshal(body, &bodyJSON); err != nil {
			clog.Errorf(ctx, "Failed to parse body as JSON: %v", err)
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			failedToStartStream = true
			return
		}
		for key, value := range bodyJSON {
			reqBodyForRunner[key] = value
		}

		reqBodyBytes, err := json.Marshal(reqBodyForRunner)
		if err != nil {
			clog.Errorf(ctx, "Failed to marshal request body err=%v", err)
			http.Error(w, "Failed to marshal request body", http.StatusInternalServerError)
			failedToStartStream = true
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(reqBodyBytes))
		// set the headers
		req.Header.Add("Content-Length", r.Header.Get("Content-Length"))
		req.Header.Add("Content-Type", r.Header.Get("Content-Type"))
		// use for routing to worker if reverse proxy in front of workers
		req.Header.Add("X-Stream-Id", orchJob.Req.ID)

		start := time.Now()
		resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
		if err != nil {
			clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
			respondWithError(w, "Error sending request to worker", http.StatusInternalServerError)
			failedToStartStream = true
			return
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Error reading response body: %v", err)
			respondWithError(w, "Error reading response body", http.StatusInternalServerError)
			failedToStartStream = true
			return
		}
		defer resp.Body.Close()

		//error response from worker but assume can retry and pass along error response and status code
		if resp.StatusCode > 399 {
			clog.Errorf(ctx, "error processing stream start request statusCode=%d", resp.StatusCode)

			bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
			//return error response from the worker
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
			failedToStartStream = true
			return
		}

		bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
		w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))

		clog.V(common.SHORT).Infof(ctx, "stream start processed successfully took=%v balance=%v", time.Since(start), bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))

		//setup the stream
		stream, err := bso.node.ExternalCapabilities.AddStream(orchJob.Req.ID, orchJob.Req.Capability, reqBodyBytes)
		if err != nil {
			clog.Errorf(ctx, "Error adding stream to external capabilities: %v", err)
			respondWithError(w, "Error adding stream to external capabilities", http.StatusInternalServerError)
			failedToStartStream = true
			return
		}

		stream.SetChannels(pubCh, subCh, controlPubCh, eventsCh, dataCh)

		//start stream monitoring
		go bso.monitorOrchStream(orchJob)

		//send back the trickle urls set in header
		w.WriteHeader(http.StatusOK)
	})
}

func (bso *BYOCOrchestratorServer) monitorOrchStream(job *orchJob) {
	if job == nil {
		return
	}

	ctx := context.Background()
	streamID := job.Req.ID
	capability := job.Req.Capability
	sender := job.Req.Sender

	stream, exists := bso.node.ExternalCapabilities.GetStream(streamID)
	if !exists {
		clog.Infof(ctx, "Stream not found for payment monitoring, exiting monitoring stream_id=%s", streamID)
		return
	}

	ctx = clog.AddVal(ctx, "stream_id", streamID)
	ctx = clog.AddVal(ctx, "capability", capability)

	pmtCheckDur := 23 * time.Second //run slightly faster than gateway so can return updated balance
	pmtTicker := time.NewTicker(pmtCheckDur)
	senderAddr := ethcommon.HexToAddress(sender)
	defer pmtTicker.Stop()
	shouldStopStreamNextRound := false
	for {
		select {
		case <-stream.StreamCtx.Done():
			bso.orch.FreeExternalCapabilityCapacity(capability)
			clog.Infof(ctx, "Stream ended, stopping payment monitoring and released capacity")
			return
		case <-pmtTicker.C:
			// Check payment status
			extCap, ok := bso.node.ExternalCapabilities.Capabilities[capability]
			if !ok {
				clog.Errorf(ctx, "Capability not found for payment monitoring, exiting monitoring capability=%s", capability)
				return
			}
			jobPriceRat := big.NewRat(job.JobPrice.PricePerUnit, job.JobPrice.PixelsPerUnit)
			if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
				//lock during balance update to complete balance update
				extCap.Mu.Lock()
				bso.orch.DebitFees(senderAddr, core.ManifestID(capability), job.JobPrice, int64(pmtCheckDur.Seconds()))
				senderBalance := bso.getPaymentBalance(senderAddr, capability)
				extCap.Mu.Unlock()
				if senderBalance != nil {
					if senderBalance.Cmp(big.NewRat(0, 1)) < 0 {
						if !shouldStopStreamNextRound {
							//warn once
							clog.Warningf(ctx, "Insufficient balance for stream capability, will stop stream next round if not replenished sender=%s capability=%s balance=%s", sender, capability, senderBalance.FloatString(0))
							shouldStopStreamNextRound = true
							continue
						}

						clog.Infof(ctx, "Insufficient balance, stopping stream %s for sender %s", streamID, sender)
						_, exists := bso.node.ExternalCapabilities.GetStream(streamID)
						if exists {
							bso.node.ExternalCapabilities.RemoveStream(streamID)
						}

						return
					}

					clog.V(8).Infof(ctx, "Payment balance for stream capability is good balance=%v", senderBalance.FloatString(0))
				}
			}

			//check if stream still exists
			// if not, send stop to worker and exit monitoring
			stream, exists := bso.node.ExternalCapabilities.GetStream(streamID)
			if !exists {
				req, err := http.NewRequestWithContext(ctx, "POST", job.Req.CapabilityUrl+"/stream/stop", nil)
				// set the headers
				resp, err := sendReqWithTimeout(req, time.Duration(job.Req.Timeout)*time.Second)
				if err != nil {
					clog.Errorf(ctx, "Error sending request to worker %v: %v", job.Req.CapabilityUrl, err)
					return
				}
				defer resp.Body.Close()
				io.Copy(io.Discard, resp.Body)

				//end monitoring of stream
				return
			}

			//check if control channel is still open, end if not
			if !stream.IsActive() {
				// Stop the stream and free capacity
				bso.node.ExternalCapabilities.RemoveStream(streamID)
				return
			}
		}
	}
}

func (bso *BYOCOrchestratorServer) StopStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orchJob, err := bso.setupOrchJob(ctx, r, false)
		if err != nil {
			respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid err=%v", err), http.StatusBadRequest)
			return
		}

		var jobDetails JobRequestDetails
		err = json.Unmarshal([]byte(orchJob.Req.Request), &jobDetails)
		if err != nil {
			respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid, failed to parse stream id err=%v", err), http.StatusBadRequest)
			return
		}
		clog.Infof(ctx, "Stopping stream %s", jobDetails.StreamId)

		// Read the original body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		workerRoute := orchJob.Req.CapabilityUrl + "/stream/stop"
		req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(body))
		if err != nil {
			clog.Errorf(ctx, "failed to create /stream/stop request to worker err=%v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// use for routing to worker if reverse proxy in front of workers
		req.Header.Add("X-Stream-Id", jobDetails.StreamId)
		resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
		if err != nil {
			clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
		}

		var respBody []byte
		respStatusCode := http.StatusOK // default to 200, if not nill will be overwritten
		if resp != nil {
			respBody, err = io.ReadAll(resp.Body)
			if err != nil {
				clog.Errorf(ctx, "Error reading response body: %v", err)
			}
			defer resp.Body.Close()

			respStatusCode = resp.StatusCode
			if resp.StatusCode > 399 {
				clog.Errorf(ctx, "error processing stream stop request statusCode=%d", resp.StatusCode)
			}
		}

		// Stop the stream and free capacity
		bso.node.ExternalCapabilities.RemoveStream(jobDetails.StreamId)

		w.WriteHeader(respStatusCode)
		w.Write(respBody)
	})
}

func (bso *BYOCOrchestratorServer) UpdateStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orchJob, err := bso.setupOrchJob(ctx, r, false)
		if err != nil {
			respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid err=%v", err), http.StatusBadRequest)
			return
		}

		var jobDetails JobRequestDetails
		err = json.Unmarshal([]byte(orchJob.Req.Request), &jobDetails)
		if err != nil {
			respondWithError(w, fmt.Sprintf("Failed to stop stream, request not valid, failed to parse stream id err=%v", err), http.StatusBadRequest)
			return
		}
		clog.Infof(ctx, "Stopping stream %s", jobDetails.StreamId)

		// Read the original body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		workerRoute := orchJob.Req.CapabilityUrl + "/stream/params"
		req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(body))
		if err != nil {
			clog.Errorf(ctx, "failed to create /stream/params request to worker err=%v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.Header.Add("Content-Type", "application/json")
		// use for routing to worker if reverse proxy in front of workers
		req.Header.Add("X-Stream-Id", jobDetails.StreamId)

		resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
		if err != nil {
			clog.Errorf(ctx, "Error sending request to worker %v: %v", workerRoute, err)
			respondWithError(w, "Error sending request to worker", http.StatusInternalServerError)
			return
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Error reading response body: %v", err)
			respondWithError(w, "Error reading response body", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode > 399 {
			clog.Errorf(ctx, "error processing stream update request statusCode=%d", resp.StatusCode)
		}

		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	})
}

func (bso *BYOCOrchestratorServer) ProcessStreamPayment() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orch := bso.orch
		ctx := r.Context()

		//this will validate the request and process the payment
		orchJob, err := bso.setupOrchJob(ctx, r, false)
		if err != nil {
			respondWithError(w, fmt.Sprintf("Failed to process payment, request not valid err=%v", err), http.StatusBadRequest)
			return
		}
		ctx = clog.AddVal(ctx, "stream_id", orchJob.Details.StreamId)
		ctx = clog.AddVal(ctx, "capability", orchJob.Req.Capability)
		ctx = clog.AddVal(ctx, "sender", orchJob.Req.Sender)

		senderAddr := ethcommon.HexToAddress(orchJob.Req.Sender)

		capBal := orch.Balance(senderAddr, core.ManifestID(orchJob.Req.Capability))
		if capBal != nil {
			capBal, err = common.PriceToInt64(capBal)
			if err != nil {
				clog.Errorf(ctx, "could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), orchJob.Req.Capability, err.Error())
				capBal = big.NewRat(0, 1)
			}
		} else {
			capBal = big.NewRat(0, 1)
		}

		w.Header().Set(jobPaymentBalanceHdr, capBal.FloatString(0))
		w.WriteHeader(http.StatusOK)
	})
}
