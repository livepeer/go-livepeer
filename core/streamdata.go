package core

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"

	"github.com/livepeer/lpms/ffmpeg"
)

var ErrManifestID = errors.New("ErrManifestID")

const (
	DefaultManifestIDLength = 4
)

type SegTranscodingMetadata struct {
	ManifestID ManifestID
	Seq        int64
	Hash       ethcommon.Hash
	Profiles   []ffmpeg.VideoProfile
	OS         *net.OSInfo
	Duration   int
}

func (md *SegTranscodingMetadata) Flatten() []byte {
	profiles := common.ProfilesToHex(md.Profiles)
	seq := big.NewInt(md.Seq).Bytes()
	buf := make([]byte, len(md.ManifestID)+32+len(md.Hash.Bytes())+len(profiles))
	i := copy(buf[0:], []byte(md.ManifestID))
	i += copy(buf[i:], ethcommon.LeftPadBytes(seq, 32))
	i += copy(buf[i:], md.Hash.Bytes())
	i += copy(buf[i:], []byte(profiles))
	// i += copy(buf[i:], []byte(s.OS))
	return buf
}

type ManifestID string

// The StreamID represents a particular variant of a stream.
type StreamID struct {
	// Base playback ID that related renditions are grouped under
	ManifestID ManifestID

	// Specifies the stream variant: the HLS source, transcoding profile, etc.
	// Also used for RTMP: when unguessable,this can function as a stream key.
	Rendition string
}

func MakeStreamIDFromString(mid string, rendition string) StreamID {
	return StreamID{
		ManifestID: ManifestID(mid),
		Rendition:  rendition,
	}
}

func MakeStreamID(mid ManifestID, profile *ffmpeg.VideoProfile) StreamID {
	return MakeStreamIDFromString(string(mid), profile.Name)
}

func SplitStreamIDString(str string) StreamID {
	parts := strings.SplitN(str, "/", 2)
	if len(parts) <= 0 {
		return MakeStreamIDFromString("", "")
	}
	if len(parts) == 1 {
		return MakeStreamIDFromString(parts[0], "")
	}
	return MakeStreamIDFromString(parts[0], parts[1])
}

func (id StreamID) String() string {
	return fmt.Sprintf("%v/%v", id.ManifestID, id.Rendition)
}

func RandomManifestID() ManifestID {
	return ManifestID(common.RandomIDGenerator(DefaultManifestIDLength))
}
