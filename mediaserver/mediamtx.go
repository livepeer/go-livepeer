package mediaserver

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

func (ls *MediaMTXClient) KickInputConnection(mediaMTXHost, sourceID, sourceType string) error {
	var apiPath string
	switch sourceType {
	case MediaMTXWebrtcSession:
		apiPath = "webrtcsessions"
	case MediaMTXRtmpConn:
		apiPath = "rtmpconns"
	default:
		return fmt.Errorf("invalid sourceType: %s", sourceType)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%s/v3/%s/kick/%s", mediaMTXHost, mediaMTXControlPort, apiPath, sourceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create kick request: %w", err)
	}
	req.SetBasicAuth(mediaMTXControlUser, ls.apiPassword)
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
