package eth

import (
	"context"
	"fmt"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
	ErrNotRewardCaller      = fmt.Errorf("not the reward caller")
)

// CheckRewardCaller returns nil when the client's account is transcoder or its reward
// caller (LIP-118), and an error wrapping ErrNotRewardCaller when it is neither.
func CheckRewardCaller(client LivepeerEthClient, transcoder ethcommon.Address) error {
	account := client.Account().Address
	if transcoder == account {
		return nil
	}
	rewardCaller, err := client.GetRewardCaller(transcoder)
	if err != nil {
		return fmt.Errorf("could not look up reward caller for orchestrator %v: %w", transcoder.Hex(), err)
	}
	if rewardCaller != account {
		return fmt.Errorf("account %v is %w for orchestrator %v (on-chain: %v); set it with livepeer_cli from the orchestrator's account",
			account.Hex(), ErrNotRewardCaller, transcoder.Hex(), rewardCaller.Hex())
	}
	return nil
}

type RewardService struct {
	client       LivepeerEthClient
	working      bool
	orchAddr     ethcommon.Address
	cancelWorker context.CancelFunc
	tw           timeWatcher
	mu           sync.Mutex
}

// NewRewardService returns a service that calls reward once per round for orchAddr. When
// orchAddr is not the node's own account, the node acts as its reward caller (LIP-118).
func NewRewardService(client LivepeerEthClient, tw timeWatcher, orchAddr ethcommon.Address) *RewardService {
	if orchAddr == (ethcommon.Address{}) {
		orchAddr = client.Account().Address
	}
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
	// Eligibility is read from the transcoder, not the caller, whose record is empty.
	transcoder := s.orchAddr
	isRewardCaller := transcoder != account

	t, err := s.client.GetTranscoder(transcoder)
	if err != nil {
		return err
	}

	if t.LastRewardRound.Cmp(currentRound) != -1 || !t.Active {
		return nil
	}

	var tx *types.Transaction
	if isRewardCaller {
		if err := CheckRewardCaller(s.client, transcoder); err != nil {
			return err
		}
		tx, err = s.client.RewardForTranscoder(transcoder)
	} else {
		tx, err = s.client.Reward()
	}
	if err != nil {
		return err
	}

	if err := s.client.CheckTx(tx); err != nil {
		return err
	}

	glog.Infof("Called reward for round %v", currentRound)

	return nil
}
