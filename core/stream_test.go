package core

import (
	"fmt"
	"testing"
)

func TestStreamID(t *testing.T) {
	id := MakeStreamID(NodeID("nid"), RandomVideoID(), "")
	fmt.Println(id)
	id.String()
}
