package core

import (
	"io/ioutil"
	"os"
	"testing"

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

	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	res, err := tc.Transcode("", "test.ts", profiles)
	if err != nil {
		t.Error("Error transcoding ", err)
	}
	if len(res.Segments) != len(profiles) {
		t.Error("Mismatched results")
	}
	if Over1Pct(len(res.Segments[0].Data), 135924) {
		t.Errorf("Wrong data %v", len(res.Segments[0].Data))
	}
	if Over1Pct(len(res.Segments[1].Data), 199468) {
		t.Errorf("Wrong data %v", len(res.Segments[1].Data))
	}
}

func TestNvidiaTranscoder(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	tc := NewNvidiaTranscoder("123,456", tmp)
	ffmpeg.InitFFmpeg()

	// test device selection
	nv, ok := tc.(*NvidiaTranscoder)
	if !ok || "456" != nv.getDevice() || "123" != nv.getDevice() ||
		"456" != nv.getDevice() || "123" != nv.getDevice() {
		t.Error("Error when getting devices")
	}

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
	tc = NewNvidiaTranscoder(dev, tmp)
	res, err := tc.Transcode("", fname, profiles)
	if err != nil {
		t.Error(err)
	}
	if Over1Pct(len(res.Segments[0].Data), 650292) {
		t.Errorf("Wrong data %v", len(res.Segments[0].Data))
	}
	if Over1Pct(len(res.Segments[1].Data), 884164) {
		t.Errorf("Wrong data %v", len(res.Segments[1].Data))
	}
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
	opts := []ffmpeg.TranscodeOptions{ffmpeg.TranscodeOptions{Oname: "badfile"}}
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

	opts = []ffmpeg.TranscodeOptions{ffmpeg.TranscodeOptions{Oname: file2.Name()}}
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
	assert.Equal("foo/out_bar.ts", opts[0].Oname)
	assert.Equal(ffmpeg.Software, opts[0].Accel)
	assert.Equal(ffmpeg.P144p30fps16x9, opts[0].Profile)
	assert.Equal("copy", opts[0].AudioEncoder.Name)

	// Test > 1 profile
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Software, profiles)
	assert.Equal(2, len(opts))

	for i, p := range profiles {
		assert.Equal("foo/out_bar.ts", opts[i].Oname)
		assert.Equal(ffmpeg.Software, opts[i].Accel)
		assert.Equal(p, opts[i].Profile)
		assert.Equal("copy", opts[i].AudioEncoder.Name)
	}

	// Test different acceleration value
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Nvidia, profiles)
	assert.Equal(2, len(opts))

	for i, p := range profiles {
		assert.Equal("foo/out_bar.ts", opts[i].Oname)
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
	out := []ffmpeg.TranscodeOptions{ffmpeg.TranscodeOptions{
		Oname:        audioSample,
		VideoEncoder: ffmpeg.ComponentOptions{Name: "drop"},
		AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
	}}
	_, err := ffmpeg.Transcode3(in, out)
	assert.Nil(err)

	profs := []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9} // dummy
	res, err := tc.Transcode("", audioSample, profs)
	assert.Nil(err)

	o, err := ioutil.ReadFile(audioSample)
	assert.Nil(err)
	assert.Equal(o, res.Segments[0].Data)
}
