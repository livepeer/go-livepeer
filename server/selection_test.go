package server

import (
	"container/heap"
	"errors"
	"math"
	"math/big"
	"sort"
	"strconv"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
)

type stubStakeReader struct {
	stakes map[ethcommon.Address]*big.Int
	err    error
}

func newStubStakeReader() *stubStakeReader {
	return &stubStakeReader{stakes: make(map[ethcommon.Address]*big.Int)}
}

func (r *stubStakeReader) Stakes(addrs []ethcommon.Address) (map[ethcommon.Address]*big.Int, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.stakes, nil
}

func (r *stubStakeReader) SetStakes(stakes map[ethcommon.Address]*big.Int) {
	r.stakes = stakes
}

func TestSessHeap(t *testing.T) {
	assert := assert.New(t)

	h := &sessHeap{}
	heap.Init(h)
	assert.Zero(h.Len())
	// Return nil for empty heap
	assert.Nil(h.Peek())

	sess1 := &BroadcastSession{LatencyScore: 1.0}
	heap.Push(h, sess1)
	assert.Equal(h.Len(), 1)
	assert.Equal(h.Peek().(*BroadcastSession), sess1)

	sess2 := &BroadcastSession{LatencyScore: 1.1}
	heap.Push(h, sess2)
	assert.Equal(h.Len(), 2)
	assert.Equal(h.Peek().(*BroadcastSession), sess1)

	sess3 := &BroadcastSession{LatencyScore: .9}
	heap.Push(h, sess3)
	assert.Equal(h.Len(), 3)
	assert.Equal(h.Peek().(*BroadcastSession), sess3)

	assert.Equal(heap.Pop(h).(*BroadcastSession), sess3)
	assert.Equal(heap.Pop(h).(*BroadcastSession), sess1)
	assert.Equal(heap.Pop(h).(*BroadcastSession), sess2)
	assert.Zero(h.Len())
}

func TestMinLSSelector(t *testing.T) {
	assert := assert.New(t)

	sel := NewMinLSSelector(nil, 1.0)
	assert.Zero(sel.Size())

	sessions := []*BroadcastSession{
		&BroadcastSession{},
		&BroadcastSession{},
		&BroadcastSession{},
	}

	sel.Add(sessions)
	assert.Equal(sel.Size(), 3)
	for _, sess := range sessions {
		assert.Contains(sel.unknownSessList, sess)
	}

	// Select from unknownSessList
	sess1 := sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessList), 2)

	// Set sess1.LatencyScore to not be good enough
	sess1.LatencyScore = 1.1
	sel.Complete(sess1)
	assert.Equal(sel.Size(), 3)
	assert.Equal(len(sel.unknownSessList), 2)
	assert.Equal(sel.knownSessList.Len(), 1)

	// Select from unknownSessList
	sess2 := sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessList), 1)
	assert.Equal(sel.knownSessList.Len(), 1)

	// Set sess2.LatencyScore to be good enough
	sess2.LatencyScore = .9
	sel.Complete(sess2)
	assert.Equal(sel.Size(), 3)
	assert.Equal(len(sel.unknownSessList), 1)
	assert.Equal(sel.knownSessList.Len(), 2)

	// Select from knownSessList
	knownSess := sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessList), 1)
	assert.Equal(sel.knownSessList.Len(), 1)
	assert.Equal(knownSess, sess2)

	sel.Clear()
	assert.Zero(sel.Size())
	assert.Nil(sel.unknownSessList)
	assert.Zero(sel.knownSessList.Len())
	assert.Nil(sel.stakeRdr)
}

func TestMinLSSelector_UnknownSessListSelect_Errors(t *testing.T) {
	assert := assert.New(t)

	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	sel.Add(
		[]*BroadcastSession{
			&BroadcastSession{
				OrchestratorInfo: &net.OrchestratorInfo{
					TicketParams: &net.TicketParams{Recipient: []byte("foo")},
				},
			},
		},
	)

	// Test error when reading stake
	stakeRdr.err = errors.New("Stakes error")
	assert.Nil(sel.unknownSessListSelect())
}

func TestMinLSSelector_UnknownSessListSelect_UniqueWeights(t *testing.T) {
	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	sessions := make([]*BroadcastSession, 10)
	stakes := make([]*big.Int, 10)
	stakeMap := make(map[ethcommon.Address]*big.Int)
	for i := 0; i < 10; i++ {
		addr := ethcommon.BytesToAddress([]byte(strconv.Itoa(i)))
		stake := big.NewInt(int64(1000 * i))

		sessions[i] = &BroadcastSession{
			OrchestratorInfo: &net.OrchestratorInfo{
				TicketParams: &net.TicketParams{Recipient: addr.Bytes()},
			},
		}
		stakes[i] = stake
		stakeMap[addr] = stake
	}

	stakeRdr.SetStakes(stakeMap)
	sel.Add(sessions)

	// Run unknownSessListSelect() x100000 and record # of times a session weight is selected
	// Each session has a unique stake weight so we will record the # of selections per stake weight
	stakeCount := make(map[*big.Int]int)
	for i := 0; i < 100000; i++ {
		sess := sel.unknownSessListSelect()
		addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
		stake := stakeMap[addr]
		stakeCount[stake]++

		// Call Add() to add the session back to unknownSessList
		sel.Add([]*BroadcastSession{sess})
	}

	sort.Slice(stakes, func(i, j int) bool {
		return stakes[i].Cmp(stakes[j]) < 0
	})

	// Check that higher stake weight sessions are selected more often than lower stake weight sessions
	for i, stake := range stakes[:len(stakes)-1] {
		nextStake := stakes[i+1]
		assert.Less(t, stakeCount[stake], stakeCount[nextStake])
	}
}

func TestMinLSSelector_UnknownSessListSelect_UniformWeights(t *testing.T) {
	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	sessions := make([]*BroadcastSession, 10)
	stakeMap := make(map[ethcommon.Address]*big.Int)
	for i := 0; i < 10; i++ {
		addr := ethcommon.BytesToAddress([]byte(strconv.Itoa(i)))

		sessions[i] = &BroadcastSession{
			OrchestratorInfo: &net.OrchestratorInfo{
				TicketParams: &net.TicketParams{Recipient: addr.Bytes()},
			},
		}
		stakeMap[addr] = big.NewInt(1000)
	}

	stakeRdr.SetStakes(stakeMap)
	sel.Add(sessions)

	// Run unknownSessListSelect() x1000000 and record # of times a session is selected
	sessCount := make(map[*BroadcastSession]int)
	for i := 0; i < 1000000; i++ {
		sess := sel.unknownSessListSelect()
		sessCount[sess]++

		// Call Add() to add the session back to unknownSessList
		sel.Add([]*BroadcastSession{sess})
	}

	// Check that the difference between the selection count of each session is less than some small delta
	maxDelta := .015
	for i, sess := range sessions[:len(sessions)-1] {
		nextSess := sessions[i+1]
		diff := math.Abs(float64(sessCount[sess] - sessCount[nextSess]))
		delta := diff / float64(sessCount[sess])
		assert.Less(t, delta, maxDelta)
	}
}

func TestMinLSSelector_UnknownSessListSelect_NilStakeReader(t *testing.T) {
	sel := NewMinLSSelector(nil, 1.0)

	sessions := make([]*BroadcastSession, 10)
	for i := 0; i < 10; i++ {
		sessions[i] = &BroadcastSession{}
	}

	sel.Add(sessions)

	i := 0
	// Check that we select sessions based on the order of unknownSessList and that the size of
	// unknownSessList decreases with each selection
	for sel.Size() > 0 {
		sess := sel.unknownSessListSelect()
		assert.Equal(t, sess, sessions[i])
		i++
	}
}
