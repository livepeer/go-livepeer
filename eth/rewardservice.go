package eth

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
)

type RewardService struct {
	client  LivepeerEthClient
	working bool
	// orchAddr is the transcoder reward is called for. It is the zero address when the
	// node's own account is the transcoder; otherwise the node acts as a reward caller
	// authorized by orchAddr (LIP-118).
	orchAddr     ethcommon.Address
	cancelWorker context.CancelFunc
	tw           timeWatcher
	mu           sync.Mutex
}

// NewRewardService returns a service that calls reward once per round for orchAddr.
// Pass the zero address when the node's own account is the registered transcoder.
func NewRewardService(client LivepeerEthClient, tw timeWatcher, orchAddr ethcommon.Address) *RewardService {
	return &RewardService{
		client:   client,
		tw:       tw,
		orchAddr: orchAddr,
	}
}

func (s *RewardService) Start(ctx context.Context) error {
	if s.working {
		return ErrRewardServiceStarted
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	s.cancelWorker = cancel

	roundSink := make(chan types.Log, 10)
	sub := s.tw.SubscribeRounds(roundSink)
	defer sub.Unsubscribe()

	s.working = true
	defer func() {
		s.working = false
	}()

	for {
		select {
		case err := <-sub.Err():
			if err != nil {
				glog.Errorf("Round subscription error err=%q", err)
			}
		case <-roundSink:
			go func() {
				err := s.tryReward()
				if err != nil {
					glog.Errorf("Error trying to call reward for round %v err=%q", s.tw.LastInitializedRound(), err)
					if monitor.Enabled {
						monitor.RewardCallError(err.Error())
					}
				}
			}()
		case <-cancelCtx.Done():
			glog.V(5).Infof("Reward service done")
			return nil
		}
	}
}

func (s *RewardService) Stop() error {
	if !s.working {
		return ErrRewardServiceStopped
	}

	s.cancelWorker()
	s.working = false

	return nil
}

func (s *RewardService) IsWorking() bool {
	return s.working
}

func (s *RewardService) tryReward() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentRound := s.tw.LastInitializedRound()

	account := s.client.Account().Address
	// Eligibility is always a property of the transcoder, never of the caller. Reading
	// the caller's record here would return an empty, inactive transcoder and silently
	// skip the reward call for the whole round.
	transcoder := s.orchAddr
	delegated := transcoder != (ethcommon.Address{}) && transcoder != account
	if !delegated {
		transcoder = account
	}

	t, err := s.client.GetTranscoder(transcoder)
	if err != nil {
		return err
	}

	if t.LastRewardRound.Cmp(currentRound) != -1 || !t.Active {
		return nil
	}

	if !delegated {
		return s.sendReward(currentRound, func() (*types.Transaction, error) { return s.client.Reward() })
	}

	// Re-check authorization every round so a revoked reward caller surfaces as a clear
	// error rather than a bare on-chain revert.
	rewardCaller, err := s.client.GetRewardCaller(transcoder)
	if err != nil {
		return fmt.Errorf("could not look up reward caller for transcoder %v: %w", transcoder.Hex(), err)
	}
	if rewardCaller != account {
		return fmt.Errorf(
			"node account %v is not the reward caller for transcoder %v (on-chain reward caller is %v); "+
				"set it with livepeer_cli from the transcoder's wallet",
			account.Hex(), transcoder.Hex(), rewardCaller.Hex(),
		)
	}

	return s.sendReward(currentRound, func() (*types.Transaction, error) {
		return s.client.RewardForTranscoder(transcoder)
	})
}

func (s *RewardService) sendReward(currentRound *big.Int, send func() (*types.Transaction, error)) error {
	tx, err := send()
	if err != nil {
		return err
	}

	if err := s.client.CheckTx(tx); err != nil {
		return err
	}

	glog.Infof("Called reward for round %v", currentRound)

	return nil
}
