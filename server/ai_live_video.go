package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/lpms/ffmpeg"
)

func startTricklePublish(ctx context.Context, url *url.URL, params aiRequestParams, sess *AISession) {
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		clog.Infof(ctx, "error publishing trickle. err=%s", err)
	}

	// Start payments which probes a segment every "paymentProcessInterval" and sends a payment
	ctx, cancel := context.WithCancel(context.Background())
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

	params.liveParams.segmentReader.SwitchReader(func(reader media.CloneableReader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				clog.Infof(ctx, "Error closing trickle publisher. err=%s", err)
			}
			cancel()
			return
		}
		go func() {
			var r io.Reader = reader
			if paymentProcessor != nil {
				r = paymentProcessor.process(reader)
			}

			clog.V(8).Infof(ctx, "trickle publish writing data")
			// TODO this blocks! very bad!
			if err := publisher.Write(r); err != nil {
				clog.Infof(ctx, "Error writing to trickle publisher. err=%s", err)
			}
		}()
	})
	clog.Infof(ctx, "trickle pub")
}

func startTrickleSubscribe(ctx context.Context, url *url.URL, params aiRequestParams) {
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
			segment, err = subscriber.Read()
			if err != nil {
				if errors.Is(err, trickle.EOS) {
					params.liveParams.stopPipeline(fmt.Errorf("trickle subscribe end of stream: %w", err))
					return
				}
				// TODO if not EOS then signal a new orchestrator is needed
				err = fmt.Errorf("trickle subscribe error reading: %w", err)
				clog.Infof(ctx, "%s", err)
				if retries > maxRetries {
					params.liveParams.stopPipeline(err)
					return
				}
				retries++
				time.Sleep(retryPause)
				continue
			}
			retries = 0
			clog.V(8).Infof(ctx, "trickle subscribe read data")

			if err = copySegment(segment, w); err != nil {
				params.liveParams.stopPipeline(fmt.Errorf("trickle subscribe error copying: %w", err))
				return
			}
		}
	}()

	go func() {
		defer r.Close()
		for {
			if !params.inputStreamExists() {
				clog.Errorf(ctx, "Stopping output rtmp stream, input stream does not exist.")
				break
			}

			_, err = ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
				Fname: fmt.Sprintf("pipe:%d", r.Fd()),
			}, []ffmpeg.TranscodeOptions{{
				Oname:        params.liveParams.outputRTMPURL,
				AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
				VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
				Muxer:        ffmpeg.ComponentOptions{Name: "flv"},
			}})
			if err != nil {
				clog.Infof(ctx, "Error sending RTMP out: %s", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func copySegment(segment *http.Response, w io.Writer) error {
	defer segment.Body.Close()
	_, err := io.Copy(w, segment.Body)
	return err
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

func startEventsSubscribe(ctx context.Context, url *url.URL, params aiRequestParams) {
	subscriber := trickle.NewTrickleSubscriber(url.String())
	stream := params.liveParams.stream
	streamId := params.liveParams.streamID

	clog.Infof(ctx, "Starting event subscription for URL: %s", url.String())

	go func() {
		defer StreamStatusStore.Clear(streamId)
		for {
			clog.Infof(ctx, "Reading from event subscription for URL: %s", url.String())
			segment, err := subscriber.Read()
			if err != nil {
				clog.Infof(ctx, "Error reading events subscription: %s", err)
				return
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

			clog.Infof(ctx, "Received event for stream=%s event=%+v", stream, event)

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
				if logs, ok := event["last_restart_logs"]; !ok || logs == nil {
					event["last_restart_logs"] = lastStreamStatus["last_restart_logs"]
				}
				if params, ok := event["last_params"]; !ok || params == nil {
					event["last_params"] = lastStreamStatus["last_params"]
				}
				StreamStatusStore.Store(streamId, event)
			}

			monitor.SendQueueEventAsync(queueEventType, event)
		}
	}()
}
