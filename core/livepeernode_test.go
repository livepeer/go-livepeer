package core

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"net/url"
	"os"
	"testing"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StubTranscoder struct {
	Profiles      []ffmpeg.VideoProfile
	SegCount      int
	StoppedCount  int
	FailTranscode bool
	TranscodeFn   func() error
}

func newStubTranscoder(d string) TranscoderSession {
	return &StubTranscoder{}
}

func stubTranscoderWithProfiles(profiles []ffmpeg.VideoProfile) *StubTranscoder {
	return &StubTranscoder{Profiles: profiles}
}

func (t *StubTranscoder) Transcode(md *SegTranscodingMetadata) (*TranscodeData, error) {
	if t.FailTranscode {
		return nil, ErrTranscode
	}

	var err error
	if t.TranscodeFn != nil {
		err = t.TranscodeFn()
	}

	t.SegCount++

	segments := make([]*TranscodedSegmentData, 0)
	for _, p := range t.Profiles {
		segments = append(segments, &TranscodedSegmentData{Data: []byte(fmt.Sprintf("Transcoded_%v", p.Name))})
	}

	return &TranscodeData{Segments: segments}, err
}

func (t *StubTranscoder) Stop() {
	t.StoppedCount++
}

func TestTranscodeAndBroadcast(t *testing.T) {
	ffmpeg.InitFFmpeg()
	p := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}
	tr := stubTranscoderWithProfiles(p)
	storage := drivers.NewMemoryDriver(nil).NewSession("")
	config := transcodeConfig{LocalOS: storage, OS: storage}

	tmpdir, _ := ioutil.TempDir("", "")
	n, err := NewLivepeerNode(nil, tmpdir, nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	n.Transcoder = tr

	md := &SegTranscodingMetadata{Profiles: p}
	ss := StubSegment()
	res := n.transcodeSeg(config, ss, md)
	if res.Err != nil {
		t.Errorf("Error: %v", res.Err)
	}

	if len(res.TranscodeData.Segments) != len(p) {
		t.Errorf("Expecting %v profiles, got %v", len(p), len(res.TranscodeData.Segments))
	}

	//Should have 1 transcoded segment into 2 different profiles
	if tr.SegCount != 1 {
		t.Errorf("Expecting 1 segment to be transcoded, got %v", tr.SegCount)
	}

	// Check playlist was updated
	// XXX Fix this
	/*for _, v := range strmIds {
		pl := n.VideoSource.GetHLSMediaPlaylist(v)
		if pl == nil {
			t.Error("Expected media playlist; got none")
		}
		if len(pl.Segments) != 1 && pl.SeqNo != 100 {
			t.Error("Mismatched segments (expected 1) or seq (expected 100), got ", pl.Segments, pl.SeqNo)
		}
	}*/

	// TODO check sig?

	// Test when transcoder fails
	tr.FailTranscode = true
	res = n.transcodeSeg(config, ss, md)
	if res.Err == nil {
		t.Error("Expecting a transcode error")
	}
	tr.FailTranscode = false

	// Test when the number of results mismatchches expectations
	tr.Profiles = []ffmpeg.VideoProfile{p[0]}
	res = n.transcodeSeg(config, ss, md)
	if res.Err == nil || res.Err.Error() != "MismatchedSegments" {
		t.Error("Did not get mismatched segments as expected")
	}
	tr.Profiles = p
}

func TestServiceURIChange(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	n, err := NewLivepeerNode(nil, "", nil)
	require.Nil(err)
	assert.Equal("", n.GetServiceURI().String()) // sanity

	// Check default
	origUrl, err := url.Parse("orig://url")
	assert.Nil(err)
	n.serviceURI = *origUrl
	assert.Equal(origUrl, n.GetServiceURI())

	// Set ServiceURI to a URL
	sUrl, err := url.Parse("test://testurl.com")
	require.Nil(err)
	n.SetServiceURI(sUrl)
	assert.Equal(sUrl, n.GetServiceURI())

	// Set ServiceURI to another URL
	newUrl, err := url.Parse("test://newurl.com")
	require.Nil(err)
	n.SetServiceURI(newUrl)
	assert.Equal(newUrl, n.GetServiceURI())
}

func TestSetAndGetBasePrice(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	n, err := NewLivepeerNode(nil, "", nil)
	require.Nil(err)

	price := big.NewRat(1, 1)

	n.SetBasePrice(price)
	assert.Zero(n.priceInfo.Cmp(price))
	assert.Zero(n.GetBasePrice().Cmp(price))
}
