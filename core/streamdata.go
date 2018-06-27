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
	HashLength   = 32
	NodeIDLength = 68
)

func RandomVideoID() []byte {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, HashLength, HashLength)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return x
}

//StreamID is NodeID|VideoID|Rendition
type StreamID string

func MakeStreamID(nodeID NodeID, id []byte, rendition string) (StreamID, error) {
	if len(nodeID) != NodeIDLength || len(id) == 0 || rendition == "" {
		return "", ErrManifestID
	}
	return StreamID(fmt.Sprintf("%v%x%v", nodeID, id, rendition)), nil
}

func (id *StreamID) GetNodeID() NodeID {
	return NodeID((*id)[:NodeIDLength])
}

func (id *StreamID) GetVideoID() []byte {
	vid, err := hex.DecodeString(string((*id)[NodeIDLength : NodeIDLength+(2*HashLength)]))
	if err != nil {
		return nil
	}
	return vid
}

func (id *StreamID) GetRendition() string {
	return string((*id)[NodeIDLength+2*HashLength:])
}

func (id *StreamID) IsValid() bool {
	return len(*id) > (NodeIDLength + 2*HashLength)
}

func (id StreamID) String() string {
	return string(id)
}

//ManifestID is NodeID|VideoID
type ManifestID string

func MakeManifestID(nodeID NodeID, id []byte) (ManifestID, error) {
	if nodeID == "" || len(nodeID) != NodeIDLength {
		return "", ErrStreamID
	}
	return ManifestID(fmt.Sprintf("%v%x", nodeID, id)), nil
}

func (id *ManifestID) GetNodeID() NodeID {
	return NodeID((*id)[:NodeIDLength])
}

func (id *ManifestID) GetVideoID() []byte {
	vid, err := hex.DecodeString(string((*id)[NodeIDLength : NodeIDLength+(2*HashLength)]))
	if err != nil {
		return nil
	}
	return vid
}

func (id StreamID) ManifestIDFromStreamID() ManifestID {
	// Ignore error since StreamID should be a valid object;
	// getting the node and video IDs don't fail (unless it segfaults)
	mid, _ := MakeManifestID(id.GetNodeID(), id.GetVideoID())
	return mid
}

func (id *ManifestID) IsValid() bool {
	return len(*id) == (NodeIDLength + 2*HashLength)
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
