package transcoder

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/livepeer/lpms/ffmpeg"
)

func Over1Pct(val int, cmp int) bool {
	return float32(val) > float32(cmp)*1.01 || float32(val) < float32(cmp)*0.99
}

func TestTrans(t *testing.T) {
	configs := []ffmpeg.VideoProfile{
		ffmpeg.P144p30fps16x9,
		ffmpeg.P240p30fps16x9,
		ffmpeg.P576p30fps16x9,
	}
	ffmpeg.InitFFmpeg()
	tr := NewFFMpegSegmentTranscoder(configs, "./")
	r, err := tr.Transcode("test.ts")
	if err != nil {
		t.Errorf("Error transcoding: %v", err)
	}

	if r == nil {
		t.Errorf("Did not get output")
	}

	if len(r) != 3 {
		t.Errorf("Expecting 2 output segments, got %v", len(r))
	}

	if len(r[0]) < 250000 || len(r[0]) > 280000 {
		t.Errorf("Expecting output size to be between 250000 and 280000 , got %v", len(r[0]))
	}

	if len(r[1]) < 280000 || len(r[1]) > 310000 {
		t.Errorf("Expecting output size to be between 280000 and 310000 , got %v", len(r[1]))
	}

	if len(r[2]) < 600000 || len(r[2]) > 700000 {
		t.Errorf("Expecting output size to be between 600000 and 700000, got %v", len(r[2]))
	}
}

func TestInvalidProfiles(t *testing.T) {

	// 11 profiles; max 10
	configs := []ffmpeg.VideoProfile{
		ffmpeg.P144p30fps16x9,
		ffmpeg.P240p30fps16x9,
		ffmpeg.P576p30fps16x9,
		ffmpeg.P360p30fps16x9,
		ffmpeg.P720p30fps16x9,
		ffmpeg.P144p30fps16x9,
		ffmpeg.P240p30fps16x9,
		ffmpeg.P576p30fps16x9,
		ffmpeg.P360p30fps16x9,
		ffmpeg.P720p30fps16x9,
		ffmpeg.P144p30fps16x9,
	}
	ffmpeg.InitFFmpeg()
	tr := NewFFMpegSegmentTranscoder(configs, "./")
	_, err := tr.Transcode("test.ts")
	if err == nil {
		t.Errorf("Expected an error transcoding too many segments")
	} else if err.Error() != "Invalid argument" {
		t.Errorf("Did not get the expected error while transcoding: %v", err)
	}

	// no profiles
	configs = []ffmpeg.VideoProfile{}
	tr = NewFFMpegSegmentTranscoder(configs, "./")
	_, err = tr.Transcode("test.ts")
	if err != nil {
		t.Error(err)
	}
}

type StreamTest struct {
	Tempdir    string
	Tempfile   string
	Transcoder *FFMpegSegmentTranscoder
}

func NewStreamTest(t *testing.T, configs []ffmpeg.VideoProfile) (*StreamTest, error) {
	d, err := ioutil.TempDir("", "lp-"+t.Name())
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to get tempdir ", err))
	}
	f := fmt.Sprintf("%v/tmp.ts", d)
	tr := NewFFMpegSegmentTranscoder(configs, "./")
	ffmpeg.InitFFmpeg()
	return &StreamTest{Tempdir: d, Tempfile: f, Transcoder: tr}, nil
}

func (s *StreamTest) Close() {
	os.RemoveAll(s.Tempdir)
}

func (s *StreamTest) CmdCompareSize(cmd string, sz int) error {
	c := exec.Command("ffmpeg", strings.Split(cmd+" "+s.Tempfile, " ")...)
	err := c.Run()
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to run ffmpeg %v %v- %v", cmd, s.Tempfile, err))
	}
	r, err := s.Transcoder.Transcode(s.Tempfile)
	if err != nil {
		return errors.New(fmt.Sprintf("Error transcoding ", err))
	}
	if Over1Pct(len(r[0]), sz) {
		errors.New(fmt.Sprintf("Expecting output to be within 1pct of %v, got %v (%v)", sz, len(r[0]), float32(len(r[0]))/float32(sz)))
	}
	return nil
}

func TestSingleStream(t *testing.T) {

	configs := []ffmpeg.VideoProfile{
		ffmpeg.P144p30fps16x9,
	}
	st, err := NewStreamTest(t, configs)
	if err != nil {
		t.Error(err)
	}
	defer st.Close()

	// omit audio
	err = st.CmdCompareSize("-i test.ts -an -c:v copy -y", 64108)
	if err != nil {
		t.Error(err)
	}

	// omit video
	err = st.CmdCompareSize("-i test.ts -vn -c:a copy -y", 204356)
	if err != nil {
		t.Error(err)
	}

	// XXX test no stream case
}

func TestInvalidFile(t *testing.T) {
	configs := []ffmpeg.VideoProfile{
		ffmpeg.P144p30fps16x9,
	}
	tr := NewFFMpegSegmentTranscoder(configs, "./")
	ffmpeg.InitFFmpeg()

	// nonexistent file
	_, err := tr.Transcode("nothere.ts")
	if err == nil {
		t.Errorf("Expected an error transcoding a nonexistent file")
	} else if err.Error() != "No such file or directory" {
		t.Errorf("Did not get the expected error while transcoding: %v", err)
	}

	// existing but invalid file
	thisfile := "ffmpeg_segment_transcoder_test.go"
	_, err = os.Stat(thisfile)
	if os.IsNotExist(err) {
		t.Errorf("The file '%v' does not exist", thisfile)
	}
	_, err = tr.Transcode(thisfile)
	if err == nil {
		t.Errorf("Expected an error transcoding an invalid file")
	} else if err.Error() != "Invalid data found when processing input" {
		t.Errorf("Did not get the expected error while transcoding: %v", err)
	}

	// test invalid output params
	vp := ffmpeg.VideoProfile{
		Name: "OddDimension", Bitrate: "100k", Framerate: 10,
		AspectRatio: "6:5", Resolution: "852x481"}
	st, err := NewStreamTest(t, []ffmpeg.VideoProfile{vp})
	defer st.Close()
	if err != nil {
		t.Error(err)
	}
	_, err = st.Transcoder.Transcode("test.ts")
	// XXX Make the returned error more descriptive;
	// here x264 doesn't like odd heights
	if err == nil || err.Error() != "Generic error in an external library" {
		t.Error(err)
	}

	// test bad output file names / directories
	tr = NewFFMpegSegmentTranscoder(configs, "/asdf/qwerty!")
	_, err = tr.Transcode("test.ts")
	if err == nil || err.Error() != "No such file or directory" {
		t.Error(err)
	}
}
