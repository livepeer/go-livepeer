package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/lpms/ffmpeg"
)

func startTricklePublish(url *url.URL, params aiRequestParams, sess *AISession, mid string) {
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		slog.Info("error publishing trickle", "err", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	paymentProcessInterval := 1 * time.Second
	paymentSender := livePaymentSender{segmentsToPayUpfront: 5}
	f := func(inPixels int64) error {
		return paymentSender.SendPayment(context.Background(), &SegmentInfoSender{
			sess:      sess.BroadcastSession,
			inPixels:  inPixels,
			priceInfo: sess.OrchestratorInfo.PriceInfo,
			mid:       mid,
		})
	}
	paymentProcessor := NewLivePaymentProcessor(ctx, paymentProcessInterval, f)

	params.segmentReader.SwitchReader(func(reader io.Reader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				slog.Info("Error closing trickle publisher", "err", err)
			}
			cancel()
			return
		}
		go func() {
			reader := paymentProcessor.process(reader)

			// TODO this blocks! very bad!
			if err := publisher.Write(reader); err != nil {
				slog.Info("Error writing to trickle publisher", "err", err)
			}
		}()
	})
	slog.Info("trickle pub", "url", url)
}

func startTrickleSubscribe(url *url.URL, params aiRequestParams) {
	// subscribe to the outputs and send them into LPMS
	subscriber := trickle.NewTrickleSubscriber(url.String())
	r, w, err := os.Pipe()
	if err != nil {
		slog.Info("error getting pipe for trickle-ffmpeg", "url", url, "err", err)
	}

	// read segments from trickle subscription
	go func() {
		defer w.Close()
		for {
			segment, err := subscriber.Read()
			if err != nil {
				// TODO if not EOS then signal a new orchestrator is needed
				slog.Info("Error reading trickle subscription", "url", url, "err", err)
				return
			}
			defer segment.Body.Close()
			// TODO send this into ffmpeg
			io.Copy(w, segment.Body)
		}
	}()

	// lpms
	go func() {
		ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
			Fname: fmt.Sprintf("pipe:%d", r.Fd()),
		}, []ffmpeg.TranscodeOptions{{
			Oname:        params.outputRTMPURL,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			Muxer:        ffmpeg.ComponentOptions{Name: "flv"},
		}})
	}()
}

func mediamtxSourceTypeToString(s string) (string, error) {
	switch s {
	case "webrtcSession":
		return "whip", nil
	case "rtmpConn":
		return "rtmp", nil
	default:
		return "", errors.New("unknown media source")
	}
}

func startControlPublish(control *url.URL, params aiRequestParams) {
	controlPub, err := trickle.NewTricklePublisher(control.String())
	if err != nil {
		slog.Info("error starting control publisher", "stream", params.stream, "err", err)
		return
	}
	params.node.LiveMu.Lock()
	defer params.node.LiveMu.Unlock()
	params.node.LivePipelines[params.stream] = &core.LivePipeline{ControlPub: controlPub}
}
