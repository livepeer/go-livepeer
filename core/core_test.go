package core

import "testing"

// Should Create Node
func TestNewLivepeerNode(t *testing.T) {
	n := NewLivepeerNode()
	if n == nil {
		t.Errorf("Cannot set up new livepeer node")
	}
}

// Should Start Node
// func TestNodeStart(t *testing.T) {
// 	n := NewLivepeerNode()
// 	n.Start()
// }
