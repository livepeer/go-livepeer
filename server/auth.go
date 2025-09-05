package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

const LIVERPEER_TRANSCODE_CONFIG_HEADER = "Livepeer-Transcode-Configuration"

// Call a webhook URL, passing the request URL we received
// Based on the response, we can authenticate and confirm whether to accept an incoming stream
func authenticateStream(authURL *url.URL, incomingRequestURL string) (*authWebhookResponse, error) {
	if authURL == nil {
		return nil, nil
	}
	started := time.Now()

	jsonValue, err := json.Marshal(map[string]string{"url": incomingRequestURL})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(authURL.String(), "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}

	rbody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status=%d error=%s", resp.StatusCode, string(rbody))
	}
	if len(rbody) == 0 {
		return nil, nil
	}

	var authResp authWebhookResponse
	if err = json.Unmarshal(rbody, &authResp); err != nil {
		return nil, err
	}
	if authResp.ManifestID == "" {
		return nil, errors.New("empty manifest id not allowed")
	}

	took := time.Since(started)
	glog.Infof("Stream authentication for authURL=%s url=%s dur=%s", authURL, incomingRequestURL, took)
	if monitor.Enabled {
		monitor.AuthWebhookFinished(took)
	}

	return &authResp, nil
}

func getTranscodeConfiguration(r *http.Request) (*authWebhookResponse, error) {
	transcodeConfigurationHeader := r.Header.Get(LIVERPEER_TRANSCODE_CONFIG_HEADER)
	if transcodeConfigurationHeader == "" {
		return nil, nil
	}

	var transcodeConfiguration authWebhookResponse
	err := json.Unmarshal([]byte(transcodeConfigurationHeader), &transcodeConfiguration)

	return &transcodeConfiguration, err
}

// Compare two sets of profiles. Since there's no deep equality method in Go,
// we marshal to JSON and compare the resulting strings
func (a authWebhookResponse) areProfilesEqual(b authWebhookResponse) bool {
	// Return quickly in simple cases without trying to marshal JSON
	if len(a.Profiles) != len(b.Profiles) {
		return false
	}
	if len(a.Profiles) == 0 {
		return true
	}

	profilesA, err := json.Marshal(a.Profiles)
	if err != nil {
		return false
	}

	profilesB, err := json.Marshal(b.Profiles)
	if err != nil {
		return false
	}

	return string(profilesA) == string(profilesB)
}

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
