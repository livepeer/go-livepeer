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
	Stream string `json:"stream"`

	// Stream type, eg RTMP or WHIP
	Type string `json:"type"`

	// Query parameters that came with the stream, if any
	QueryParams string `json:"query_params,omitempty"`

	// TODO not sure what params we need yet
}

func authenticateAIStream(authURL *url.URL, req AIAuthRequest) error {
	if authURL == nil {
		return nil
	}
	started := time.Now()

	jsonValue, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(authURL.String(), "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}

	rbody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status=%d error=%s", resp.StatusCode, string(rbody))
	}

	took := time.Since(started)
	glog.Infof("AI Stream authentication for authURL=%s stream=%s dur=%s", authURL, req.Stream, took)
	if monitor.Enabled {
		monitor.AuthWebhookFinished(took)
	}

	return nil
}
