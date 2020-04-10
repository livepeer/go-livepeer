package discovery

import (
	"container/heap"

	"github.com/livepeer/go-livepeer/net"
)

// A suspensionQueue implements heap.Interface and holds suspensions.
type suspensionQueue []*suspension

// A suspension is the item we manage in the priority queue.
type suspension struct {
	orch    *net.OrchestratorInfo
	penalty int
}

func (sq suspensionQueue) Len() int { return len(sq) }

func (sq suspensionQueue) Less(i, j int) bool {
	return sq[i].penalty < sq[j].penalty
}

func (sq suspensionQueue) Swap(i, j int) {
	sq[i], sq[j] = sq[j], sq[i]
}

func (sq *suspensionQueue) Push(x interface{}) {
	item := x.(*suspension)
	*sq = append(*sq, item)
}

func (sq *suspensionQueue) Pop() interface{} {
	old := *sq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*sq = old[0 : n-1]
	return item
}

func newSuspensionQueue() *suspensionQueue {
	sq := &suspensionQueue{}
	heap.Init(sq)
	return sq
}
