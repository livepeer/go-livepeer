package hive

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
)

type JobSource string

const (
	JobSourceLivepeer JobSource = "Livepeer"
)

type JobStatus string

type ActivateWorkerRequest struct {
	WorkerIP string `json:"worker_ip"`
	Pipeline string `json:"pipeline"`
	Model    string `json:"model"`
}

const (
	JobStatusProcessing JobStatus = "Processing"
	JobStatusCompleted  JobStatus = "Completed"
	JobStatusFailed     JobStatus = "Failed"
)

type JobFilters struct {
	FromTimestamp time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	ToTimestamp   time.Time `form:"to" time_format:"2006-01-02T15:04:05Z07:00"`
	Status        string    `form:"status"`
	WorkerID      string    `form:"worker_id"`
}

type PaginationQuery struct {
	Page    int `form:"page" binding:"gte=1"`
	Limit   int `form:"limit" binding:"gte=1,lte=100"`
	Filters JobFilters
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
}

type User struct {
	ID             string    `json:"id" db:"id"`
	Username       string    `json:"username" db:"username,omitempty"`
	Email          string    `json:"email" db:"email,omitempty"`
	Address        string    `json:"address" db:"address"`
	PendingBalance float64   `json:"pendingBalance" db:"pendingBalance"`
	CreatedAt      time.Time `json:"createdAt" db:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt" db:"updatedAt"`
}

type Worker struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"userID" db:"userID"`
	IPPort        string    `json:"ipPort" db:"ipPort"`
	Pipeline      string    `json:"pipeline" db:"pipeline"`
	Model         string    `json:"model" db:"model"`
	Active        bool      `json:"active" db:"active"`
	Earnings      float64   `json:"earnings" db:"earnings"`
	JobsCompleted int       `json:"jobsCompleted" db:"jobsCompleted"`
	JobsFailed    int       `json:"jobsFailed" db:"jobsFailed"`
	CreatedAt     time.Time `json:"createdAt" db:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt" db:"updatedAt"`
}

type Job struct {
	ID           string    `json:"id" db:"id"`
	Orchestrator string    `json:"orchestrator" db:"orchestrator"`
	WorkerID     string    `json:"workerId" db:"workerId"`
	Status       JobStatus `json:"status" db:"status"`
	Pipeline     string    `json:"pipeline" db:"pipeline"`
	Model        string    `json:"model" db:"model"`
	Tokens       float64   `json:"tokens" db:"tokens"`
	Source       JobSource `json:"source" db:"source"`
	ErrorMsg     string    `json:"errorMsg" db:"errorMsg,omitempty"`
	CreatedAt    time.Time `json:"createdAt" db:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updatedAt"`
}

type CreateJobRequest struct {
	WorkerID     string    `json:"worker_id"`
	Orchestrator string    `json:"orchestrator"`
	Pipeline     string    `json:"pipeline"`
	Model        string    `json:"model"`
	Tokens       float64   `json:"tokens"`
	Source       JobSource `json:"source"`
}

type CompleteJobRequest struct {
	TokensUsed float64   `json:"usage"`
	Status     JobStatus `json:"status"`
	ErrorMsg   string    `json:"errorMsg"`
}

// generateHMACSignature creates the HMAC signature for authentication
func (h *Hive) generateHMACSignature(message string) string {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// Error handling
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %d - %s", e.StatusCode, e.Message)
}

type Hive struct {
	baseURI    string
	httpClient *http.Client
	secret     string
}

type ClientOption func(*Hive)

func NewHive(baseURI string, secret string, opts ...ClientOption) *Hive {
	hive := &Hive{
		baseURI: baseURI,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		secret: secret,
	}

	for _, opt := range opts {
		opt(hive)
	}

	return hive
}

// Options
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(h *Hive) {
		h.httpClient = httpClient
	}
}

// Worker methods
func (h *Hive) ActivateWorker(ctx context.Context, workerID, workerIP, pipeline, model string) error {
	req := &ActivateWorkerRequest{
		WorkerIP: workerIP,
		Pipeline: pipeline,
		Model:    model,
	}

	endpoint := fmt.Sprintf("/api/v1/workers/%v/activate", workerID)
	err := h.sendRequest(ctx, http.MethodPatch, endpoint, req, nil)
	if err != nil {
		return err
	}
	return nil
}

func (h *Hive) DeactivateWorker(ctx context.Context, workerID string) error {
	endpoint := fmt.Sprintf("/api/v1/workers/%s/deactivate", workerID)
	err := h.sendRequest(ctx, http.MethodPatch, endpoint, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

// Job methods
func (h *Hive) CreateJob(ctx context.Context, jobID string, req *CreateJobRequest) error {
	glog.Infof("Received create job request ID=%s worker=%s", jobID, req.WorkerID)
	job := Job{
		ID:           jobID,
		Orchestrator: "hive-ai",
		WorkerID:     req.WorkerID,
		Status:       JobStatusProcessing,
		Pipeline:     req.Pipeline,
		Model:        req.Model,
		Tokens:       0,
		Source:       req.Source,
	}
	endpoint := fmt.Sprintf("/api/v1/jobs/%s", jobID)
	err := h.sendRequest(ctx, http.MethodPost, endpoint, job, nil)
	if err != nil {
		return err
	}
	return nil
}

func (h *Hive) CompleteJob(ctx context.Context, jobID string, req *CompleteJobRequest) error {
	endpoint := fmt.Sprintf("/api/v1/jobs/%s", jobID)
	err := h.sendRequest(ctx, http.MethodPatch, endpoint, req, nil)
	if err != nil {
		return err
	}
	return nil
}

// Helper methods
func (h *Hive) sendRequest(ctx context.Context, method, endpoint string, body, response interface{}) error {
	var buf bytes.Buffer
	url := fmt.Sprintf("%s%s", h.baseURI, endpoint)
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Generate timestamp in RFC3339 format
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Create message for HMAC
	message := fmt.Sprintf("%s", timestamp)

	// Generate signature
	signature := h.generateHMACSignature(message)

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return fmt.Errorf("request failed with status code: %d", resp.StatusCode)
		}
		apiErr.StatusCode = resp.StatusCode
		return &apiErr
	}

	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
