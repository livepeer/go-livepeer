package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

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

	rbody, err := ioutil.ReadAll(resp.Body)
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
