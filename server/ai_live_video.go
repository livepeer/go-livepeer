package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/livepeer/go-livepeer/common"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/lpms/ffmpeg"
)

func startTricklePublish(url *url.URL, params aiRequestParams, sess *AISession) {
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		slog.Info("error publishing trickle", "err", err)
	}

	// Start payments which probes a segment every "paymentProcessInterval" and sends a payment
	ctx, cancel := context.WithCancel(context.Background())
	priceInfo := sess.OrchestratorInfo.PriceInfo
	var paymentProcessor *LivePaymentProcessor
	if priceInfo != nil && priceInfo.PricePerUnit != 0 {
		paymentSender := livePaymentSender{}
		sendPaymentFunc := func(inPixels int64) error {
			clog.V(common.DEBUG).Infof(ctx, "Sending payment, mid=%v, inPixels=%v, pricePerUnit=%v, pixelsPerUnit=%v", extractMid(url.Path), inPixels, priceInfo.PricePerUnit, priceInfo.PixelsPerUnit)
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

	params.liveParams.segmentReader.SwitchReader(func(reader io.Reader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				slog.Info("Error closing trickle publisher", "err", err)
			}
			cancel()
			return
		}
		go func() {
			clog.V(8).Infof(context.Background(), "publishing trickle. url=%s", url.Redacted())

			r := reader
			if paymentProcessor != nil {
				r = paymentProcessor.process(reader)
			}

			// TODO this blocks! very bad!
			if err := publisher.Write(r); err != nil {
				slog.Info("Error writing to trickle publisher", "err", err)
			}
		}()
	})
	slog.Info("trickle pub", "url", url)
}

func startTrickleSubscribe(ctx context.Context, url *url.URL, params aiRequestParams) {
	// subscribe to the outputs and send them into LPMS
	subscriber := trickle.NewTrickleSubscriber(url.String())
	r, w, err := os.Pipe()
	if err != nil {
		slog.Info("error getting pipe for trickle-ffmpeg", "url", url, "err", err)
	}
	ctx = clog.AddVal(ctx, "url", url.Redacted())

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
			defer segment.Body.Close()
			if _, err = io.Copy(w, segment.Body); err != nil {
				clog.Infof(ctx, "Error copying to ffmpeg stdin: %s", err)
				return
			}
		}
	}()

	go func() {
		defer r.Close()
		retryCount := 0
		// TODO check whether stream is actually terminated
		//      so we aren't just looping unnecessarily
		for retryCount < 10 {
			_, err := ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
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
			retryCount++
			time.Sleep(5 * time.Second)
		}
	}()
}

func mediamtxSourceTypeToString(s string) (string, error) {
	switch s {
	case mediaMTXWebrtcSession:
		return "whip", nil
	case mediaMTXRtmpConn:
		return "rtmp", nil
	default:
		return "", errors.New("unknown media source")
	}
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
	params.node.LivePipelines[stream] = &core.LivePipeline{ControlPub: controlPub}
}

const (
	mediaMTXControlPort   = "9997"
	mediaMTXControlUser   = "admin"
	mediaMTXWebrtcSession = "webrtcSession"
	mediaMTXRtmpConn      = "rtmpConn"
)

func (ls *LivepeerServer) kickInputConnection(mediaMTXHost, sourceID, sourceType string) error {
	var apiPath string
	switch sourceType {
	case mediaMTXWebrtcSession:
		apiPath = "webrtcsessions"
	case mediaMTXRtmpConn:
		apiPath = "rtmpconns"
	default:
		return fmt.Errorf("invalid sourceType: %s", sourceType)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%s/v3/%s/kick/%s", mediaMTXHost, mediaMTXControlPort, apiPath, sourceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create kick request: %w", err)
	}
	req.SetBasicAuth(mediaMTXControlUser, ls.mediaMTXApiPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to kick connection: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick connection failed with status code: %d body: %s", resp.StatusCode, body)
	}
	return nil
}
