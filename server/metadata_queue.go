package server

import (
	"context"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/livepeer-data/pkg/event"
	"github.com/livepeer/lpms/stream"
)

const queuePublishTimeout = 1 * time.Second

var MetadataQueue event.Producer

func BackgroundPublish(queue event.Producer, key string, body interface{}) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), queuePublishTimeout)
		defer cancel()
		if err := queue.Publish(ctx, key, body, false); err != nil {
			glog.Errorf("Error publishing event: key=%q, err=%q", key, err)
		}
	}()
}

// Event Types

type OrchestratorMetadata struct {
	Address       string `json:"address"`
	TranscoderUri string `json:"transcodeUri"`
}

type TranscodeAttemptInfo struct {
	Orchestrator OrchestratorMetadata `json:"orchestrator"`
	LatencyMs    int64                `json:"latencyMs"`
	Error        *string              `json:"error"`
}

type SegmentMetadata struct {
	Name     string  `json:"name"`
	SeqNo    uint64  `json:"seqNo"`
	Duration float64 `json:"duration"`
	ByteSize int     `json:"byteSize"`
}

type StreamHealthTranscodeEvent struct {
	NodeID     string                 `json:"nodeId"`
	ManifestID string                 `json:"manifestId"`
	Segment    SegmentMetadata        `json:"segment"`
	Timestamp  int64                  `json:"timestamp"`
	StartTime  int64                  `json:"startTime"`
	LatencyMs  int64                  `json:"latencyMs"`
	Success    bool                   `json:"success"`
	Attempts   []TranscodeAttemptInfo `json:"attempts"`
}

func NewStreamHealthTranscodeEvent(mid string, seg *stream.HLSSegment, startTime time.Time, success bool, attempts []TranscodeAttemptInfo) StreamHealthTranscodeEvent {
	now := time.Now()
	return StreamHealthTranscodeEvent{
		NodeID:     monitor.NodeID,
		ManifestID: string(mid),
		Segment: SegmentMetadata{
			Name:     seg.Name,
			SeqNo:    seg.SeqNo,
			Duration: seg.Duration,
			ByteSize: len(seg.Data),
		},
		Timestamp: now.UnixNano(),
		StartTime: startTime.UnixNano(),
		LatencyMs: now.Sub(startTime).Milliseconds(),
		Success:   success,
		Attempts:  attempts,
	}
}
