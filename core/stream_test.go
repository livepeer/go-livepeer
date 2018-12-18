package core

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/lpms/ffmpeg"
)

func TestSegmentFlatten(t *testing.T) {
	md := SegTranscodingMetadata{
		ManifestID: ManifestID("abcdef"),
		Seq:        1234,
		Hash:       ethcommon.BytesToHash(ethcommon.RightPadBytes([]byte("browns"), 32)),
		Profiles:   []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9},
	}
	sHash := ethcommon.FromHex("e97461de03dcb5bf7f2e95c4ca9c99db2d049fb18a6df67dd9d557b2c05f6473")
	if !bytes.Equal(ethcrypto.Keccak256(md.Flatten()), sHash) {
		t.Error("Flattened segment + hash did not match expected hash")
	}
}

func TestStreamID(t *testing.T) {
	rand.Seed(123)
	mid := RandomManifestID()
	profile := ffmpeg.P144p30fps16x9

	// Test random manifest ID generation
	if string(mid) != "17b336b6" {
		t.Error("Unexxpected ManifestID ", mid)
	}

	// Test normal cases
	id := MakeStreamID(mid, &profile)
	if id.ManifestID != mid || id.Rendition != profile.Name {
		t.Error("Unexpected StreamID ", id)
	}

	id = MakeStreamIDFromString(string(mid), profile.Name)
	if id.ManifestID != mid || id.Rendition != profile.Name {
		t.Error("Unexpected StreamID ", id)
	}

	id = SplitStreamIDString(fmt.Sprintf("%v/%v", mid, profile.Name))
	if id.ManifestID != mid || id.Rendition != profile.Name {
		t.Error("Unexpected StreamID ", id)
	}

	// Test missing ManifestID
	id = SplitStreamIDString("/" + profile.Name)
	if id.ManifestID != ManifestID("") || id.Rendition != profile.Name {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString("//" + profile.Name)
	if id.ManifestID != ManifestID("") || id.Rendition != "/"+profile.Name {
		t.Error("Unexpected StreamID ", id)
	}

	// Test missing rendition
	id = SplitStreamIDString(string(mid))
	if id.ManifestID != mid || id.Rendition != "" {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString(string(mid) + "/")
	if id.ManifestID != mid || id.Rendition != "" {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString(string(mid) + "//")
	if id.ManifestID != mid || id.Rendition != "/" {
		t.Error("Unexpected StreamID ", id)
	}

	// Test missing ManifestID and rendition
	id = SplitStreamIDString("")
	if id.ManifestID != ManifestID("") || id.Rendition != "" {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString("/")
	if id.ManifestID != ManifestID("") || id.Rendition != "" {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString("//")
	if id.ManifestID != ManifestID("") || id.Rendition != "/" {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString("///")
	if id.ManifestID != ManifestID("") || id.Rendition != "//" {
		t.Error("Unexpected StreamID ", id)
	}

	// Test some unusual cases -- these should all technically be acceptable
	// (Acceptable until we decide they're not)
	id = SplitStreamIDString("../../..")
	if id.ManifestID != ".." || id.Rendition != "../.." {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString("  /  /  ")
	if id.ManifestID != "  " || id.Rendition != "  /  " {
		t.Error("Unexpected StreamID ", id)
	}

	// Test stringification
	id = MakeStreamID(mid, &profile)
	if id.String() != fmt.Sprintf("%v/%v", mid, profile.Name) {
		t.Error("Unexpected StreamID ", id)
	}
	id = SplitStreamIDString("")
	if id.String() != "/" {
		t.Error("Unexpected StreamID ", id)
	}
}
