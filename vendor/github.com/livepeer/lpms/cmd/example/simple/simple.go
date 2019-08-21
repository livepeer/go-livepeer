package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/stream"
)

type exampleStream string

func (t exampleStream) StreamID() string {
	return string(t)
}

func randString(n int) string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, n, n)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x", x)
}

func main() {
	glog.Info("hi")

	dir, err := os.Getwd()
	if err != nil {
		glog.Infof("Error getting work directory: %v", err)
	}
	glog.Infof("Settig working directory %v", fmt.Sprintf("%v/.tmp", dir))

	lpms := core.New(&core.LPMSOpts{WorkDir: fmt.Sprintf("%v/.tmp", dir)})

	lpms.HandleRTMPPublish(
		func(url *url.URL) stream.AppData {
			glog.Infof("Stream has been started!: %v", url)
			return exampleStream(randString(10))
		},

		func(url *url.URL, rs stream.RTMPVideoStream) (err error) {
			streamFormat := rs.GetStreamFormat()
			glog.Infof("Stream is in play!: string format:%v", streamFormat)
			return nil
		},

		func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
			width := rtmpStrm.Width()
			h := rtmpStrm.Height()
			glog.Infof("height and width: %v , %v", h, width)
			rtmpStrm.Close()
			return nil
		},
	)
	lpms.Start(context.Background())

}
