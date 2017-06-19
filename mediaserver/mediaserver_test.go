package mediaserver

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/libp2p-livepeer/core"
	"github.com/livepeer/lpms/stream"
)

// Should create media server
func TestNewMediaServer(t *testing.T) {
	NewLivepeerMediaServer("1935", "8080", "", core.NewLivepeerNode())
}

// Should start media server, and should kick off unsubscribe worker
func TestStartMediaServer(t *testing.T) {
	s := NewLivepeerMediaServer("1935", "8080", "", core.NewLivepeerNode())

	var err error
	go func() { err = s.StartLPMS(context.Background()) }()

	time.Sleep(time.Second * 1)
	if err != nil {
		t.Errorf("Error starting Livepeer Media Server: %v", err)
	}

	if s.hlsSubTimer == nil {
		t.Errorf("hlsSubTimer should be initiated")
	}

	if s.hlsWorkerRunning == false {
		t.Errorf("Should have kicked off hlsUnsubscribeWorker go routine")
	}
}

// Should publish RTMP stream
func TestGotRTMPStreamHandler(t *testing.T) {
	s := NewLivepeerMediaServer("1935", "8080", "", core.NewLivepeerNode())
	handler := s.makeGotRTMPStreamHandler()

	url, _ := url.Parse("http://localhost/stream/test")
	strm := stream.NewVideoStream("", stream.RTMP)

	//Stream already exists
	s.LivepeerNode.StreamDB.AddStream("test", stream.NewVideoStream("test", stream.RTMP))
	_, err := handler(url, strm)
	if err != ErrAlreadyExists {
		t.Errorf("Expecting publish error because stream already exists, but got: %v", err)
	}
	s.LivepeerNode.StreamDB.DeleteStream("test")

}

// Should subscribe RTMP stream

// Should turn RTMP stream into HLS stream
