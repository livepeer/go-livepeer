package discovery

import (
	"container/heap"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/net"
)

type suspensionList struct {
	mu   sync.Mutex
	list map[string]time.Time
}

func newSuspensionList() *suspensionList {
	return &suspensionList{
		list: make(map[string]time.Time),
	}
}

func (l *suspensionList) suspend(orch string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list[orch] = time.Now()
}

func (l *suspensionList) remove(orch string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.list, orch)
}

func (l *suspensionList) suspendedAt(orch string) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list[orch]
}

func (l *suspensionList) isSuspended(orch string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.list[orch]
	return ok
}

// An Item is something we manage in a priority queue.
type suspension struct {
	orch *net.OrchestratorInfo
	time time.Time
}

// A PriorityQueue implements heap.Interface and holds Items.
type priorityQueue []*suspension

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].time.After(pq[j].time)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*suspension)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[0 : n-1]
	return item
}

func newPriorityQueue() *priorityQueue {
	pq := &priorityQueue{}
	heap.Init(pq)
	return pq
}
