package core

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalTranscoder(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	tc := NewLocalTranscoder(tmp)
	ffmpeg.InitFFmpeg()

	res, err := tc.Transcode("", "test.ts", videoProfiles)
	if err != nil {
		t.Error("Error transcoding ", err)
	}
	if len(res.Segments) != len(videoProfiles) {
		t.Error("Mismatched results")
	}
	if Over1Pct(len(res.Segments[0].Data), 164876) {
		t.Errorf("Wrong data %v", len(res.Segments[0].Data))
	}
	if Over1Pct(len(res.Segments[1].Data), 224848) {
		t.Errorf("Wrong data %v", len(res.Segments[1].Data))
	}
}

func TestNvidia_Transcoder(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	StartNvidiaTranscoders("123,456", tmp)
	tc := NewNvidiaTranscoder("123")
	ffmpeg.InitFFmpeg()

	// test.ts sample isn't in a supported pixel format, so use this instead
	fname := "test2.ts"

	// transcoding should fail due to invalid devices
	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	_, err := tc.Transcode("", fname, profiles)
	if err == nil ||
		(err.Error() != "Unknown error occurred" &&
			err.Error() != "Cannot allocate memory") {
		t.Error(err)
	}

	dev := os.Getenv("NV_DEVICE")
	if dev == "" {
		t.Skip("No device specified; skipping remainder of Nvidia tests")
		return
	}
	StartNvidiaTranscoders(dev, tmp)
	tc = NewNvidiaTranscoder(dev)
	res, err := tc.Transcode("", fname, profiles)
	if err != nil {
		t.Error(err)
	}
	if Over1Pct(len(res.Segments[0].Data), 485416) {
		t.Errorf("Wrong data %v", len(res.Segments[0].Data))
	}
	if Over1Pct(len(res.Segments[1].Data), 771740) {
		t.Errorf("Wrong data %v", len(res.Segments[1].Data))
	}
}

func TestNvidia_Stack(t *testing.T) {
	assert := assert.New(t)
	ss := newSegStack("")
	assert.Empty(ss.segs, "Sanity check for empty stack")
	for i := 0; i < 1000; i++ {
		fname := fmt.Sprintf("%d", i)
		ss.push(&nvSegData{fname: fname})
	}
	for i := 999; i >= 0; i-- {
		fname := fmt.Sprintf("%d", i)
		seg := ss.pop()
		assert.Equal(fname, seg.fname)
	}
	assert.Empty(ss.segs, "Stack nonempty") // sanity check

	// "Check" popping an empty stack blocks. Approximate this with a timeout
	wg := newWg(1)
	go func() {
		ss.pop()
		wg.Done()
	}()
	assert.False(wgWait2(wg, 100*time.Millisecond), "Did not time out on empty stack")
}

func TestNvidia_ConcurrentStack(t *testing.T) {
	// Run this under `-race`
	ss := newSegStack("")
	wg := newWg(1000)
	for i := 0; i < 1000; i++ {
		go ss.push(&nvSegData{})
		go func() {
			// can't sanity check returned values here because
			// order of insertion is unpredictable. Just run this under `-race`
			ss.pop()
			wg.Done()
		}()
	}
	wgWait(wg)
}

func TestNvidia_ConcurrentTranscodes(t *testing.T) {
	assert := assert.New(t)
	dev := os.Getenv("NV_DEVICE")
	if dev == "" {
		t.Skip("No device specified; skipping remainder of ", t.Name())
		return
	}
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	StartNvidiaTranscoders(dev, tmp)
	tc := NewNvidiaTranscoder(dev)
	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	wg := newWg(5)
	for i := 0; i < 5; i++ {
		go func() {
			res, err := tc.Transcode("", "test2.ts", profiles)
			assert.Nil(err, "Error transcoding")
			assert.InEpsilon(487484, len(res.Segments[0].Data), 0.01, fmt.Sprintf("Expected within 1%% of %d", len(res.Segments[0].Data)))
			assert.InEpsilon(766288, len(res.Segments[1].Data), 0.01, fmt.Sprintf("Expected within 1%% of %d", len(res.Segments[1].Data)))
			wg.Done()
		}()
	}
	assert.True(wgWait2(wg, 20*time.Second), "Transcodes timed out") // can be slow
}

func TestResToTranscodeData(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fileDNE := func(fname string) bool {
		_, err := os.Stat(fname)
		return os.IsNotExist(err)
	}

	// Test lengths of results and options different error
	res := &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 1)}
	_, err := resToTranscodeData(res, []ffmpeg.TranscodeOptions{})
	assert.EqualError(err, "lengths of results and options different")

	// Test immediate read error
	opts := []ffmpeg.TranscodeOptions{{Oname: "badfile"}}
	_, err = resToTranscodeData(res, opts)
	assert.EqualError(err, "open badfile: no such file or directory")

	// Test error after a successful read
	res = &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 3)}
	tempDir, err := ioutil.TempDir("", "TestResToTranscodeData")
	require.Nil(err)
	defer os.Remove(tempDir)

	file1, err := ioutil.TempFile(tempDir, "foo")
	require.Nil(err)
	file2, err := ioutil.TempFile(tempDir, "bar")
	require.Nil(err)

	opts = make([]ffmpeg.TranscodeOptions, 3)
	opts[0].Oname = file1.Name()
	opts[1].Oname = "badfile"
	opts[2].Oname = file2.Name()

	_, err = resToTranscodeData(res, opts)
	assert.EqualError(err, "open badfile: no such file or directory")
	assert.True(fileDNE(file1.Name()))
	assert.False(fileDNE(file2.Name()))

	// Test success for 1 output file
	res = &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 1)}
	res.Encoded[0].Pixels = 100

	opts = []ffmpeg.TranscodeOptions{{Oname: file2.Name()}}
	tData, err := resToTranscodeData(res, opts)
	assert.Nil(err)
	assert.Equal(1, len(tData.Segments))
	assert.Equal(int64(100), tData.Segments[0].Pixels)
	assert.True(fileDNE(file2.Name()))

	// Test succes for 2 output files
	res = &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 2)}
	res.Encoded[0].Pixels = 200
	res.Encoded[1].Pixels = 300

	file1, err = ioutil.TempFile(tempDir, "foo")
	require.Nil(err)
	file2, err = ioutil.TempFile(tempDir, "bar")
	require.Nil(err)

	opts = make([]ffmpeg.TranscodeOptions, 2)
	opts[0].Oname = file1.Name()
	opts[1].Oname = file2.Name()

	tData, err = resToTranscodeData(res, opts)
	assert.Nil(err)
	assert.Equal(2, len(tData.Segments))
	assert.Equal(int64(200), tData.Segments[0].Pixels)
	assert.Equal(int64(300), tData.Segments[1].Pixels)
	assert.True(fileDNE(file1.Name()))
	assert.True(fileDNE(file2.Name()))
}

func TestProfilesToTranscodeOptions(t *testing.T) {
	workDir := "foo"

	assert := assert.New(t)

	oldRandIDFunc := common.RandomIDGenerator
	common.RandomIDGenerator = func(uint) string {
		return "bar"
	}
	defer func() { common.RandomIDGenerator = oldRandIDFunc }()

	// Test 0 profiles
	profiles := []ffmpeg.VideoProfile{}
	opts := profilesToTranscodeOptions(workDir, ffmpeg.Software, profiles)
	assert.Equal(0, len(opts))

	// Test 1 profile
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Software, profiles)
	assert.Equal(1, len(opts))
	assert.Equal("foo/out_bar.tempfile", opts[0].Oname)
	assert.Equal(ffmpeg.Software, opts[0].Accel)
	assert.Equal(ffmpeg.P144p30fps16x9, opts[0].Profile)
	assert.Equal("copy", opts[0].AudioEncoder.Name)

	// Test > 1 profile
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Software, profiles)
	assert.Equal(2, len(opts))

	for i, p := range profiles {
		assert.Equal("foo/out_bar.tempfile", opts[i].Oname)
		assert.Equal(ffmpeg.Software, opts[i].Accel)
		assert.Equal(p, opts[i].Profile)
		assert.Equal("copy", opts[i].AudioEncoder.Name)
	}

	// Test different acceleration value
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Nvidia, profiles)
	assert.Equal(2, len(opts))

	for i, p := range profiles {
		assert.Equal("foo/out_bar.tempfile", opts[i].Oname)
		assert.Equal(ffmpeg.Nvidia, opts[i].Accel)
		assert.Equal(p, opts[i].Profile)
		assert.Equal("copy", opts[i].AudioEncoder.Name)
	}
}

func TestAudioCopy(t *testing.T) {
	assert := assert.New(t)
	dir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(dir)
	tc := NewLocalTranscoder(dir)
	ffmpeg.InitFFmpeg()

	// Ensure that audio is actually copied.
	// Do this by generating an audio-only stream, running it through the
	// LocalTranscoder API and checking that the result is identical.
	audioSample := dir + "/audio-copy.ts"
	in := &ffmpeg.TranscodeOptionsIn{Fname: "test.ts"}
	out := []ffmpeg.TranscodeOptions{{
		Oname:        audioSample,
		VideoEncoder: ffmpeg.ComponentOptions{Name: "drop"},
		AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
	}}
	_, err := ffmpeg.Transcode3(in, out)
	assert.Nil(err)

	res, err := tc.Transcode("", audioSample, videoProfiles)
	assert.Nil(err)

	o, err := ioutil.ReadFile(audioSample)
	assert.Nil(err)
	assert.Equal(o, res.Segments[0].Data)
}
