package monitor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

// Enabled true if metrics was enabled in command line
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

func Init(url, nodeType, nodeID, version string) {
	eventsURL = url
	metrics.nodeID = nodeID
	metrics.nodeType = nodeType
	go sendLoop(metrics.ch)
	initCensus(nodeType, nodeID, version)
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
	glog.Infof("Logging SegmentTranscodeStarting... seqNo=%d manifestID=%s",
		seqNo, manifestID)

	props := map[string]interface{}{
		"seqNo":      seqNo,
		"manifestID": manifestID,
	}

	sendPost("SegmentTranscodeStarting", 0, props)
}

func LogSegmentTranscodeEnded(seqNo uint64, manifestID string, d time.Duration,
	profiles string) {
	glog.Infof("Logging SegmentTranscodeEnded... seqNo=%d manifestID=%s duration=%s",
		seqNo, manifestID, d)
	census.segmentTranscoded(0, seqNo, d, 0, profiles)

	props := map[string]interface{}{
		"seqNo":      seqNo,
		"manifestID": manifestID,
	}

	sendPost("SegmentTranscodeEnded", 0, props)
}

func LogStreamCreatedEvent(hlsStrmID string, nonce uint64) {
	glog.Infof("Logging StreamCreated... nonce=%d strid=%s", nonce, hlsStrmID)
	census.streamCreated(nonce)

	props := map[string]interface{}{
		"hlsStrmID": hlsStrmID,
	}

	sendPost("StreamCreated", nonce, props)
}

func LogStreamStartedEvent(nonce uint64) {
	glog.Infof("Logging StreamStarted... nonce=%d", nonce)
	census.streamStarted(nonce)

	sendPost("StreamStarted", nonce, nil)
}

func LogStreamEndedEvent(nonce uint64) {
	glog.Infof("Logging StreamEnded... nonce=%d", nonce)
	census.streamEnded(nonce)

	sendPost("StreamEnded", nonce, nil)
}

func LogStreamCreateFailed(nonce uint64, reason string) {
	glog.Errorf("Logging StreamCreateFailed... nonce=%d reason='%s'", nonce, reason)
	census.streamCreateFailed(nonce, reason)

	props := map[string]interface{}{
		"reason": reason,
	}

	sendPost("StreamCreateFailed", nonce, props)
}

func LogSegmentUploadFailed(nonce, seqNo uint64, code SegmentUploadError, reason string, permanent bool) {
	if code == SegmentUploadErrorUnknown {
		if strings.Contains(reason, "Client.Timeout") {
			code = SegmentUploadErrorTimeout
		} else if reason == "Session ended" {
			code = SegmentUploadErrorSessionEnded
		}
	}
	glog.Errorf("Logging SegmentUploadFailed... code=%v reason='%s'", code, reason)

	census.segmentUploadFailed(nonce, seqNo, code, permanent)

	props := map[string]interface{}{
		"reason": reason,
		"seqNo":  seqNo,
	}

	sendPost("SegmentUploadFailed", nonce, props)
}

func LogTranscodedSegmentAppeared(nonce, seqNo uint64, profile string) {
	glog.Infof("Logging LogTranscodedSegmentAppeared... nonce=%d SeqNo=%d profile=%s", nonce, seqNo, profile)
	census.segmentTranscodedAppeared(nonce, seqNo, profile)

	props := map[string]interface{}{
		"seqNo":   seqNo,
		"profile": profile,
	}

	sendPost("TranscodedSegmentAppeared", nonce, props)
}

func LogSourceSegmentAppeared(nonce, seqNo uint64, manifestID, profile string) {
	glog.Infof("Logging LogSourceSegmentAppeared... nonce=%d seqNo=%d manifestid=%s profile=%s", nonce,
		seqNo, manifestID, profile)
	census.segmentSourceAppeared(nonce, seqNo, profile)
	props := map[string]interface{}{
		"seqNo":      seqNo,
		"profile":    profile,
		"manifestID": manifestID,
	}

	sendPost("SourceSegmentAppeared", nonce, props)
}

func LogSegmentEmerged(nonce, seqNo uint64, profilesNum int) {
	glog.Infof("Logging SegmentEmerged... nonce=%d seqNo=%d", nonce, seqNo)
	census.segmentEmerged(nonce, seqNo, profilesNum)

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
	glog.Infof("Logging SegmentUploaded... nonce=%d seqNo=%d uploadduration=%s", nonce, seqNo, uploadDur)
	census.segmentUploaded(nonce, seqNo, uploadDur)

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

func LogSegmentTranscoded(nonce, seqNo uint64, transcodeDur, totalDur time.Duration,
	profiles string) {
	glog.Infof("Logging SegmentTranscoded... nonce=%d seqNo=%d transcode_duration=%s total_dur=%s",
		nonce, seqNo, transcodeDur, totalDur)

	census.segmentTranscoded(nonce, seqNo, transcodeDur, totalDur, profiles)

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

func LogSegmentTranscodeFailed(subType SegmentTranscodeError, nonce, seqNo uint64, err error, permanent bool) {
	glog.Errorf("Logging LogSegmentTranscodeFailed subtype=%v nonce=%d seqNo=%d error='%s'", subType, nonce, seqNo, err.Error())

	census.segmentTranscodeFailed(nonce, seqNo, subType, permanent)
	if err == nil {
		return
	}

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
