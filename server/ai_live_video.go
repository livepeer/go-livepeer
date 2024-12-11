package server

import (
	"context"
	"encoding/json"
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
		slog.Info("error getting pipe for trickle-ffmpeg", "url", url, "err", err)
	}
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	ctx = clog.AddVal(ctx, "outputRTMPURL", params.liveParams.outputRTMPURL)

	// read segments from trickle subscription
	go func() {
		defer w.Close()
		for {
			segment, err := subscriber.Read()
			if err != nil {
				// TODO if not EOS then signal a new orchestrator is needed
				clog.Infof(ctx, "Error reading trickle subscription: %s", err)
				return
			}
			clog.V(8).Infof(ctx, "trickle subscribe read data")

			if err = copySegment(segment, w); err != nil {
				clog.Infof(ctx, "Error copying to ffmpeg stdin: %s", err)
				return
			}
		}
	}()

	go func() {
		defer r.Close()
		for {
			_, ok := params.node.LivePipelines[params.liveParams.stream]
			if !ok {
				clog.Errorf(ctx, "Stopping output rtmp stream, input stream does not exist. err=%s", err)
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

	clog.Infof(ctx, "Starting event subscription for URL: %s", url.String())

	go func() {
		for {
			clog.Infof(ctx, "Attempting to read from event subscription for URL: %s", url.String())
			segment, err := subscriber.Read()
			if err != nil {
				clog.Infof(ctx, "Error reading events subscription: %s", err)
				// TODO
				// monitor.DeletePipelineStatus(params.liveParams.stream)
				return
			}

			clog.Infof(ctx, "Successfully read segment from event subscription for URL: %s", url.String())

			body, err := io.ReadAll(segment.Body)
			segment.Body.Close()

			if err != nil {
				clog.Infof(ctx, "Error reading events subscription body: %s", err)
				continue
			}

			stream := params.liveParams.stream

			if stream == "" {
				clog.Infof(ctx, "Stream ID is missing")
				continue
			}

			var status monitor.PipelineStatus
			if err := json.Unmarshal(body, &status); err != nil {
				clog.Infof(ctx, "Failed to parse JSON from events subscription: %s", err)
				continue
			}

			status.StreamID = &stream

			// TODO: update the in-memory pipeline status
			// monitor.UpdatePipelineStatus(stream, status)

			clog.Infof(ctx, "Received event for stream=%s status=%+v", stream, status)

			monitor.SendQueueEventAsync(
				"stream_status",
				status,
			)
		}
	}()
}
