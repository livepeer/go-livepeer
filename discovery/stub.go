package discovery

import (
	"math/big"
	"net/url"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
)

type stubOrchestratorPool struct {
	uris  []*url.URL
	bcast common.Broadcaster
}

func stringsToURIs(addresses []string) []*url.URL {
	var uris []*url.URL

	for _, addr := range addresses {
		uri, err := url.ParseRequestURI(addr)
		if err == nil {
			uris = append(uris, uri)
		}
	}
	return uris
}

func StubOrchestratorPool(addresses []string) *stubOrchestratorPool {
	uris := stringsToURIs(addresses)
	node, _ := core.NewLivepeerNode(nil, "", nil)
	bcast := core.NewBroadcaster(node)

	return &stubOrchestratorPool{bcast: bcast, uris: uris}
}

func StubOrchestrators(addresses []string) []*lpTypes.Transcoder {
	var orchestrators []*lpTypes.Transcoder

	for _, addr := range addresses {
		address := ethcommon.BytesToAddress([]byte(addr))
		transc := &lpTypes.Transcoder{
			ServiceURI:        addr,
			Address:           address,
			ActivationRound:   big.NewInt(0),
			DeactivationRound: big.NewInt(0),
		}
		orchestrators = append(orchestrators, transc)
	}

	return orchestrators
}

type stubTicketParamsValidator struct {
	err error
}

func (s *stubTicketParamsValidator) ValidateTicketParams(params *pm.TicketParams) error { return s.err }

type stubRoundsManager struct {
	round *big.Int
}

func (s *stubRoundsManager) LastInitializedRound() *big.Int { return s.round }

type orchTest struct {
	EthereumAddr  string
	ServiceURI    string
	PricePerPixel int64
}

func toOrchTest(addr, serviceURI string, pricePerPixel int64) orchTest {
	return orchTest{EthereumAddr: addr, ServiceURI: serviceURI, PricePerPixel: pricePerPixel}
}

type stubSuspender struct {
	list map[string]int
}

func newStubSuspender() *stubSuspender {
	return &stubSuspender{make(map[string]int)}
}

func (s *stubSuspender) Suspended(orch string) int {
	return s.list[orch]
}
