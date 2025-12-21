package byoc

//NOTE: This file is largely duplicated code from server/ai_live_video.go.  Main modifications are to interact directly
// with BYOC server structures rather than core LivepeerNode structures.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"

	"github.com/dustin/go-humanize"
)

const (
	recentSwapInterval  = 3 * time.Minute
	maxRecentSwapsCount = 2
)

var (
	liveAISaveNSegments = 10
)

func (bsg *BYOCGatewayServer) startTricklePublish(ctx context.Context, url *url.URL, params byocAIRequestParams, orchAddr string, orchUrl string) {
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("trickle publish init err: %w", err))
		return
	}
	ctx, cancel := context.WithCancel(ctx)

	// TODO: implement live payment sender for BYOC streaming
	// Start payments which probes a segment every "paymentProcessInterval" and sends a payment
	//priceInfo := sess.OrchestratorInfo.PriceInfo
	//var paymentProcessor *LivePaymentProcessor
	// Only start payment processor if we have valid price info and auth token
	// BYOC does not require AuthToken for payment, so this will skip the live payment processor for BYOC streaming
	//if priceInfo != nil && priceInfo.PricePerUnit != 0 && sess.OrchestratorInfo.AuthToken != nil {
	//	paymentSender := livePaymentSender{}
	//	sendPaymentFunc := func(inPixels int64) error {
	//		return paymentSender.SendPayment(context.Background(), &SegmentInfoSender{
	//			sess:      sess.BroadcastSession,
	//			inPixels:  inPixels,
	//			priceInfo: priceInfo,
	//			mid:       params.liveParams.manifestID,
	//		})
	//	}
	//	paymentProcessor = NewLivePaymentProcessor(ctx, params.liveParams.paymentProcessInterval, sendPaymentFunc)
	//} else {
	//	clog.Warningf(ctx, "No price info found from Orchestrator, Gateway will not send payments for the video processing")
	//}

	slowOrchChecker := &SlowOrchChecker{}

	firstSegment := true

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
			bsg.suspendOrchestrator(ctx, params)
			cancel()
			stopProcessing(ctx, params, errors.New("orchestrator is slow"))
			return
		}
		go func(seq int) {
			defer slowOrchChecker.EndSegment()
			var r io.Reader = reader
			//if paymentProcessor != nil {
			//	paymentProcessor.process(ctx)
			//}

			clog.V(8).Infof(ctx, "trickle publish writing data seq=%d", seq)
			segment, err := publisher.Next()
			if err != nil {
				clog.Infof(ctx, "error getting next publish handle; dropping segment err=%v", err)
				params.liveParams.sendErrorEvent(fmt.Errorf("Missing next handle %v", err))
				return
			}
			for {
				select {
				case <-ctx.Done():
					clog.Info(ctx, "trickle publish done")
					return
				default:
				}

				startTime := time.Now()
				currentSeq := slowOrchChecker.GetCount()
				if seq != currentSeq {
					clog.Infof(ctx, "Next segment has already started; skipping this one seq=%d currentSeq=%d", seq, currentSeq)
					params.liveParams.sendErrorEvent(fmt.Errorf("Next segment has started"))
					segment.Close()
					return
				}
				params.liveParams.mu.Lock()
				params.liveParams.lastSegmentTime = startTime
				params.liveParams.mu.Unlock()
				logToDisk(ctx, reader, params.node.WorkDir, params.liveParams.requestID, params.liveParams.manifestID, seq)
				n, err := segment.Write(r)
				if err == nil {
					// no error, all done, let's leave
					if monitor.Enabled && firstSegment {
						firstSegment = false
						monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
							"type":        "gateway_send_first_ingest_segment",
							"timestamp":   time.Now().UnixMilli(),
							"stream_id":   params.liveParams.streamID,
							"pipeline_id": params.liveParams.pipelineID,
							"request_id":  params.liveParams.requestID,
							"orchestrator_info": map[string]interface{}{
								"address": orchAddr,
								"url":     orchUrl,
							},
						})
					}
					clog.Info(ctx, "trickle publish complete", "wrote", humanize.Bytes(uint64(n)), "seq", seq, "took", time.Since(startTime))
					return
				}
				if errors.Is(err, trickle.StreamNotFoundErr) {
					stopProcessing(ctx, params, errors.New("stream no longer exists on orchestrator; terminating"))
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
				// Clone in case read head was incremented somewhere, which cloning resets
				r = reader.Clone()
				time.Sleep(250 * time.Millisecond)
			}
		}(thisSeq)
	})
	clog.Infof(ctx, "trickle pub")
}

func (bsg *BYOCGatewayServer) suspendOrchestrator(ctx context.Context, params byocAIRequestParams) {
	if !bsg.streamPipelineExists(params.liveParams.streamID) {
		// If the ingest was closed, then do not suspend the orchestrator
		return
	}

	//noop right now, with local gateways this would be in the client implementation
}

func (bsg *BYOCGatewayServer) startTrickleSubscribe(ctx context.Context, url *url.URL, params byocAIRequestParams, orchAddr string, orchUrl string) {
	// subscribe to inference outputs and send them into the world

	subscriber, err := trickle.NewTrickleSubscriber(trickle.TrickleSubscriberConfig{
		URL: url.String(),
		Ctx: ctx,
	})
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("trickle subscription init failed: %w", err))
		return
	}

	ctx = clog.AddVal(ctx, "url", url.Redacted())

	// Set up output buffers and ffmpeg processes
	rbc := media.RingBufferConfig{BufferLen: 5_000_000} // 5 MB, 20-30 seconds at current rates
	outWriter, err := media.NewRingBuffer(&rbc)
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("ringbuffer init failed: %w", err))
		return
	}
	bsg.setOutWriter(ctx, outWriter, params) // for WHEP
	// Launch ffmpeg for each configured RTMP output
	for _, outURL := range params.liveParams.rtmpOutputs {
		go bsg.ffmpegOutput(ctx, outURL, outWriter, params)
	}

	// watchdog that gets reset on every segment to catch output stalls
	segmentTimeout := params.liveParams.outSegmentTimeout
	if segmentTimeout <= 0 {
		segmentTimeout = 30 * time.Second
	}
	segmentTicker := time.NewTicker(segmentTimeout)

	// read segments from trickle subscription
	go func() {
		defer outWriter.Close()
		defer segmentTicker.Stop()

		var err error
		firstSegment := true
		var segmentsReceived int64

		retries := 0
		// we're trying to keep (retryPause x maxRetries) duration to fall within one output GOP length
		const retryPause = 300 * time.Millisecond
		const maxRetries = 5
		for {
			select {
			case <-ctx.Done():
				clog.Info(ctx, "trickle subscribe done")
				return
			default:
			}
			if !bsg.streamPipelineExists(params.liveParams.streamID) {
				clog.Infof(ctx, "trickle subscribe stopping, input stream does not exist.")
				break
			}
			segmentTicker.Reset(segmentTimeout) // reset ticker on each iteration.
			var segment *http.Response
			clog.V(8).Infof(ctx, "trickle subscribe read data await")
			segment, err = subscriber.Read()
			if err != nil {
				if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
					stopProcessing(ctx, params, fmt.Errorf("trickle subscribe stopping, stream not found, err=%w", err))
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
					stopProcessing(ctx, params, errors.New("trickle subscribe stopping, retries exceeded"))
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
			copyStartTime := time.Now()

			n, err := copySegment(ctx, segment, outWriter, seq, params)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					clog.Info(ctx, "trickle subscribe stopping - context canceled")
					return
				}
				// Check whether the client has sent data recently.
				// TODO ensure the threshold is some multiple of LIVE_AI_MIN_SEG_DUR
				params.liveParams.mu.Lock()
				lastSegmentTime := params.liveParams.lastSegmentTime
				params.liveParams.mu.Unlock()
				segmentAge := time.Since(lastSegmentTime)
				maxSegmentDelay := params.liveParams.outSegmentTimeout / 2
				if segmentAge < maxSegmentDelay && bsg.streamPipelineExists(params.liveParams.streamID) {
					// we have some recent input but no output from orch, so kick
					bsg.suspendOrchestrator(ctx, params)
					stopProcessing(ctx, params, fmt.Errorf("trickle subscribe error, swapping: %w", err))
					return
				}
				clog.InfofErr(ctx, "trickle subscribe error copying segment seq=%d", seq, err)
				subscriber.SetSeq(seq)
				retries++
				continue
			}
			if firstSegment {
				firstSegment = false
				delayMs := time.Since(params.liveParams.startTime).Milliseconds()
				if monitor.Enabled {
					//monitor.AIFirstSegmentDelay(delayMs, params.liveParams.sess.OrchestratorInfo)
					monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
						"type":        "gateway_receive_first_processed_segment",
						"timestamp":   time.Now().UnixMilli(),
						"stream_id":   params.liveParams.streamID,
						"pipeline_id": params.liveParams.pipelineID,
						"request_id":  params.liveParams.requestID,
						"orchestrator_info": map[string]interface{}{
							"address": orchAddr,
							"url":     orchUrl,
						},
					})
				}
				clog.V(common.VERBOSE).Infof(ctx, "First Segment delay=%dms streamID=%s", delayMs, params.liveParams.streamID)
			}
			segmentsReceived += 1
			if segmentsReceived == 3 && monitor.Enabled {
				// We assume that after receiving 3 segments, the runner started successfully
				// and we should be able to start the playback
				monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
					"type":        "gateway_receive_few_processed_segments",
					"timestamp":   time.Now().UnixMilli(),
					"stream_id":   params.liveParams.streamID,
					"pipeline_id": params.liveParams.pipelineID,
					"request_id":  params.liveParams.requestID,
					"orchestrator_info": map[string]interface{}{
						"address": orchAddr,
						"url":     orchUrl,
					},
				})

			}
			clog.Info(ctx, "trickle subscribe read data completed", "seq", seq, "bytes", humanize.Bytes(uint64(n)), "took", time.Since(copyStartTime))
		}
	}()

	// watchdog: fires if orch does not produce segments for too long
	go func() {
		for {
			select {
			case <-segmentTicker.C:
				// check whether this timeout is due to missing input
				// only suspend orchestrator if there is recent input
				// ( no input == no output, so don't suspend for that )
				params.liveParams.mu.Lock()
				lastInputSegmentTime := params.liveParams.lastSegmentTime
				params.liveParams.mu.Unlock()
				lastInputSegmentAge := time.Since(lastInputSegmentTime)
				hasRecentInput := lastInputSegmentAge < segmentTimeout/2
				if hasRecentInput && bsg.streamPipelineExists(params.liveParams.streamID) {
					// abandon the orchestrator
					bsg.suspendOrchestrator(ctx, params)
					stopProcessing(ctx, params, fmt.Errorf("timeout waiting for segments"))
					segmentTicker.Stop()
					return
				}
			}
		}
	}()

}

func (bsg *BYOCGatewayServer) ffmpegOutput(ctx context.Context, outputUrl string, outWriter *media.RingBuffer, params byocAIRequestParams) {
	// Clone the context since we can call this function multiple times
	// Adding rtmpOut val multiple times to the same context will just stomp over old ones
	ctx = clog.Clone(ctx, ctx)
	ctx = clog.AddVal(ctx, "rtmpOut", outputUrl)

	defer func() {
		if rec := recover(); rec != nil {
			// panicked, so shut down the stream and handle it
			err, ok := rec.(error)
			if !ok {
				err = errors.New("unknown error")
			}
			stopProcessing(ctx, params, fmt.Errorf("ffmpeg panic: %w", err))
		}
	}()
	for {
		clog.V(6).Infof(ctx, "Starting output rtmp")
		if !bsg.streamPipelineExists(params.liveParams.streamID) {
			clog.Errorf(ctx, "Stopping output rtmp stream, input stream does not exist.")
			break
		}

		// we receive opus by default, but re-encode to AAC for non-local outputs
		acodec := "copy"
		if params.liveParams.localRTMPPrefix != "" && !strings.Contains(outputUrl, params.liveParams.localRTMPPrefix) {
			acodec = "libfdk_aac"
		}

		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-analyzeduration", "2500000", // 2.5 seconds
			"-i", "pipe:0",
			"-c:a", acodec,
			"-c:v", "copy",
			"-f", "flv",
			outputUrl,
		)
		// Change Cancel function to send a SIGTERM instead of SIGKILL. Still send a SIGKILL after 5s (WaitDelay) if it's stuck.
		cmd.Cancel = func() error {
			return cmd.Process.Signal(syscall.SIGTERM)
		}
		cmd.WaitDelay = 5 * time.Second
		cmd.Stdin = outWriter.MakeReader() // start at leading edge of output for each retry
		output, err := cmd.CombinedOutput()
		clog.Infof(ctx, "Process err=%v output: %s", err, output)

		select {
		case <-ctx.Done():
			clog.Info(ctx, "Context done, stopping rtmp output")
			return // Returns context.Canceled or context.DeadlineExceeded
		default:
			// Context is still active, continue with normal processing
		}

		time.Sleep(5 * time.Second)
	}
}

func copySegment(ctx context.Context, segment *http.Response, w io.Writer, seq int, params byocAIRequestParams) (int64, error) {
	defer segment.Body.Close()
	var reader io.Reader = segment.Body
	if seq < liveAISaveNSegments {
		p := filepath.Join(params.node.WorkDir, fmt.Sprintf("%s-%s-out-%d.ts", params.liveParams.requestID, params.liveParams.manifestID, seq))
		outFile, err := os.Create(p)
		if err != nil {
			clog.Info(ctx, "Could not create output segment file for logging", "err", err)
		} else {
			defer outFile.Close()
			reader = io.TeeReader(segment.Body, outFile)
		}
	}

	timeout := params.liveParams.outSegmentTimeout
	if timeout <= 0 {
		return io.Copy(w, reader)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		n   int64
		err error
	}

	resultChan := make(chan result, 1)
	go func() {
		n, err := io.Copy(w, reader)
		resultChan <- result{n, err}
	}()

	select {
	case <-ctx.Done():
		// NB: if the orch context is cancelled, it isn't really a timeout
		return 0, fmt.Errorf("copy operation timed out: %w", ctx.Err())
	case res := <-resultChan:
		return res.n, res.err
	}
}

func (bsg *BYOCGatewayServer) setOutWriter(ctx context.Context, writer *media.RingBuffer, params byocAIRequestParams) {
	streamId, requestID := params.liveParams.streamID, params.liveParams.requestID
	stream, err := bsg.streamPipeline(streamId)

	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	if err != nil || stream.RequestID != requestID {
		clog.Info(ctx, "Did not set output writer due to nonexistent stream or mismatched request ID", "exists", err == nil, "requestID", requestID, "session-requestID", stream.RequestID)
		return
	}
	stream.OutWriter = writer
	stream.OutCond.Broadcast()
}

func (bsg *BYOCGatewayServer) startControlPublish(ctx context.Context, control *url.URL, params byocAIRequestParams, orchAddr string, orchUrl string) {
	controlPub, err := trickle.NewTricklePublisher(control.String())
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("error starting control publisher, err=%w", err))
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	done := make(chan bool, 1)
	once := sync.Once{}
	stop := func() {
		once.Do(func() {
			ticker.Stop()
			done <- true
		})
	}

	reportUpdate := func(data []byte) {
		// send the param update to kafka
		monitor.SendQueueEventAsync("ai_stream_events", map[string]interface{}{
			"type":        "params_update",
			"stream_id":   params.liveParams.streamID,
			"request_id":  params.liveParams.requestID,
			"pipeline":    params.liveParams.pipeline,
			"pipeline_id": params.liveParams.pipelineID,
			"params":      json.RawMessage(data),
			"orchestrator_info": map[string]interface{}{
				"address": orchAddr,
				"url":     orchUrl,
			},
		})
	}

	stream, err := bsg.streamPipeline(params.liveParams.streamID)
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("control session did not exist"))
		return
	}
	if err != nil || stream.RequestID != params.liveParams.requestID {
		stopProcessing(ctx, params, fmt.Errorf("control session did not exist"))
		return
	}

	bsg.mu.Lock()
	if stream.ControlPub != nil {
		// clean up from existing orchestrator
		go stream.ControlPub.Close()
	}
	stream.ControlPub = controlPub
	stream.StopControl = stop
	stream.ReportUpdate = reportUpdate
	bsg.mu.Unlock()

	if monitor.Enabled {
		bsg.monitorCurrentLiveSessions()
	}

	// Send any cached control params in a goroutine outside the lock.
	msg := stream.Params

	go func() {
		if msg == nil {
			return
		}
		var err error
		for i := 0; i < 3; i++ {
			err = controlPub.Write(bytes.NewReader(msg))
			if err == nil {
				reportUpdate(msg)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		stopProcessing(ctx, params, fmt.Errorf("control write failed: %w", err))
	}()

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
					stopProcessing(ctx, params, errors.New("control channel does not exist"))
					continue // loop back to consume the `done` chan
				}
				// if there was another type of error, we'll just retry anyway
			case <-done:
				return
			case <-ctx.Done():
				stop()
			}
		}
	}()
}

const clearStreamDelay = 1 * time.Minute

func (bsg *BYOCGatewayServer) startEventsSubscribe(
	ctx context.Context,
	url *url.URL,
	params byocAIRequestParams,
	orchAddr string,
	orchUrl string,
) {
	subscriber, err := trickle.NewTrickleSubscriber(trickle.TrickleSubscriberConfig{
		URL: url.String(),
		Ctx: ctx,
	})
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("event sub init failed: %w", err))
		return
	}

	streamId := params.liveParams.streamID

	const (
		eventCheckInterval = 10 * time.Second
		maxEventGap        = 30 * time.Second
		maxRetries         = 5
		retryPause         = 300 * time.Millisecond
	)

	eventTicker := time.NewTicker(eventCheckInterval)
	eventsDone := make(chan struct{}, 1)

	var (
		lastEventMu sync.Mutex
		lastEvent   = time.Now()
	)

	clog.Infof(ctx, "Starting event subscription for URL: %s", url.String())

	// Clear stream state after delay unless canceled
	go func() {
		select {
		case <-time.After(clearStreamDelay):
			bsg.statusStore.Clear(streamId)
		case <-ctx.Done():
		}
	}()

	// Event reader goroutine
	go func() {
		defer func() {
			eventTicker.Stop()
			select {
			case eventsDone <- struct{}{}:
			default:
			}
		}()

		retries := 0

		for {
			select {
			case <-ctx.Done():
				clog.Info(ctx, "event subscription done")
				return
			default:
			}

			clog.Infof(ctx, "Reading from event subscription for URL: %s", url.String())
			segment, err := subscriber.Read()
			if err != nil {
				if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
					clog.Infof(ctx, "Stopping subscription due to %s", err)
					return
				}

				var seqErr *trickle.SequenceNonexistent
				if errors.As(err, &seqErr) {
					subscriber.SetSeq(seqErr.Latest)
				}

				if retries >= maxRetries {
					stopProcessing(ctx, params, fmt.Errorf(
						"too many errors reading events; stopping subscription: %w", err,
					))
					return
				}

				clog.Infof(ctx, "Error reading events subscription: err=%v retry=%d", err, retries)
				retries++

				select {
				case <-time.After(retryPause):
				case <-ctx.Done():
					return
				}
				continue
			}

			retries = 0

			body, err := io.ReadAll(segment.Body)
			segment.Body.Close()
			if err != nil {
				clog.Infof(ctx, "Error reading events subscription body: %s", err)
				continue
			}

			var eventWrapper struct {
				QueueEventType string                 `json:"queue_event_type"`
				Event          map[string]interface{} `json:"event"`
			}
			if err := json.Unmarshal(body, &eventWrapper); err != nil {
				clog.Infof(ctx, "Failed to parse JSON from events subscription: %s", err)
				continue
			}

			event := eventWrapper.Event
			queueEventType := eventWrapper.QueueEventType
			if event == nil {
				event = make(map[string]interface{})
				if err := json.Unmarshal(body, &event); err != nil {
					clog.Infof(ctx, "Failed to parse JSON as direct event: %s", err)
					continue
				}
				queueEventType = "ai_stream_events"
			}

			event["stream_id"] = streamId
			event["request_id"] = params.liveParams.requestID
			event["pipeline_id"] = params.liveParams.pipelineID
			event["orchestrator_info"] = map[string]interface{}{
				"address": orchAddr,
				"url":     orchUrl,
			}

			clog.V(8).Infof(ctx, "Received event for seq=%d event=%+v",
				trickle.GetSeq(segment), event,
			)

			lastEventMu.Lock()
			lastEvent = time.Now()
			lastEventMu.Unlock()

			eventType, ok := event["type"].(string)
			if !ok {
				eventType = "unknown"
				clog.Warningf(ctx, "Received event without a type stream=%s event=%+v",
					streamId, event,
				)
			}

			if eventType == "status" {
				queueEventType = "ai_stream_status"

				lastStreamStatus, _ := bsg.statusStore.Get(streamId)
				inferenceStatus, hasInference := event["inference_status"].(map[string]interface{})
				lastInferenceStatus, hasLastInference := lastStreamStatus["inference_status"].(map[string]interface{})

				if hasInference {
					if logs, ok := inferenceStatus["last_restart_logs"]; !ok || logs == nil {
						if hasLastInference {
							inferenceStatus["last_restart_logs"] =
								lastInferenceStatus["last_restart_logs"]
						}
					}
					if params, ok := inferenceStatus["last_params"]; !ok || params == nil {
						if hasLastInference {
							inferenceStatus["last_params"] =
								lastInferenceStatus["last_params"]
						}
					}
				}

				bsg.statusStore.Store(streamId, event)
			}

			monitor.SendQueueEventAsync(queueEventType, event)
		}
	}()

	// Heartbeat watchdog
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-eventsDone:
				return
			case <-eventTicker.C:
				lastEventMu.Lock()
				eventTime := lastEvent
				lastEventMu.Unlock()

				if time.Since(eventTime) > maxEventGap {
					stopProcessing(ctx, params, fmt.Errorf("timeout waiting for events"))
					return
				}
			}
		}
	}()
}

func (bsg *BYOCGatewayServer) getOutWriter(streamId string) (*media.RingBuffer, string) {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	stream, err := bsg.streamPipeline(streamId)
	if err != nil || stream.Closed {
		return nil, ""
	}
	// could be nil if we haven't gotten an orchestrator yet
	for stream.OutWriter == nil {
		stream.OutCond.Wait()
		if stream.Closed {
			return nil, ""
		}
	}
	return stream.OutWriter, stream.RequestID
}

func (bsg *BYOCGatewayServer) startDataSubscribe(ctx context.Context, url *url.URL, params byocAIRequestParams, orchAddr string, orchUrl string) {
	//only start DataSubscribe if enabled
	if params.liveParams.dataWriter == nil {
		return
	}

	// subscribe to the outputs
	subscriber, err := trickle.NewTrickleSubscriber(trickle.TrickleSubscriberConfig{
		URL: url.String(),
		Ctx: ctx,
	})
	if err != nil {
		clog.Infof(ctx, "Failed to create data subscriber: %s", err)
		return
	}

	dataWriter := params.liveParams.dataWriter

	// read segments from trickle subscription
	go func() {
		defer dataWriter.Close()

		var err error
		firstSegment := true

		retries := 0
		// keep similar total duration of (retryPause x maxRetries) similar to startTrickleSubscribe to within one output GOP length
		const retryPause = 300 * time.Millisecond
		const maxRetries = 5
		for {
			select {
			case <-ctx.Done():
				clog.Info(ctx, "data subscribe done")
				return
			default:
			}
			if !params.inputStreamExists(params.liveParams.streamID) {
				clog.Infof(ctx, "data subscribe stopping, input stream does not exist.")
				break
			}
			var segment *http.Response
			readBytes, readMessages := 0, 0
			clog.V(8).Infof(ctx, "data subscribe await")
			segment, err = subscriber.Read()
			if err != nil {
				if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
					stopProcessing(ctx, params, fmt.Errorf("data subscribe stopping, stream not found, err=%w", err))
					return
				}
				var sequenceNonexistent *trickle.SequenceNonexistent
				if errors.As(err, &sequenceNonexistent) {
					// stream exists but segment doesn't, so skip to leading edge
					subscriber.SetSeq(sequenceNonexistent.Latest)
				}
				// TODO if not EOS then signal a new orchestrator is needed
				err = fmt.Errorf("data subscribe error reading: %w", err)
				clog.Infof(ctx, "%s", err)
				if retries > maxRetries {
					stopProcessing(ctx, params, errors.New("data subscribe stopping, retries exceeded"))
					return
				}
				retries++
				params.liveParams.sendErrorEvent(err)
				time.Sleep(retryPause)
				continue
			}
			retries = 0
			seq := trickle.GetSeq(segment)
			clog.V(8).Infof(ctx, "data subscribe received seq=%d", seq)
			copyStartTime := time.Now()

			defer segment.Body.Close()
			scanner := bufio.NewScanner(segment.Body)
			for scanner.Scan() {
				writer, err := dataWriter.Next()
				clog.V(8).Infof(ctx, "data subscribe writing seq=%d", seq)
				if err != nil {
					if err != io.EOF {
						stopProcessing(ctx, params, fmt.Errorf("data subscribe could not get next: %w", err))
					}
					return
				}
				n, err := writer.Write(scanner.Bytes())
				if err != nil {
					stopProcessing(ctx, params, fmt.Errorf("data subscribe could not write: %w", err))
				}
				readBytes += n
				readMessages += 1

				writer.Close()
			}
			if err := scanner.Err(); err != nil {
				clog.InfofErr(ctx, "data subscribe error reading seq=%d", seq, err)
				subscriber.SetSeq(seq)
				retries++
				continue
			}

			if firstSegment {
				firstSegment = false
				delayMs := time.Since(params.liveParams.startTime).Milliseconds()
				if monitor.Enabled {
					monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
						"type":        "gateway_receive_first_data_segment",
						"timestamp":   time.Now().UnixMilli(),
						"stream_id":   params.liveParams.streamID,
						"pipeline_id": params.liveParams.pipelineID,
						"request_id":  params.liveParams.requestID,
						"orchestrator_info": map[string]interface{}{
							"address": orchAddr,
							"url":     orchUrl,
						},
					})
				}

				clog.V(common.VERBOSE).Infof(ctx, "First Data Segment delay=%dms streamID=%s", delayMs, params.liveParams.streamID)
			}

			clog.V(8).Info(ctx, "data subscribe read completed", "seq", seq, "bytes", humanize.Bytes(uint64(readBytes)), "messages", readMessages, "took", time.Since(copyStartTime))
		}
	}()
}

func (bs *BYOCGatewayServer) LiveErrorEventSender(ctx context.Context, streamID string, event map[string]string) func(err error) {
	return func(err error) {
		bs.statusStore.StoreIfNotExists(streamID, "error", map[string]interface{}{
			"error_message": err.Error(),
			"error_time":    time.Now().UnixMilli(),
		})

		ev := maps.Clone(event)
		ev["capability"] = clog.GetVal(ctx, "capability")
		ev["message"] = err.Error()
		monitor.SendQueueEventAsync("ai_stream_events", ev)
	}
}

func logToDisk(ctx context.Context, r media.CloneableReader, workdir string, requestID, manifestID string, seq int) {
	// NB these segments are cleaned up periodically by the temp file sweeper in rtmp2segment
	if seq > liveAISaveNSegments {
		return
	}
	go func() {
		reader := r.Clone()
		p := filepath.Join(workdir, fmt.Sprintf("%s-%s-%d.ts", requestID, manifestID, seq))
		file, err := os.Create(p)
		if err != nil {
			clog.InfofErr(ctx, "Could not create segment file for logging", err)
			return
		}
		defer file.Close()
		_, err = io.Copy(file, reader)
		if err != nil {
			clog.InfofErr(ctx, "Could not log segment", err)
			return
		}
	}()
}
