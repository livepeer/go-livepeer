package vidplayer

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"time"

	"net/url"

	"github.com/ericxtang/m3u8"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/avutil"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

func TestRTMP(t *testing.T) {
	server := &joy4rtmp.Server{Addr: ":1936"}
	player := &VidPlayer{RtmpServer: server}
	var demuxer av.Demuxer
	gotUpvid := false
	gotPlayvid := false
	player.RtmpServer.HandlePublish = func(conn *joy4rtmp.Conn) {
		gotUpvid = true
		demuxer = conn
	}

	player.HandleRTMPPlay(func(ctx context.Context, reqPath string, dst av.MuxCloser) error {
		gotPlayvid = true
		fmt.Println(reqPath)
		avutil.CopyFile(dst, demuxer)
		return nil
	})
}

func TestHLS(t *testing.T) {
	player := &VidPlayer{}
	s := stream.NewVideoStream("test", stream.HLS)
	s.HLSTimeout = time.Second * 5
	//Write some packets into the stream
	s.WriteHLSPlaylistToStream(m3u8.MediaPlaylist{})
	s.WriteHLSSegmentToStream(stream.HLSSegment{})
	var buffer *stream.HLSBuffer
	player.HandleHLSPlay(
		//getMasterPlaylist
		func(url *url.URL) (*m3u8.MasterPlaylist, error) {
			return nil, nil
		},
		//getMediaPlaylist
		func(url *url.URL) (*m3u8.MediaPlaylist, error) {
			return buffer.LatestPlaylist()
		},
		//getSegment
		func(url *url.URL) ([]byte, error) {
			return nil, nil
		})

	//TODO: Add tests for checking if packets were written, etc.
}

type TestRWriter struct {
	bytes  []byte
	header map[string][]string
}

func (t *TestRWriter) Header() http.Header { return t.header }
func (t *TestRWriter) Write(b []byte) (int, error) {
	t.bytes = b
	return 0, nil
}
func (*TestRWriter) WriteHeader(int) {}

func TestHandleHLS(t *testing.T) {
	testBuf := stream.NewHLSBuffer(10, 100)
	req := &http.Request{URL: &url.URL{Path: "test.m3u8"}}
	rw := &TestRWriter{header: make(map[string][]string)}

	pl, _ := m3u8.NewMediaPlaylist(10, 10)
	pl.Append("url_1.ts", 2, "")
	pl.Append("url_2.ts", 2, "")
	pl.Append("url_3.ts", 2, "")
	pl.Append("url_4.ts", 2, "")
	pl.SeqNo = 1

	testBuf.WriteSegment(1, "url_1.ts", 2, []byte{0, 0})
	testBuf.WriteSegment(2, "url_2.ts", 2, []byte{0, 0})
	testBuf.WriteSegment(3, "url_3.ts", 2, []byte{0, 0})
	testBuf.WriteSegment(4, "url_4.ts", 2, []byte{0, 0})

	p1, _ := m3u8.NewMediaPlaylist(10, 10)
	err := p1.DecodeFrom(bytes.NewReader(pl.Encode().Bytes()), true)
	if err != nil {
		t.Errorf("Error decoding pl :%v", err)
	}

	segLen := 0
	for _, s := range p1.Segments {
		if s != nil {
			segLen = segLen + 1
		}
	}

	if segLen != 4 {
		t.Errorf("Expecting 4 segments, got %v", segLen)
	}

}
