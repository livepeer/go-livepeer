package discovery

import (
	"math/rand"
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

func TestPoolSize(t *testing.T) {
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}

	assert := assert.New(t)
	pool := NewOrchestratorPool(nil, addresses)
	assert.Equal(3, pool.Size())

	pool = NewOrchestratorPool(nil, nil)
	assert.Equal(0, pool.Size())

}

func TestCacheRegisteredTranscoders_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	err = cacheRegisteredTranscoders(node)
	require.Nil(err)
}

func TestNewDBOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh

	// check size for empty db
	node.Eth = &eth.StubClient{}
	emptyPool := NewDBOrchestratorPoolCache(node)
	require.NotNil(emptyPool)
	assert.Equal(0, emptyPool.Size())

	// adding orchestrators to DB
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

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

	// check size
	assert.Equal(3, dbOrch.Size())
}

func TestNewOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	expected := []string{"https://127.0.0.1:8938", "https://127.0.0.1:8937", "https://127.0.0.1:8936"}
	assert := assert.New(t)

	// creating NewOrchestratorPool with orch addresses
	rand.Seed(321)
	perm = func(len int) []int { return rand.Perm(3) }

	offchainOrch := NewOrchestratorPool(node, addresses)

	for i, uri := range offchainOrch.uris {
		assert.Equal(uri.String(), expected[i])
	}

	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	// testing NewOnchainOrchestratorPool
	rand.Seed(321)
	perm = func(len int) []int { return rand.Perm(3) }
	offchainOrchFromOnchainList := NewOnchainOrchestratorPool(node)
	for i, uri := range offchainOrchFromOnchainList.uris {
		assert.Equal(uri.String(), expected[i])
	}
}
