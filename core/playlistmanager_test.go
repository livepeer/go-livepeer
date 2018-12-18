package core

import (
	"bytes"
	"strings"
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
	err := c.InsertHLSSegment(hlsStrmID, 1, segName, 12)
	if err != nil {
		t.Fatal(err)
	}
	pl := m3u8.NewMasterPlaylist()
	pl.Append(hlsStrmID.String()+".m3u8", nil, ffmpeg.VideoProfileToVariantParams(vProfile))
	testpl := c.GetHLSMasterPlaylist()

	if testpl.String() != pl.String() {
		t.Errorf("Expecting %v, got %v", pl.String(), testpl.String())
	}

	mpl := c.GetHLSMediaPlaylist(hlsStrmID)
	if mpl == nil {
		t.Fatalf("Expecting pl, got nil")
	}
	s := mpl.Segments[0]
	if s.URI != segName {
		t.Errorf("Expecting %s, got %s", segName, s.URI)
	}
}

func TestForWrongStream(t *testing.T) {
	vProfile := ffmpeg.P144p30fps16x9
	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
	mid := hlsStrmID.ManifestID
	c := NewBasicPlaylistManager(mid, nil)
	hlsStrmID2 := MakeStreamID(RandomManifestID(), &vProfile)
	err := c.InsertHLSSegment(hlsStrmID2, 1, "test_uri", 12)
	if err == nil {
		t.Fatalf("Should fail here")
	}
	if !strings.Contains(err.Error(), "Wrong manifest id") {
		t.Fatalf("Wrong error, should contain 'Wrong stream id', but has %s", err.Error())
	}
}

func TestCleanup(t *testing.T) {
	vProfile := ffmpeg.P144p30fps16x9
	hlsStrmID := MakeStreamID(RandomManifestID(), &vProfile)
	mid := hlsStrmID.ManifestID
	osd := drivers.NewMemoryDriver("test://some.host")
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
