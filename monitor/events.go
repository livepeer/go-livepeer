package monitor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/glog"
)

var Enabled bool

type metricsVars struct {
	lastSegmentEmergedAt time.Time
	lastSegmentNonce     uint64
	lastSeqNo            int64
	segmentsInFlight     int
	ch                   chan *event
	nodeType             string
	nodeID               string
}

var metrics = metricsVars{ch: make(chan *event, 1024)}

type event struct {
	Name       string                 `json:"event"`
	Nonce      string                 `json:"nonce"`
	Ts         string                 `json:"ts"`
	NodeID     string                 `json:"nodeId"`
	NodeType   string                 `json:"nodeType"`
	Properties map[string]interface{} `json:"properties"`
}

var eventsURL string

func Init(url string, nodeType string, nodeID string) {
	eventsURL = url
	metrics.nodeID = nodeID
	metrics.nodeType = nodeType
	go sendLoop(metrics.ch)
}

func sendLoop(inCh chan *event) {
	var client = &http.Client{}
	for {
		e := <-inCh
		jsonStr, err := json.Marshal(e)
		if err != nil {
			glog.Errorf("Error sending event to logger.")
			continue
		}
		req, err := http.NewRequest("POST", eventsURL, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			glog.Errorf("Error sending event to logger (%s).", eventsURL)
			continue
		}
		resp.Body.Close()
	}
}

func LogSegmentTranscodeStarting(seqNo uint64, manifestID string) {
	glog.Infof("Logging SegmentTranscodeStarting...")

	props := map[string]interface{}{
		"seqNo":      seqNo,
		"manifestID": manifestID,
	}

	sendPost("SegmentTranscodeStarting", 0, props)
}

func LogSegmentTranscodeEnded(seqNo uint64, manifestID string) {
	glog.Infof("Logging SegmentTranscodeEnded...")

	props := map[string]interface{}{
		"seqNo":      seqNo,
		"manifestID": manifestID,
	}

	sendPost("SegmentTranscodeEnded", 0, props)
}

func LogStreamCreatedEvent(hlsStrmID string, nonce uint64) {
	glog.Infof("Logging StreamCreated...")

	props := map[string]interface{}{
		"hlsStrmID": hlsStrmID,
	}

	sendPost("StreamCreated", nonce, props)
}

func LogStreamStartedEvent(nonce uint64) {
	glog.Infof("Logging StreamStarted...")

	sendPost("StreamStarted", nonce, nil)
}

func LogStreamEndedEvent(nonce uint64) {
	glog.Infof("Logging StreamEnded...")

	sendPost("StreamEnded", nonce, nil)
}

func LogStreamCreateFailed(nonce uint64, reason string) {
	glog.Infof("Logging StreamCreateFailed...")

	props := map[string]interface{}{
		"reason": reason,
	}

	sendPost("StreamCreateFailed", nonce, props)
}

func LogSegmentUploadFailed(nonce, seqNo uint64, reason string) {
	glog.Infof("Logging SegmentUploadFailed...")

	props := map[string]interface{}{
		"reason": reason,
		"seqNo":  seqNo,
	}

	sendPost("SegmentUploadFailed", nonce, props)
}

func LogTranscodedSegmentAppeared(nonce, seqNo uint64, profile string) {
	glog.Infof("Logging LogTranscodedSegmentAppeared...")
	props := map[string]interface{}{
		"seqNo":   seqNo,
		"profile": profile,
	}

	sendPost("TranscodedSegmentAppeared", nonce, props)
}

func LogSourceSegmentAppeared(nonce, seqNo uint64, manifestID, profile string) {
	glog.Infof("Logging LogSourceSegmentAppeared...")
	props := map[string]interface{}{
		"seqNo":      seqNo,
		"profile":    profile,
		"manifestID": manifestID,
	}

	sendPost("SourceSegmentAppeared", nonce, props)
}

func LogSegmentEmerged(nonce, seqNo uint64) {
	glog.Infof("Logging SegmentEmerged...")

	var sincePrevious time.Duration
	now := time.Now()

	if metrics.lastSegmentNonce == nonce {
		sincePrevious = now.Sub(metrics.lastSegmentEmergedAt)
	} else {
		metrics.segmentsInFlight = 0
		metrics.lastSegmentNonce = nonce
		metrics.lastSeqNo = int64(seqNo) - 1
	}
	metrics.lastSegmentEmergedAt = now
	props := map[string]interface{}{
		"seqNo":         seqNo,
		"sincePrevious": uint64(sincePrevious / time.Millisecond),
	}

	sendPost("SegmentEmerged", nonce, props)
}

func SegmentUploadStart(nonce, seqNo uint64) {
	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight++
	}
}

func LogSegmentUploaded(nonce, seqNo uint64, uploadDur time.Duration) {
	glog.Infof("Logging SegmentUploaded...")

	props := map[string]interface{}{
		"seqNo":          seqNo,
		"uploadDuration": uint64(uploadDur / time.Millisecond),
	}

	sendPost("SegmentUploaded", nonce, props)
}

func detectSeqDif(props map[string]interface{}, nonce, seqNo uint64) {
	if metrics.lastSegmentNonce == nonce {
		seqDif := int64(seqNo) - metrics.lastSeqNo
		metrics.lastSeqNo = int64(seqNo)
		if seqDif != 1 {
			props["seqNoDif"] = seqDif
		}
	}
}

func LogSegmentTranscoded(nonce, seqNo uint64, transcodeDur, totalDur time.Duration) {
	glog.Infof("Logging SegmentTranscoded...")

	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight--
	}
	props := map[string]interface{}{
		"seqNo":             seqNo,
		"transcodeDuration": uint64(transcodeDur / time.Millisecond),
		"totalDuration":     uint64(totalDur / time.Millisecond),
	}
	if metrics.segmentsInFlight != 0 {
		props["segmentsInFlight"] = metrics.segmentsInFlight
	}
	detectSeqDif(props, nonce, seqNo)

	sendPost("SegmentTranscoded", nonce, props)
}

func LogSegmentTranscodeFailed(subType string, nonce, seqNo uint64, err error) {
	glog.Info("Logging LogSegmentTranscodeFailed", subType)

	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight--
	}
	props := map[string]interface{}{
		"subType": subType,
		"reason":  err.Error(),
		"seqNo":   seqNo,
	}
	if metrics.segmentsInFlight != 0 {
		props["segmentsInFlight"] = metrics.segmentsInFlight
	}
	detectSeqDif(props, nonce, seqNo)

	sendPost("SegmentTranscodeFailed", nonce, props)
}

func LogStartBroadcastClientFailed(nonce uint64, serviceURI, transcoderAddress string, jobID string, reason string) {
	glog.Infof("Logging StartBroadcastClientFailed...")

	props := map[string]interface{}{
		"jobID":             jobID,
		"serviceURI":        serviceURI,
		"reason":            reason,
		"transcoderAddress": transcoderAddress,
	}

	sendPost("StartBroadcastClientFailed", nonce, props)
}

func sendPost(name string, nonce uint64, props map[string]interface{}) {
	if eventsURL == "" {
		return
	}
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	e := &event{
		Name:       name,
		Nonce:      strconv.FormatUint(nonce, 10),
		Ts:         strconv.FormatInt(ts, 10),
		NodeID:     metrics.nodeID,
		NodeType:   metrics.nodeType,
		Properties: props,
	}
	metrics.ch <- e
}
