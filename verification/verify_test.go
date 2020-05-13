package verification

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
)

func TestFatalRetryable(t *testing.T) {
	errNonRetryable := errors.New("NonRetryable")
	assert.False(t, IsRetryable(errNonRetryable))
	assert.False(t, IsFatal(errNonRetryable))

	errRetryable := Retryable{errors.New("Retryable")}
	assert.True(t, IsRetryable(errRetryable))
	assert.False(t, IsFatal(errRetryable))

	errFatalRetryable := Fatal{Retryable{errors.New("FatalRetryable")}}
	assert.True(t, IsRetryable(errFatalRetryable))
	assert.True(t, IsFatal(errFatalRetryable))

	errNonFatalRetryable := Retryable{errors.New("NonFatalRetryable")}
	assert.True(t, IsRetryable(errNonFatalRetryable))
	assert.False(t, IsFatal(errNonFatalRetryable))

	// check that these Retryable errors are classified
	// correctly as Fatal / Non-fatal:

	// check ErrAudioMismatch
	assert.True(t, IsRetryable(ErrAudioMismatch))
	assert.True(t, IsFatal(ErrAudioMismatch))

	// check ErrPixelMismatch
	assert.True(t, IsRetryable(ErrPixelMismatch))
	assert.False(t, IsFatal(ErrPixelMismatch))

	// check ErrTampered
	assert.True(t, IsRetryable(ErrTampered))
	assert.False(t, IsFatal(ErrTampered))
}

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
	assert.NotNil(res)
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

	// check pixel count fails w/ external verifier
	data.Segments = append(data.Segments, &net.TranscodedSegmentData{Url: "def", Pixels: verifier.results.Pixels[1]})
	assert.Len(data.Segments, len(verifier.results.Pixels)) // sanity check
	res, err = sv.Verify(&Params{Results: data})
	assert.Nil(res)
	assert.Equal(ErrPixelMismatch, err)

	// Check pixel count succeeds w/ external verifier
	data.Segments[0].Pixels = verifier.results.Pixels[0]
	res, err = sv.Verify(&Params{Results: data})
	assert.Nil(err)
	assert.NotNil(res)

	// check pixel count fails w/o external verifier
	sv = NewSegmentVerifier(&Policy{Verifier: nil, Retries: 2}) // reset
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "abc", Pixels: verifier.results.Pixels[0] + 1},
	}}
	data.Segments = append(data.Segments, &net.TranscodedSegmentData{Url: "def", Pixels: verifier.results.Pixels[1]})
	assert.Len(data.Segments, len(verifier.results.Pixels)) // sanity check
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{}})
	assert.Nil(res)
	assert.Equal(ErrPixelsAbsent, err)

	// Check pixel count succeeds w/o external verifier
	sv = &SegmentVerifier{policy: &Policy{Verifier: nil, Retries: 2}, verifySig: func(ethcommon.Address, []byte, []byte) bool { return true }}
	pxls, err := pixels("../server/test.flv")

	a, err := ioutil.ReadFile("../server/test.flv")
	assert.Nil(err)
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "../server/test.flv", Pixels: pxls},
		{Url: "../server/test.flv", Pixels: pxls},
	}}
	renditions := [][]byte{a, a}
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{TicketParams: &net.TicketParams{}}, Renditions: renditions})
	assert.Nil(err)
	assert.NotNil(res)

	// check pixel count fails w/o external verifier w populated renditions but incorrect pixel counts
	sv = NewSegmentVerifier(&Policy{Verifier: nil, Retries: 2}) // reset
	pxls, err = pixels("../server/test.flv")
	assert.Nil(err)
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "../server/test.flv", Pixels: 123},
		{Url: "../server/test.flv", Pixels: 456},
	}}
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{}, Renditions: renditions})
	assert.Nil(res)
	assert.Equal(ErrPixelMismatch, err)

	// Check pixel count succeeds w/o external verifier
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "../server/test.flv", Pixels: pxls},
		{Url: "../server/test.flv", Pixels: pxls},
	}}
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{}, Renditions: renditions})
	assert.Nil(err)
	assert.NotNil(res)

	// Check sig verifier fails when len(params.Results.Segments) != len(params.Renditions)
	pxls, err = pixels("../server/test.flv")
	assert.Nil(err)
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "../server/test.flv", Pixels: pxls},
		{Url: "../server/test.flv", Pixels: pxls},
	}}
	renditions = [][]byte{}
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{TicketParams: &net.TicketParams{}}, Renditions: renditions})
	assert.Equal(errPMCheckFailed, err)
	assert.Nil(res)

	// Check sig verifier passes when len(params.Results.Segments) == len(params.Renditions) and sig is valid
	// We use this stubVerifier to make sure that ffmpeg doesnt try to read from a file
	// Verify sig against ticket recipient address
	recipientAddr := ethcommon.BytesToAddress([]byte("foo"))
	sv = &SegmentVerifier{policy: &Policy{Verifier: &stubVerifier{
		results: nil,
		err:     nil,
	}, Retries: 2},
		verifySig: func(addr ethcommon.Address, msg []byte, sig []byte) bool { return addr == recipientAddr },
	}

	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "xyz", Pixels: pxls},
		{Url: "xyz", Pixels: pxls},
	}, Sig: []byte{}}

	renditions = [][]byte{{0}, {0}}
	params := &net.TicketParams{Recipient: recipientAddr.Bytes()}
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{TicketParams: params}, Renditions: renditions})
	assert.Nil(err)
	assert.NotNil(res)

	// Verify sig against orchestrator provided address
	orchAddr := ethcommon.BytesToAddress([]byte("bar"))
	sv = &SegmentVerifier{
		policy:    &Policy{Verifier: &stubVerifier{}, Retries: 2},
		verifySig: func(addr ethcommon.Address, msg []byte, sig []byte) bool { return addr == orchAddr },
	}

	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{TicketParams: params, Address: orchAddr.Bytes()}, Renditions: renditions})
	assert.Nil(err)
	assert.NotNil(res)

	// Check sig verifier fails when sig is missing / invalid
	sv = NewSegmentVerifier(&Policy{Verifier: &stubVerifier{
		results: nil,
		err:     nil,
	}, Retries: 2})
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "uvw", Pixels: pxls},
		{Url: "xyz", Pixels: pxls},
	}}
	res, err = sv.Verify(&Params{Results: data, Orchestrator: &net.OrchestratorInfo{TicketParams: &net.TicketParams{}}, Renditions: renditions})
	assert.Equal(errPMCheckFailed, err)
	assert.Nil(res)

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
	data = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{
		{Url: "abc", Pixels: verifier.results.Pixels[0]},
		{Url: "def", Pixels: verifier.results.Pixels[1]},
	}}
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

	// A high score should still fail if audio checking fails
	sv.policy = &Policy{Verifier: &stubVerifier{
		results: &Results{Score: 21.0, Pixels: []int64{-1, -2}},
		err:     ErrAudioMismatch,
	}, Retries: 2}
	res, err = sv.Verify(&Params{ManifestID: "vws", Results: data})
	assert.Equal(ErrAudioMismatch, err)
	assert.Equal("mno", string(res.ManifestID))

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

	p, err := pixels("foo")
	assert.EqualError(err, "No such file or directory")
	assert.Equal(int64(0), p)

	// Assume that ffmpeg.Transcode3() returns the correct pixel count so we just
	// check that no error is returned
	p, err = pixels("../server/test.flv")
	assert.Nil(err)
	assert.NotZero(p)
}

// helper function for TestVerifyPixels to test countPixels()
func verifyPixels(fname string, data []byte, reportedPixels int64) error {
	c, err := countPixels(data)
	if err != nil {
		return err
	}

	if c != reportedPixels {
		return ErrPixelMismatch
	}

	return nil
}

func TestVerifyPixels(t *testing.T) {
	ffmpeg.InitFFmpeg()

	require := require.New(t)
	assert := assert.New(t)

	// Create memory session and save a test file
	bos := drivers.NewMemoryDriver(nil).NewSession("foo")
	data, err := ioutil.ReadFile("../server/test.flv")
	require.Nil(err)
	fname, err := bos.SaveData("test.ts", data)
	require.Nil(err)
	memOS, ok := bos.(*drivers.MemorySession)
	require.True(ok)

	// Test error for relative URI and no memory storage if the file does not exist on disk
	// Will try to use the relative URI to read the file from disk and fail
	err = verifyPixels(fname, nil, 50)
	assert.EqualError(err, "Invalid data found when processing input")

	// Test writing temp file for relative URI and local memory storage with incorrect pixels
	err = verifyPixels(fname, memOS.GetData(fname), 50)
	assert.Equal(ErrPixelMismatch, err)

	// Test writing temp file for relative URI and local memory storage with correct pixels
	// Make sure that verifyPixels() checks against the output of pixels()
	p, err := pixels("../server/test.flv")
	require.Nil(err)
	err = verifyPixels(fname, memOS.GetData(fname), p)
	assert.Nil(err)

	// Test nil data
	err = verifyPixels("../server/test.flv", nil, 50)
	assert.EqualError(err, "Invalid data found when processing input")
}
