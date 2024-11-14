package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"

	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/lpms/ffmpeg"
)

func startTricklePublish(url *url.URL, params aiRequestParams) {
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		slog.Info("error publishing trickle", "err", err)
	}
	params.segmentReader.SwitchReader(func(reader io.Reader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				slog.Info("Error closing trickle publisher", "err", err)
			}
			return
		}
		go func() {
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
			// TODO take from params
			Oname:        "rtmp://localhost/out-stream",
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			Muxer:        ffmpeg.ComponentOptions{Name: "flv"},
		}})
	}()
}
