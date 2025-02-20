package server

import (
	"context"
	"math/big"
	"sort"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
)

// MinLSSelector selects the next BroadcastSession with the lowest latency score if it is good enough.
// Otherwise, it selects a session that does not have a latency score yet
// MinLSSelector is not concurrency safe so the caller is responsible for ensuring safety for concurrent method calls
type AIMinLSSelector struct {
	sessions []*BroadcastSession

	stakeRdr           stakeReader
	selectionAlgorithm common.SelectionAlgorithm
	perfScore          *common.PerfScore
	capabilities       common.CapabilityComparator

	minLS float64
}

// NewMinLSSelector returns an instance of MinLSSelector configured with a good enough latency score
func NewAIMinLSSelector(stakeRdr stakeReader, minLS float64, selectionAlgorithm common.SelectionAlgorithm, perfScore *common.PerfScore, capabilities common.CapabilityComparator) *MinLSSelector {
	return &MinLSSelector{
		stakeRdr:           stakeRdr,
		selectionAlgorithm: selectionAlgorithm,
		perfScore:          perfScore,
		capabilities:       capabilities,
		minLS:              minLS,
	}
}

// Add adds the sessions to the selector's list of sessions without a latency score
func (s *AIMinLSSelector) Add(sessions []*BroadcastSession) {
	s.sessions = append(s.sessions, sessions...)
}

// Complete adds the session to the selector's list sessions with a latency score
func (s *AIMinLSSelector) Complete(sess *BroadcastSession) {
	s.sessions = append(s.sessions, sess)

	sort.Slice(s.sessions, func(i, j int) bool {
		return s.sessions[i].LatencyScore < s.sessions[j].LatencyScore
	})
}

// Select returns the session with the lowest latency score if it is good enough.
// Otherwise, a session without a latency score yet is returned
func (s *AIMinLSSelector) Select(ctx context.Context) *BroadcastSession {
	for {
		if len(s.sessions) > 0 {
			sess := s.selectSession(ctx)
			if sess.LatencyScore > s.minLS {
				return sess
			}
		} else {
			//no sessions left
			return nil
		}
	}
}

// Size returns the number of sessions stored by the selector
func (s *AIMinLSSelector) Size() int {
	return len(s.sessions)
}

// Clear resets the selector's state
func (s *AIMinLSSelector) Clear() {
	s.sessions = nil
}

// Use selection algorithm to select from unknownSessions
func (s *AIMinLSSelector) selectSession(ctx context.Context) *BroadcastSession {
	if len(s.sessions) == 0 {
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
			s.removeSession(i)
			return sess
		}
	}

	return nil
}

func (s *AIMinLSSelector) removeSession(i int) {
	n := len(s.sessions)
	s.sessions[n-1], s.sessions[i] = s.sessions[i], s.sessions[n-1]
	s.sessions = s.sessions[:n-1]
}
