package core

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/url"
	"testing"

	"github.com/golang/glog"
	"github.com/livepeer/go-tools/drivers"
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

func (t *StubTranscoder) EndTranscodingSession(sessionId string) {

}

func (t *StubTranscoder) Transcode(ctx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error) {
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

	n, err := NewLivepeerNode(nil, t.TempDir(), nil)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	n.Transcoder = tr

	md := &SegTranscodingMetadata{Profiles: p, AuthToken: stubAuthToken()}
	ss := StubSegment()
	res := n.transcodeSeg(context.TODO(), config, ss, md)
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
	res = n.transcodeSeg(context.TODO(), config, ss, md)
	if res.Err == nil {
		t.Error("Expecting a transcode error")
	}
	tr.FailTranscode = false

	// Test when the number of results mismatchches expectations
	tr.Profiles = []ffmpeg.VideoProfile{p[0]}
	res = n.transcodeSeg(context.TODO(), config, ss, md)
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

	sUrl, err := url.Parse("test://testurl.com")
	require.Nil(err)
	n.SetServiceURI(sUrl)

	drivers.NodeStorage = drivers.NewMemoryDriver(n.GetServiceURI())
	sesh := drivers.NodeStorage.NewSession("testpath")
	savedUrl, err := sesh.SaveData(context.TODO(), "testdata1", bytes.NewReader([]byte{0, 0, 0}), nil, 0)
	require.Nil(err)
	assert.Equal("test://testurl.com/stream/testpath/testdata1", savedUrl)

	glog.Infof("Setting service URL to newurl")
	newUrl, err := url.Parse("test://newurl.com")
	n.SetServiceURI(newUrl)
	require.Nil(err)
	furl, err := sesh.SaveData(context.TODO(), "testdata2", bytes.NewReader([]byte{0, 0, 0}), nil, 0)
	require.Nil(err)
	assert.Equal("test://newurl.com/stream/testpath/testdata2", furl)

	glog.Infof("Setting service URL to secondurl")
	secondUrl, err := url.Parse("test://secondurl.com")
	n.SetServiceURI(secondUrl)
	require.Nil(err)
	surl, err := sesh.SaveData(context.TODO(), "testdata3", bytes.NewReader([]byte{0, 0, 0}), nil, 0)
	require.Nil(err)
	assert.Equal("test://secondurl.com/stream/testpath/testdata3", surl)
}

func TestSetAndGetBasePrice(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	n, err := NewLivepeerNode(nil, "", nil)
	require.Nil(err)

	price := big.NewRat(1, 1)

	n.SetBasePrice("default", NewFixedPrice(price))
	assert.Zero(n.priceInfo["default"].Value().Cmp(price))
	assert.Zero(n.GetBasePrice("default").Cmp(price))
	assert.Zero(n.GetBasePrices()["default"].Cmp(price))

	addr1 := "0x0000000000000000000000000000000000000000"
	addr2 := "0x1000000000000000000000000000000000000000"
	price1 := big.NewRat(2, 1)
	price2 := big.NewRat(3, 1)

	n.SetBasePrice(addr1, NewFixedPrice(price1))
	n.SetBasePrice(addr2, NewFixedPrice(price2))
	assert.Zero(n.priceInfo[addr1].Value().Cmp(price1))
	assert.Zero(n.priceInfo[addr2].Value().Cmp(price2))
	assert.Zero(n.GetBasePrices()[addr1].Cmp(price1))
	assert.Zero(n.GetBasePrices()[addr2].Cmp(price2))
}
