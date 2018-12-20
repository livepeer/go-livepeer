package discovery

import (
	"net/url"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/server"
)

type stubOrchestratorPool struct {
	uri   []*url.URL
	bcast server.Broadcaster
}

func StubOrchestratorPool(addresses []string) *stubOrchestratorPool {
	var uris []*url.URL

	for _, addr := range addresses {
		uri, err := url.ParseRequestURI(addr)
		if err == nil {
			uris = append(uris, uri)
		}
	}
	node, _ := core.NewLivepeerNode(nil, "", nil)
	bcast := core.NewBroadcaster(node)

	return &stubOrchestratorPool{bcast: bcast, uri: uris}
}

func StubOrchestrators(addresses []string) []*lpTypes.Transcoder {
	var orchestrators []*lpTypes.Transcoder

	for _, addr := range addresses {
		address := ethcommon.BytesToAddress([]byte(addr))
		transc := &lpTypes.Transcoder{ServiceURI: addr, Address: address}
		orchestrators = append(orchestrators, transc)
	}

	return orchestrators
}

func TestNewDBOrchestratorPoolCache(t *testing.T) {
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)

	dbh, dbraw, err := common.TempDB(t)
	if err != nil {
		return
	}
	defer dbh.Close()
	defer dbraw.Close()
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	cachedOrchs, err := CacheDBOrchs(node, orchestrators)
	if err != nil || len(cachedOrchs) != 3 || cachedOrchs[1].ServiceURI != addresses[1] {
		t.Error("Unexpected error in cachedOrchs: ", err)
	}

	orchs, err := node.Database.SelectOrchs()
	if err != nil || len(orchs) != 3 || orchs[0].ServiceURI != addresses[0] {
		t.Error("Unexpected error retrieving saved orchs: ", err)
	}

	dbOrch := NewDBOrchestratorPoolCache(node)
	if dbOrch == nil {
		t.Error("Did not create NewDBOrchestratorPoolCache ")
	}
}

func TestNewOrchestratorPool(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	expectedOffchainOrch := StubOrchestratorPool(addresses)

	offchainOrch := NewOrchestratorPool(node, addresses)

	for i, uri := range offchainOrch.uri {
		if uri.String() != expectedOffchainOrch.uri[i].String() {
			t.Error("Uri(s) in NewOrchestratorPool do not match expected values")
		}
	}

	addresses[0] = "https://127.0.0.1:89"
	expectedOffchainOrch = StubOrchestratorPool(addresses)

	if offchainOrch.uri[0].String() == expectedOffchainOrch.uri[0].String() {
		t.Error("Uri string from NewOrchestratorPool not expected to match expectedOffchainOrch")
	}

	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	expectedRegisteredTranscoders, err := node.Eth.RegisteredTranscoders()
	if err != nil {
		t.Error("Unable to get expectedRegisteredTranscoders")
	}

	offchainOrchFromOnchainList := NewOnchainOrchestratorPool(node)
	for i, uri := range offchainOrchFromOnchainList.uri {
		if uri.String() != expectedRegisteredTranscoders[i].ServiceURI {
			t.Error("Uri(s) in NewOrchestratorPoolFromOnchainList do not match expected values")
		}
	}
}
