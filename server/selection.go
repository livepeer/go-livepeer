package server

import (
	"container/heap"
	"math/rand"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

// BroadcastSessionsSelector selects the next BroadcastSession to use
type BroadcastSessionsSelector interface {
	Add(sessions []*BroadcastSession)
	Complete(sess *BroadcastSession)
	Select() *BroadcastSession
	Size() int
	Clear()
}

type sessHeap []*BroadcastSession

func (h sessHeap) Len() int {
	return len(h)
}

func (h sessHeap) Less(i, j int) bool {
	return h[i].LatencyScore < h[j].LatencyScore
}

func (h sessHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *sessHeap) Push(x interface{}) {
	sess := x.(*BroadcastSession)
	*h = append(*h, sess)
}

func (h *sessHeap) Pop() interface{} {
	// Pop from the end because heap.Pop() swaps the 0th index element with the last element
	// before calling this method
	// See https://golang.org/src/container/heap/heap.go?s=2190:2223#L50
	old := *h
	n := len(old)
	sess := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]

	return sess
}

func (h *sessHeap) Peek() interface{} {
	if h.Len() == 0 {
		return nil
	}

	// The minimum element is at the 0th index as long as we always modify
	// sessHeap using the heap.Push() and heap.Pop() methods
	// See https://golang.org/pkg/container/heap/
	return (*h)[0]
}

type stakeReader interface {
	Stakes(addrs []ethcommon.Address) (map[ethcommon.Address]int64, error)
}

type storeStakeReader struct {
	store common.OrchestratorStore
}

func (r *storeStakeReader) Stakes(addrs []ethcommon.Address) (map[ethcommon.Address]int64, error) {
	orchs, err := r.store.SelectOrchs(&common.DBOrchFilter{Addresses: addrs})
	if err != nil {
		return nil, err
	}

	// The returned map may not have the stake weights for all addresses and the caller should handle this case
	stakes := make(map[ethcommon.Address]int64)
	for _, orch := range orchs {
		stakes[ethcommon.HexToAddress(orch.EthereumAddr)] = orch.Stake
	}

	return stakes, nil
}

// MinLSSelector selects the next BroadcastSession with the lowest latency score if it is good enough.
// Otherwise, it selects a session that does not have a latency score yet
// MinLSSelector is not concurrency safe so the caller is responsible for ensuring safety for concurrent method calls
type MinLSSelector struct {
	unknownSessions []*BroadcastSession
	knownSessions   *sessHeap

	stakeRdr stakeReader

	minLS float64
}

// NewMinLSSelector returns an instance of MinLSSelector configured with a good enough latency score
func NewMinLSSelector(stakeRdr stakeReader, minLS float64) *MinLSSelector {
	knownSessions := &sessHeap{}
	heap.Init(knownSessions)

	return &MinLSSelector{
		knownSessions: knownSessions,
		stakeRdr:      stakeRdr,
		minLS:         minLS,
	}
}

// Add adds the sessions to the selector's list of sessions without a latency score
func (s *MinLSSelector) Add(sessions []*BroadcastSession) {
	s.unknownSessions = append(s.unknownSessions, sessions...)
}

// Complete adds the session to the selector's list sessions with a latency score
func (s *MinLSSelector) Complete(sess *BroadcastSession) {
	heap.Push(s.knownSessions, sess)
}

// Select returns the session with the lowest latency score if it is good enough.
// Otherwise, a session without a latency score yet is returned
func (s *MinLSSelector) Select() *BroadcastSession {
	sess := s.knownSessions.Peek()
	if sess == nil {
		return s.selectUnknownSession()
	}

	minSess := sess.(*BroadcastSession)
	if minSess.LatencyScore > s.minLS && len(s.unknownSessions) > 0 {
		return s.selectUnknownSession()
	}

	return heap.Pop(s.knownSessions).(*BroadcastSession)
}

// Size returns the number of sessions stored by the selector
func (s *MinLSSelector) Size() int {
	return len(s.unknownSessions) + s.knownSessions.Len()
}

// Clear resets the selector's state
func (s *MinLSSelector) Clear() {
	s.unknownSessions = nil
	s.knownSessions = &sessHeap{}
	s.stakeRdr = nil
}

// Use stake weighted random selection to select from unknownSessions
func (s *MinLSSelector) selectUnknownSession() *BroadcastSession {
	if len(s.unknownSessions) == 0 {
		return nil
	}

	if s.stakeRdr == nil {
		// Sessions are selected based on the order of unknownSessions in off-chain mode
		sess := s.unknownSessions[0]
		s.unknownSessions = s.unknownSessions[1:]
		return sess
	}

	var addrs []ethcommon.Address
	addrCount := make(map[ethcommon.Address]int)
	for _, sess := range s.unknownSessions {
		addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
		if _, ok := addrCount[addr]; !ok {
			addrs = append(addrs, addr)
		}
		addrCount[addr]++
	}

	// Fetch stake weights for all addresses
	// We handle the possibility of missing stake weights for addresses when we run weighted random selection on unknownSessions
	stakes, err := s.stakeRdr.Stakes(addrs)
	// If we fail to read stake weights of unknownSessions we should not continue with selection
	if err != nil {
		glog.Errorf("failed to read stake weights for selection: %v", err)
		return nil
	}

	totalStake := int64(0)
	for _, stake := range stakes {
		totalStake += stake
	}

	r := int64(0)
	// Generate a random stake weight between 1 and totalStake
	if totalStake > 0 {
		r = 1 + rand.Int63n(totalStake)
	}

	// Generate a random integer used to determine how to divide stake between sessions for the same address
	stakeDivRand := rand.Int()
	// Run a weighted random selection on unknownSessions
	// We iterate through each session and subtract the stake weight for the session's orchestrator from r (initialized to a random stake weight)
	// If subtracting the stake weight for the current session from r results in a value <= 0, we select the current session
	// The greater the stake weight of a session, the more likely that it will be selected because subtracting its stake weight from r
	// will result in a value <= 0
	for i, sess := range s.unknownSessions {
		addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
		// If we could not fetch the stake weight for addrs[i] then its stake weight defaults to 0
		// The stake weight for a session is the stake weight of the address divided by the # of sessions the
		// address is associated with
		stake := stakes[addr] / int64(addrCount[addr])
		// If stakeDivRand % addrCount[addr] == 0 then the session is assigned any "leftover" stake that cannot be evenly divided
		// between all sessions for addr
		if stakeDivRand%addrCount[addr] == 0 {
			stake += stakes[addr] % int64(addrCount[addr])
		}
		r -= stake
		// Decrement stake weight available for the address' remaining sessions so the last session
		// uses the remaining stake weight available for the address
		stakes[addr] -= stake
		addrCount[addr]--

		if r <= 0 {
			s.removeUnknownSession(i)
			return sess
		}
	}

	return nil
}

func (s *MinLSSelector) removeUnknownSession(i int) {
	n := len(s.unknownSessions)
	s.unknownSessions[n-1], s.unknownSessions[i] = s.unknownSessions[i], s.unknownSessions[n-1]
	s.unknownSessions = s.unknownSessions[:n-1]
}

// LIFOSelector selects the next BroadcastSession in LIFO order
type LIFOSelector []*BroadcastSession

// Add adds the sessions to the front of the selector's list
func (s *LIFOSelector) Add(sessions []*BroadcastSession) {
	*s = append(sessions, *s...)
}

// Complete adds the session to the end of the selector's list
func (s *LIFOSelector) Complete(sess *BroadcastSession) {
	*s = append(*s, sess)
}

// Select returns the last session in the selector's list
func (s *LIFOSelector) Select() *BroadcastSession {
	sessList := *s
	last := len(sessList) - 1
	sess, sessions := sessList[last], sessList[:last]
	*s = sessions
	return sess
}

// Size returns the number of sessions stored by the selector
func (s *LIFOSelector) Size() int {
	return len(*s)
}

// Clear resets the selector's state
func (s *LIFOSelector) Clear() {
	*s = nil
}
