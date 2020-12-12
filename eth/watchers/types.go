package watchers

import (
	"github.com/0xProject/0x-mesh/ethereum/blockwatch"
	"github.com/0xProject/0x-mesh/ethereum/miniheader"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

type BlockWatcher interface {
	Subscribe(sink chan<- []*blockwatch.Event) event.Subscription
	GetLatestBlockProcessed() (*miniheader.MiniHeader, error)
}

type timeWatcher interface {
	SubscribeRounds(sink chan<- types.Log) event.Subscription
}
