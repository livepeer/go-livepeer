package server

import (
	"context"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-data/pkg/event"
)

const queuePublishTimeout = 1 * time.Second

var MetadataQueue event.Producer

func BackgroundPublish(queue event.Producer, key string, body interface{}) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), queuePublishTimeout)
		defer cancel()
		if err := queue.Publish(ctx, key, body, false); err != nil {
			glog.Errorf("Error publishing event: key=%q, err=%q", key, err)
		}
	}()
}
