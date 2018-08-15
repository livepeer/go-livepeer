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

	bad := StreamID("streamid")
	if bad.IsValid() {
		t.Error("Did not expect streamid to be valid")
	}
}

func TestManifestID(t *testing.T) {
	vid := RandomVideoID()
	mid, _ := MakeManifestID(vid)

	if bytes.Compare(mid.GetVideoID(), vid) != 0 {
		t.Error("Manifest ID did not match video ID")
	}

	sid, _ := MakeStreamID(vid, ffmpeg.P144p30fps16x9.Name)
	msid := sid.ManifestIDFromStreamID()
	if mid.String() != msid.String() {
		t.Error("Manifest was not properly derived from stream ID")
	}

	if bytes.Compare(vid, msid.GetVideoID()) != 0 {
		t.Error("Derived manifest did not match video ID")
	}
}
