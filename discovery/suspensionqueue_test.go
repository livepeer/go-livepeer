package discovery

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuspensionQueue(t *testing.T) {
	assert := assert.New(t)
	sq := newSuspensionQueue()
	susp0 := &suspension{penalty: 4}
	heap.Push(sq, susp0)
	assert.Equal(sq.Len(), 1)
	assert.Equal(heap.Pop(sq).(*suspension), susp0)
	assert.Equal(sq.Len(), 0)

	susp1 := &suspension{penalty: 6}
	heap.Push(sq, susp0)
	assert.Equal(sq.Len(), 1)
	heap.Push(sq, susp1)
	assert.Equal(sq.Len(), 2)
	assert.True(sq.Less(0, 1))
	assert.Equal(heap.Pop(sq).(*suspension), susp0)
	assert.Equal(sq.Len(), 1)
	assert.Equal(heap.Pop(sq).(*suspension), susp1)
	assert.Equal(sq.Len(), 0)

	susp2 := &suspension{penalty: 2}
	heap.Push(sq, susp0)
	assert.Equal(sq.Len(), 1)
	heap.Push(sq, susp1)
	assert.Equal(sq.Len(), 2)
	heap.Push(sq, susp2)
	assert.Equal(sq.Len(), 3)
	assert.Equal(heap.Pop(sq).(*suspension), susp2)
	assert.Equal(sq.Len(), 2)
	assert.Equal(heap.Pop(sq).(*suspension), susp0)
	assert.Equal(sq.Len(), 1)
	assert.Equal(heap.Pop(sq).(*suspension), susp1)
	assert.Equal(sq.Len(), 0)
}
