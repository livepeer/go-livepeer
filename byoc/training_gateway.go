package byoc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
)

// Gateway handler for training job submission -- proxies to orchestrator
func (bsg *BYOCGatewayServer) SubmitTrainingJob() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		gatewayJob, err := bsg.setupGatewayJob(ctx, r.Header.Get(jobRequestHdr), r.Header.Get(jobOrchSearchTimeoutHdr), r.Header.Get(jobOrchSearchRespTimeoutHdr), false)
		if err != nil {
			clog.Errorf(ctx, "Error setting up training job: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx = clog.AddVal(ctx, "job_id", gatewayJob.Job.Req.ID)
		ctx = clog.AddVal(ctx, "capability", gatewayJob.Job.Req.Capability)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		// Try each orchestrator
		for _, orchToken := range gatewayJob.Orchs {
			err := gatewayJob.sign()
			if err != nil {
				clog.Errorf(ctx, "Error signing training job: %v", err)
				return
			}

			orchUrl := orchToken.ServiceAddr + "/process/train/" + gatewayJob.Job.Req.Capability
			req, err := http.NewRequestWithContext(ctx, "POST", orchUrl, bytes.NewBuffer(body))
			if err != nil {
				clog.Errorf(ctx, "Unable to create training request: %v", err)
				continue
			}

			req.Header.Add("Content-Type", "application/json")
			req.Header.Add(jobRequestHdr, gatewayJob.SignedJobReq)
			if orchToken.Price.PricePerUnit > 0 {
				paymentHdr, err := bsg.createPayment(ctx, gatewayJob.Job.Req, &orchToken)
				if err != nil {
					clog.Errorf(ctx, "Unable to create payment: %v", err)
					continue
				}
				if paymentHdr != "" {
					req.Header.Add(jobPaymentHeaderHdr, paymentHdr)
				}
			}

			resp, err := sendJobReqWithTimeout(req, 30*time.Second)
			if err != nil {
				clog.Errorf(ctx, "Training job submit to orchestrator %v failed: %v", orchToken.ServiceAddr, err)
				continue
			}

			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				clog.Errorf(ctx, "Unable to read orchestrator response: %v", err)
				continue
			}

			if resp.StatusCode >= 400 {
				clog.Errorf(ctx, "Orchestrator %v rejected training job: %s", orchToken.ServiceAddr, string(data))
				if resp.StatusCode < 500 {
					http.Error(w, string(data), resp.StatusCode)
					return
				}
				continue
			}

			// Add orchestrator URL to response
			var respData map[string]interface{}
			if err := json.Unmarshal(data, &respData); err == nil {
				respData["orchestrator_url"] = orchToken.ServiceAddr
				// Rewrite status_url to go through gateway -> orch
				if statusURL, ok := respData["status_url"].(string); ok {
					respData["status_url"] = orchToken.ServiceAddr + statusURL
				}
				data, _ = json.Marshal(respData)
			}

			clog.V(common.SHORT).Infof(ctx, "Training job submitted to orchestrator %v", orchToken.ServiceAddr)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Orchestrator-Url", orchToken.ServiceAddr)
			w.WriteHeader(resp.StatusCode)
			w.Write(data)
			return
		}

		http.Error(w, "No orchestrator could accept the training job", http.StatusServiceUnavailable)
	})
}

// Gateway handler to proxy training job status from orchestrator
func (bsg *BYOCGatewayServer) GetTrainingJobStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// The orchestrator URL should be passed as query param or header
		orchURL := r.URL.Query().Get("orch_url")
		if orchURL == "" {
			orchURL = r.Header.Get("X-Orchestrator-Url")
		}
		if orchURL == "" {
			http.Error(w, "orch_url query parameter or X-Orchestrator-Url header required", http.StatusBadRequest)
			return
		}

		// Extract job ID from path
		path := strings.TrimPrefix(r.URL.Path, "/process/job/")
		jobID := strings.Split(path, "/")[0]

		statusURL := orchURL + "/process/job/" + jobID
		req, err := http.NewRequestWithContext(r.Context(), "GET", statusURL, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := sendReqWithTimeout(req, 10*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(data)
	})
}
