package core

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"

	"github.com/livepeer/lpms/ffmpeg"
)

var ErrManifestID = errors.New("ErrManifestID")

const (
	DefaultManifestIDLength = 4
)

type StreamParameters struct {
	ManifestID        ManifestID
	ExternalStreamID  string
	SessionID         string
	RtmpKey           string
	Profiles          []ffmpeg.VideoProfile
	Resolution        string
	Format            ffmpeg.Format
	OS                drivers.OSSession
	RecordOS          drivers.OSSession
	Capabilities      *Capabilities
	VerificationFreq  uint
	Nonce             uint64
	Codec             ffmpeg.VideoCodec
	PixelFormat       ffmpeg.PixelFormat
	TimeoutMultiplier int // Used in the VOD workflow to allow us to be more lenient with timeouts
}

func (s *StreamParameters) StreamID() string {
	return string(s.ManifestID) + "/" + s.RtmpKey
}

type SegmentClip struct {
	From time.Duration
	To   time.Duration
}

type SegmentParameters struct {
	Clip               *SegmentClip
	ForceSessionReinit bool
}

type SegTranscodingMetadata struct {
	ManifestID         ManifestID
	Fname              string
	Seq                int64
	Hash               ethcommon.Hash
	Profiles           []ffmpeg.VideoProfile
	OS                 *net.OSInfo
	Duration           time.Duration
	Caps               *Capabilities
	AuthToken          *net.AuthToken
	CalcPerceptualHash bool
	SegmentParameters  *SegmentParameters
	Metadata           map[string]string
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

func NetSegData(md *SegTranscodingMetadata) (*net.SegData, error) {
	fullProfiles, err := common.FFmpegProfiletoNetProfile(md.Profiles)
	if err != nil {
		return nil, err
	}
	storage := []*net.OSInfo{}
	if md.OS != nil {
		storage = append(storage, md.OS)
	}

	// Generate serialized segment info
	segData := &net.SegData{
		ManifestId:         []byte(md.ManifestID),
		Seq:                md.Seq,
		Hash:               md.Hash.Bytes(),
		Storage:            storage,
		Duration:           int32(md.Duration / time.Millisecond),
		Capabilities:       md.Caps.ToNetCapabilities(),
		AuthToken:          md.AuthToken,
		CalcPerceptualHash: md.CalcPerceptualHash,
		// Triggers failure on Os that don't know how to use FullProfiles/2/3
		Profiles: []byte("invalid"),
	}
	if md.SegmentParameters != nil {
		segData.ForceSessionReinit = md.SegmentParameters.ForceSessionReinit
		if md.SegmentParameters.Clip != nil {
			segData.SegmentParameters = &net.SegParameters{
				From: uint64(md.SegmentParameters.Clip.From.Milliseconds()),
				To:   uint64(md.SegmentParameters.Clip.To.Milliseconds()),
			}
		}
	}

	// If all outputs are mpegts, use the older SegData.FullProfiles field
	// for compatibility with older orchestrators
	allTS := true
	for i := 0; i < len(md.Profiles) && allTS; i++ {
		switch md.Profiles[i].Format {
		case ffmpeg.FormatNone: // default output is mpegts for FormatNone
		case ffmpeg.FormatMPEGTS:
		default:
			allTS = false
		}
	}
	allIntFPS := true
	allDefaultProfiles := true
	allDefaultGOPs := true
	for i := 0; i < len(md.Profiles) && allIntFPS && allDefaultProfiles && allDefaultGOPs; i++ {
		if md.Profiles[i].FramerateDen != 0 {
			allIntFPS = false
		}
		if md.Profiles[i].Profile != ffmpeg.ProfileNone {
			allDefaultProfiles = false
		}
		if md.Profiles[i].GOP != time.Duration(0) {
			allDefaultGOPs = false
		}
	}
	if allIntFPS && allDefaultProfiles && allDefaultGOPs {
		if allTS {
			segData.FullProfiles = fullProfiles
		} else {
			segData.FullProfiles2 = fullProfiles
		}
	} else {
		segData.FullProfiles3 = fullProfiles
	}

	return segData, nil
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

func MakeMetadata(id string) map[string]string {
	s := fmt.Sprintf("Livepeer Transcoder %s (%s)", LivepeerVersion, id)
	return map[string]string{
		"service_provider": s,                   // for mpegts
		"comment":          "Processed by " + s, // for mp4
	}
}
