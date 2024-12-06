package server

import (
	"context"
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
	controlPub, err := trickle.NewTricklePublisher(control.String())
	stream := params.liveParams.stream
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
	ctx = clog.AddVal(ctx, "url", url.Redacted())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
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

				clog.Infof(ctx, "Received from events trickle: %s", string(body))
				monitor.SendQueueEventAsync(
					"stream_status",
					string(body),
				)
			}
		}
	}()
}
