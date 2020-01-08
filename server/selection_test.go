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
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
)

type stubOrchestratorStore struct {
	orchs []*common.DBOrch
	err   error
}

func (s *stubOrchestratorStore) OrchCount(filter *common.DBOrchFilter) (int, error) { return 0, nil }
func (s *stubOrchestratorStore) UpdateOrch(orch *common.DBOrch) error               { return nil }
func (s *stubOrchestratorStore) SelectOrchs(filter *common.DBOrchFilter) ([]*common.DBOrch, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.orchs, nil
}

func TestStoreStakeReader(t *testing.T) {
	assert := assert.New(t)

	store := &stubOrchestratorStore{}
	rdr := &storeStakeReader{store: store}

	store.err = errors.New("SelectOrchs error")
	_, err := rdr.Stakes(nil)
	assert.EqualError(err, store.err.Error())

	store.err = nil
	store.orchs = []*common.DBOrch{&common.DBOrch{}}
	_, err = rdr.Stakes(nil)
	assert.EqualError(err, "could not fetch all stake weights")

	store.orchs = []*common.DBOrch{
		&common.DBOrch{EthereumAddr: "foo", Stake: 77},
		&common.DBOrch{EthereumAddr: "bar", Stake: 88},
	}
	stakes, err := rdr.Stakes([]ethcommon.Address{ethcommon.Address{}, ethcommon.Address{}})
	assert.Nil(err)

	for _, orch := range store.orchs {
		addr := ethcommon.HexToAddress(orch.EthereumAddr)
		assert.Contains(stakes, addr)
		assert.Equal(stakes[addr], orch.Stake)
	}
}

type stubStakeReader struct {
	stakes map[ethcommon.Address]int64
	err    error
}

func newStubStakeReader() *stubStakeReader {
	return &stubStakeReader{stakes: make(map[ethcommon.Address]int64)}
}

func (r *stubStakeReader) Stakes(addrs []ethcommon.Address) (map[ethcommon.Address]int64, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.stakes, nil
}

func (r *stubStakeReader) SetStakes(stakes map[ethcommon.Address]int64) {
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

	// Return nil when there are no sessions
	assert.Nil(sel.Select())

	sel.Add(sessions)
	assert.Equal(sel.Size(), 3)
	for _, sess := range sessions {
		assert.Contains(sel.unknownSessions, sess)
	}

	// Select from unknownSessions
	sess1 := sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessions), 2)

	// Set sess1.LatencyScore to not be good enough
	sess1.LatencyScore = 1.1
	sel.Complete(sess1)
	assert.Equal(sel.Size(), 3)
	assert.Equal(len(sel.unknownSessions), 2)
	assert.Equal(sel.knownSessions.Len(), 1)

	// Select from unknownSessions
	sess2 := sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessions), 1)
	assert.Equal(sel.knownSessions.Len(), 1)

	// Set sess2.LatencyScore to be good enough
	sess2.LatencyScore = .9
	sel.Complete(sess2)
	assert.Equal(sel.Size(), 3)
	assert.Equal(len(sel.unknownSessions), 1)
	assert.Equal(sel.knownSessions.Len(), 2)

	// Select from knownSessions
	knownSess := sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessions), 1)
	assert.Equal(sel.knownSessions.Len(), 1)
	assert.Equal(knownSess, sess2)

	// Set knownSess.LatencyScore to not be good enough
	knownSess.LatencyScore = 1.1
	sel.Complete(knownSess)
	// Clear unknownSessions
	sess := sel.Select()
	sess.LatencyScore = 2.1
	sel.Complete(sess)
	assert.Equal(len(sel.unknownSessions), 0)
	assert.Equal(sel.knownSessions.Len(), 3)

	// Select from knownSessions
	knownSess = sel.Select()
	assert.Equal(sel.Size(), 2)
	assert.Equal(len(sel.unknownSessions), 0)
	assert.Equal(sel.knownSessions.Len(), 2)

	sel.Clear()
	assert.Zero(sel.Size())
	assert.Nil(sel.unknownSessions)
	assert.Zero(sel.knownSessions.Len())
	assert.Nil(sel.stakeRdr)
}

func TestMinLSSelector_SelectUnknownSession_Errors(t *testing.T) {
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
	errorLogsBefore := glog.Stats.Error.Lines()
	assert.Nil(sel.selectUnknownSession())
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
}

func TestMinLSSelector_SelectUnknownSession_UniqueWeights(t *testing.T) {
	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	sessions := make([]*BroadcastSession, 10)
	stakes := make([]int64, 10)
	stakeMap := make(map[ethcommon.Address]int64)
	totalStake := int64(0)
	for i := 0; i < 10; i++ {
		addr := ethcommon.BytesToAddress([]byte(strconv.Itoa(i)))
		stake := int64(1000 * (i + 1))
		totalStake += stake

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

	// Run selectUnknownSession() x100000 and record # of times a session weight is selected
	// Each session has a unique stake weight so we will record the # of selections per stake weight
	stakeCount := make(map[int64]int)
	for i := 0; i < 100000; i++ {
		sess := sel.selectUnknownSession()
		addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
		stake := stakeMap[addr]
		stakeCount[stake]++

		// Call Add() to add the session back to unknownSessions
		sel.Add([]*BroadcastSession{sess})
	}

	sort.Slice(stakes, func(i, j int) bool {
		return stakes[i] < stakes[j]
	})

	// Check that higher stake weight sessions are selected more often than lower stake weight sessions
	for i, stake := range stakes[:len(stakes)-1] {
		nextStake := stakes[i+1]
		assert.Less(t, stakeCount[stake], stakeCount[nextStake])
	}

	// Check that the difference between the selection count ratio and the stake weight ratio of a session is less than some small delta
	maxDelta := .015
	for stake, count := range stakeCount {
		// Selection count ratio = # times selected / total selections
		countRat := big.NewRat(int64(count), 100000)
		// Stake weight ratio = stake / totalStake
		stakeRat := big.NewRat(stake, totalStake)
		deltaRat := new(big.Rat).Sub(stakeRat, countRat)
		deltaRat.Abs(deltaRat)
		delta, _ := deltaRat.Float64()
		assert.Less(t, delta, maxDelta)
	}
}

func TestMinLSSelector_SelectUnknownSession_UniformWeights(t *testing.T) {
	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	sessions := make([]*BroadcastSession, 10)
	stakeMap := make(map[ethcommon.Address]int64)
	for i := 0; i < 10; i++ {
		addr := ethcommon.BytesToAddress([]byte(strconv.Itoa(i)))

		sessions[i] = &BroadcastSession{
			OrchestratorInfo: &net.OrchestratorInfo{
				TicketParams: &net.TicketParams{Recipient: addr.Bytes()},
			},
		}
		stakeMap[addr] = 1000
	}

	stakeRdr.SetStakes(stakeMap)
	sel.Add(sessions)

	// Run selectUnknownSession() x1000000 and record # of times a session is selected
	sessCount := make(map[*BroadcastSession]int)
	for i := 0; i < 1000000; i++ {
		sess := sel.selectUnknownSession()
		sessCount[sess]++

		// Call Add() to add the session back to unknownSessions
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

func TestMinLSSelector_SelectUnknownSession_NilStakeReader(t *testing.T) {
	sel := NewMinLSSelector(nil, 1.0)

	sessions := make([]*BroadcastSession, 10)
	for i := 0; i < 10; i++ {
		sessions[i] = &BroadcastSession{}
	}

	sel.Add(sessions)

	i := 0
	// Check that we select sessions based on the order of unknownSessions and that the size of
	// unknownSessions decreases with each selection
	for sel.Size() > 0 {
		sess := sel.selectUnknownSession()
		assert.Same(t, sess, sessions[i])
		i++
	}
}
