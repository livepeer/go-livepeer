package core

import (
	"fmt"
	"math/rand"
	"time"
)

const (
	HashLength = 32
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

func MakeStreamID(nodeID NodeID, id []byte, rendition string) StreamID {
	if rendition == "" {
		return StreamID(fmt.Sprintf("%v%x", nodeID, id))
	} else {
		return StreamID(fmt.Sprintf("%v%x%v", nodeID, id, rendition))
	}
}

func (id *StreamID) GetNodeID() string {
	return ""
}

func (id *StreamID) GetVideoID() string {
	return ""
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
