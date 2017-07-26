package vidlistener

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av/pubsub"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

func TestListener(t *testing.T) {
	server := &joy4rtmp.Server{Addr: ":1937"}
	listener := &VidListener{RtmpServer: server}
	q := pubsub.NewQueue()

	listener.HandleRTMPPublish(
		//makeStreamID
		func(url *url.URL) string {
			return "testID"
		},
		//gotStream
		func(url *url.URL, rtmpStrm *stream.VideoStream) (err error) {
			//Read the stream into q
			go rtmpStrm.ReadRTMPFromStream(context.Background(), q)
			return nil
		},
		//endStream
		func(url *url.URL, rtmpStrm *stream.VideoStream) error {
			if rtmpStrm.GetStreamID() != "testID" {
				t.Errorf("Expecting 'testID', found %v", rtmpStrm.GetStreamID())
			}
			return nil
		})

	//Stream test stream into the rtmp server
	ffmpegCmd := "ffmpeg"
	ffmpegArgs := []string{"-re", "-i", "../data/bunny2.mp4", "-c", "copy", "-f", "flv", "rtmp://localhost:1937/movie/stream"}
	cmd := exec.Command(ffmpegCmd, ffmpegArgs...)
	go cmd.Run()

	//Start the server
	go listener.RtmpServer.ListenAndServe()

	//Wait for the stream to run for a little, then finish.
	time.Sleep(time.Second * 1)
	err := cmd.Process.Kill()
	if err != nil {
		fmt.Println("Error killing ffmpeg")
	}

	codecs, err := q.Oldest().Streams()
	if err != nil || codecs == nil {
		t.Errorf("Expecting codecs, got nil.  Error: %v", err)
	}

	pkt, err := q.Oldest().ReadPacket()
	if err != nil || len(pkt.Data) == 0 {
		t.Errorf("Expecting pkt, got nil.  Error: %v", err)
	}
}
