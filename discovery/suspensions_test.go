package discovery

import (
	"container/heap"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSuspensionList(t *testing.T) {
	assert := assert.New(t)
	sl := newSuspensionList()

	sl.suspend("foo")
	assert.True(sl.isSuspended("foo"))
	sl.remove("foo")
	assert.False(sl.isSuspended("foo"))

	now := time.Now()
	sl.list["foo"] = now
	at := sl.suspendedAt("foo")
	assert.Equal(now, at)

	now = time.Now().Add(1 * time.Hour)
	sl.list["foo"] = now
	assert.True(sl.isSuspended("foo"))
}

func TestPriorityQueue(t *testing.T) {
	assert := assert.New(t)
	pq := newPriorityQueue()
	susp0 := &suspension{time: time.Now()}
	heap.Push(pq, susp0)
	assert.Equal(pq.Len(), 1)
	assert.Equal(pq.Pop().(*suspension), susp0)
	assert.Equal(pq.Len(), 0)

	susp1 := &suspension{time: time.Now().Add(1 * time.Hour)}
	heap.Push(pq, susp0)
	assert.Equal(pq.Len(), 1)
	heap.Push(pq, susp1)
	assert.Equal(pq.Len(), 2)
	assert.Equal(pq.Pop().(*suspension), susp0)
	assert.Equal(pq.Len(), 1)
	assert.Equal(pq.Pop().(*suspension), susp1)
	assert.Equal(pq.Len(), 0)

	susp2 := &suspension{time: time.Now().Add(-1 * time.Hour)}
	heap.Push(pq, susp0)
	assert.Equal(pq.Len(), 1)
	heap.Push(pq, susp1)
	assert.Equal(pq.Len(), 2)
	heap.Push(pq, susp2)
	assert.Equal(pq.Len(), 3)
	assert.Equal(pq.Pop().(*suspension), susp2)
	assert.Equal(pq.Len(), 2)
	assert.Equal(pq.Pop().(*suspension), susp0)
	assert.Equal(pq.Len(), 1)
	assert.Equal(pq.Pop().(*suspension), susp1)
	assert.Equal(pq.Len(), 0)
}
