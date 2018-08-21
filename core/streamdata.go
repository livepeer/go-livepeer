package core

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/lpms/stream"
)

var ErrStreamID = errors.New("ErrStreamID")
var ErrManifestID = errors.New("ErrManifestID")

const (
	HashLength = 32
)

func RandomVideoID() []byte {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, HashLength, HashLength)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return x
}

//StreamID is VideoID|Rendition
type StreamID string

func MakeStreamID(id []byte, rendition string) (StreamID, error) {
	if len(id) != HashLength || rendition == "" {
		return "", ErrStreamID
	}
	return StreamID(fmt.Sprintf("%x%v", id, rendition)), nil
}

func (id *StreamID) GetVideoID() []byte {
	if len(*id) < 2*HashLength {
		return nil
	}
	vid, err := hex.DecodeString(string((*id)[:2*HashLength]))
	if err != nil {
		return nil
	}
	return vid
}

func (id *StreamID) GetRendition() string {
	return string((*id)[2*HashLength:])
}

func (id *StreamID) IsValid() bool {
	return len(*id) > 2*HashLength
}

func (id StreamID) String() string {
	return string(id)
}

//ManifestID is VideoID
type ManifestID string

func MakeManifestID(id []byte) (ManifestID, error) {
	if len(id) != HashLength {
		return ManifestID(""), ErrManifestID
	}
	return ManifestID(fmt.Sprintf("%x", id)), nil
}

func (id *ManifestID) GetVideoID() []byte {
	if len(*id) < 2*HashLength {
		return nil
	}
	vid, err := hex.DecodeString(string((*id)[:2*HashLength]))
	if err != nil {
		return nil
	}
	return vid
}

func (id StreamID) ManifestIDFromStreamID() (ManifestID, error) {
	if !id.IsValid() {
		return "", ErrManifestID
	}
	mid, err := MakeManifestID(id.GetVideoID())
	return mid, err
}

func (id *ManifestID) IsValid() bool {
	return len(*id) == 2*HashLength
}

func (id ManifestID) String() string {
	return string(id)
}

//Segment and its signature by the broadcaster
type SignedSegment struct {
	Seg stream.HLSSegment
	Sig []byte
}

//Convenience function to convert between SignedSegments and byte slices to put on the wire.
func SignedSegmentToBytes(ss SignedSegment) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(ss)
	if err != nil {
		glog.Errorf("Error encoding segment to []byte: %v", err)
		return nil, err
	}
	return buf.Bytes(), nil
}

//Convenience function to convert between SignedSegments and byte slices to put on the wire.
func BytesToSignedSegment(data []byte) (SignedSegment, error) {
	dec := gob.NewDecoder(bytes.NewReader(data))
	var ss SignedSegment
	err := dec.Decode(&ss)
	if err != nil {
		glog.Errorf("Error decoding byte array into segment: %v", err)
		return SignedSegment{}, err
	}
	return ss, nil
}
