package core

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"testing"

	"github.com/livepeer/go-livepeer/drivers"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/m3u8"
)

func readJSONPlaylist(t *testing.T, fileName string) *JsonPlaylist {
	fd, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	jspl := &JsonPlaylist{}

	err = json.Unmarshal(fd, jspl)
	if err != nil {
		t.Error(err)
	}
	return jspl
}

func TestJSONListJoinTrack(t *testing.T) {
	mainJspl := NewJSONPlaylist()
	jspl := readJSONPlaylist(t, "playlist_1595101950232701011.json")
	mainJspl.AddMaster(jspl)
	mainJspl.AddTrack(jspl, "source")
	jspl = readJSONPlaylist(t, "playlist_1595102011582863985.json")
	mainJspl.AddMaster(jspl)
	mainJspl.AddTrack(jspl, "source")
	var lastSeq uint64
	for _, seg := range mainJspl.Segments["source"] {
		if seg.SeqNo < lastSeq {
			t.Errorf("got seq %d but last %d", seg.SeqNo, lastSeq)
		}
		lastSeq = seg.SeqNo
	}
}

func TestGetMasterPlaylist(t *testing.T) {
	vProfile := ffmpeg.P144p30fps16x9
	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
	mid := hlsStrmID.ManifestID
	c := NewBasicPlaylistManager(mid, nil, nil)
	segName := "test_seg/1.ts"
	err := c.InsertHLSSegment(&vProfile, 1, segName, 12)
	if err != nil {
		t.Fatal(err)
	}
	pl := m3u8.NewMasterPlaylist()
	pl.Append(hlsStrmID.String()+".m3u8", nil, ffmpeg.VideoProfileToVariantParams(vProfile))
	testpl := c.GetHLSMasterPlaylist()

	if testpl.String() != pl.String() {
		t.Errorf("Expecting %v, got %v", pl.String(), testpl.String())
	}

	mpl := c.GetHLSMediaPlaylist(vProfile.Name)
	if mpl == nil {
		t.Fatalf("Expecting pl, got nil")
	}
	s := mpl.Segments[0]
	if s.URI != segName {
		t.Errorf("Expecting %s, got %s", segName, s.URI)
	}
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
	uri, err := c.GetOSSession().SaveData("testName", testData, nil)
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
