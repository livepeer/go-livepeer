package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"
)

func startStreamTricklePublish(ctx context.Context, url *url.URL, streamInfo *core.StreamInfo) {
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	params := streamInfo.Params.(aiRequestParams)

	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("trickle publish init err: %w", err))
		return
	}

	// Start payments which probes a segment every "paymentProcessInterval" and sends a payment
	ctx, cancel := context.WithCancel(ctx)
	var paymentProcessor *LivePaymentProcessor
	//byoc sets as context values
	orchAddr := clog.GetVal(ctx, "orch")
	orchUrl := clog.GetVal(ctx, "orch_url")

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
			streamInfo.ExcludeOrch(orchUrl) //suspendOrchestrator(ctx, params)
			cancel()
			stopProcessing(ctx, params, errors.New("orchestrator is slow"))
			return
		}
		go func(seq int) {
			defer slowOrchChecker.EndSegment()
			var r io.Reader = reader
			if paymentProcessor != nil {
				r = paymentProcessor.process(ctx, reader)
			}

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
				logToDisk(ctx, reader, params.node.WorkDir, params.liveParams.requestID, seq)
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

func startStreamTrickleSubscribe(ctx context.Context, url *url.URL, streamInfo *core.StreamInfo) {
	// subscribe to inference outputs and send them into the world
	params := streamInfo.Params.(aiRequestParams)
	orchAddr := clog.GetVal(ctx, "orch")
	orchUrl := clog.GetVal(ctx, "orch_url")
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
	// Launch ffmpeg for each configured RTMP output
	for _, outURL := range params.liveParams.rtmpOutputs {
		go ffmpegOutput(ctx, outURL, outWriter, params)
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
			if !params.inputStreamExists() {
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
				if segmentAge < maxSegmentDelay && params.inputStreamExists() {
					// we have some recent input but no output from orch, so kick
					suspendOrchestrator(ctx, params)
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
					monitor.AIFirstSegmentDelay(delayMs, params.liveParams.sess.OrchestratorInfo)
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
			clog.V(8).Info(ctx, "trickle subscribe read data completed", "seq", seq, "bytes", humanize.Bytes(uint64(n)), "took", time.Since(copyStartTime))
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
				if hasRecentInput && streamInfo.IsActive() {
					// abandon the orchestrator
					streamInfo.ExcludeOrch(orchUrl)
					stopProcessing(ctx, params, fmt.Errorf("timeout waiting for segments"))
					segmentTicker.Stop()
					return
				}
			}
		}
	}()

}

func startStreamControlPublish(ctx context.Context, control *url.URL, streamInfo *core.StreamInfo) {
	params := streamInfo.Params.(aiRequestParams)
	controlPub, err := trickle.NewTricklePublisher(control.String())
	if err != nil {
		stopProcessing(ctx, params, fmt.Errorf("error starting control publisher, err=%w", err))
		return
	}
	params.node.LiveMu.Lock()
	defer params.node.LiveMu.Unlock()

	ticker := time.NewTicker(10 * time.Second)
	done := make(chan bool, 1)
	once := sync.Once{}
	stop := func() {
		once.Do(func() {
			ticker.Stop()
			done <- true
		})
	}

	//sess, exists := params.node.LivePipelines[stream]
	//if !exists || sess.RequestID != params.liveParams.requestID {
	//	stopProcessing(ctx, params, fmt.Errorf("control session did not exist"))
	//	return
	//}
	//if sess.ControlPub != nil {
	//	// clean up from existing orchestrator
	//	go sess.ControlPub.Close()
	//}
	streamInfo.ControlPub = controlPub
	streamInfo.StopControl = stop

	if monitor.Enabled {
		monitorCurrentLiveSessions(params.node.LivePipelines)
	}

	// Send any cached control params in a goroutine outside the lock.
	msg := streamInfo.JobParams
	go func() {
		if msg == "" {
			return
		}
		var err error
		for i := 0; i < 3; i++ {
			err = controlPub.Write(strings.NewReader(msg))
			if err == nil {
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

func startStreamDataSubscribe(ctx context.Context, url *url.URL, streamInfo *core.StreamInfo) {
	//only start DataSubscribe if enabled
	params := streamInfo.Params.(aiRequestParams)
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
		// we're trying to keep (retryPause x maxRetries) duration to fall within one output GOP length
		const retryPause = 300 * time.Millisecond
		const maxRetries = 5
		for {
			select {
			case <-ctx.Done():
				clog.Info(ctx, "data subscribe done")
				return
			default:
			}
			if !params.inputStreamExists() {
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
					monitor.AIFirstSegmentDelay(delayMs, params.liveParams.sess.OrchestratorInfo)
					monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
						"type":        "gateway_receive_first_data_segment",
						"timestamp":   time.Now().UnixMilli(),
						"stream_id":   params.liveParams.streamID,
						"pipeline_id": params.liveParams.pipelineID,
						"request_id":  params.liveParams.requestID,
						"orchestrator_info": map[string]interface{}{
							"address": params.liveParams.sess.Address(),
							"url":     params.liveParams.sess.Transcoder(),
						},
					})
				}
			}

			clog.V(8).Info(ctx, "data subscribe read completed", "seq", seq, "bytes", humanize.Bytes(uint64(readBytes)), "messages", readMessages, "took", time.Since(copyStartTime))
		}
	}()
}
