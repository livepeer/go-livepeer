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
	"github.com/livepeer/go-livepeer/core"
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

	// Test when we receive results for only some addresses
	store.err = nil
	store.orchs = []*common.DBOrch{{EthereumAddr: "foo", Stake: 77}}
	stakes, err := rdr.Stakes([]ethcommon.Address{{}, {}})
	assert.Nil(err)
	assert.Len(stakes, 1)
	assert.Equal(stakes[ethcommon.HexToAddress("foo")], int64(77))

	// Test when we receive results for all addresses
	store.orchs = []*common.DBOrch{
		{EthereumAddr: "foo", Stake: 77},
		{EthereumAddr: "bar", Stake: 88},
	}
	stakes, err = rdr.Stakes([]ethcommon.Address{{}, {}})
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

	stakes := make(map[ethcommon.Address]int64)
	for _, addr := range addrs {
		stakes[addr] = r.stakes[addr]
	}

	return stakes, nil
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
		{},
		{},
		{},
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
			{
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

	addrNoInfo := ethcommon.BytesToAddress([]byte(strconv.Itoa(len(sessions) + 1)))
	sessions = append(sessions, &BroadcastSession{OrchestratorInfo: &net.OrchestratorInfo{}})
	stakeMap[addrNoInfo] = 1000
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
	for i, sess := range sessions[:len(sessions)-2] {
		nextSess := sessions[i+1]
		diff := math.Abs(float64(sessCount[sess] - sessCount[nextSess]))
		delta := diff / float64(sessCount[sess])
		assert.Less(t, delta, maxDelta)
	}
	assert.Zero(t, sessCount[sessions[len(sessions)-1]])
}

func TestMinLSSelector_SelectUnknownSession_SameAddress(t *testing.T) {
	stakeRdr := newStubStakeReader()

	selections := 100000

	createSessions := func(num int, addr ethcommon.Address) []*BroadcastSession {
		sessions := make([]*BroadcastSession, num)
		for i := 0; i < num; i++ {
			sessions[i] = &BroadcastSession{
				OrchestratorInfo: &net.OrchestratorInfo{
					TicketParams: &net.TicketParams{Recipient: addr.Bytes()},
				},
			}
		}
		return sessions
	}

	countSelections := func(sessions []*BroadcastSession) map[*BroadcastSession]int {
		// Record # of times a session is selected
		sessCount := make(map[*BroadcastSession]int)
		for i := 0; i < selections; i++ {
			sel := NewMinLSSelector(stakeRdr, 1.0)
			sel.Add(sessions)
			sess := sel.selectUnknownSession()
			sessCount[sess]++
		}
		return sessCount
	}

	checkSelectionCount := func(sessions []*BroadcastSession, sessCount map[*BroadcastSession]int, stakeMap map[ethcommon.Address]int64, totalStake int64) {
		// Check that the difference between the selection count ratio and the stake weight ratio of a session is less than some small delta
		maxDelta := .017
		addrSeenMap := make(map[ethcommon.Address]bool)
		for _, sess := range sessions {
			addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
			// Selection count ratio = # times selected / total selections
			countRat := big.NewRat(int64(sessCount[sess]), int64(selections))
			// Stake weight ratio = 0 by default
			stakeRat := big.NewRat(0, 1)
			// Stake weight ratio = (stake / totalStake) **only if first session for a particular address**
			if _, ok := addrSeenMap[addr]; !ok {
				stakeRat = big.NewRat(stakeMap[addr], totalStake)
				addrSeenMap[addr] = true
			}
			deltaRat := new(big.Rat).Sub(stakeRat, countRat)
			deltaRat.Abs(deltaRat)
			delta, _ := deltaRat.Float64()
			assert.Less(t, delta, maxDelta)
		}
	}

	// 3 sessions with the same address, 2 sessions with a different address
	addr := ethcommon.BytesToAddress([]byte("foo"))
	sessions := createSessions(3, addr)
	// Include a session with a different address than the rest of the sessions
	otherAddr := ethcommon.BytesToAddress([]byte("other"))
	sessions = append(sessions, createSessions(2, otherAddr)...)
	// Give both address 1000 stake
	stake := int64(1000)
	stakeMap := map[ethcommon.Address]int64{addr: stake, otherAddr: stake}
	totalStake := 2 * stake
	stakeRdr.SetStakes(stakeMap)
	sessCount := countSelections(sessions)
	checkSelectionCount(sessions, sessCount, stakeMap, totalStake)
}

func TestMinLSSelector_SelectUnknownSession_AllMissingStake(t *testing.T) {
	assert := assert.New(t)

	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	// Initialize stake reader with empty stake map so all sessions are missing stake
	stakeRdr.SetStakes(make(map[ethcommon.Address]int64))

	sess1 := StubBroadcastSession("")
	sess1.OrchestratorInfo.TicketParams = &net.TicketParams{Recipient: []byte("foo")}
	sess2 := StubBroadcastSession("")
	sess2.OrchestratorInfo.TicketParams = &net.TicketParams{Recipient: []byte("bar")}

	sel.Add([]*BroadcastSession{sess1, sess2})

	// The stake weight of both sessions defaults to 0 so they should be selected in the order that they were added
	assert.Same(sess1, sel.Select())
	assert.Same(sess2, sel.Select())
}

func TestMinLSSelector_SelectUnknownSession_SomeMissingStake(t *testing.T) {
	assert := assert.New(t)

	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelector(stakeRdr, 1.0)

	sess1 := StubBroadcastSession("")
	sess1.OrchestratorInfo.TicketParams = &net.TicketParams{Recipient: []byte("foo")}
	addr2 := ethcommon.BytesToAddress([]byte("bar"))
	sess2 := StubBroadcastSession("")
	sess2.OrchestratorInfo.TicketParams = &net.TicketParams{Recipient: addr2.Bytes()}

	// Initialize stake reader so that some sessions are missing stake
	stakeMap := map[ethcommon.Address]int64{addr2: 100}
	stakeRdr.SetStakes(stakeMap)

	// The stake weight of sess1 defaults to 0 so sess2 should always be selected first
	for i := 0; i < 1000; i++ {
		sel.Add([]*BroadcastSession{sess1, sess2})
		assert.Same(sess2, sel.Select())
		assert.Same(sess1, sel.Select())
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

func TestMinLSSelector_SelectUnknownSession_RandFreq(t *testing.T) {
	assert := assert.New(t)

	stakeRdr := newStubStakeReader()
	sel := NewMinLSSelectorWithRandFreq(stakeRdr, 1.0, 1.0)

	sessions := make([]*BroadcastSession, 10)
	stakes := make([]int64, 10)
	stakeMap := make(map[ethcommon.Address]int64)

	// Give one session a lot of stake
	addr := ethcommon.BytesToAddress([]byte(strconv.Itoa(0)))
	sessions[0] = &BroadcastSession{
		OrchestratorInfo: &net.OrchestratorInfo{
			TicketParams: &net.TicketParams{Recipient: addr.Bytes()},
		},
	}
	stake := int64(10000000000000)
	stakes[0] = stake
	stakeMap[addr] = stake
	// Give the other sessions very little stake
	for i := 1; i < 10; i++ {
		addr := ethcommon.BytesToAddress([]byte(strconv.Itoa(i)))
		stake := int64(1)

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

	// When randFreq = 1.0 we should select randomly instead of selecting the session with the most stake
	sess := sel.selectUnknownSession()
	assert.NotEqual(sess.OrchestratorInfo.TicketParams.Recipient, addr.Bytes())

	// When randFreq = 0.0 we should select the session with the most stake
	sel.randFreq = 0.0
	sess = sel.selectUnknownSession()
	assert.Equal(sess.OrchestratorInfo.TicketParams.Recipient, addr.Bytes())
}

func TestMinLSSelector_RemoveUnknownSession(t *testing.T) {
	assert := assert.New(t)

	sel := NewMinLSSelector(nil, 1.0)

	// Use ManifestID to identify each session
	sessions := []*BroadcastSession{
		{Params: &core.StreamParameters{ManifestID: "foo"}},
		{Params: &core.StreamParameters{ManifestID: "bar"}},
		{Params: &core.StreamParameters{ManifestID: "baz"}},
	}

	resetUnknownSessions := func() {
		// Make a copy of the original slice so we can reset unknownSessions to the original slice
		sel.unknownSessions = make([]*BroadcastSession, len(sessions))
		copy(sel.unknownSessions, sessions)
	}

	// Test remove from front of list
	resetUnknownSessions()
	sel.removeUnknownSession(0)
	assert.Len(sel.unknownSessions, 2)
	assert.Equal("baz", string(sel.unknownSessions[0].Params.ManifestID))
	assert.Equal("bar", string(sel.unknownSessions[1].Params.ManifestID))

	// Test remove from middle of list
	resetUnknownSessions()
	sel.removeUnknownSession(1)
	assert.Len(sel.unknownSessions, 2)
	assert.Equal("foo", string(sel.unknownSessions[0].Params.ManifestID))
	assert.Equal("baz", string(sel.unknownSessions[1].Params.ManifestID))

	// Test remove from back of list
	resetUnknownSessions()
	sel.removeUnknownSession(2)
	assert.Len(sel.unknownSessions, 2)
	assert.Equal("foo", string(sel.unknownSessions[0].Params.ManifestID))
	assert.Equal("bar", string(sel.unknownSessions[1].Params.ManifestID))

	// Test remove when list length = 1
	sel.unknownSessions = []*BroadcastSession{{}}
	sel.removeUnknownSession(0)
	assert.Empty(sel.unknownSessions)
}
