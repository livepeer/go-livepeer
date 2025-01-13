package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"

	"github.com/dustin/go-humanize"
)

func startTricklePublish(ctx context.Context, url *url.URL, params aiRequestParams, sess *AISession) {
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		clog.Infof(ctx, "error publishing trickle. err=%s", err)
		params.liveParams.stopPipeline(fmt.Errorf("Error publishing trickle %w", err))
		return
	}

	// Start payments which probes a segment every "paymentProcessInterval" and sends a payment
	ctx, cancel := context.WithCancel(ctx)
	priceInfo := sess.OrchestratorInfo.PriceInfo
	var paymentProcessor *LivePaymentProcessor
	if priceInfo != nil && priceInfo.PricePerUnit != 0 {
		paymentSender := livePaymentSender{}
		sendPaymentFunc := func(inPixels int64) error {
			return paymentSender.SendPayment(context.Background(), &SegmentInfoSender{
				sess:      sess.BroadcastSession,
				inPixels:  inPixels,
				priceInfo: priceInfo,
				mid:       extractMid(url.Path),
			})
		}
		paymentProcessor = NewLivePaymentProcessor(ctx, params.liveParams.paymentProcessInterval, sendPaymentFunc)
	} else {
		clog.Warningf(ctx, "No price info found from Orchestrator, Gateway will not send payments for the video processing")
	}

	slowOrchChecker := &SlowOrchChecker{}

	params.liveParams.segmentReader.SwitchReader(func(reader media.CloneableReader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				clog.Infof(ctx, "Error closing trickle publisher. err=%v", err)
			}
			cancel()
			return
		}
		thisSeq, atMax := slowOrchChecker.BeginSegment()
		if atMax {
			clog.Infof(ctx, "Orchestrator is slow - terminating")
			params.liveParams.stopPipeline(fmt.Errorf("slow orchestrator"))
			cancel()
			return
			// TODO switch orchestrators
		}
		go func(seq int) {
			defer slowOrchChecker.EndSegment()
			var r io.Reader = reader
			if paymentProcessor != nil {
				r = paymentProcessor.process(reader)
			}

			clog.V(8).Infof(ctx, "trickle publish writing data seq=%d", seq)
			segment, err := publisher.Next()
			if err != nil {
				clog.Infof(ctx, "error getting next publish handle; dropping segment err=%v", err)
				params.liveParams.sendErrorEvent(fmt.Errorf("Missing next handle %v", err))
				return
			}
			for {
				currentSeq := slowOrchChecker.GetCount()
				if seq != currentSeq {
					clog.Infof(ctx, "Next segment has already started; skipping this one seq=%d currentSeq=%d", seq, currentSeq)
					params.liveParams.sendErrorEvent(fmt.Errorf("Next segment has started"))
					segment.Close()
					return
				}
				n, err := segment.Write(r)
				if err == nil {
					// no error, all done, let's leave
					return
				}
				if errors.Is(err, trickle.StreamNotFoundErr) {
					clog.Infof(ctx, "Stream no longer exists on orchestrator; terminating")
					params.liveParams.stopPipeline(fmt.Errorf("Stream does not exist"))
					return
				}
				// Retry segment only if nothing has been sent yet
				// and the next segment has not yet started
				// otherwise drop
				if n > 0 {
					clog.Infof(ctx, "Error publishing segment; dropping remainder wrote=%d err=%v", n, err)
					params.liveParams.sendErrorEvent(fmt.Errorf("Error publishing, wrote %d dropping %v", n, err))
					segment.Close()
					return
				}
				clog.Infof(ctx, "Error publishing segment before writing; retrying err=%v", err)
				// Clone in case read head was incremented somewhere, which cloning ressets
				r = reader.Clone()
				time.Sleep(250 * time.Millisecond)
			}
		}(thisSeq)
	})
	clog.Infof(ctx, "trickle pub")
}

func startTrickleSubscribe(ctx context.Context, url *url.URL, params aiRequestParams, onFistSegment func()) {
	// subscribe to the outputs and send them into LPMS
	subscriber := trickle.NewTrickleSubscriber(url.String())
	r, w, err := os.Pipe()
	if err != nil {
		params.liveParams.stopPipeline(fmt.Errorf("error getting pipe for trickle-ffmpeg. url=%s %w", url, err))
		return
	}
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	ctx = clog.AddVal(ctx, "outputRTMPURL", params.liveParams.outputRTMPURL)

	// read segments from trickle subscription
	go func() {
		var err error
		firstSegment := true

		defer w.Close()
		retries := 0
		// we're trying to keep (retryPause x maxRetries) duration to fall within one output GOP length
		const retryPause = 300 * time.Millisecond
		const maxRetries = 5
		for {
			if !params.inputStreamExists() {
				clog.Infof(ctx, "trickle subscribe stopping, input stream does not exist.")
				break
			}
			var segment *http.Response
			clog.V(8).Infof(ctx, "trickle subscribe read data await")
			segment, err = subscriber.Read()
			if err != nil {
				if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
					params.liveParams.stopPipeline(fmt.Errorf("trickle subscribe end of stream: %w", err))
					return
				}
				var sequenceNonexistent *trickle.SequenceNonexistent
				if errors.As(err, &sequenceNonexistent) {
					// stream exists but segment doesn't, so skip to leading edge
					subscriber.SetSeq(sequenceNonexistent.Latest)
				}
				// TODO if not EOS then signal a new orchestrator is needed
				err = fmt.Errorf("trickle subscribe error reading: %w", err)
				clog.Infof(ctx, "%s", err)
				if retries > maxRetries {
					params.liveParams.stopPipeline(err)
					return
				}
				retries++
				params.liveParams.sendErrorEvent(err)
				time.Sleep(retryPause)
				continue
			}
			retries = 0
			seq := trickle.GetSeq(segment)
			clog.V(8).Infof(ctx, "trickle subscribe read data received seq=%d", seq)

			n, err := copySegment(segment, w)
			if err != nil {
				params.liveParams.stopPipeline(fmt.Errorf("trickle subscribe error copying: %w", err))
				return
			}
			if firstSegment {
				firstSegment = false
				onFistSegment()
			}
			clog.V(8).Infof(ctx, "trickle subscribe read data completed seq=%d bytes=%s", seq, humanize.Bytes(uint64(n)))
		}
	}()

	go func() {
		defer func() {
			r.Close()
			if rec := recover(); rec != nil {
				// panicked, so shut down the stream and handle it
				err, ok := rec.(error)
				if !ok {
					err = errors.New("unknown error")
				}
				clog.Errorf(ctx, "LPMS panic err=%v", err)
				params.liveParams.stopPipeline(fmt.Errorf("LPMS panic %w", err))
			}
		}()
		for {
			if !params.inputStreamExists() {
				clog.Errorf(ctx, "Stopping output rtmp stream, input stream does not exist.")
				break
			}

			cmd := exec.Command("ffmpeg",
				"-i", "pipe:0",
				"-c:a", "copy",
				"-c:v", "copy",
				"-f", "flv",
				params.liveParams.outputRTMPURL,
			)
			cmd.Stdin = r
			output, err := cmd.CombinedOutput()
			if err != nil {
				clog.Errorf(ctx, "Error sending RTMP out: %v", err)
				clog.Infof(ctx, "Process output: %s", output)
				return
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func copySegment(segment *http.Response, w io.Writer) (int64, error) {
	defer segment.Body.Close()
	return io.Copy(w, segment.Body)
}

func startControlPublish(control *url.URL, params aiRequestParams) {
	stream := params.liveParams.stream
	controlPub, err := trickle.NewTricklePublisher(control.String())
	if err != nil {
		slog.Info("error starting control publisher", "stream", stream, "err", err)
		return
	}
	params.node.LiveMu.Lock()
	defer params.node.LiveMu.Unlock()

	ticker := time.NewTicker(10 * time.Second)
	done := make(chan bool, 1)
	stop := func() {
		ticker.Stop()
		done <- true
	}

	params.node.LivePipelines[stream] = &core.LivePipeline{
		ControlPub:  controlPub,
		StopControl: stop,
	}
	if monitor.Enabled {
		monitor.AICurrentLiveSessions(len(params.node.LivePipelines))
	}

	// send a keepalive periodically to keep both ends of the connection alive
	go func() {
		for {
			select {
			case <-ticker.C:
				const msg = `{"keep":"alive"}`
				err := controlPub.Write(strings.NewReader(msg))
				if err == trickle.StreamNotFoundErr {
					// the channel doesn't exist anymore, so stop
					stop()
					continue // loop back to consume the `done` chan
				}
				// if there was another type of error, we'll just retry anyway
			case <-done:
				return
			}
		}
	}()
}

func startEventsSubscribe(ctx context.Context, url *url.URL, params aiRequestParams, sess *AISession) {
	subscriber := trickle.NewTrickleSubscriber(url.String())
	stream := params.liveParams.stream
	streamId := params.liveParams.streamID

	clog.Infof(ctx, "Starting event subscription for URL: %s", url.String())

	go func() {
		defer StreamStatusStore.Clear(streamId)
		const maxRetries = 5
		const retryPause = 300 * time.Millisecond
		retries := 0
		for {
			clog.Infof(ctx, "Reading from event subscription for URL: %s", url.String())
			segment, err := subscriber.Read()
			if err == nil {
				retries = 0
			} else {
				// handle errors from event read
				if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
					clog.Infof(ctx, "Stopping subscription due to %s", err)
					return
				}
				var seqErr *trickle.SequenceNonexistent
				if errors.As(err, &seqErr) {
					// stream exists but segment doesn't, so skip to leading edge
					subscriber.SetSeq(seqErr.Latest)
				}
				if retries > maxRetries {
					clog.Infof(ctx, "Too many errors reading events; stopping subscription err=%v", err)
					err = fmt.Errorf("Error reading subscription: %w", err)
					params.liveParams.stopPipeline(err)
					return
				}
				clog.Infof(ctx, "Error reading events subscription: err=%v retry=%d", err, retries)
				retries++
				time.Sleep(retryPause)
				continue
			}

			body, err := io.ReadAll(segment.Body)
			segment.Body.Close()

			if err != nil {
				clog.Infof(ctx, "Error reading events subscription body: %s", err)
				continue
			}

			var event map[string]interface{}
			if err := json.Unmarshal(body, &event); err != nil {
				clog.Infof(ctx, "Failed to parse JSON from events subscription: %s", err)
				continue
			}

			event["stream_id"] = streamId
			event["request_id"] = params.liveParams.requestID
			event["pipeline_id"] = params.liveParams.pipelineID
			if sess != nil {
				event["orchestrator_info"] = map[string]interface{}{
					"address": sess.Address(),
					"url":     sess.Transcoder(),
				}
			}

			clog.V(8).Infof(ctx, "Received event for stream=%s event=%+v", stream, event)

			eventType, ok := event["type"].(string)
			if !ok {
				eventType = "unknown"
				clog.Warningf(ctx, "Received event without a type stream=%s event=%+v", stream, event)
			}

			queueEventType := "ai_stream_events"
			if eventType == "status" {
				queueEventType = "ai_stream_status"
				// The large logs and params fields are only sent once and then cleared to save bandwidth. So coalesce the
				// incoming status with the last non-null value that we received on such fields for the status API.
				lastStreamStatus, _ := StreamStatusStore.Get(streamId)

				// Check if inference_status exists in both current and last status
				inferenceStatus, hasInference := event["inference_status"].(map[string]interface{})
				lastInferenceStatus, hasLastInference := lastStreamStatus["inference_status"].(map[string]interface{})

				if hasInference {
					if logs, ok := inferenceStatus["last_restart_logs"]; !ok || logs == nil {
						if hasLastInference {
							inferenceStatus["last_restart_logs"] = lastInferenceStatus["last_restart_logs"]
						}
					}
					if params, ok := inferenceStatus["last_params"]; !ok || params == nil {
						if hasLastInference {
							inferenceStatus["last_params"] = lastInferenceStatus["last_params"]
						}
					}
				}

				StreamStatusStore.Store(streamId, event)
			}

			monitor.SendQueueEventAsync(queueEventType, event)
		}
	}()
}

// Detect 'slow' orchs by keeping track of in-flight segments
// Count the difference between segments produced and segments completed
type SlowOrchChecker struct {
	mu            sync.Mutex
	segmentCount  int
	completeCount int
}

// Number of in flight segments to allow.
// Should generally not be less than 1, because
// sometimes the beginning of the current segment
// may briefly overlap with the end of the previous segment
const maxInflightSegments = 3

// Returns the number of segments begun so far and
// whether the max number of inflight segments was hit.
// Number of segments is not incremented if inflight max is hit.
// If inflight max is hit, returns true, false otherwise.
func (s *SlowOrchChecker) BeginSegment() (int, bool) {
	// Returns `false` if there are multiple segments in-flight
	// this means the orchestrator is slow reading them
	// If all-OK, returns `true`
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.segmentCount >= s.completeCount+maxInflightSegments {
		// There is > 1 segment in flight ... orchestrator is slow reading
		return s.segmentCount, true
	}
	s.segmentCount += 1
	return s.segmentCount, false
}

func (s *SlowOrchChecker) EndSegment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeCount += 1
}

func (s *SlowOrchChecker) GetCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.segmentCount
}

func LiveErrorEventSender(ctx context.Context, event map[string]string) func(err error) {
	return func(err error) {
		ev := maps.Clone(event)
		ev["capability"] = clog.GetVal(ctx, "capability")
		ev["message"] = err.Error()
		monitor.SendQueueEventAsync("ai_stream_events", ev)
	}
}
