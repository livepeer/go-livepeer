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

const (
	HashLength   = 32
	NodeIDLength = 68
)

//StreamID is NodeID|VideoID|Rendition
type StreamID string

func RandomVideoID() []byte {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, HashLength, HashLength)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return x
}

func MakeStreamID(nodeID NodeID, id []byte, rendition string) (StreamID, error) {
	if len(nodeID) != NodeIDLength {
		return "", ErrStreamID
	}

	if rendition == "" {
		return StreamID(fmt.Sprintf("%v%x", nodeID, id)), nil
	} else {
		return StreamID(fmt.Sprintf("%v%x%v", nodeID, id, rendition)), nil
	}
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
	return ""
}

func (id *StreamID) IsMasterPlaylistID() bool {
	return false
}

func (id *StreamID) String() string {
	return string(*id)
}

type SignedSegment struct {
	Seg stream.HLSSegment
	Sig []byte
}

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
