package byoc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"

	"github.com/golang/glog"
)

// Training job status constants
const (
	TrainingStatusSubmitted = "submitted"
	TrainingStatusRunning   = "running"
	TrainingStatusCompleted = "completed"
	TrainingStatusFailed    = "failed"
	TrainingStatusCancelled = "cancelled"
)

// TrainingJob represents an async training job tracked by the orchestrator
type TrainingJob struct {
	JobID      string                 `json:"job_id"`
	Capability string                 `json:"capability"`
	ModelID    string                 `json:"model_id"`
	Status     string                 `json:"status"`
	Progress   int                    `json:"progress"`
	Result     map[string]interface{} `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`

	// Metering: cost accrued and current balance (wei)
	Cost    string `json:"cost"`    // total cost charged so far
	Balance string `json:"balance"` // remaining sender balance

	// Internal: adapter-side job ID for polling
	AdapterJobID string `json:"adapter_job_id,omitempty"`
	AdapterURL   string `json:"-"`

	// Internal: payment fields (not serialized)
	sender   ethcommon.Address `json:"-"`
	jobPrice *net.PriceInfo    `json:"-"`
}

// TrainingJobStore is an in-memory store for training jobs with TTL cleanup
type TrainingJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*TrainingJob
	ttl  time.Duration // how long to keep completed/failed jobs
}

func NewTrainingJobStore(ttl time.Duration) *TrainingJobStore {
	s := &TrainingJobStore{
		jobs: make(map[string]*TrainingJob),
		ttl:  ttl,
	}
	// Start cleanup goroutine
	go s.cleanupLoop()
	return s
}

func (s *TrainingJobStore) Store(job *TrainingJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = job
}

func (s *TrainingJobStore) Get(jobID string) (*TrainingJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	return job, ok
}

func (s *TrainingJobStore) Update(jobID string, status string, progress int, result map[string]interface{}, errMsg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return false
	}
	job.Status = status
	job.Progress = progress
	if result != nil {
		job.Result = result
	}
	if errMsg != "" {
		job.Error = errMsg
	}
	job.UpdatedAt = time.Now()
	return true
}

func (s *TrainingJobStore) List(statusFilter string) []*TrainingJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var jobs []*TrainingJob
	for _, job := range s.jobs {
		if statusFilter != "" && job.Status != statusFilter {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs
}

func (s *TrainingJobStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, job := range s.jobs {
			if job.Status == TrainingStatusCompleted || job.Status == TrainingStatusFailed || job.Status == TrainingStatusCancelled {
				if now.Sub(job.UpdatedAt) > s.ttl {
					delete(s.jobs, id)
				}
			}
		}
		s.mu.Unlock()
	}
}

// TrainingRequest is the JSON body for POST /process/train/{capability}
type TrainingRequest struct {
	ModelID     string                 `json:"model_id"`
	CallbackURL string                `json:"callback_url,omitempty"`
	Params      map[string]interface{} `json:"params"` // training hyperparameters
}

// --- Orchestrator training handlers ---

func (bso *BYOCOrchestratorServer) SubmitTrainingJob() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		remoteAddr := getRemoteAddr(r)
		ctx = clog.AddVal(ctx, "client_ip", remoteAddr)

		// Verify job credentials and reserve capacity
		orchJob, err := bso.setupOrchJob(ctx, r, true)
		if err != nil {
			if err == errNoCapabilityCapacity {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}

		// Read training request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			bso.orch.FreeExternalCapabilityCapacity(orchJob.Req.Capability)
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		var trainReq TrainingRequest
		if err := json.Unmarshal(body, &trainReq); err != nil {
			bso.orch.FreeExternalCapabilityCapacity(orchJob.Req.Capability)
			http.Error(w, fmt.Sprintf("Invalid training request: %v", err), http.StatusBadRequest)
			return
		}

		if trainReq.ModelID == "" {
			bso.orch.FreeExternalCapabilityCapacity(orchJob.Req.Capability)
			http.Error(w, "model_id is required", http.StatusBadRequest)
			return
		}

		// Create training job with payment info for metering
		jobID := string(core.RandomManifestID())
		job := &TrainingJob{
			JobID:      jobID,
			Capability: orchJob.Req.Capability,
			ModelID:    trainReq.ModelID,
			Status:     TrainingStatusSubmitted,
			AdapterURL: orchJob.Req.CapabilityUrl,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Cost:       "0",
			Balance:    "0",
			sender:     orchJob.Sender,
			jobPrice:   orchJob.JobPrice,
		}
		bso.trainingStore.Store(job)

		ctx = clog.AddVal(ctx, "training_job_id", jobID)
		ctx = clog.AddVal(ctx, "capability", orchJob.Req.Capability)
		clog.V(common.SHORT).Infof(ctx, "Training job submitted model_id=%v", trainReq.ModelID)

		// Submit to adapter asynchronously. Detach the goroutine's context
		// from the inbound HTTP request — that context is cancelled the
		// moment the 202 response is written, which would immediately kill
		// the outbound POST to /train (manifests as
		// `Post "...": context canceled`). Preserve the clog values so
		// `training_job_id` / `capability` still appear in async logs.
		jobCtx := context.WithoutCancel(ctx)
		go bso.runTrainingJob(jobCtx, job, orchJob, body)

		// Return 202 Accepted immediately
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":     jobID,
			"status":     TrainingStatusSubmitted,
			"status_url": fmt.Sprintf("/process/job/%s", jobID),
			"cancel_url": fmt.Sprintf("/process/job/%s/cancel", jobID),
		})
	})
}

func (bso *BYOCOrchestratorServer) GetTrainingJobStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract job_id from path: /process/job/{job_id}
		prefix := "/process/job/"
		jobID := strings.TrimPrefix(r.URL.Path, prefix)
		// Remove trailing /cancel or /status if present
		jobID = strings.TrimSuffix(jobID, "/")

		job, ok := bso.trainingStore.Get(jobID)
		if !ok {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	})
}

func (bso *BYOCOrchestratorServer) CancelTrainingJob() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract job_id from path: /process/job/{job_id}/cancel
		path := strings.TrimPrefix(r.URL.Path, "/process/job/")
		jobID := strings.TrimSuffix(path, "/cancel")

		job, ok := bso.trainingStore.Get(jobID)
		if !ok {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		if job.Status == TrainingStatusCompleted || job.Status == TrainingStatusFailed || job.Status == TrainingStatusCancelled {
			http.Error(w, fmt.Sprintf("Job already %s", job.Status), http.StatusBadRequest)
			return
		}

		bso.trainingStore.Update(jobID, TrainingStatusCancelled, job.Progress, nil, "Cancelled by user")
		bso.orch.FreeExternalCapabilityCapacity(job.Capability)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"job_id": jobID,
			"status": TrainingStatusCancelled,
		})
	})
}

func (bso *BYOCOrchestratorServer) ListTrainingJobs() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		statusFilter := r.URL.Query().Get("status")
		jobs := bso.trainingStore.List(statusFilter)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jobs":  jobs,
			"total": len(jobs),
		})
	})
}

// RefreshTrainingPayment accepts a fresh Livepeer-Payment header and
// credits the user's deposit ledger for an in-flight training job
// (PR-5 of byoc-payment-fleet-2026-05, design §3.A refresh-on-watermark).
//
// Unlike SubmitTrainingJob, this handler does NOT re-run verifyJobCreds —
// the job is already authenticated by its initial submit handshake. The
// orch trusts the (job_id, sender) binding established at submit time.
// Idempotency on duplicate refresh is enforced by the underlying
// ProcessPayment, which already deduplicates ticket nonces.
//
// Invariants enforced here:
//   I5 (no double-charge on retry): ProcessPayment is nonce-keyed at the
//     PM layer; identical headers credit once.
//   I6 (sender attribution): the refresh's sender is read from the
//     payment header and compared to job.sender — mismatch → 403.
func (bso *BYOCOrchestratorServer) RefreshTrainingPayment() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract job_id from path: /process/job/{jobId}/refresh-payment
		path := strings.TrimPrefix(r.URL.Path, "/process/job/")
		jobID := strings.TrimSuffix(path, "/refresh-payment")

		// Job must exist + not be in a terminal state
		job, ok := bso.trainingStore.Get(jobID)
		if !ok {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		if job.Status == TrainingStatusCompleted ||
			job.Status == TrainingStatusFailed ||
			job.Status == TrainingStatusCancelled {
			http.Error(w,
				fmt.Sprintf("Job is %s, cannot refresh payment", job.Status),
				http.StatusBadRequest)
			return
		}

		// Read + validate the payment header
		paymentHdr := r.Header.Get("Livepeer-Payment")
		if paymentHdr == "" {
			http.Error(w, "Livepeer-Payment header required", http.StatusBadRequest)
			return
		}

		payment, err := getPayment(paymentHdr)
		if err != nil {
			clog.Errorf(ctx, "Refresh %s: invalid payment header: %v", jobID, err)
			http.Error(w, "Invalid Livepeer-Payment header", http.StatusBadRequest)
			return
		}

		// I6: enforce sender match with original submit
		paymentSender := ethcommon.BytesToAddress(payment.Sender)
		if paymentSender != job.sender {
			clog.Errorf(ctx,
				"Refresh %s: sender mismatch (job=%s payment=%s)",
				jobID, job.sender.Hex(), paymentSender.Hex())
			http.Error(w,
				fmt.Sprintf("Refresh sender %s does not match submit sender %s",
					paymentSender.Hex(), job.sender.Hex()),
				http.StatusForbidden)
			return
		}

		// Process the payment (credits the deposit ledger). Idempotency on
		// duplicate nonces is enforced by the PM layer.
		if err := bso.orch.ProcessPayment(ctx, payment, core.ManifestID(job.Capability)); err != nil {
			clog.Errorf(ctx, "Refresh %s: ProcessPayment failed: %v", jobID, err)
			http.Error(w, fmt.Sprintf("Payment processing failed: %v", err),
				http.StatusBadRequest)
			return
		}

		// Update job balance after credit
		newBal := bso.getPaymentBalance(job.sender, job.Capability)
		bso.trainingStore.mu.Lock()
		if j, ok := bso.trainingStore.jobs[jobID]; ok {
			j.Balance = newBal.FloatString(0)
			j.UpdatedAt = time.Now()
		}
		bso.trainingStore.mu.Unlock()

		clog.V(common.SHORT).Infof(ctx,
			"Refresh %s: credited tickets, new balance=%s",
			jobID, newBal.FloatString(0))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"job_id":          jobID,
			"new_balance_wei": newBal.FloatString(0),
		})
	})
}

// chargeTrainingTick charges for one tick of training compute and updates the job's cost/balance fields.
// Returns false if the sender has insufficient balance (job should be cancelled).
func (bso *BYOCOrchestratorServer) chargeTrainingTick(job *TrainingJob, seconds int64) bool {
	if job.jobPrice == nil {
		return true
	}
	priceRat := big.NewRat(job.jobPrice.PricePerUnit, job.jobPrice.PixelsPerUnit)
	if priceRat.Cmp(big.NewRat(0, 1)) <= 0 {
		return true // free, no charging needed
	}

	bso.orch.DebitFees(job.sender, core.ManifestID(job.Capability), job.jobPrice, seconds)
	bal := bso.getPaymentBalance(job.sender, job.Capability)

	// Update cost and balance on the job
	tickCost := new(big.Rat).Mul(priceRat, big.NewRat(seconds, 1))
	bso.trainingStore.mu.Lock()
	if j, ok := bso.trainingStore.jobs[job.JobID]; ok {
		// Accumulate cost
		prevCost, _ := new(big.Rat).SetString(j.Cost)
		if prevCost == nil {
			prevCost = big.NewRat(0, 1)
		}
		totalCost := new(big.Rat).Add(prevCost, tickCost)
		j.Cost = totalCost.FloatString(0)
		j.Balance = bal.FloatString(0)
	}
	bso.trainingStore.mu.Unlock()

	return bal.Cmp(big.NewRat(0, 1)) >= 0
}

// runTrainingJob submits the training job to the adapter and polls for completion
func (bso *BYOCOrchestratorServer) runTrainingJob(ctx context.Context, job *TrainingJob, orchJob *orchJob, body []byte) {
	defer bso.orch.FreeExternalCapabilityCapacity(job.Capability)

	// Build adapter training URL
	adapterURL := job.AdapterURL
	// Replace /inference with /train in the URL
	if strings.HasSuffix(adapterURL, "/inference") {
		adapterURL = strings.TrimSuffix(adapterURL, "/inference") + "/train"
	} else {
		adapterURL = strings.TrimSuffix(adapterURL, "/") + "/train"
	}

	// Submit to adapter
	req, err := http.NewRequestWithContext(ctx, "POST", adapterURL, bytes.NewBuffer(body))
	if err != nil {
		clog.Errorf(ctx, "Training job %s: failed to create request: %v", job.JobID, err)
		bso.trainingStore.Update(job.JobID, TrainingStatusFailed, 0, nil, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sendReqWithTimeout(req, 30*time.Second)
	if err != nil {
		clog.Errorf(ctx, "Training job %s: adapter submit failed: %v", job.JobID, err)
		bso.trainingStore.Update(job.JobID, TrainingStatusFailed, 0, nil, err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Training job %s: failed to read adapter response: %v", job.JobID, err)
		bso.trainingStore.Update(job.JobID, TrainingStatusFailed, 0, nil, err.Error())
		return
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("adapter returned %d: %s", resp.StatusCode, string(respBody))
		clog.Errorf(ctx, "Training job %s: %s", job.JobID, errMsg)
		bso.trainingStore.Update(job.JobID, TrainingStatusFailed, 0, nil, errMsg)
		return
	}

	var adapterResp map[string]interface{}
	if err := json.Unmarshal(respBody, &adapterResp); err != nil {
		clog.Errorf(ctx, "Training job %s: failed to parse adapter response: %v", job.JobID, err)
		bso.trainingStore.Update(job.JobID, TrainingStatusFailed, 0, nil, err.Error())
		return
	}

	adapterJobID, _ := adapterResp["job_id"].(string)
	if adapterJobID == "" {
		// Adapter returned result directly (synchronous)
		bso.trainingStore.Update(job.JobID, TrainingStatusCompleted, 100, adapterResp, "")
		clog.V(common.SHORT).Infof(ctx, "Training job %s: completed (synchronous)", job.JobID)
		return
	}

	// Store adapter job ID for polling
	func() {
		bso.trainingStore.mu.Lock()
		defer bso.trainingStore.mu.Unlock()
		if j, ok := bso.trainingStore.jobs[job.JobID]; ok {
			j.AdapterJobID = adapterJobID
			j.Status = TrainingStatusRunning
			j.UpdatedAt = time.Now()
		}
	}()

	clog.V(common.SHORT).Infof(ctx, "Training job %s: adapter job_id=%s, polling...", job.JobID, adapterJobID)

	// Poll adapter for status
	adapterStatusURL := strings.TrimSuffix(job.AdapterURL, "/inference")
	if strings.HasSuffix(adapterStatusURL, "/inference") {
		adapterStatusURL = strings.TrimSuffix(adapterStatusURL, "/inference")
	}
	statusURL := fmt.Sprintf("%s/train/%s", adapterStatusURL, adapterJobID)

	pollInterval := 5 * time.Second
	chargeInterval := 30 * time.Second
	pollTimeout := 8 * time.Hour
	start := time.Now()
	elapsed := time.Duration(0)
	lastChargeTime := start

	// PR-4 (byoc-payment-fleet-2026-05) §3.B fixes:
	//
	// (1) First-tick grace: don't fire chargeTick until the adapter
	//     reports IN_PROGRESS at least once. Prevents billing the user
	//     for fal-side setup (zip download, GPU spin-up). Without this,
	//     the orch bills 30s of "training" that was really 30s of
	//     queuing on fal's worker pool.
	//     `seenInProgress` flips true on first IN_PROGRESS observation
	//     and stays true. While false, `lastChargeTime` is continuously
	//     pushed forward so no charge accumulates.
	//
	// (2) Stalled-adapter pause: when status polls fail consecutively,
	//     pause chargeTick. The orch shouldn't bill for time when it
	//     can't verify the adapter is making progress.
	//     `consecutivePollFails` counter resets to 0 on successful poll.
	//     While >= 3, lastChargeTime is also pushed forward (matches the
	//     grace-period behavior).
	seenInProgress := false
	consecutivePollFails := 0
	const stallThreshold = 3

	// Set initial balance
	if job.jobPrice != nil {
		bal := bso.getPaymentBalance(job.sender, job.Capability)
		bso.trainingStore.mu.Lock()
		if j, ok := bso.trainingStore.jobs[job.JobID]; ok {
			j.Balance = bal.FloatString(0)
		}
		bso.trainingStore.mu.Unlock()
	}

	for elapsed < pollTimeout {
		// Check if job was cancelled
		currentJob, ok := bso.trainingStore.Get(job.JobID)
		if !ok || currentJob.Status == TrainingStatusCancelled {
			clog.Infof(ctx, "Training job %s: cancelled, stopping poll", job.JobID)
			// Charge for time used so far (only if we ever started billing)
			if seenInProgress {
				finalSecs := int64(math.Ceil(time.Since(lastChargeTime).Seconds()))
				if finalSecs > 0 {
					bso.chargeTrainingTick(job, finalSecs)
				}
			}
			return
		}

		// Charge every chargeInterval, but only AFTER first IN_PROGRESS
		// AND when adapter isn't stalled. While in grace / stall, push
		// lastChargeTime forward to drop the accrued time on the floor.
		if !seenInProgress || consecutivePollFails >= stallThreshold {
			lastChargeTime = time.Now()
			// Log stall transitions once to aid debugging
			if consecutivePollFails == stallThreshold {
				clog.Infof(ctx, "Training job %s: adapter stalled (%d failed polls), pausing billing",
					job.JobID, consecutivePollFails)
			}
		} else if time.Since(lastChargeTime) >= chargeInterval {
			secs := int64(math.Ceil(time.Since(lastChargeTime).Seconds()))
			if !bso.chargeTrainingTick(job, secs) {
				clog.Infof(ctx, "Training job %s: insufficient balance, cancelling", job.JobID)
				bso.trainingStore.Update(job.JobID, TrainingStatusCancelled, currentJob.Progress, nil, "Insufficient balance")
				return
			}
			lastChargeTime = time.Now()
			clog.V(common.DEBUG).Infof(ctx, "Training job %s: charged %ds, cost=%s balance=%s",
				job.JobID, secs, currentJob.Cost, currentJob.Balance)
		}

		time.Sleep(pollInterval)
		elapsed += pollInterval

		statusReq, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
		if err != nil {
			consecutivePollFails++
			continue
		}

		statusResp, err := sendReqWithTimeout(statusReq, 15*time.Second)
		if err != nil {
			consecutivePollFails++
			glog.Warningf("Training job %s: status poll failed: %v (consecutive=%d)",
				job.JobID, err, consecutivePollFails)
			continue
		}

		statusBody, err := io.ReadAll(statusResp.Body)
		statusResp.Body.Close()
		if err != nil {
			consecutivePollFails++
			continue
		}

		var statusData map[string]interface{}
		if err := json.Unmarshal(statusBody, &statusData); err != nil {
			consecutivePollFails++
			continue
		}

		// Status read succeeded — reset stall counter. If we were
		// previously stalled and just recovered, log it.
		if consecutivePollFails >= stallThreshold {
			clog.Infof(ctx, "Training job %s: adapter recovered after %d failed polls, resuming billing",
				job.JobID, consecutivePollFails)
		}
		consecutivePollFails = 0

		status, _ := statusData["status"].(string)
		progress, _ := statusData["progress"].(float64)

		// First-tick grace: flip seenInProgress on the first status that
		// indicates the adapter is actually running the job. Statuses
		// "submitted" / "" don't qualify; everything else does (running,
		// completed, failed, cancelled all imply work happened).
		if !seenInProgress && status != "" && status != "submitted" {
			seenInProgress = true
			// Reset lastChargeTime so the first billable interval starts
			// from the moment we observed IN_PROGRESS, not job submit.
			lastChargeTime = time.Now()
			clog.V(common.SHORT).Infof(ctx, "Training job %s: in-progress observed, billing starts now",
				job.JobID)
		}

		switch status {
		case "completed":
			result, _ := statusData["result"].(map[string]interface{})
			if result == nil {
				result = statusData
			}
			// Final charge for remaining seconds since last tick — only
			// if we ever started billing
			if seenInProgress {
				finalSecs := int64(math.Ceil(time.Since(lastChargeTime).Seconds()))
				if finalSecs > 0 {
					bso.chargeTrainingTick(job, finalSecs)
				}
			}
			bso.trainingStore.Update(job.JobID, TrainingStatusCompleted, 100, result, "")
			clog.V(common.SHORT).Infof(ctx, "Training job %s: completed (elapsed=%v cost=%s)",
				job.JobID, time.Since(start), job.Cost)
			return
		case "failed":
			errMsg, _ := statusData["error"].(string)
			if seenInProgress {
				finalSecs := int64(math.Ceil(time.Since(lastChargeTime).Seconds()))
				if finalSecs > 0 {
					bso.chargeTrainingTick(job, finalSecs)
				}
			}
			bso.trainingStore.Update(job.JobID, TrainingStatusFailed, int(progress), nil, errMsg)
			clog.Errorf(ctx, "Training job %s: failed: %s (cost=%s)", job.JobID, errMsg, job.Cost)
			return
		case "cancelled":
			if seenInProgress {
				finalSecs := int64(math.Ceil(time.Since(lastChargeTime).Seconds()))
				if finalSecs > 0 {
					bso.chargeTrainingTick(job, finalSecs)
				}
			}
			bso.trainingStore.Update(job.JobID, TrainingStatusCancelled, int(progress), nil, "")
			return
		default:
			bso.trainingStore.Update(job.JobID, TrainingStatusRunning, int(progress), nil, "")
		}
	}

	// Timed out -- charge for elapsed time (only if billing started)
	if seenInProgress {
		finalSecs := int64(math.Ceil(time.Since(lastChargeTime).Seconds()))
		if finalSecs > 0 {
			bso.chargeTrainingTick(job, finalSecs)
		}
	}
	bso.trainingStore.Update(job.JobID, TrainingStatusFailed, 0, nil, fmt.Sprintf("Timed out after %v", pollTimeout))
	clog.Errorf(ctx, "Training job %s: timed out (cost=%s)", job.JobID, job.Cost)
}
