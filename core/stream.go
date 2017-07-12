package core

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"time"
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
