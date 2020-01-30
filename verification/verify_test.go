package verification

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
)

type stubVerifier struct {
	results *Results
	err     error
}

func (sv *stubVerifier) Verify(params *Params) (*Results, error) {
	return sv.results, sv.err
}

func TestVerify(t *testing.T) {

	assert := assert.New(t)

	verifier := &stubVerifier{
		results: &Results{Score: 9.3, Pixels: []int64{123, 456}},
		err:     errors.New("Stub Verifier Error")}

	// Check empty policy and verifier
	sv := NewSegmentVerifier(nil)
	res, err := sv.Verify(&Params{})
	assert.Nil(res)
	assert.Nil(err)
	sv = NewSegmentVerifier(&Policy{Retries: 3})
	res, err = sv.Verify(&Params{})
	assert.Nil(res)
	assert.Nil(err)

	// Check verifier error is propagated
	sv = NewSegmentVerifier(&Policy{Verifier: verifier, Retries: 3})
	res, err = sv.Verify(&Params{})
	assert.Nil(res)
	assert.Equal(verifier.err, err)

	// Check successful verification
	// Should skip pixel counts since parameters don't specify pixels
	verifier.err = nil
	res, err = sv.Verify(&Params{})
	assert.Nil(err)
	assert.NotNil(res)

	// Check pixel list from verifier isn't what's expected
	data := &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "abc", Pixels: verifier.results.Pixels[0] + 1},
	}}
	res, err = sv.Verify(&Params{Results: data})
	assert.Nil(res)
	assert.Equal(ErrPixelsAbsent, err)

	// check pixel count fails
	data.Segments = append(data.Segments, &net.TranscodedSegmentData{Url: "def", Pixels: verifier.results.Pixels[1]})
	assert.Len(data.Segments, len(verifier.results.Pixels)) // sanity check
	res, err = sv.Verify(&Params{Results: data})
	assert.Nil(res)
	assert.Equal(ErrPixelMismatch, err)

	// Check pixel count succeeds
	data.Segments[0].Pixels = verifier.results.Pixels[0]
	res, err = sv.Verify(&Params{Results: data})
	assert.Nil(err)
	assert.NotNil(res)

	// Check retryable: 3 attempts
	sv = NewSegmentVerifier(&Policy{Verifier: verifier, Retries: 2}) // reset
	verifier.err = Retryable{errors.New("Stub Verifier Retryable Error")}
	assert.True(IsRetryable(verifier.err))
	// first attempt
	verifier.results = &Results{Score: 1.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "abc", Results: data})
	assert.Equal(err, verifier.err)
	assert.Nil(res)
	// second attempt
	verifier.results = &Results{Score: 3.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "def", Results: data})
	assert.Equal(err, verifier.err)
	assert.Nil(res)
	// final attempt should return highest scoring
	verifier.results = &Results{Score: 2.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "ghi", Results: data})
	assert.Equal(err, verifier.err)
	assert.NotNil(res)
	assert.Equal("def", string(res.ManifestID))
	// Additional attempts should still return best score winner
	verifier.results = &Results{Score: -1.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "jkl", Results: data})
	assert.Equal(err, verifier.err)
	assert.NotNil(res)
	assert.Equal("def", string(res.ManifestID))
	// If we pass in a result with a better score, that should be returned
	verifier.results = &Results{Score: 4.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "mno", Results: data})
	assert.Equal(err, verifier.err)
	assert.NotNil(res)
	assert.Equal("mno", string(res.ManifestID))

	// Pixel count handling
	verifier.err = nil
	// Good score but incorrect pixel list should still fail
	verifier.results = &Results{Score: 10.0, Pixels: []int64{789}}
	res, err = sv.Verify(&Params{ManifestID: "pqr", Results: data})
	assert.Equal(ErrPixelsAbsent, err)
	assert.Equal("mno", string(res.ManifestID)) // Still return best result
	// Higher score and incorrect pixel list should prioritize pixel count
	verifier.results = &Results{Score: 20.0, Pixels: []int64{789}}
	res, err = sv.Verify(&Params{ManifestID: "stu", Results: data})
	assert.Equal(ErrPixelsAbsent, err)
	assert.Equal("mno", string(res.ManifestID)) // Still return best result

	// Check *not* retryable; should never get a result
	sv = NewSegmentVerifier(&Policy{Verifier: verifier, Retries: 1}) // reset
	verifier.err = errors.New("Stub Verifier Non-Retryable Error")
	// first attempt
	verifier.results = &Results{Score: 1.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "abc", Results: data})
	assert.Equal(err, verifier.err)
	assert.Nil(res)
	// second attempt
	verifier.results = &Results{Score: 3.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "def", Results: data})
	assert.Equal(err, verifier.err)
	assert.Nil(res)
	// third attempt, just to make sure?
	verifier.results = &Results{Score: 2.0, Pixels: []int64{123, 456}}
	res, err = sv.Verify(&Params{ManifestID: "ghi", Results: data})
	assert.Equal(err, verifier.err)
	assert.Nil(res)
}

func TestPixels(t *testing.T) {
	ffmpeg.InitFFmpeg()

	assert := assert.New(t)

	p, err := Pixels("foo")
	assert.EqualError(err, "No such file or directory")
	assert.Equal(int64(0), p)

	// Assume that ffmpeg.Transcode3() returns the correct pixel count so we just
	// check that no error is returned
	p, err = Pixels("test.flv")
	assert.Nil(err)
	assert.NotZero(p)
}

func TestVerifyPixels(t *testing.T) {
	ffmpeg.InitFFmpeg()

	require := require.New(t)
	assert := assert.New(t)

	// Create memory session and save a test file
	bos := drivers.NewMemoryDriver(nil).NewSession("foo")
	data, err := ioutil.ReadFile("test.flv")
	require.Nil(err)
	fname, err := bos.SaveData("test.ts", data)
	require.Nil(err)

	// Test error for relative URI and no memory storage if the file does not exist on disk
	// Will try to use the relative URI to read the file from disk and fail
	err = VerifyPixels(fname, nil, 50)
	assert.EqualError(err, "No such file or directory")

	// Test error for relative URI and local memory storage if the file does not exist in storage
	// Will try to use the relative URI to read the file from storage and fail
	err = VerifyPixels("/stream/bar/dne.ts", bos, 50)
	assert.EqualError(err, "error fetching data from local memory storage")

	// Test writing temp file for relative URI and local memory storage with incorrect pixels
	err = VerifyPixels(fname, bos, 50)
	assert.EqualError(err, "mismatch between calculated and reported pixels")

	// Test writing temp file for relative URI and local memory storage with correct pixels
	// Make sure that verifyPixels() checks against the output of pixels()
	p, err := Pixels("test.flv")
	require.Nil(err)
	err = VerifyPixels(fname, bos, p)
	assert.Nil(err)

	// Test no writing temp file with incorrect pixels
	err = VerifyPixels("test.flv", nil, 50)
	assert.EqualError(err, "mismatch between calculated and reported pixels")

	// Test no writing temp file with correct pixels
	err = VerifyPixels("test.flv", nil, p)
	assert.Nil(err)
}
