package verification

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"

	"github.com/stretchr/testify/assert"
)

func TestEpic_EpicResultsToVerificationResults(t *testing.T) {
	assert := assert.New(t)

	// This function name is too long, shorten for this test
	fn := epicResultsToVerificationResults

	// Empty case
	r := &epicResults{}
	res, err := fn(r)
	assert.Nil(err)
	assert.NotNil(res)
	assert.Equal(0., res.Score)
	assert.Empty(res.Pixels)

	// Success case
	r = &epicResults{
		Results: []epicResultFields{
			{VideoAvailable: true, Pixels: 123, Tamper: 2.0},
			{VideoAvailable: true, Pixels: 456, Tamper: 3.0},
		},
	}
	res, err = fn(r)
	assert.Nil(err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((2.0+3.0)/2., res.Score)

	// Check various errors. Should always count pixels/score for each rendition

	// Check audio mismatch. Should still count pixels / score
	r.Results[0].AudioAvailable = true
	r.Results[0].AudioDistance = 1.0
	res, err = fn(r)
	assert.Equal(ErrAudioMismatch, err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((2.0+3.0)/2., res.Score)

	// Check tampering. Tamper first result only.
	// We have audio mismatch set already, which takes precedence, being fatal
	r.Results[0].Tamper = -2.0
	res, err = fn(r)
	assert.Equal(ErrAudioMismatch, err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((-2.0+3.0)/2., res.Score)
	// Disable audio mismatch and we should get a tamper error
	r.Results[0].AudioDistance = 0.0
	res, err = fn(r)
	assert.Equal(ErrTampered, err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((-2.0+3.0)/2., res.Score)

	// Tamper both results
	r.Results[1].Tamper = -3.0
	res, err = fn(r)
	assert.Equal(ErrTampered, err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((-2.0+-3.0)/2., res.Score)

	// Tamper second result only
	r.Results[0].Tamper = 2.0
	res, err = fn(r)
	assert.Equal(ErrTampered, err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((2.0-3.0)/2., res.Score)

	// Unset tamper, sanity check success
	r.Results[1].Tamper = 3.0
	res, err = fn(r)
	assert.Nil(err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((2.0+3.0)/2., res.Score)

	// Unset video availability
	r.Results[0].VideoAvailable = false
	res, err = fn(r)
	assert.Equal(ErrVideoUnavailable, err)
	assert.Equal([]int64{123, 456}, res.Pixels) // Does this even make sense?
	assert.Equal((2.0+3.0)/2., res.Score)
}

func TestEpic_WriteSegments(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", t.Name())
	assert.Nil(err)
	defer os.RemoveAll(dir) // clean up
	baseDir := filepath.Base(dir)
	bucketPath := "https://livepeer.s3.amazonaws.com"

	// empty case
	_, _, err = writeSegments(&Params{}, dir)
	assert.Equal(ErrMissingSource, err)

	// successful cases
	p := &Params{
		Source: &stream.HLSSegment{
			Name: bucketPath + "/a/b/c",
			Data: []byte("SourceData"),
		},
		Renditions: [][]byte{
			[]byte("Rendition1"),
			[]byte("Rendition2"),
		},
		URIs:     []string{bucketPath + "/r1/s", bucketPath + "r2/s"},
		Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9},
	}

	// Should write to disk since we haven't set an external bucket
	srcPath, rPaths, err := writeSegments(p, dir)
	assert.Nil(err)
	checkContents := func(fname string, generatedName string, contents []byte) {
		assert.Equal("/stream/"+baseDir+"/"+fname, generatedName)
		data, err := ioutil.ReadFile(filepath.Join(dir, fname))
		assert.Nil(err)
		assert.Equal(contents, data)
	}
	checkContents("source", srcPath, p.Source.Data)
	checkContents("P144p30fps16x9", rPaths[0], p.Renditions[0])
	checkContents("P240p30fps16x9", rPaths[1], p.Renditions[1])

	// TODO Trigger an error writing to disk, for both source and renditions

	// Set an external OS
	storageURI := "s3://K:S@eu-central-1/livepeer"
	os, err := drivers.ParseOSURL(storageURI, false)
	p.OS = os.NewSession("path")
	assert.Nil(err)
	drivers.NodeStorage = os
	srcPath, rPaths, err = writeSegments(p, dir)
	assert.Nil(err)
	assert.Equal(p.Source.Name, srcPath)
	assert.Equal(p.URIs[0], rPaths[0])
	assert.Equal(p.URIs[1], rPaths[1])

	// Zero out renditions; sanity check things are still OK
	p.URIs = nil
	srcPath, rPaths, err = writeSegments(p, dir)
	assert.Nil(err)
	assert.Equal(p.Source.Name, srcPath)
	assert.Empty(rPaths)
}

func TestEpic_Verify(t *testing.T) {
	assert := assert.New(t)

	// Use external S3 bucket
	storageURI := "s3://K:S@eu-central-1/livepeer"
	os, err := drivers.ParseOSURL(storageURI, false)
	assert.Nil(err)

	ts, mux := stubVerificationServer()
	defer ts.Close()
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		buf, err := json.Marshal(&epicResults{
			Results: []epicResultFields{
				{VideoAvailable: true, Tamper: -1.0},
			},
		})
		assert.Nil(err)
		w.Write(buf)
	})

	ec := &EpicClassifier{Addr: ts.URL + "/verify"}
	// There's a lot room to segfault here if `Params` isn't correctly populated
	params := &Params{
		Source:       &stream.HLSSegment{SeqNo: 73},
		Results:      &net.TranscodeData{},
		Orchestrator: &net.OrchestratorInfo{Transcoder: "pretend"},
		OS:           os.NewSession("path"),
	}
	_, err = ec.Verify(params)
	assert.Equal(ErrTampered, err)

	// Check invalid URLs
	ec.Addr = ""
	_, err = ec.Verify(params)
	assert.NotNil(err)
	assert.Contains(err.Error(), "unsupported protocol scheme")

	ec.Addr = ts.URL + "/nonexistent" // 404s
	_, err = ec.Verify(params)
	assert.NotNil(err)
	assert.Equal(ErrVerifierStatus, err)

	// TODO Error out on `resp.Body` read and ensure the error is there?

	// Nil JSON body
	mux.HandleFunc("/nilJSON", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	ec.Addr = ts.URL + "/nilJSON"
	_, err = ec.Verify(params)
	assert.NotNil(err)
	assert.IsType(&json.SyntaxError{}, err)

	// check various request parameters
	bucketPath := "https://livepeer.s3.amazonaws.com"
	params.Source.Name = bucketPath + "/source"
	params.Results = &net.TranscodeData{Segments: []*net.TranscodedSegmentData{{}, {}}}
	params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P720p60fps16x9}
	params.URIs = []string{bucketPath + "/r1", bucketPath + "/r2"}
	mux.HandleFunc("/checkReq", func(w http.ResponseWriter, r *http.Request) {
		var req epicRequest
		defer r.Body.Close()
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.Nil(err)
		// Build up our expected value struct and do a single comparison
		expected := epicRequest{
			Source:         params.Source.Name,
			OrchestratorID: params.Orchestrator.Transcoder,
			Renditions: []epicRendition{
				{URI: params.URIs[0], Framerate: 30, Resolution: epicResolution{Width: 426, Height: 240}},
				{URI: params.URIs[1], Framerate: 60, Resolution: epicResolution{Width: 1280, Height: 720}},
			},
			Model: "https://storage.googleapis.com/verification-models/verification.tar.xz",
		}
		assert.Equal(expected, req)

		buf, err := json.Marshal(&epicResults{
			Results: []epicResultFields{
				{VideoAvailable: true, Tamper: 1.0, Pixels: 123},
				{VideoAvailable: true, Tamper: 2.0, Pixels: 456},
			},
		})
		assert.Nil(err)
		w.Write(buf)
	})
	ec.Addr = ts.URL + "/checkReq"
	res, err := ec.Verify(params)
	assert.Nil(err)
	assert.Equal([]int64{123, 456}, res.Pixels)
	assert.Equal((1.0+2.0)/2.0, res.Score)
}

func stubVerificationServer() (*httptest.Server, *http.ServeMux) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	return ts, mux
}
