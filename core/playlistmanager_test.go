package core

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/ericxtang/m3u8"
	"github.com/livepeer/go-livepeer/drivers"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

func TestGetMasterPlaylist(t *testing.T) {
	vProfile := ffmpeg.P144p30fps16x9
	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
	mid := hlsStrmID.ManifestID
	c := NewBasicPlaylistManager(mid, nil)
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

	c := NewBasicPlaylistManager(RandomManifestID(), nil)
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

	c := NewBasicPlaylistManager(RandomManifestID(), nil)
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

	// insert a "duplicate" seqno. Should not work but does. Fix.
	if err := c.InsertHLSSegment(vProfile, seg.SeqId, seg.URI, seg.Duration); err != nil {
		t.Error("HLS insertion")
	}
	if len(pl.Segments) != int(LIVE_LIST_LENGTH) || !compareSeg(seg, pl.Segments[0]) || !compareSeg(pl.Segments[0], pl.Segments[1]) {
		t.Error("Unexpected playlist/segment properties")
	}

	// Insert out of order. Playlist should accommodate this. Fix.
	seg = &m3u8.MediaSegment{SeqId: 3, URI: "abc", Duration: -11.1}
	if err := c.InsertHLSSegment(vProfile, seg.SeqId, seg.URI, seg.Duration); err != nil {
		t.Error("HLS insertion")
	}
	if !compareSeg(seg, pl.Segments[2]) {
		t.Error("Unexpected seg properties")
	}

	// Ensure we have different segments between two playlists
	newSeg := &m3u8.MediaSegment{SeqId: 3, URI: "abc", Duration: -11.1}
	newProfile := &ffmpeg.P240p30fps16x9
	if err := c.InsertHLSSegment(newProfile, newSeg.SeqId, newSeg.URI, newSeg.Duration); err != nil {
		t.Error("HLS insertion")
	}
	newPL := c.GetHLSMediaPlaylist(newProfile.Name)
	if !compareSeg(seg, newPL.Segments[0]) || compareSeg(pl.Segments[0], newPL.Segments[0]) {
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

	c := NewBasicPlaylistManager(mid, osSession)
	uri, err := c.GetOSSession().SaveData("testName", testData)
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
