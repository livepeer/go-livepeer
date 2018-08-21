package monitor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/glog"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
)

var Enabled bool

type metricsVars struct {
	lastSegmentEmergedAt time.Time
	lastSegmentNonce     uint64
	lastSeqNo            int64
	segmentsInFlight     int
}

var metrics = metricsVars{}

type event struct {
	Name       string                 `json:"event"`
	Nonce      string                 `json:"nonce"`
	Properties map[string]interface{} `json:"properties"`
}

var eventsURL string

func SetURL(url string) {
	eventsURL = url
}

func LogJobCreatedEvent(job *ethTypes.Job, nonce uint64) {
	glog.Infof("Logging JobCreated...")

	props := map[string]interface{}{
		"jobID":              job.JobId.Uint64(),
		"streamID":           job.StreamId,
		"broadcasterAddress": job.BroadcasterAddress.Hex(),
		"transcoderAddress":  job.TranscoderAddress.Hex(),
		"creationRound":      job.CreationRound.Uint64(),
		"creationBlock":      job.CreationBlock.Uint64(),
		"endBlock":           job.EndBlock.Uint64(),
	}

	sendPost("JobCreated", nonce, props)
}

func LogJobReusedEvent(job *ethTypes.Job, startSeq int, nonce uint64) {
	glog.Infof("Logging JobReused...")

	props := map[string]interface{}{
		"jobID":              job.JobId.Uint64(),
		"streamID":           job.StreamId,
		"broadcasterAddress": job.BroadcasterAddress.Hex(),
		"transcoderAddress":  job.TranscoderAddress.Hex(),
		"creationBlock":      job.CreationBlock.Uint64(),
		"endBlock":           job.EndBlock.Uint64(),
		"startSeq":           startSeq,
	}

	sendPost("JobReused", nonce, props)
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

func LogSegmentTranscodeFailed(ev string, nonce, seqNo uint64, err error) {
	glog.Info("Logging ", ev)

	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight--
	}
	props := map[string]interface{}{
		"reason": err.Error(),
		"seqNo":  seqNo,
	}
	if metrics.segmentsInFlight != 0 {
		props["segmentsInFlight"] = metrics.segmentsInFlight
	}
	detectSeqDif(props, nonce, seqNo)

	sendPost(ev, nonce, props)
}

func LogStartBroadcastClientFailed(nonce uint64, serviceURI, transcoderAddress string, jobID uint64, reason string) {
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
	go _sendPost(name, nonce, props)
}

func _sendPost(name string, nonce uint64, props map[string]interface{}) {
	e := &event{
		Name:       name,
		Nonce:      strconv.FormatUint(nonce, 10),
		Properties: props,
	}
	jsonStr, err := json.Marshal(e)
	if err != nil {
		glog.Errorf("Error sending event to logger.")
		return
	}
	req, err := http.NewRequest("POST", eventsURL, bytes.NewBuffer(jsonStr))
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
