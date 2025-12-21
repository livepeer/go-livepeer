package byoc

//NOTE: methods in this file are duplicated from server package.  The auth webhook methods are not
// exported from server package and cannot be imported directly.  Not changes have been made to the logic.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

var LiveAIAuthWebhookURL *url.URL

type AIAuthRequest struct {
	// Stream name or stream key
	Stream    string `json:"stream"`
	StreamKey string `json:"stream_key"`

	// Stream type, eg RTMP or WHIP
	Type string `json:"type"`

	// Query parameters that came with the stream, if any
	QueryParams string `json:"query_params,omitempty"`

	// Gateway host
	GatewayHost string `json:"gateway_host"`
	WhepURL     string `json:"whep_url"`
	StatusURL   string `json:"status_url"`
	UpdateURL   string `json:"update_url"`
}

// Contains the configuration parameters for this AI job
type AIAuthResponse struct {
	// Where to send the output video
	RTMPOutputURL string `json:"rtmp_output_url"`

	// Name of the pipeline to run
	Pipeline string `json:"pipeline"`

	// ID of the pipeline to run
	PipelineID string `json:"pipeline_id"`

	// ID of the stream
	StreamID string `json:"stream_id"`

	// Parameters for the pipeline
	PipelineParams json.RawMessage        `json:"pipeline_parameters"`
	paramsMap      map[string]interface{} // unmarshaled params
}

func authenticateAIStream(authURL *url.URL, apiKey string, req AIAuthRequest) (*AIAuthResponse, error) {
	req.StreamKey = req.Stream
	if authURL == nil {
		return nil, fmt.Errorf("No auth URL configured")
	}
	started := time.Now()

	jsonValue, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", authURL.String(), bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", apiKey)
	request.Header.Set("Authorization", apiKey)

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	rbody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status=%d error=%s", resp.StatusCode, string(rbody))
	}

	took := time.Since(started)
	glog.Infof("AI Stream authentication for authURL=%s stream=%s dur=%s", authURL, req.Stream, took)
	if monitor.Enabled {
		monitor.AuthWebhookFinished(took)
	}

	var authResp AIAuthResponse
	if err := json.Unmarshal(rbody, &authResp); err != nil {
		return nil, err
	}
	if len(authResp.PipelineParams) > 0 {
		if err := json.Unmarshal([]byte(authResp.PipelineParams), &authResp.paramsMap); err != nil {
			return nil, err
		}
	}

	return &authResp, nil
}
