package server

import (
	"io"
	"log/slog"
	"net/url"

	"github.com/livepeer/go-livepeer/trickle"
)

func startTricklePublish(url *url.URL, params aiRequestParams) {
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		slog.Info("error publishing trickle", "err", err)
	}
	params.segmentReader.SwitchReader(func(reader io.Reader) {
		go func() {
			// TODO this blocks! very bad!
			publisher.Write(reader)
		}()
	})
	// TODO close trickle publish on stream termination
	slog.Info("trickle pub", "url", url)
}

func startTrickleSubscribe(url *url.URL) {
}
