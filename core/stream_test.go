package core

import (
	"testing"

	ffmpeg "github.com/livepeer/lpms/ffmpeg"

	"bytes"
)

func TestStreamID(t *testing.T) {
	vid := RandomVideoID()
	id, err := MakeStreamID(vid, ffmpeg.P144p30fps16x9.Name)
	if err != nil {
		t.Error("Error making StreamID ", err)
	}

	if bytes.Compare(vid, id.GetVideoID()) != 0 {
		t.Errorf("Expecting: %v, got %v", vid, id.GetVideoID())
	}

	if ffmpeg.P144p30fps16x9.Name != id.GetRendition() {
		t.Error("Rendition not matching")
	}
	if !id.IsValid() {
		t.Error("Streamid not valid")
	}

	// invalid videoid
	if _, err := MakeStreamID([]byte("abc"), "def"); err != ErrStreamID {
		t.Error("Did not receive expected error; ", err)
	}
	// invalid rendition
	if _, err := MakeStreamID(vid, ""); err != ErrStreamID {
		t.Error("Did not receive expected streamid error ", err)
	}
	// force a too-short streamid
	bad := StreamID("streamid")
	if bad.GetVideoID() != nil {
		t.Error("Expected a nil videoid from streamid")
	}
	if bad.IsValid() {
		t.Error("Did not expect streamid to be valid")
	}
}

func TestManifestID(t *testing.T) {
	vid := RandomVideoID()
	mid, err := MakeManifestID(vid)

	if err != nil || !mid.IsValid() {
		t.Error("Error or invalid manifestid ", err)
	}

	if bytes.Compare(mid.GetVideoID(), vid) != 0 {
		t.Error("Manifest ID did not match video ID")
	}

	sid, _ := MakeStreamID(vid, ffmpeg.P144p30fps16x9.Name)
	msid, err := sid.ManifestIDFromStreamID()
	if mid.String() != msid.String() || err != nil {
		t.Error("Manifest was not properly derived from stream ID ", err)
	}

	if bytes.Compare(vid, msid.GetVideoID()) != 0 {
		t.Error("Derived manifest did not match video ID")
	}

	//invalid manifestid
	if _, err := MakeManifestID([]byte("abc")); err != ErrManifestID {
		t.Error("Did not receive expected manifestid error ", err)
	}
	bad := ManifestID("manifestid")
	if bad.GetVideoID() != nil {
		t.Error("Expected nil videoid from manifestid")
	}
	if _, err := StreamID("bad").ManifestIDFromStreamID(); err != ErrManifestID {
		t.Error("Expected manifestid error from bad streamid ", err)
	}
	if bad.IsValid() {
		t.Error("did not expect manifestd to be valid")
	}
}
