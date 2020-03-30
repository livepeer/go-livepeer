package watchers

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
)

type BlockWatcher interface {
	Subscribe(sink chan<- []*blockwatch.Event) event.Subscription
	GetLatestBlock() (*blockwatch.MiniHeader, error)
}

type timeWatcher interface {
	SubscribeRounds(sink chan<- types.Log) event.Subscription
}
