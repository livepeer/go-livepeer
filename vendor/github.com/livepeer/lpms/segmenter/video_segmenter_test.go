package segmenter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"testing"

	"time"

	"strconv"

	"io/ioutil"

	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/avutil"
	"github.com/nareix/joy4/format"
	"github.com/nareix/joy4/format/rtmp"
)

type TestStream struct{}

func (s TestStream) String() string                       { return "" }
func (s *TestStream) GetStreamFormat() stream.VideoFormat { return stream.RTMP }
func (s *TestStream) GetStreamID() string                 { return "test" }
func (s *TestStream) Len() int64                          { return 0 }
func (s *TestStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) (chan struct{}, error) {
	format.RegisterAll()
	wd, _ := os.Getwd()
	file, err := avutil.Open(wd + "/test.flv")
	if err != nil {
		fmt.Println("Error opening file: ", err)
		return nil, err
	}
	header, err := file.Streams()
	if err != nil {
		glog.Errorf("Error reading headers: %v", err)
		return nil, err
	}

	dst.WriteHeader(header)
	eof := make(chan struct{})
	go func(eof chan struct{}) {
		for {
			pkt, err := file.ReadPacket()
			if err == io.EOF {
				dst.WriteTrailer()
				eof <- struct{}{}
			}
			dst.WritePacket(pkt)
		}
	}(eof)
	return eof, nil
}
func (s *TestStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) (chan struct{}, error) {
	return nil, nil
}
func (s *TestStream) WriteHLSPlaylistToStream(pl m3u8.MediaPlaylist) error                { return nil }
func (s *TestStream) WriteHLSSegmentToStream(seg stream.HLSSegment) error                 { return nil }
func (s *TestStream) ReadHLSFromStream(ctx context.Context, buffer stream.HLSMuxer) error { return nil }
func (s *TestStream) ReadHLSSegment() (stream.HLSSegment, error)                          { return stream.HLSSegment{}, nil }
func (s *TestStream) Width() int                                                          { return 0 }
func (s *TestStream) Height() int                                                         { return 0 }
func (s *TestStream) Close()                                                              {}

func RunRTMPToHLS(vs *FFMpegVideoSegmenter, ctx context.Context) error {
	// hack cuz listener might not be ready
	t := time.NewTicker(100 * time.Millisecond)
	max := time.After(3 * time.Second)
	c := make(chan error, 1)
	go func() {
		var err error
		for _ = range t.C {
			err = vs.RTMPToHLS(ctx, false)
			if err == nil || err.Error() != "Connection refused" {
				break
			}
			glog.Infof("Unable to connect start segmenter (%v), retrying", err)
		}
		t.Stop()
		c <- err
	}()
	select {
	case <-max:
		return errors.New("Segmenter timed out")
	case err := <-c:
		return err
	}
}

func TestSegmenter(t *testing.T) {
	ffmpeg.InitFFmpeg()
	wd, _ := os.Getwd()
	workDir := wd + "/tmp"
	os.RemoveAll(workDir)

	//Create a test stream from stub
	strm := &TestStream{}
	strmUrl := fmt.Sprintf("rtmp://localhost:%v/stream/%v", "1939", strm.GetStreamID())
	opt := SegmenterOptions{SegLength: time.Second * 4}
	vs := NewFFMpegVideoSegmenter(workDir, strm.GetStreamID(), strmUrl, opt)
	server := &rtmp.Server{Addr: ":1939"}
	player := vidplayer.NewVidPlayer(server, "", nil)

	player.HandleRTMPPlay(
		func(url *url.URL) (stream.RTMPVideoStream, error) {
			return strm, nil
		})

	//Kick off RTMP server
	go func() {
		err := player.RtmpServer.ListenAndServe()
		if err != nil {
			t.Errorf("Error kicking off RTMP server: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	//Kick off FFMpeg to create segments
	err := RunRTMPToHLS(vs, ctx)
	if err != nil {
		t.Errorf("Since it's not a real stream, ffmpeg should finish instantly. But instead got: %v", err)
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

		timeDiff := seg.Length - opt.SegLength
		if timeDiff > time.Millisecond*500 || timeDiff < -time.Millisecond*500 {
			t.Errorf("Expecting %v sec segments, got %v.  Diff: %v", opt.SegLength, seg.Length, timeDiff)
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

func TestSetStartSeq(t *testing.T) {
	ffmpeg.InitFFmpeg()
	wd, _ := os.Getwd()
	workDir := wd + "/tmp"
	os.RemoveAll(workDir)

	startSeq := 1234 // Base value

	//Create a test stream from stub
	strm := &TestStream{}
	strmUrl := fmt.Sprintf("rtmp://localhost:%v/stream/%v", "1936", strm.GetStreamID())
	opt := SegmenterOptions{SegLength: time.Second * 4, StartSeq: startSeq}
	vs := NewFFMpegVideoSegmenter(workDir, strm.GetStreamID(), strmUrl, opt)
	server := &rtmp.Server{Addr: ":1936"}
	player := vidplayer.NewVidPlayer(server, "", nil)

	player.HandleRTMPPlay(
		func(url *url.URL) (stream.RTMPVideoStream, error) {
			return strm, nil
		})

	//Kick off RTMP server
	go func() {
		err := player.RtmpServer.ListenAndServe()
		if err != nil {
			t.Errorf("Error kicking off RTMP server: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	//Kick off FFMpeg to create segments
	err := RunRTMPToHLS(vs, ctx)
	if err != nil {
		t.Errorf("Since it's not a real stream, ffmpeg should finish instantly. But instead got: %v", err)
	}

	if vs.curSegment != startSeq {
		t.Errorf("Segment counter should start with %v.  But got: %v", startSeq, vs.curSegment)
	}

	for j := 0; j < 2; j++ {
		seg, err := vs.PollSegment(ctx)
		i := startSeq + j

		if err != nil {
			t.Errorf("Got error: %v", err)
		}

		if vs.curSegment != i+1 {
			t.Errorf("Segment counter should move to %v.But got: %v", i+1, vs.curSegment)
		}

		fn := "test_" + strconv.Itoa(i) + ".ts"
		if seg.Name != fn {
			t.Errorf("Expecting %v, got %v", fn, seg.Name)
		}

		if seg.SeqNo != uint64(i) {
			t.Errorf("Expecting SeqNo %v, got %v", uint(i), seg.SeqNo)
		}
	}

	//Clean up
	os.RemoveAll(workDir)
}

func TestPollPlaylistError(t *testing.T) {
	opt := SegmenterOptions{}
	vs := NewFFMpegVideoSegmenter("./sometestdir", "test", "", opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	_, err := vs.PollPlaylist(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expect to exceed deadline, but got: %v", err)
	}
}

func TestPollSegmentError(t *testing.T) {
	opt := SegmenterOptions{}
	vs := NewFFMpegVideoSegmenter("./sometestdir", "test", "", opt)
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

	opt := SegmenterOptions{SegLength: time.Millisecond * 100}
	vs := NewFFMpegVideoSegmenter(workDir, "test", "", opt)
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

	opt := SegmenterOptions{SegLength: time.Millisecond * 100}
	vs := NewFFMpegVideoSegmenter(workDir, "test", "", opt)
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

func TestNoRTMPListener(t *testing.T) {
	url := "rtmp://localhost:19355"
	opt := SegmenterOptions{}
	vs := NewFFMpegVideoSegmenter("tmp", "test", url, opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := vs.RTMPToHLS(ctx, false)
	if err == nil {
		t.Errorf("error was unexpectedly nil; is something running on %v?", url)
	} else if err.Error() != "Connection refused" {
		t.Error("error was not nil; got ", err)
	}
}

type ServerDisconnectStream struct {
	TestStream
}

func (s *ServerDisconnectStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) (chan struct{}, error) {
	file, err := avutil.Open("test.flv")
	if err != nil {
		glog.Errorf("Error reading headers: %v", err)
		return nil, err
	}
	header, err := file.Streams()
	dst.WriteHeader(header)
	dst.Close()
	return make(chan struct{}), nil
}

func TestServerDisconnectMidStream(t *testing.T) {
	ffmpeg.InitFFmpeg()
	port := "1938" // because we can't yet close the listener on 1935?
	strm := &ServerDisconnectStream{}
	strmUrl := fmt.Sprintf("rtmp://localhost:%v/stream/%v", port, strm.GetStreamID())
	opt := SegmenterOptions{SegLength: time.Second * 4}
	vs := NewFFMpegVideoSegmenter("tmp", strm.GetStreamID(), strmUrl, opt)
	server := &rtmp.Server{Addr: ":" + port}
	player := vidplayer.NewVidPlayer(server, "", nil)
	player.HandleRTMPPlay(
		func(url *url.URL) (stream.RTMPVideoStream, error) {
			return strm, nil
		})

	//Kick off RTMP server
	go func() {
		err := player.RtmpServer.ListenAndServe()
		if err != nil {
			t.Errorf("Error kicking off RTMP server: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := RunRTMPToHLS(vs, ctx)
	if err == nil || err.Error() != "Input/output error" {
		t.Error("Expected 'Input/output error' but instead got ", err)
	}
}

func TestSegmentDefaults(t *testing.T) {
	opt := SegmenterOptions{}
	vs := NewFFMpegVideoSegmenter("", "test", "", opt)
	if vs.SegLen != 4*time.Second {
		t.Errorf("Expected 4 second default segment length but got %v", opt.SegLength)
	}
	opt = SegmenterOptions{SegLength: 100 * time.Millisecond}
	vs = NewFFMpegVideoSegmenter("", "test", "", opt)
	if vs.SegLen != 100*time.Millisecond {
		t.Errorf("Expected 100ms default segment length but got %v", opt.SegLength)
	}
}

func ffprobe_firstframeflags(fname string) (string, error) {
	cmd := "ffprobe -loglevel quiet -hide_banner -select_streams v -show_packets "
	cmd = cmd + fname + " | grep flag | head -1"
	out, err := exec.Command("bash", "-c", cmd).Output()
	return strings.TrimSpace(string(out)), err
}

func TestMissingKeyframe(t *testing.T) {
	// sanity check that test file has a keyframe at the beginning
	out, err := ffprobe_firstframeflags("test.flv")
	if err != nil || out != "flags=K_" {
		t.Errorf("First video packet of test file was not a keyframe '%v' - %v", out, err)
		return
	}

	// remove the first keyframe from test file, store in tempfile
	dir, err := ioutil.TempDir("", "lp-"+t.Name())
	defer os.RemoveAll(dir)
	if err != nil {
		t.Errorf(fmt.Sprintf("Unable to get tempfile %v", err))
		return
	}
	fname := path.Join(dir, "tmp.flv")
	oname := path.Join(dir, "out.m3u8")
	cmd := "-i test.flv -bsf:v noise=dropamount=10:amount=2147483647 -c:a copy -c:v copy -copyinkf -y " + fname
	c := exec.Command("ffmpeg", strings.Split(cmd, " ")...)
	err = c.Run()
	if err != nil {
		t.Errorf(fmt.Sprintf("Unable to run 'ffmpeg %v' - %v", cmd, err))
		return
	}

	// sanity check tempfile doesn't have a video keyframe at the beginning
	out, err = ffprobe_firstframeflags(fname)
	if err != nil || out != "flags=__" {
		t.Errorf("First video packet of temp file unexpected; %v - %v", out, err)
		return
	}

	// actually segment
	ffmpeg.InitFFmpeg()
	err = ffmpeg.RTMPToHLS(fname, oname, path.Join(dir, "out")+"_%d.ts", "4", 0)
	if err != nil {
		t.Errorf("Error segmenting - %v", err)
		return
	}
	// and now check that segmented result does have keyframe at beginning
	out, err = ffprobe_firstframeflags(path.Join(dir, "out_0.ts"))
	if err != nil || out != "flags=K_" {
		t.Errorf("Segment did not have keyframe at beginning %v - %v", out, err)
		return
	}
}
