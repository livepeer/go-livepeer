package common

import (
	"net/url"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/net"
)

type Broadcaster interface {
	Address() ethcommon.Address
	Sign([]byte) ([]byte, error)
}

type OrchestratorPool interface {
	GetURLs() []*url.URL
	GetOrchestrators(int) ([]*net.OrchestratorInfo, error)
	Size() int
}

type OrchestratorStore interface {
	OrchCount(filter *DBOrchFilter) (int, error)
	SelectOrchs(filter *DBOrchFilter) ([]*DBOrch, error)
	UpdateOrch(orch *DBOrch) error
}
