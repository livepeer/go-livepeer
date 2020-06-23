package core

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
)

func TestStreamParameters(t *testing.T) {
	assert := assert.New(t)

	// empty params
	s := StreamParameters{}
	assert.Equal("/", s.StreamID())

	// key only
	s = StreamParameters{RtmpKey: "source"}
	assert.Equal("/source", s.StreamID())

	// mid only
	mid := ManifestID("abc")
	s = StreamParameters{ManifestID: mid}
	assert.Equal("abc/", s.StreamID())

	// both mid + key
	s = StreamParameters{ManifestID: mid, RtmpKey: "source"}
	assert.Equal("abc/source", s.StreamID())
}

func TestSegmentFlatten(t *testing.T) {
	md := SegTranscodingMetadata{
		ManifestID: ManifestID("abcdef"),
		Seq:        1234,
		Hash:       ethcommon.BytesToHash(ethcommon.RightPadBytes([]byte("browns"), 32)),
		Profiles:   []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9},
	}
	sHash := ethcommon.FromHex("08a398e7e7a5a545b0b07d9e7e1eab52b4131e0df1621bd53ee6e7550a15465b")
	if !bytes.Equal(ethcrypto.Keccak256(md.Flatten()), sHash) {
		t.Error("Flattened segment + hash did not match expected hash")
	}
}

func TestRandomIdGenerator(t *testing.T) {
	rand.Seed(123)
	res := common.RandomIDGenerator(DefaultManifestIDLength)
	if res != "17b336b6" {
		t.Error("Unexpected RNG result")
	}
}

func TestStreamID(t *testing.T) {
	rand.Seed(123)
	mid := RandomManifestID()
	profile := ffmpeg.P144p30fps16x9

	// Test random manifest ID generation
	if string(mid) != "17b336b6" {
		t.Error("Unexpected ManifestID ", mid)
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
