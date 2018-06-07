package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang/glog"
)

type Event struct {
	Name       string            `json:"event"`
	Properties map[string]string `json:"properties"`
}

var EventsURL = "http://metrics.livepeer.org/events"

func LogStreamCreatedEvent(manifestID string, nonce uint64) {
	e := &Event{
		Name: "StreamCreated",
		Properties: map[string]string{
			"manifestID": manifestID,
			"nonce":      fmt.Sprintf("%v", nonce),
		},
	}

	sendPost(e)
}

func LogStreamStartedEvent(manifestID string, nonce uint64) {
	e := &Event{
		Name: "StreamStarted",
		Properties: map[string]string{
			"manifestID": manifestID,
			"nonce":      fmt.Sprintf("%v", nonce),
		},
	}

	sendPost(e)
}

func LogStreamEndedEvent(manifestID string, nonce uint64) {
	e := &Event{
		Name: "StreamEnded",
		Properties: map[string]string{
			"manifestID": manifestID,
			"nonce":      fmt.Sprintf("%v", nonce),
		},
	}

	sendPost(e)
}

func LogStreamCreateFailed(manifestID string, nonce uint64, reason string) {
	glog.Infof("Logging StreamCreateFailed...")
	e := &Event{
		Name: "StreamCreateFailed",
		Properties: map[string]string{
			"manifestID": manifestID,
			"reason":     reason,
			"nonce":      fmt.Sprintf("%v", nonce),
		},
	}

	sendPost(e)
}

func sendPost(e *Event) {
	jsonStr, err := json.Marshal(e)
	if err != nil {
		glog.Errorf("Error sending event to logger.")
		return
	}
	req, err := http.NewRequest("POST", EventsURL, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		glog.Errorf("Error sending event to logger.")
		return
	}
}
