package watchers

import (
	"github.com/ethereum/go-ethereum/event"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
)

type BlockWatcher interface {
	Subscribe(sink chan<- []*blockwatch.Event) event.Subscription
}
