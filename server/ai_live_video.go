package server

import (
	"io"
	"log/slog"
	"net/url"

	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
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

func startTrickleSubscribe(url *url.URL) {
}
