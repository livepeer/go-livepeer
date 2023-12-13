package core

import (
	"bytes"
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-tools/drivers"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/m3u8"
	"github.com/stretchr/testify/assert"
)

func init() {
	JsonPlaylistQuitTimeout = 0 * time.Second
}

func TestJSONList1(t *testing.T) {
	assert := assert.New(t)
	jspl1 := NewJSONPlaylist()
	vProfile := ffmpeg.P144p30fps16x9
	vProfile.Name = "source"
	jspl1.InsertHLSSegment(&vProfile, 1, "manifestID/test_seg/1.ts", 2.1)
	jspl1.InsertHLSSegment(&vProfile, 3, "manifestID/test_seg/3.ts", 2.5)
	jspl1.InsertHLSSegment(&vProfile, 4, "manifestID/test_seg/4.ts", 2.5)
	assert.Len(jspl1.Segments, 1)
	assert.Len(jspl1.Segments["source"], 3)
	assert.Equal(uint64(2100+2500+2500), jspl1.DurationMs)

	jspl2 := NewJSONPlaylist()
	jspl2.InsertHLSSegment(&vProfile, 2, "manifestID/test_seg/2.ts", 2)
	jspl2.InsertHLSSegment(&vProfile, 4, "manifestID/test_seg/4.ts", 2)
	assert.Len(jspl2.Segments, 1)
	assert.Len(jspl2.Tracks, 1)
	assert.Len(jspl2.Segments["source"], 2)
	assert.Equal(uint64(4000), jspl2.DurationMs)

	vProfile = ffmpeg.P144p30fps16x9
	vProfile.Name = "trans1"
	jspl3 := NewJSONPlaylist()
	jspl3.InsertHLSSegment(&vProfile, 1, "manifestID/test_seg/1.ts", 2)
	jspl3.InsertHLSSegment(&vProfile, 4, "manifestID/test_seg/4.ts", 2)
	assert.Len(jspl3.Segments, 1)
	assert.Len(jspl3.Tracks, 1)
	assert.Len(jspl3.Segments["trans1"], 2)
	assert.Equal(uint64(0), jspl3.DurationMs)

	mjspl := NewJSONPlaylist()
	mjspl.AddMaster(jspl1)
	mjspl.AddMaster(jspl2)
	mjspl.AddMaster(jspl3)
	mjspl.AddTrack(jspl1, "source")
	mjspl.AddTrack(jspl2, "source")
	mjspl.AddTrack(jspl3, "trans1")
	assert.Len(mjspl.Tracks, 2)
	assert.Len(mjspl.Segments, 2)
	assert.Len(mjspl.Segments["source"], 4)
	assert.Len(mjspl.Segments["trans1"], 2)
	assert.Equal(uint64(2), mjspl.Segments["source"][1].SeqNo)
	assert.Equal("manifestID/test_seg/2.ts", mjspl.Segments["source"][1].URI)
	assert.Equal(uint64(2000), mjspl.Segments["source"][1].DurationMs)

	var lastSeq uint64
	for _, seg := range mjspl.Segments["source"] {
		if seg.SeqNo < lastSeq {
			t.Errorf("got seq %d but last %d", seg.SeqNo, lastSeq)
		}
		lastSeq = seg.SeqNo
	}
	segments := mjspl.Segments["source"]
	mpl, err := m3u8.NewMediaPlaylist(uint(len(segments)), uint(len(segments)))
	assert.Nil(err)
	assert.NotNil(mpl)
	mpl.Live = false
	mjspl.AddSegmentsToMPL([]string{"manifestID"}, "source", mpl, "")
	mpls := string(mpl.Encode().Bytes())
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-TARGETDURATION:3\n#EXTINF:2.100,\ntest_seg/1.ts\n#EXTINF:2.000,\ntest_seg/2.ts\n#EXTINF:2.500,\ntest_seg/3.ts\n#EXTINF:2.500,\ntest_seg/4.ts\n#EXT-X-ENDLIST\n", mpls)
}

func TestJSONListJoin(t *testing.T) {
	assert := assert.New(t)
	jspl1 := NewJSONPlaylist()
	vProfile := ffmpeg.P144p30fps16x9
	vProfile.Name = "source"
	jspl1.InsertHLSSegment(&vProfile, 1, "manifestID/test_seg/1.ts", 2.1)
	jspl1.InsertHLSSegment(&vProfile, 3, "manifestID/test_seg/3.ts", 2.5)
	jspl1.InsertHLSSegment(&vProfile, 4, "manifestID/test_seg/4.ts", 2.5)
	assert.Len(jspl1.Segments, 1)
	assert.Len(jspl1.Segments["source"], 3)
	assert.Equal(uint64(2100+2500+2500), jspl1.DurationMs)

	jspl2 := NewJSONPlaylist()
	jspl2.InsertHLSSegment(&vProfile, 2, "manifestID2/test_seg/2.ts", 2)
	jspl2.InsertHLSSegment(&vProfile, 4, "manifestID2/test_seg/4.ts", 2)
	assert.Len(jspl2.Segments, 1)
	assert.Len(jspl2.Tracks, 1)
	assert.Len(jspl2.Segments["source"], 2)
	assert.Equal(uint64(4000), jspl2.DurationMs)
	jspl1.AddDiscontinuedTrack(jspl2, "source")
	assert.Len(jspl1.Segments["source"], 5)
	assert.True(jspl1.Segments["source"][3].discontinuity)
}

//func TestJsonFlush(t *testing.T) {
//	assert := assert.New(t)
//	os := drivers.NewMemoryDriver(nil)
//	msess := os.NewSession("sess1")
//	vProfile := ffmpeg.P144p30fps16x9
//	vProfile.Name = "source"
//	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
//	mid := hlsStrmID.ManifestID
//	c := NewBasicPlaylistManager(mid, nil, msess)
//	assert.Equal(msess, c.GetRecordOSSession())
//	segName := "test_seg/1.ts"
//	c.InsertHLSSegmentJSON(&vProfile, 1, segName, 12*60*60)
//	assert.NotNil(c.jsonList)
//	assert.True(c.jsonList.hasTrack(vProfile.Name))
//	assert.Len(c.jsonList.Segments, 1)
//	assert.Len(c.jsonList.Segments[vProfile.Name], 1)
//	plName := c.jsonList.name
//	c.FlushRecord()
//	assert.False(c.jsonList.hasTrack(vProfile.Name))
//	assert.Len(c.jsonList.Segments, 0)
//	assert.Len(c.jsonList.Segments[vProfile.Name], 0)
//	time.Sleep(50 * time.Millisecond)
//	fir, err := msess.ReadData(context.Background(), "sess1/"+plName)
//	assert.Nil(err)
//	assert.NotNil(fir)
//	data, err := ioutil.ReadAll(fir.Body)
//	assert.Nil(err)
//	assert.Equal(`{"duration_ms":43200000,"tracks":[{"name":"source","bandwidth":400000,"resolution":"256x144"}],"segments":{"source":[{"seq_no":1,"uri":"test_seg/1.ts","duration_ms":43200000}]}}`, string(data))
//	c.Cleanup()
//}

func TestGetMasterPlaylist(t *testing.T) {
	assert := assert.New(t)
	vProfile := ffmpeg.P144p30fps16x9
	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
	mid := hlsStrmID.ManifestID
	c := NewBasicPlaylistManager(mid, nil, nil)
	segName := "test_seg/1.ts"
	err := c.InsertHLSSegment(&vProfile, 1, segName, 12)
	assert.Nil(err)
	pl := m3u8.NewMasterPlaylist()
	pl.Append(hlsStrmID.String()+".m3u8", nil, ffmpeg.VideoProfileToVariantParams(vProfile))
	testpl := c.GetHLSMasterPlaylist()

	if testpl.String() != pl.String() {
		t.Errorf("Expecting %v, got %v", pl.String(), testpl.String())
	}

	mpl := c.GetHLSMediaPlaylist(vProfile.Name)
	assert.NotNil(mpl)
	s := mpl.Segments[0]
	assert.Equal(segName, s.URI)
	c.Cleanup()
}

func TestGetOrCreatePL(t *testing.T) {

	c := NewBasicPlaylistManager(RandomManifestID(), nil, nil)
	vProfile := &ffmpeg.P144p30fps16x9

	// Sanity check some properties of an empty master playlist
	masterPL := c.GetHLSMasterPlaylist()
	if len(masterPL.Variants) != 0 {
		t.Error("Master PL had some unexpected variants")
	}

	// insert one
	pl, err := c.getOrCreatePL(vProfile)
	if err != nil {
		t.Error("Unexpected error ", err)
	}

	// Sanity check some master PL properties
	expectedRes := vProfile.Resolution
	if len(masterPL.Variants) != 1 || masterPL.Variants[0].Resolution != expectedRes {
		t.Error("Master PL had some unexpected variants or properties")
	}

	// using the same profile name should return the original profile
	orig := pl
	vProfile = &ffmpeg.VideoProfile{Name: vProfile.Name}
	pl, err = c.getOrCreatePL(vProfile)
	if err != nil || orig != pl {
		t.Error("Mismatched profile or error ", err)
	}
	// Further sanity check some master PL properties
	if len(masterPL.Variants) != 1 || masterPL.Variants[0].Resolution != expectedRes {
		t.Error("Master PL had some unexpected variants or properties")
	}

	// using a different profile name should return a different profile
	vProfile = &ffmpeg.P240p30fps16x9
	pl, err = c.getOrCreatePL(vProfile)
	if err != nil || orig == pl {
		t.Error("Matched profile or error ", err)
	}
	// Further sanity check some master PL properties
	if len(masterPL.Variants) != 2 || masterPL.Variants[1].Resolution != vProfile.Resolution {
		t.Error("Master PL had some unexpected variants or properties")
	}
	c.Cleanup()
}

func TestPlaylists(t *testing.T) {

	c := NewBasicPlaylistManager(RandomManifestID(), nil, nil)
	vProfile := &ffmpeg.P144p30fps16x9

	// Check getting a nonexistent media PL
	if pl := c.GetHLSMediaPlaylist("nonexistent"); pl != nil {
		t.Error("Recevied a nonexistent playlist ", pl)
	}

	// Insert one segment
	compareSeg := func(a, b *m3u8.MediaSegment) bool {
		return a.SeqId == b.SeqId && a.URI == b.URI && a.Duration == b.Duration
	}
	seg := &m3u8.MediaSegment{SeqId: 9, URI: "abc", Duration: -11.1}
	if err := c.InsertHLSSegment(vProfile, seg.SeqId, seg.URI, seg.Duration); err != nil {
		t.Error("HLS insertion")
	}
	// Sanity check some PL properties
	pl := c.GetHLSMediaPlaylist(vProfile.Name)
	if pl == nil {
		t.Error("No playlist")
	}
	if len(pl.Segments) != int(LIVE_LIST_LENGTH) || !compareSeg(seg, pl.Segments[0]) || pl.Segments[1] != nil {
		t.Error("Unexpected playlist/segment properties")
	}

	if err := c.InsertHLSSegment(vProfile, seg.SeqId, seg.URI, seg.Duration); err == nil {
		t.Error("Unexpected HLS insertion: segment already exists and should not have been inserted")
	}

	// Sanity check to make sure repeated segment was not inserted into playlist
	if len(pl.Segments) != int(LIVE_LIST_LENGTH) || !compareSeg(seg, pl.Segments[0]) || pl.Segments[1] != nil {
		t.Error("Unexpected playlist/segment properties")
	}

	// Insert out of order. Playlist should accommodate this.
	seg1 := &m3u8.MediaSegment{SeqId: 3, URI: "def", Duration: -11.1}
	if err := c.InsertHLSSegment(vProfile, seg1.SeqId, seg1.URI, seg1.Duration); err != nil {
		t.Error("HLS insertion")
	}

	if !compareSeg(seg1, pl.Segments[0]) || !compareSeg(seg, pl.Segments[1]) {
		t.Error("Unexpected seg properties")
	}

	// Sanity Check. Insert out of order. Playlist should accommodate this.
	seg2 := &m3u8.MediaSegment{SeqId: 1, URI: "ghi", Duration: -11.1}
	if err := c.InsertHLSSegment(vProfile, seg2.SeqId, seg2.URI, seg2.Duration); err != nil {
		t.Error("HLS insertion")
	}

	if !compareSeg(seg2, pl.Segments[0]) || !compareSeg(seg, pl.Segments[2]) || !compareSeg(seg1, pl.Segments[1]) {
		t.Error("Unexpected seg properties")
	}

	// Ensure we have different segments between two playlists
	newSeg := &m3u8.MediaSegment{SeqId: 3, URI: "def", Duration: -11.1}
	newProfile := &ffmpeg.P240p30fps16x9
	if err := c.InsertHLSSegment(newProfile, newSeg.SeqId, newSeg.URI, newSeg.Duration); err != nil {
		t.Error("HLS insertion")
	}

	newPL := c.GetHLSMediaPlaylist(newProfile.Name)
	if !compareSeg(seg1, newPL.Segments[0]) || !compareSeg(pl.Segments[1], newPL.Segments[0]) {
		t.Error("Unexpected seg properties in new playlist")
	}
	c.Cleanup()

}

func TestCleanup(t *testing.T) {
	vProfile := ffmpeg.P144p30fps16x9
	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
	mid := hlsStrmID.ManifestID
	url, _ := url.ParseRequestURI("test://some.host")
	osd := drivers.NewMemoryDriver(url)
	osSession := osd.NewSession("testPath")
	memoryOS := osSession.(*drivers.MemorySession)
	testData := []byte{1, 2, 3, 4}

	c := NewBasicPlaylistManager(mid, osSession, nil)
	uri, err := c.GetOSSession().SaveData(context.TODO(), "testName", bytes.NewReader(testData), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	expectedURI := "test://some.host/stream/testPath/testName"
	if uri != expectedURI {
		t.Fatalf("Expected %s, got %s", expectedURI, uri)
	}
	data := memoryOS.GetData("testPath/testName")
	if bytes.Compare(testData, data) != 0 {
		t.Logf("should be: %x", testData)
		t.Logf("got      : %s", data)
		t.Fatal("Data should be equal")
	}
	c.Cleanup()
	data = memoryOS.GetData("testPath/testName")
	if data != nil {
		t.Fatal("Data should be cleaned up")
	}
}
