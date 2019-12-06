package server

import (
	"container/heap"
	"crypto/rand"
	"errors"
	"math/big"

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

	return (*h)[0]
}

type stakeReader interface {
	Stakes(addrs []ethcommon.Address) (map[ethcommon.Address]*big.Int, error)
}

type storeStakeReader struct {
	store common.OrchestratorStore
}

func (r *storeStakeReader) Stakes(addrs []ethcommon.Address) (map[ethcommon.Address]*big.Int, error) {
	orchs, err := r.store.SelectOrchs(&common.DBOrchFilter{Addresses: addrs})
	if err != nil {
		return nil, err
	}

	if len(orchs) != len(addrs) {
		return nil, errors.New("could not fetch all stake weights")
	}

	stakes := make(map[ethcommon.Address]*big.Int)
	for _, orch := range orchs {
		stakes[ethcommon.HexToAddress(orch.EthereumAddr)] = new(big.Int).SetBytes(orch.Stake)
	}

	return stakes, nil
}

// MinLSSelector selects the next BroadcastSession with the lowest latency score if it is good enough.
// Otherwise, it selects a session that does not have a latency score yet
type MinLSSelector struct {
	unknownSessList []*BroadcastSession
	knownSessList   *sessHeap

	stakeRdr stakeReader

	goodEnoughLS float64
}

// NewMinLSSelector returns an instance of MinLSSelector configured with a good enough latency score
func NewMinLSSelector(stakeRdr stakeReader, goodEnoughLS float64) *MinLSSelector {
	knownSessList := &sessHeap{}
	heap.Init(knownSessList)

	return &MinLSSelector{
		knownSessList: knownSessList,
		stakeRdr:      stakeRdr,
		goodEnoughLS:  goodEnoughLS,
	}
}

// Add adds the sessions to the selector's list of sessions without a latency score
func (s *MinLSSelector) Add(sessions []*BroadcastSession) {
	s.unknownSessList = append(s.unknownSessList, sessions...)
}

// Complete adds the session to the selector's list sessions with a latency score
func (s *MinLSSelector) Complete(sess *BroadcastSession) {
	heap.Push(s.knownSessList, sess)
}

// Select returns the session with the lowest latency score if it is good enough.
// Otherwise, a session without a latency score yet is returned
func (s *MinLSSelector) Select() *BroadcastSession {
	if s.knownSessList.Len() == 0 {
		return s.unknownSessListSelect()
	}

	// knownSessList is not empty at this point so we know minSess is not nil
	minSess := s.knownSessList.Peek().(*BroadcastSession)
	if minSess.LatencyScore > s.goodEnoughLS && len(s.unknownSessList) > 0 {
		return s.unknownSessListSelect()
	}

	return heap.Pop(s.knownSessList).(*BroadcastSession)
}

// Size returns the number of sessions stored by the selector
func (s *MinLSSelector) Size() int {
	return len(s.unknownSessList) + s.knownSessList.Len()
}

// Clear resets the selector's state
func (s *MinLSSelector) Clear() {
	s.unknownSessList = nil
	s.knownSessList = &sessHeap{}
	s.stakeRdr = nil
}

// Use stake weighted random selection to select from unknownSessList
func (s *MinLSSelector) unknownSessListSelect() *BroadcastSession {
	if s.stakeRdr == nil {
		// Sessions are selected based on the order of unknownSessList in off-chain mode
		sess := s.unknownSessList[0]
		s.unknownSessList = s.unknownSessList[1:]
		return sess
	}

	addrs := make([]ethcommon.Address, len(s.unknownSessList))
	for i, sess := range s.unknownSessList {
		addrs[i] = ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
	}

	stakes, err := s.stakeRdr.Stakes(addrs)
	// If we fail to read stake weights of unknownSessList we should not continue with selection
	if err != nil {
		glog.Errorf("failed to read stake weights for selection: %v", err)
		return nil
	}

	totalStake := big.NewInt(0)
	for _, stake := range stakes {
		totalStake.Add(totalStake, stake)
	}

	r := big.NewInt(0)
	if totalStake.Cmp(r) > 0 {
		randStake, err := rand.Int(rand.Reader, totalStake)
		// If we fail to generate a random stake weight we should not continue with selection
		if err != nil {
			glog.Errorf("failed to generate random stake weight for selection: %v", err)
			return nil
		}

		r = randStake
	}

	for i, sess := range s.unknownSessList {
		r.Sub(r, stakes[addrs[i]])

		if r.Cmp(big.NewInt(0)) <= 0 {
			n := len(s.unknownSessList)
			s.unknownSessList[n-1], s.unknownSessList[i] = s.unknownSessList[i], s.unknownSessList[n-1]
			s.unknownSessList = s.unknownSessList[:n-1]

			return sess
		}
	}

	return nil
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
