package media

import (
	"fmt"
	"io"
	"net/http"
)

type MediaMTXClient struct {
	apiPassword string
}

func NewMediaMTXClient(apiPassword string) *MediaMTXClient {
	return &MediaMTXClient{apiPassword: apiPassword}
}

const (
	mediaMTXControlPort   = "9997"
	mediaMTXControlUser   = "admin"
	MediaMTXWebrtcSession = "webrtcSession"
	MediaMTXRtmpConn      = "rtmpConn"
)

func getApiPath(sourceType string) (string, error) {
	var apiPath string
	switch sourceType {
	case MediaMTXWebrtcSession:
		apiPath = "webrtcsessions"
	case MediaMTXRtmpConn:
		apiPath = "rtmpconns"
	default:
		return "", fmt.Errorf("invalid sourceType: %s", sourceType)
	}
	return apiPath, nil
}

func (mc *MediaMTXClient) KickInputConnection(mediaMTXHost, sourceID, sourceType string) error {
	apiPath, err := getApiPath(sourceType)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%s/v3/%s/kick/%s", mediaMTXHost, mediaMTXControlPort, apiPath, sourceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create kick request: %w", err)
	}
	req.SetBasicAuth(mediaMTXControlUser, mc.apiPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to kick connection: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick connection failed with status code: %d body: %s", resp.StatusCode, body)
	}
	return nil
}

func (mc *MediaMTXClient) StreamExists(mediaMTXHost, sourceID, sourceType string) (bool, error) {
	apiPath, err := getApiPath(sourceType)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%s/v3/%s/get/%s", mediaMTXHost, mediaMTXControlPort, apiPath, sourceID), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create get stream request: %w", err)
	}
	req.SetBasicAuth(mediaMTXControlUser, mc.apiPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to get stream: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("get stream failed with status code: %d body: %s", resp.StatusCode, body)
	}
	return true, nil
}
