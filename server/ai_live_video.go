package server

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/livepeer/go-livepeer/trickle"
)

// TODO: This will not be a global variable, but a param injected somewhere
var paymentSender RealtimePaymentSender

func startTricklePublish(url *url.URL, params aiRequestParams, sess *AISession) {
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		slog.Info("error publishing trickle", "err", err)
	}
	paymentSender := realtimePaymentSender{segmentsToPayUpfront: 5}
	params.segmentReader.SwitchReader(func(reader io.Reader) {
		go func() {
			paymentSender.SendPayment(context.TODO(), &SegmentInfoSender{
				sess: sess.BroadcastSession,
				// TODO: Get inPixels and dur from the segment
				inPixels: 4000,
				dur:      time.Second,
			})
			// TODO this blocks! very bad!
			publisher.Write(reader)
		}()
	})
	// TODO close trickle publish on stream termination
	slog.Info("trickle pub", "url", url)
}

func startTrickleSubscribe(url *url.URL) {
}
