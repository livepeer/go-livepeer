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

var eventsURLBase string
var eventsURL string

var network = "rinkeby"

func SetNetwork(n string) {
	network = n
	eventsURL = eventsURLBase + n + "/events"
}

func SetURLBase(urlBase string) {
	eventsURLBase = urlBase
	eventsURL = eventsURLBase + network + "/events"
}

func LogJobCreatedEvent(job *ethTypes.Job, nonce uint64) {
	e := &event{
		Name:  "JobCreated",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"jobID":              job.JobId.Uint64(),
			"streamID":           job.StreamId,
			"broadcasterAddress": job.BroadcasterAddress.Hex(),
			"transcoderAddress":  job.TranscoderAddress.Hex(),
			"creationRound":      job.CreationRound.Uint64(),
			"creationBlock":      job.CreationBlock.Uint64(),
			"endBlock":           job.EndBlock.Uint64(),
		},
	}

	sendPost(e)
}

func LogJobReusedEvent(job *ethTypes.Job, startSeq int, nonce uint64) {
	e := &event{
		Name:  "JobReused",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"jobID":              job.JobId.Uint64(),
			"streamID":           job.StreamId,
			"broadcasterAddress": job.BroadcasterAddress.Hex(),
			"transcoderAddress":  job.TranscoderAddress.Hex(),
			"creationBlock":      job.CreationBlock.Uint64(),
			"endBlock":           job.EndBlock.Uint64(),
			"startSeq":           startSeq,
		},
	}

	sendPost(e)
}

func LogStreamCreatedEvent(hlsStrmID string, nonce uint64) {
	e := &event{
		Name:  "StreamCreated",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"hlsStrmID": hlsStrmID,
		},
	}

	sendPost(e)
}

func LogStreamStartedEvent(nonce uint64) {
	e := &event{
		Name:  "StreamStarted",
		Nonce: strconv.FormatUint(nonce, 10),
	}

	sendPost(e)
}

func LogStreamEndedEvent(nonce uint64) {
	e := &event{
		Name:  "StreamEnded",
		Nonce: strconv.FormatUint(nonce, 10),
	}

	sendPost(e)
}

func LogStreamCreateFailed(nonce uint64, reason string) {
	glog.Infof("Logging StreamCreateFailed...")
	e := &event{
		Name:  "StreamCreateFailed",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"reason": reason,
		},
	}

	sendPost(e)
}

func LogSegmentUploadFailed(nonce, seqNo uint64, reason string) {
	glog.Infof("Logging LogSegmentUploadFailed...")
	e := &event{
		Name:  "SegmentUploadFailed",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"reason": reason,
			"seqNo":  seqNo,
		},
	}

	sendPost(e)
}

func LogSegmentEmerged(nonce, seqNo uint64) {
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
	e := &event{
		Name:  "SegmentEmerged",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"seqNo":         seqNo,
			"sincePrevious": uint64(sincePrevious / time.Millisecond),
		},
	}

	sendPost(e)
}

func SegmentUploadStart(nonce, seqNo uint64) {
	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight++
	}
}

func LogSegmentUploaded(nonce, seqNo uint64, uploadDur time.Duration) {
	e := &event{
		Name:  "SegmentUploaded",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"seqNo":          seqNo,
			"uploadDuration": uint64(uploadDur / time.Millisecond),
		},
	}

	sendPost(e)
}

func detectSeqDif(e *event, nonce, seqNo uint64) {
	if metrics.lastSegmentNonce == nonce {
		seqDif := int64(seqNo) - metrics.lastSeqNo
		metrics.lastSeqNo = int64(seqNo)
		if seqDif != 1 {
			e.Properties["seqNoDif"] = seqDif
		}
	}
}

func LogSegmentTranscoded(nonce, seqNo uint64, transcodeDur, totalDur time.Duration) {
	glog.Infof("Logging LogSegmentTranscoded...")
	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight--
	}
	e := &event{
		Name:  "SegmentTranscoded",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"seqNo":             seqNo,
			"transcodeDuration": uint64(transcodeDur / time.Millisecond),
			"totalDuration":     uint64(totalDur / time.Millisecond),
			"segmentsInFlight":  metrics.segmentsInFlight,
		},
	}
	if metrics.segmentsInFlight != 0 {
		e.Properties["segmentsInFlight"] = metrics.segmentsInFlight
	}
	detectSeqDif(e, nonce, seqNo)

	sendPost(e)
}

func LogSegmentTranscodeFailed(nonce, seqNo uint64, reason string) {
	glog.Infof("Logging LogSegmentTranscodeFailed...")
	if metrics.lastSegmentNonce == nonce {
		metrics.segmentsInFlight--
	}
	e := &event{
		Name:  "SegmentTranscodeFailed",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"reason":           reason,
			"seqNo":            seqNo,
			"segmentsInFlight": metrics.segmentsInFlight,
		},
	}
	detectSeqDif(e, nonce, seqNo)

	sendPost(e)
}

func LogStartBroadcastClientFailed(nonce uint64, serviceURI, transcoderAddress string, jobID uint64, reason string) {
	glog.Infof("Logging LogStartBroadcastClientFailed...")
	e := &event{
		Name:  "StartBroadcastClientFailed",
		Nonce: strconv.FormatUint(nonce, 10),
		Properties: map[string]interface{}{
			"jobID":             jobID,
			"serviceURI":        serviceURI,
			"reason":            reason,
			"transcoderAddress": transcoderAddress,
		},
	}

	sendPost(e)
}

func sendPost(e *event) {
	if eventsURLBase == "" {
		return
	}
	go _sendPost(e)
}

func _sendPost(e *event) {
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
