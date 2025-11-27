package server

import (
	"container/heap"
	"context"
	"math/big"
	"sort"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
)

const SELECTOR_LATENCY_SCORE_THRESHOLD = 1.0

// BroadcastSessionsSelector selects the next BroadcastSession to use
type BroadcastSessionsSelector interface {
	Add(sessions []*BroadcastSession)
	Remove(sess *BroadcastSession)
	Complete(sess *BroadcastSession)
	Select(ctx context.Context) *BroadcastSession
	Size() int
	Clear()
}

type BroadcastSessionsSelectorFactory func() BroadcastSessionsSelector

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

// Selector is the default selector which always selects the session with the lowest initial latency.
type Selector struct {
	sessions []*BroadcastSession

	stakeRdr           stakeReader
	selectionAlgorithm common.SelectionAlgorithm
	perfScore          *common.PerfScore
	capabilities       common.CapabilityComparator
	sortCompFunc       func(sess1, sess2 *BroadcastSession) bool
}

func NewSelector(stakeRdr stakeReader, selectionAlgorithm common.SelectionAlgorithm, perfScore *common.PerfScore, capabilities common.CapabilityComparator) *Selector {
	// By default, sort by initial latency
	sortCompFunc := func(sess1, sess2 *BroadcastSession) bool {
		return sess1.InitialLatency < sess2.InitialLatency
	}
	return &Selector{
		stakeRdr:           stakeRdr,
		selectionAlgorithm: selectionAlgorithm,
		perfScore:          perfScore,
		capabilities:       capabilities,
		sortCompFunc:       sortCompFunc,
	}
}

func NewSelectorOrderByLatencyScore(stakeRdr stakeReader, selectionAlgorithm common.SelectionAlgorithm, perfScore *common.PerfScore, capabilities common.CapabilityComparator) *Selector {
	sortCompFunc := func(sess1, sess2 *BroadcastSession) bool {
		return sess1.LatencyScore < sess2.LatencyScore
	}
	return &Selector{
		stakeRdr:           stakeRdr,
		selectionAlgorithm: selectionAlgorithm,
		perfScore:          perfScore,
		capabilities:       capabilities,
		sortCompFunc:       sortCompFunc,
	}
}

func (s *Selector) Add(sessions []*BroadcastSession) {
	s.sessions = append(s.sessions, sessions...)
	s.sort()
}

func (s *Selector) Remove(sess *BroadcastSession) {
	s.sessions = removeSession(s.sessions, sess)
}

func (s *Selector) Complete(sess *BroadcastSession) {
	s.sessions = append(s.sessions, sess)
	s.sort()
}

func (s *Selector) sort() {
	sort.Slice(s.sessions, func(i, j int) bool {
		return s.sortCompFunc(s.sessions[i], s.sessions[j])
	})
}

func (s *Selector) Select(ctx context.Context) *BroadcastSession {
	availableOrchestrators := toOrchestrators(s.sessions)
	sess := s.selectUnknownSession(ctx)
	s.sort()
	clog.V(common.DEBUG).Infof(ctx, "Selected orchestrator %s from available list: %v", toOrchestrator(sess), availableOrchestrators)
	return sess
}

func toOrchestrators(sessions []*BroadcastSession) []string {
	orchestrators := make([]string, len(sessions))
	for i, sess := range sessions {
		orchestrators[i] = toOrchestrator(sess)
	}
	return orchestrators
}

func toOrchestrator(sess *BroadcastSession) string {
	if sess != nil && sess.OrchestratorInfo != nil {
		return sess.OrchestratorInfo.Transcoder
	}
	return ""
}

func (s *Selector) Size() int {
	return len(s.sessions)
}

func (s *Selector) Clear() {
	s.sessions = nil
	s.stakeRdr = nil
}

// MinLSSelector selects the next BroadcastSession with the lowest latency score if it is good enough.
// Otherwise, it selects a session that does not have a latency score yet
// MinLSSelector is not concurrency safe so the caller is responsible for ensuring safety for concurrent method calls
type MinLSSelector struct {
	knownSessions *sessHeap
	minLS         float64
	Selector
}

// NewMinLSSelector returns an instance of MinLSSelector configured with a good enough latency score
func NewMinLSSelector(stakeRdr stakeReader, minLS float64, selectionAlgorithm common.SelectionAlgorithm, perfScore *common.PerfScore, capabilities common.CapabilityComparator) *MinLSSelector {
	knownSessions := &sessHeap{}
	heap.Init(knownSessions)

	return &MinLSSelector{
		knownSessions: knownSessions,
		minLS:         minLS,
		Selector: Selector{
			stakeRdr:           stakeRdr,
			selectionAlgorithm: selectionAlgorithm,
			perfScore:          perfScore,
			capabilities:       capabilities,
		},
	}
}

// Add adds the sessions to the selector's list of sessions without a latency score
func (s *MinLSSelector) Add(sessions []*BroadcastSession) {
	s.sessions = append(s.sessions, sessions...)
}

// Remove removes the session from the selector's list of sessions without a latency score
func (s *MinLSSelector) Remove(sess *BroadcastSession) {
	s.sessions = removeSession(s.sessions, sess)
}

// Complete adds the session to the selector's list sessions with a latency score
func (s *MinLSSelector) Complete(sess *BroadcastSession) {
	heap.Push(s.knownSessions, sess)
}

// Select returns the session with the lowest latency score if it is good enough.
// Otherwise, a session without a latency score yet is returned
func (s *MinLSSelector) Select(ctx context.Context) *BroadcastSession {
	sess := s.knownSessions.Peek()
	if sess == nil {
		return s.selectUnknownSession(ctx)
	}

	minSess := sess.(*BroadcastSession)
	if minSess.LatencyScore > s.minLS && len(s.sessions) > 0 {
		return s.selectUnknownSession(ctx)
	}

	return heap.Pop(s.knownSessions).(*BroadcastSession)
}

// Size returns the number of sessions stored by the selector
func (s *MinLSSelector) Size() int {
	return len(s.sessions) + s.knownSessions.Len()
}

// Clear resets the selector's state
func (s *MinLSSelector) Clear() {
	s.sessions = nil
	s.knownSessions = &sessHeap{}
	s.stakeRdr = nil
}

// Use selection algorithm to select from unknownSessions
func (s *Selector) selectUnknownSession(ctx context.Context) *BroadcastSession {
	if len(s.sessions) == 0 {
		clog.Warningf(ctx, "there are no sessions available to select from")
		return nil
	}

	if s.stakeRdr == nil {
		// Sessions are selected based on the order of unknownSessions in off-chain mode
		sess := s.sessions[0]
		s.sessions = s.sessions[1:]
		return sess
	}

	var addrs []ethcommon.Address
	prices := map[ethcommon.Address]*big.Rat{}
	addrCount := make(map[ethcommon.Address]int)
	for _, sess := range s.sessions {
		if sess.OrchestratorInfo.GetTicketParams() == nil {
			continue
		}
		addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
		if _, ok := addrCount[addr]; !ok {
			addrs = append(addrs, addr)
		}
		addrCount[addr]++
		pi := sess.OrchestratorInfo.PriceInfo
		if pi != nil && pi.PixelsPerUnit != 0 {
			prices[addr] = big.NewRat(pi.PricePerUnit, pi.PixelsPerUnit)
		}
	}

	maxPrice := BroadcastCfg.GetCapabilitiesMaxPrice(s.capabilities)

	stakes, err := s.stakeRdr.Stakes(addrs)
	if err != nil {
		clog.Errorf(ctx, "failed to read stake weights for selection err=%q", err)
		return nil
	}
	var perfScores map[ethcommon.Address]float64
	if s.perfScore != nil {
		s.perfScore.Mu.Lock()
		perfScores = map[ethcommon.Address]float64{}
		for _, addr := range addrs {
			perfScores[addr] = s.perfScore.Scores[addr]
		}
		s.perfScore.Mu.Unlock()
	}

	selected := s.selectionAlgorithm.Select(ctx, addrs, stakes, maxPrice, prices, perfScores)

	for i, sess := range s.sessions {
		if sess.OrchestratorInfo.GetTicketParams() == nil {
			continue
		}
		addr := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient)
		if addr == selected {
			s.removeUnknownSession(i)
			return sess
		}
	}

	clog.Warningf(ctx, "failed to select an unknown session that meets the selection criteria")
	return nil
}

func (s *Selector) removeUnknownSession(i int) {
	s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
}

// LIFOSelector selects the next BroadcastSession in LIFO order
// now used only in tests
type LIFOSelector []*BroadcastSession

// Add adds the sessions to the front of the selector's list
func (s *LIFOSelector) Add(sessions []*BroadcastSession) {
	*s = append(sessions, *s...)
}

// Remove removes the session from the selector's list
func (s *LIFOSelector) Remove(sess *BroadcastSession) {
	*s = removeSession(*s, sess)
}

// Complete adds the session to the end of the selector's list
func (s *LIFOSelector) Complete(sess *BroadcastSession) {
	*s = append(*s, sess)
}

// Select returns the last session in the selector's list
func (s *LIFOSelector) Select(ctx context.Context) *BroadcastSession {
	sessList := *s
	last := len(sessList) - 1
	if last < 0 {
		return nil
	}
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

func removeSession(sessions []*BroadcastSession, sess *BroadcastSession) []*BroadcastSession {
	for i, es := range sessions {
		if es == sess {
			return append(sessions[:i], sessions[i+1:]...)
		}
	}
	return sessions
}
