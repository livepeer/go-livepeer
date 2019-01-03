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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubOrchestratorPool struct {
	uris  []*url.URL
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

	return &stubOrchestratorPool{bcast: bcast, uris: uris}
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

func TestNewDBOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	// adding orchestrators to DB
	cachedOrchs, err := cacheDBOrchs(node, orchestrators)
	require.Nil(err)
	assert.Len(cachedOrchs, 3)
	assert.Equal(cachedOrchs[1].ServiceURI, addresses[1])

	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs()
	require.Nil(err)
	assert.Len(orchs, 3)
	assert.Equal(orchs[0].ServiceURI, addresses[0])

	// creating new OrchestratorPoolCache
	dbOrch := NewDBOrchestratorPoolCache(node)
	require.NotNil(dbOrch)
}

func TestNewOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	expectedOffchainOrch := StubOrchestratorPool(addresses)
	assert := assert.New(t)
	require := require.New(t)

	// creating NewOrchestratorPool with orch addresses
	offchainOrch := NewOrchestratorPool(node, addresses)
	for i, uri := range offchainOrch.uris {
		assert.Equal(uri.String(), expectedOffchainOrch.uris[i].String())
	}

	// creating new OrchestratorPool with different first value
	addresses[0] = "https://127.0.0.1:89"
	expectedOffchainOrch = StubOrchestratorPool(addresses)
	assert.NotEqual(offchainOrch.uris[0].String(), expectedOffchainOrch.uris[0].String())

	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	expectedRegisteredTranscoders, err := node.Eth.RegisteredTranscoders()
	require.Nil(err)

	// testing NewOnchainOrchestratorPool
	offchainOrchFromOnchainList := NewOnchainOrchestratorPool(node)
	for i, uri := range offchainOrchFromOnchainList.uris {
		assert.Equal(uri.String(), expectedRegisteredTranscoders[i].ServiceURI)
	}
}
