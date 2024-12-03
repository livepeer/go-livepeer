package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/mediaserver"
	"github.com/livepeer/go-livepeer/trickle"
)

func startTricklePublish(ctx context.Context, url *url.URL, params aiRequestParams) {
	ctx = clog.AddVal(ctx, "url", url.Redacted())
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		clog.Infof(ctx, "error publishing trickle. err=%s", err)
	}
	params.liveParams.segmentReader.SwitchReader(func(reader io.Reader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				clog.Infof(ctx, "Error closing trickle publisher. err=%s", err)
			}
			return
		}
		go func() {
			clog.V(8).Infof(ctx, "trickle publish writing data")
			// TODO this blocks! very bad!
			if err := publisher.Write(reader); err != nil {
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

	// TODO: Change this to LPMS
	go func() {
		defer r.Close()
		retryCount := 0
		// TODO better retry logic
		for retryCount < 10 {
			cmd := exec.Command("ffmpeg",
				"-i", "pipe:0",
				"-c:a", "copy",
				"-c:v", "copy",
				"-f", "flv",
				params.liveParams.outputRTMPURL,
			)
			cmd.Stdin = r
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				clog.Infof(ctx, "Error running trickle subscribe ffmpeg: %s", err)
			}
			retryCount++
			time.Sleep(5 * time.Second)
		}
	}()
}

func mediamtxSourceTypeToString(s string) (string, error) {
	switch s {
	case mediaserver.MediaMTXWebrtcSession:
		return "whip", nil
	case mediaserver.MediaMTXRtmpConn:
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
