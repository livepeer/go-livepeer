package segmenter

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"time"

	"strconv"

	"io/ioutil"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/avutil"
	"github.com/nareix/joy4/format"
	"github.com/nareix/joy4/format/rtmp"
)

type TestStream struct{}

func (s *TestStream) GetStreamID() string { return "test" }
func (s *TestStream) Len() int64          { return 0 }
func (s *TestStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error {
	format.RegisterAll()
	wd, _ := os.Getwd()
	file, err := avutil.Open(wd + "/test.flv")
	if err != nil {
		fmt.Println("Error opening file: ", err)
		return err
	}
	header, err := file.Streams()
	if err != nil {
		glog.Errorf("Error reading headers: %v", err)
		return err
	}

	dst.WriteHeader(header)
	for {
		pkt, err := file.ReadPacket()
		if err == io.EOF {
			dst.WriteTrailer()
			return err
		}
		dst.WritePacket(pkt)
	}
}
func (s *TestStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error { return nil }
func (s *TestStream) WriteHLSPlaylistToStream(pl m3u8.MediaPlaylist) error            { return nil }
func (s *TestStream) WriteHLSSegmentToStream(seg stream.HLSSegment) error             { return nil }
func (s *TestStream) ReadHLSFromStream(buffer stream.HLSMuxer) error                  { return nil }

func TestSegmenter(t *testing.T) {
	wd, _ := os.Getwd()
	workDir := wd + "/tmp"
	os.RemoveAll(workDir)

	//Create a test stream from stub
	strm := &TestStream{}
	url := fmt.Sprintf("rtmp://localhost:%v/stream/%v", "1935", strm.GetStreamID())
	vs := NewFFMpegVideoSegmenter(workDir, strm.GetStreamID(), url, time.Millisecond*10, "")
	server := &rtmp.Server{Addr: ":1935"}
	player := vidplayer.VidPlayer{RtmpServer: server}

	player.HandleRTMPPlay(
		func(ctx context.Context, reqPath string, dst av.MuxCloser) error {
			return strm.ReadRTMPFromStream(ctx, dst)
		})

	//Kick off RTMP server
	go func() {
		err := player.RtmpServer.ListenAndServe()
		if err != nil {
			t.Errorf("Error kicking off RTMP server: %v", err)
		}
	}()

	se := make(chan error, 1)
	opt := SegmenterOptions{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	//Kick off FFMpeg to create segments
	go func() { se <- func() error { return vs.RTMPToHLS(ctx, opt, false) }() }()
	select {
	case err := <-se:
		if err != context.DeadlineExceeded {
			t.Errorf("Should exceed deadline (since it's not a real stream, ffmpeg should finish instantly).  But instead got: %v", err)
		}
	}

	pl, err := vs.PollPlaylist(ctx)
	if err != nil {
		t.Errorf("Got error: %v", err)
	}

	if pl.Format != stream.HLS {
		t.Errorf("Expecting HLS Playlist, got %v", pl.Format)
	}

	// p, err := m3u8.NewMediaPlaylist(100, 100)
	// err = p.DecodeFrom(bytes.NewReader(pl.Data), true)
	// if err != nil {
	// 	t.Errorf("Error decoding HLS playlist: %v", err)
	// }

	if vs.curSegment != 0 {
		t.Errorf("Segment counter should start with 0.  But got: %v", vs.curSegment)
	}

	for i := 0; i < 2; i++ {
		seg, err := vs.PollSegment(ctx)

		if vs.curSegment != i+1 {
			t.Errorf("Segment counter should move to %v.  But got: %v", i+1, vs.curSegment)
		}

		if err != nil {
			t.Errorf("Got error: %v", err)
		}

		if seg.Codec != av.H264 {
			t.Errorf("Expecting H264 segment, got: %v", seg.Codec)
		}

		if seg.Format != stream.HLS {
			t.Errorf("Expecting HLS segment, got %v", seg.Format)
		}

		timeDiff := seg.Length - time.Second*8
		if timeDiff > time.Millisecond*500 || timeDiff < -time.Millisecond*500 {
			t.Errorf("Expecting 2 sec segments, got %v", seg.Length)
		}

		fn := "test_" + strconv.Itoa(i) + ".ts"
		if seg.Name != fn {
			t.Errorf("Expecting %v, got %v", fn, seg.Name)
		}

		if seg.SeqNo != uint64(i) {
			t.Errorf("Expecting SeqNo %v, got %v", uint(i), seg.SeqNo)
		}

		segLen := len(seg.Data)
		if segLen < 20000 {
			t.Errorf("File size is too small: %v", segLen)
		}

	}

	newPl := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-ALLOW-CACHE:YES
#EXT-X-TARGETDURATION:7
#EXTINF:2.066000,
test_0.ts
#EXTINF:1.999000,
test_1.ts
#EXTINF:1.999000,
test_2.ts
#EXTINF:1.999000,
test_3.ts
#EXTINF:1.999000,
test_4.ts
#EXTINF:1.999000,
test_5.ts
#EXTINF:1.999000,
test_6.ts
`
	// bf, _ := ioutil.ReadFile(workDir + "/test.m3u8")
	// fmt.Printf("bf:%s\n", bf)
	ioutil.WriteFile(workDir+"/test.m3u8", []byte(newPl), os.ModeAppend)
	// af, _ := ioutil.ReadFile(workDir + "/test.m3u8")
	// fmt.Printf("af:%s\n", af)

	// fmt.Println("before:%v", pl.Data.Segments[0:10])
	pl, err = vs.PollPlaylist(ctx)
	if err != nil {
		t.Errorf("Got error polling playlist: %v", err)
	}
	// fmt.Println("after:%v", pl.Data.Segments[0:10])
	// segLen := len(pl.Data.Segments)
	// if segLen != 4 {
	// 	t.Errorf("Seglen should be 4.  Got: %v", segLen)
	// }

	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*400)
	defer cancel()
	pl, err = vs.PollPlaylist(ctx)
	if err == nil {
		t.Errorf("Expecting timeout error...")
	}
	//Clean up
	os.RemoveAll(workDir)
}

func TestPollPlaylistError(t *testing.T) {
	vs := NewFFMpegVideoSegmenter("./sometestdir", "test", "", time.Millisecond*100, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	_, err := vs.PollPlaylist(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expect to exceed deadline, but got: %v", err)
	}
}

func TestPollSegmentError(t *testing.T) {
	vs := NewFFMpegVideoSegmenter("./sometestdir", "test", "", time.Millisecond*10, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	_, err := vs.PollSegment(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expect to exceed deadline, but got: %v", err)
	}
}

func TestPollPlaylistTimeout(t *testing.T) {
	wd, _ := os.Getwd()
	workDir := wd + "/tmp"
	os.RemoveAll(workDir)
	os.Mkdir(workDir, 0700)

	newPl := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-ALLOW-CACHE:YES
#EXT-X-TARGETDURATION:7
#EXTINF:2.066000,
test_0.ts
`
	err := ioutil.WriteFile(workDir+"/test.m3u8", []byte(newPl), 0755)
	if err != nil {
		t.Errorf("Error writing playlist: %v", err)
	}

	vs := NewFFMpegVideoSegmenter(workDir, "test", "", time.Millisecond*100, "")
	ctx := context.Background()
	pl, err := vs.PollPlaylist(ctx)
	if pl == nil {
		t.Errorf("Expecting playlist, got nil")
	}

	pl, err = vs.PollPlaylist(ctx)
	if err != ErrSegmenterTimeout {
		t.Errorf("Expecting timeout error, got %v", err)
	}
}

func TestPollSegTimeout(t *testing.T) {
	wd, _ := os.Getwd()
	workDir := wd + "/tmp"
	os.RemoveAll(workDir)
	os.Mkdir(workDir, 0700)

	newPl := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-ALLOW-CACHE:YES
#EXT-X-TARGETDURATION:7
#EXTINF:2.066000,
test_0.ts
#EXTINF:2.066000,
test_1.ts
`
	err := ioutil.WriteFile(workDir+"/test.m3u8", []byte(newPl), 0755)
	newSeg := `some random data`
	err = ioutil.WriteFile(workDir+"/test_0.ts", []byte(newSeg), 0755)
	err = ioutil.WriteFile(workDir+"/test_1.ts", []byte(newSeg), 0755)
	if err != nil {
		t.Errorf("Error writing playlist: %v", err)
	}

	vs := NewFFMpegVideoSegmenter(workDir, "test", "", time.Millisecond*100, "")
	ctx := context.Background()
	seg, err := vs.PollSegment(ctx)
	if seg == nil {
		t.Errorf("Expecting seg, got nil")
	}

	seg, err = vs.PollSegment(ctx)
	if err != ErrSegmenterTimeout {
		t.Errorf("Expecting timeout, got %v", err)
	}

	os.RemoveAll(workDir)
}
