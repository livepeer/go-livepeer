package core

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubMetadata(sess string, profile ...ffmpeg.VideoProfile) *SegTranscodingMetadata {
	return &SegTranscodingMetadata{AuthToken: &net.AuthToken{SessionId: sess}, Profiles: profile}
}

func TestLocalTranscoder(t *testing.T) {
	tmp := t.TempDir()
	tc := NewLocalTranscoder(tmp)
	ffmpeg.InitFFmpeg()

	md := stubMetadata("", videoProfiles...)
	md.Fname = "test.ts"
	res, err := tc.Transcode(context.TODO(), md)
	if err != nil {
		t.Error("Error transcoding ", err)
	}
	if len(res.Segments) != len(videoProfiles) {
		t.Error("Mismatched results")
	}
	if Over1Pct(len(res.Segments[0].Data), 585620) {
		t.Errorf("Wrong data %v", len(res.Segments[0].Data))
	}
	if Over1Pct(len(res.Segments[1].Data), 813100) {
		t.Errorf("Wrong data %v", len(res.Segments[1].Data))
	}
}
func TestNvidia_Transcoder(t *testing.T) {
	dev := os.Getenv("NV_DEVICE")
	if dev == "" {
		t.Skip("No device specified; skipping remainder of Nvidia tests")
		return
	}

	WorkDir = t.TempDir()
	defer func() { WorkDir = "" }()
	// test.ts sample isn't in a supported pixel format, so use this instead
	fname := "test2.ts"

	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	md := stubMetadata("", profiles...)
	md.Fname = fname

	ffmpeg.InitFFmpeg()
	tc := NewNvidiaTranscoder(dev)
	res, err := tc.Transcode(context.TODO(), md)
	if err != nil {
		t.Error(err)
	}
	if Over1Pct(len(res.Segments[0].Data), 659692) {
		t.Errorf("Wrong data %v", len(res.Segments[0].Data))
	}
	if Over1Pct(len(res.Segments[1].Data), 874012) {
		t.Errorf("Wrong data %v", len(res.Segments[1].Data))
	}

	// transcoding should panic due to invalid device 123
	tc = NewNvidiaTranscoder("123")
	defer func() {
		if err := recover(); err == nil {
			t.Error("Expected error with invalid device")
		}
	}()
	_, err = tc.Transcode(context.TODO(), md)
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
	_, err := resToTranscodeData(context.TODO(), res, []ffmpeg.TranscodeOptions{})
	assert.EqualError(err, "lengths of results and options different")

	// Test immediate read error
	opts := []ffmpeg.TranscodeOptions{{Oname: "badfile"}}
	_, err = resToTranscodeData(context.TODO(), res, opts)
	assert.EqualError(err, "open badfile: no such file or directory")

	// Test error after a successful read
	res = &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 3)}
	tempDir := t.TempDir()

	file1, err := ioutil.TempFile(tempDir, "foo")
	require.Nil(err)
	file2, err := ioutil.TempFile(tempDir, "bar")
	require.Nil(err)

	opts = make([]ffmpeg.TranscodeOptions, 3)
	opts[0].Oname = file1.Name()
	opts[1].Oname = "badfile"
	opts[2].Oname = file2.Name()

	_, err = resToTranscodeData(context.TODO(), res, opts)
	assert.EqualError(err, "open badfile: no such file or directory")
	assert.True(fileDNE(file1.Name()))
	assert.False(fileDNE(file2.Name()))

	// Test success for 1 output file
	res = &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 1)}
	res.Encoded[0].Pixels = 100

	opts = []ffmpeg.TranscodeOptions{{Oname: file2.Name()}}
	tData, err := resToTranscodeData(context.TODO(), res, opts)
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

	tData, err = resToTranscodeData(context.TODO(), res, opts)
	assert.Nil(err)
	assert.Equal(2, len(tData.Segments))
	assert.Equal(int64(200), tData.Segments[0].Pixels)
	assert.Equal(int64(300), tData.Segments[1].Pixels)
	assert.True(fileDNE(file1.Name()))
	assert.True(fileDNE(file2.Name()))

	// Test signature file
	res = &ffmpeg.TranscodeResults{Encoded: make([]ffmpeg.MediaInfo, 1)}
	pHash := []byte{4, 2, 0, 6, 9}

	file1, err = ioutil.TempFile(tempDir, "foo")
	require.Nil(err)
	ioutil.WriteFile(file1.Name()+".bin", pHash, 0664)

	opts = make([]ffmpeg.TranscodeOptions, 1)
	opts[0].Oname = file1.Name()
	opts[0].CalcSign = true

	tData, err = resToTranscodeData(context.TODO(), res, opts)
	assert.Nil(err)
	assert.Equal(tData.Segments[0].PHash, pHash)
	assert.True(fileDNE(file1.Name()))
}

func TestProfilesToTranscodeOptions(t *testing.T) {
	workDir := "foo"

	assert := assert.New(t)

	oldRandIDFunc := common.RandomIDGenerator
	common.RandomIDGenerator = func(uint) string {
		return "bar"
	}
	defer func() { common.RandomIDGenerator = oldRandIDFunc }()

	makeMeta := func(p []ffmpeg.VideoProfile, c bool) *SegTranscodingMetadata {
		return &SegTranscodingMetadata{
			Profiles:           p,
			CalcPerceptualHash: c,
			Metadata: map[string]string{
				"meta": "data",
			},
		}
	}

	// Test 0 profiles
	profiles := []ffmpeg.VideoProfile{}
	opts := profilesToTranscodeOptions(workDir, ffmpeg.Software, makeMeta(profiles, false))
	assert.Equal(0, len(opts))

	// Test 1 profile
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Software, makeMeta(profiles, false))
	assert.Equal(1, len(opts))
	assert.Equal("foo/out_bar.tempfile", opts[0].Oname)
	assert.Equal(ffmpeg.Software, opts[0].Accel)
	assert.Equal(ffmpeg.P144p30fps16x9, opts[0].Profile)
	assert.Equal("copy", opts[0].AudioEncoder.Name)

	// Test > 1 profile
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Software, makeMeta(profiles, false))
	assert.Equal(2, len(opts))

	for i, p := range profiles {
		assert.Equal("foo/out_bar.tempfile", opts[i].Oname)
		assert.Equal(ffmpeg.Software, opts[i].Accel)
		assert.Equal(p, opts[i].Profile)
		assert.Equal("copy", opts[i].AudioEncoder.Name)
		assert.Equal(opts[i].Metadata, map[string]string{
			"meta": "data",
		})
	}

	// Test different acceleration value
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Nvidia, makeMeta(profiles, false))
	assert.Equal(2, len(opts))

	// Test signature calculation
	opts = profilesToTranscodeOptions(workDir, ffmpeg.Nvidia, makeMeta(profiles, true))
	assert.True(opts[0].CalcSign)
	assert.True(opts[1].CalcSign)

	for i, p := range profiles {
		assert.Equal("foo/out_bar.tempfile", opts[i].Oname)
		assert.Equal(ffmpeg.Nvidia, opts[i].Accel)
		assert.Equal(p, opts[i].Profile)
		assert.Equal("copy", opts[i].AudioEncoder.Name)
	}
}

func TestAudioCopy(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
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

	md := stubMetadata("", videoProfiles...)
	md.Fname = audioSample
	_, err = tc.Transcode(context.TODO(), md)
	assert.Equal(ffmpeg.ErrTranscoderVid, err)
}

func TestTranscoder_Formats(t *testing.T) {
	// Helps ensure the necessary ffmpeg configure options are enabled
	assert := assert.New(t)
	dir := t.TempDir()

	in := &ffmpeg.TranscodeOptionsIn{Fname: "test.ts"}
	for k, v := range ffmpeg.ExtensionFormats {
		assert.NotEqual(ffmpeg.FormatNone, v) // sanity check

		p := ffmpeg.P144p30fps16x9 // make a copy bc we mutate the profile
		p.Format = v

		// use LPMS api directly so we can transcode ; faster
		out := []ffmpeg.TranscodeOptions{{
			Oname:        dir + "/tmp" + k,
			Profile:      p,
			VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		}}
		_, err := ffmpeg.Transcode3(in, out)
		assert.Nil(err)

		// check output is reasonable
		ofile, err := ioutil.ReadFile(out[0].Oname)
		assert.Nil(err)
		assert.Greater(len(ofile), 500000) // large enough for "valid output"
		// Assume that since the file exists, the actual format is correct
	}
	// sanity check the base format wasn't overwritten (has happened before!)
	assert.Equal(ffmpeg.FormatNone, ffmpeg.P144p30fps16x9.Format)
}

func TestRecoverFromPanic(t *testing.T) {
	assert := assert.New(t)

	f := func() (err error) {
		defer recoverFromPanic(&err)
		panic(struct{}{})
	}

	err := f()

	assert.NotNil(err)
	assert.Equal("unrecoverable transcoding failure", err.Error())
	assert.IsType(UnrecoverableError{}, err)
}

func TestRecoverFromPanic_WithError(t *testing.T) {
	assert := assert.New(t)
	sampleErr := errors.New("sample error")

	f := func() (err error) {
		defer recoverFromPanic(&err)
		panic(sampleErr)
	}

	err := f()

	assert.Equal(NewUnrecoverableError(sampleErr), err)
}
