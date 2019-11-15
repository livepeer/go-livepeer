package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"math/rand"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stubOrchestratorPool struct {
	uris  []*url.URL
	bcast server.Broadcaster
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
		transc := &lpTypes.Transcoder{ServiceURI: addr, Address: address}
		orchestrators = append(orchestrators, transc)
	}

	return orchestrators
}

func TestNewOrchestratorPoolWithPred_NilEthClient_ReturnsNil_LogsError(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	errorLogsBefore := glog.Stats.Error.Lines()
	pool := NewOrchestratorPoolWithPred(node, nil, nil)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)
	assert.Nil(t, pool)
}

func TestNewDBOrchestratorPoolCache_NilEthClient_ReturnsNil_LogsError(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	errorLogsBefore := glog.Stats.Error.Lines()
	poolCache := NewDBOrchestratorPoolCache(node)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)
	assert.Nil(t, poolCache)
}

func TestDeadLock(t *testing.T) {
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{Transcoder: "transcoderfromtestserver"}, nil
	}
	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:8936")
	}
	uris := stringsToURIs(addresses)
	assert := assert.New(t)
	pool := NewOrchestratorPool(nil, uris)
	infos, err := pool.GetOrchestrators(1)
	assert.Nil(err, "Should not be error")
	assert.Len(infos, 1, "Should return one orchestrator")
	assert.Equal("transcoderfromtestserver", infos[0].Transcoder)
}

func TestDeadLock_NewOrchestratorPoolWithPred(t *testing.T) {
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Transcoder: "transcoderfromtestserver",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  5,
				PixelsPerUnit: 1,
			},
		}, nil
	}
	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:8936")
	}
	uris := stringsToURIs(addresses)

	assert := assert.New(t)
	pred := func(info *net.OrchestratorInfo) bool {
		price := server.BroadcastCfg.MaxPrice()
		if price != nil {
			return big.NewRat(info.PriceInfo.PricePerUnit, info.PriceInfo.PixelsPerUnit).Cmp(price) <= 0
		}
		return true
	}

	orchestrators := StubOrchestrators(addresses)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	pool := NewOrchestratorPoolWithPred(node, uris, pred)
	infos, err := pool.GetOrchestrators(1)

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 1, "Should return one orchestrator")
	assert.Equal("transcoderfromtestserver", infos[0].Transcoder)
}

func TestPoolSize(t *testing.T) {
	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})

	assert := assert.New(t)
	pool := NewOrchestratorPool(nil, addresses)
	assert.Equal(3, pool.Size())

	// will results in len(uris) <= 0 -> log Error
	errorLogsBefore := glog.Stats.Error.Lines()
	pool = NewOrchestratorPool(nil, nil)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(0, pool.Size())
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)
}

func TestDbOrchestratorPoolCacheSize(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{}

	errorLogsBefore := glog.Stats.Error.Lines()
	// No orchestrators on eth.Stubclient will result in no registered orchs found on chain
	emptyPool := NewDBOrchestratorPoolCache(node)
	errorLogsAfter := glog.Stats.Error.Lines()
	require.NotNil(emptyPool)
	assert.Equal(t, 0, emptyPool.Size())
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)

	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}
	nonEmptyPool := NewDBOrchestratorPoolCache(node)
	require.NotNil(nonEmptyPool)
	assert.Equal(t, len(addresses), emptyPool.Size())
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
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Transcoder: "transcoderfromtestserver",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  999,
				PixelsPerUnit: 1,
			},
		}, nil
	}

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh

	// adding orchestrators to DB
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	cachedOrchs, err := cacheDBOrchs(node, orchestrators)
	require.Nil(err)
	assert.Len(cachedOrchs, 3)
	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)
		dbO.PricePerPixel, _ = common.PriceToFixed(big.NewRat(999, 1))

		assert.Contains(cachedOrchs, dbO)
	}

	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 3)
	orchsTest := make([]orchTest, 1)
	for _, o := range orchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}

	// creating new OrchestratorPoolCache
	dbOrch := NewDBOrchestratorPoolCache(node)
	require.NotNil(dbOrch)

	// check size
	assert.Equal(3, dbOrch.Size())

	urls := dbOrch.GetURLs()
	assert.Len(urls, 3)
}

func TestNewDBOrchestratorPoolCache_TestURLs(t *testing.T) {
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Transcoder: "transcoderfromtestserver",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  999,
				PixelsPerUnit: 1,
			},
		}, nil
	}

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh

	addresses := []string{"badUrl\\://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}
	dbOrch := NewDBOrchestratorPoolCache(node)
	require.NotNil(dbOrch)
	// Not inserting bad URLs in database anymore, as there is no returnable query for getting their priceInfo
	// And if URL is updated it won't be picked up until next cache update
	assert.Equal(2, dbOrch.Size())
	urls := dbOrch.GetURLs()
	assert.Len(urls, 2)
}

func TestNewDBOrchestratorPoolCache_TestURLs_Empty(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh

	addresses := []string{}
	// Addresses is empty slice -> No orchestrators
	orchestrators := StubOrchestrators(addresses)
	// Contains empty orchestrator slice -> No registered orchs found on chain error
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}
	errorLogsBefore := glog.Stats.Error.Lines()
	dbOrch := NewDBOrchestratorPoolCache(node)
	errorLogsAfter := glog.Stats.Error.Lines()
	require.NotNil(dbOrch)
	assert.Equal(0, dbOrch.Size())
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)
	urls := dbOrch.GetURLs()
	assert.Len(urls, 0)
}

func TestNewOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})
	expected := stringsToURIs([]string{"https://127.0.0.1:8938", "https://127.0.0.1:8937", "https://127.0.0.1:8936"})
	assert := assert.New(t)

	// creating NewOrchestratorPool with orch addresses
	rand.Seed(321)
	perm = func(len int) []int { return rand.Perm(3) }

	offchainOrch := NewOrchestratorPool(node, addresses)

	for i, uri := range offchainOrch.uris {
		assert.Equal(uri, expected[i])
	}
}

func TestNewOrchestratorPoolWithPred_TestPredicate(t *testing.T) {
	pred := func(info *net.OrchestratorInfo) bool {
		price := server.BroadcastCfg.MaxPrice()
		if price != nil {
			return big.NewRat(info.PriceInfo.PricePerUnit, info.PriceInfo.PixelsPerUnit).Cmp(price) <= 0
		}
		return true
	}

	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:8936")
	}
	orchestrators := StubOrchestrators(addresses)
	uris := stringsToURIs(addresses)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	pool := NewOrchestratorPoolWithPred(node, uris, pred)

	oInfo := &net.OrchestratorInfo{
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  5,
			PixelsPerUnit: 1,
		},
	}

	// server.BroadcastCfg.maxPrice not yet set, predicate should return true
	assert.True(t, pool.pred(oInfo))

	// Set server.BroadcastCfg.maxPrice higher than PriceInfo , should return true
	server.BroadcastCfg.SetMaxPrice(big.NewRat(10, 1))
	assert.True(t, pool.pred(oInfo))

	// Set MaxBroadcastPrice lower than PriceInfo, should return false
	server.BroadcastCfg.SetMaxPrice(big.NewRat(1, 1))
	assert.False(t, pool.pred(oInfo))
}

func TestCachedPool_AllOrchestratorsTooExpensive_ReturnsEmptyList(t *testing.T) {
	// Test setup
	rand.Seed(321)
	perm = func(len int) []int { return rand.Perm(50) }

	server.BroadcastCfg.SetMaxPrice(big.NewRat(10, 1))
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Transcoder: "transcoderfromtestserver",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  999,
				PixelsPerUnit: 1,
			},
		}, nil
	}
	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:"+strconv.Itoa(8936+i))
	}

	assert := assert.New(t)

	// Create Database
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	orchestrators := StubOrchestrators(addresses)

	// Create node
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	// Add orchs to DB
	cachedOrchs, err := cacheDBOrchs(node, orchestrators)
	require.Nil(err)
	assert.Len(cachedOrchs, 50)
	orchsTest := make([]orchTest, 1)
	for _, o := range cachedOrchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}

	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	orchsTest = make([]orchTest, 1)
	for _, o := range orchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}

	// creating new OrchestratorPoolCache
	dbOrch := NewDBOrchestratorPoolCache(node)
	require.NotNil(dbOrch)

	// check size
	assert.Equal(0, dbOrch.Size())

	urls := dbOrch.GetURLs()
	assert.Len(urls, 0)
	infos, err := dbOrch.GetOrchestrators(len(addresses))

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 0)
}

func TestCachedPool_GetOrchestrators_MaxBroadcastPriceNotSet(t *testing.T) {
	// Test setup
	server.BroadcastCfg.SetMaxPrice(nil)
	perm = func(len int) []int { return rand.Perm(50) }

	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Transcoder: "transcoderfromtestserver",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  999,
				PixelsPerUnit: 1,
			},
		}, nil
	}
	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:"+strconv.Itoa(8936+i))
	}

	assert := assert.New(t)

	// Create Database
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	orchestrators := StubOrchestrators(addresses)

	// Create node
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	// Add orchs to DB
	cachedOrchs, err := cacheDBOrchs(node, orchestrators)
	require.Nil(err)
	assert.Len(cachedOrchs, 50)
	orchsTest := make([]orchTest, 1)
	for _, o := range cachedOrchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}

	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	orchsTest = make([]orchTest, 1)
	for _, o := range orchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}

	// creating new OrchestratorPoolCache
	dbOrch := NewDBOrchestratorPoolCache(node)
	require.NotNil(dbOrch)

	// check size
	assert.Equal(50, dbOrch.Size())

	urls := dbOrch.GetURLs()
	assert.Len(urls, 50)
	infos, err := dbOrch.GetOrchestrators(50)

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 50)
}

func TestCachedPool_N_OrchestratorsGoodPricing_ReturnsNOrchestrators(t *testing.T) {
	// Test setup
	perm = func(len int) []int { return rand.Perm(25) }

	server.BroadcastCfg.SetMaxPrice(big.NewRat(10, 1))
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		if i, _ := strconv.Atoi(orchestratorServer.Port()); i > 8960 {
			// Return valid pricing
			return &net.OrchestratorInfo{
				Transcoder: "goodPriceTranscoder",
				PriceInfo: &net.PriceInfo{
					PricePerUnit:  1,
					PixelsPerUnit: 1,
				},
			}, nil
		}
		// Return invalid pricing
		return &net.OrchestratorInfo{
			Transcoder: "badPriceTranscoder",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  999,
				PixelsPerUnit: 1,
			},
		}, nil
	}
	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:"+strconv.Itoa(8936+i))
	}

	assert := assert.New(t)

	// Create Database
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	orchestrators := StubOrchestrators(addresses)

	// Create node
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	// Add orchs to DB
	cachedOrchs, err := cacheDBOrchs(node, orchestrators)
	require.Nil(err)
	assert.Len(cachedOrchs, 50)
	for _, o := range orchestrators[:25] {
		dbO := ethOrchToDBOrch(o)
		dbO.PricePerPixel, _ = common.PriceToFixed(big.NewRat(999, 1))
		assert.Contains(cachedOrchs, dbO)
	}
	for _, o := range orchestrators[25:] {
		dbO := ethOrchToDBOrch(o)
		dbO.PricePerPixel, _ = common.PriceToFixed(big.NewRat(1, 1))
		assert.Contains(cachedOrchs, dbO)
	}
	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)

	orchsTest := make([]orchTest, 1)
	for _, o := range orchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}
	orchs, err = node.Database.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	require.Nil(err)
	assert.Len(orchs, 25)
	orchsTest = make([]orchTest, 1)
	for _, o := range orchs {
		orchsTest = append(orchsTest, orchTest{EthereumAddr: o.EthereumAddr, ServiceURI: o.ServiceURI})
	}
	for _, o := range orchestrators[25:] {
		dbO := toOrchTest(o.Address.String(), o.ServiceURI)

		assert.Contains(orchsTest, dbO)
	}

	// creating new OrchestratorPoolCache
	dbOrch := NewDBOrchestratorPoolCache(node)
	require.NotNil(dbOrch)

	// check size
	assert.Equal(25, dbOrch.Size())

	urls := dbOrch.GetURLs()
	assert.Len(urls, 25)
	infos, err := dbOrch.GetOrchestrators(len(orchestrators))

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 25)
	for _, info := range infos {
		assert.Equal(info.Transcoder, "goodPriceTranscoder")
	}
}

func TestCachedPool_GetOrchestrators_TicketParamsValidation(t *testing.T) {
	// Test setup
	perm = func(len int) []int { return rand.Perm(50) }

	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)

	server.BroadcastCfg.SetMaxPrice(nil)

	serverGetOrchInfo = func(ctx context.Context, bcast server.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return &net.OrchestratorInfo{
			Transcoder:   "transcoder",
			TicketParams: &net.TicketParams{},
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  999,
				PixelsPerUnit: 1,
			},
		}, nil
	}

	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:"+strconv.Itoa(8936+i))
	}

	assert := assert.New(t)
	require := require.New(t)

	// Create Database
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require.Nil(err)

	orchestrators := StubOrchestrators(addresses)

	// Create node
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Database = dbh
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}
	sender := &pm.MockSender{}
	node.Sender = sender

	// Add orchs to DB
	cachedOrchs, err := cacheDBOrchs(node, orchestrators)
	require.Nil(err)
	assert.Len(cachedOrchs, 50)

	dbOrch := NewDBOrchestratorPoolCache(node)

	// Test 25 out of 50 orchs pass ticket params validation
	sender.On("ValidateTicketParams", mock.Anything).Return(errors.New("ValidateTicketParams error")).Times(25)
	sender.On("ValidateTicketParams", mock.Anything).Return(nil).Times(25)

	infos, err := dbOrch.GetOrchestrators(len(addresses))
	assert.Nil(err)
	assert.Len(infos, 25)
	sender.AssertNumberOfCalls(t, "ValidateTicketParams", 50)

	// Test 0 out of 50 orchs pass ticket params validation
	sender = &pm.MockSender{}
	node.Sender = sender
	sender.On("ValidateTicketParams", mock.Anything).Return(errors.New("ValidateTicketParams error")).Times(50)

	infos, err = dbOrch.GetOrchestrators(len(addresses))
	assert.Nil(err)
	assert.Len(infos, 0)
	sender.AssertNumberOfCalls(t, "ValidateTicketParams", 50)
}

func TestNewWHOrchestratorPoolCache(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// mock webhook and orchestrator info request
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}

	getURLsfromWebhook = func(cbUrl *url.URL) ([]byte, error) {
		var wh []webhookResponse
		for _, addr := range addresses {
			wh = append(wh, webhookResponse{Address: addr})
		}
		return json.Marshal(&wh)
	}

	serverGetOrchInfo = func(c context.Context, b server.Broadcaster, s *url.URL) (*net.OrchestratorInfo, error) {
		return &net.OrchestratorInfo{Transcoder: "transcoder"}, nil
	}

	// mock livepeer node
	node, _ := core.NewLivepeerNode(nil, "", nil)
	perm = func(len int) []int { return rand.Perm(3) }

	// assert created webhook pool is correct length
	whURL, _ := url.ParseRequestURI("https://livepeer.live/api/orchestrator")
	whpool := NewWebhookPool(node, whURL)
	assert.Equal(3, whpool.Size())

	// assert that list is not refreshed if lastRequest is less than 1 min ago and hash is the same
	lastReq := whpool.lastRequest
	orchInfo, err := whpool.GetOrchestrators(2)
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())

	urls := whpool.pool.GetURLs()
	assert.Len(urls, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.Contains(urls, uri)
	}

	//  assert that list is not refreshed if lastRequest is more than 1 min ago and hash is the same
	lastReq = time.Now().Add(-2 * time.Minute)
	whpool.lastRequest = lastReq
	orchInfo, err = whpool.GetOrchestrators(2)
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())
	assert.NotEqual(lastReq, whpool.lastRequest)

	urls = whpool.pool.GetURLs()
	assert.Len(urls, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.Contains(urls, uri)
	}

	// mock a change in webhook addresses
	addresses = []string{"https://127.0.0.1:8932", "https://127.0.0.1:8933", "https://127.0.0.1:8934"}

	//  assert that list is not refreshed if lastRequest is less than 1 min ago and hash is not the same
	lastReq = time.Now()
	whpool.lastRequest = lastReq
	orchInfo, err = whpool.GetOrchestrators(2)
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())
	assert.Equal(lastReq, whpool.lastRequest)

	urls = whpool.pool.GetURLs()
	assert.Len(urls, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.NotContains(urls, uri)
	}

	//  assert that list is refreshed if lastRequest is longer than 1 min ago and hash is not the same
	lastReq = time.Now().Add(-2 * time.Minute)
	whpool.lastRequest = lastReq
	orchInfo, err = whpool.GetOrchestrators(2)
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())
	assert.NotEqual(lastReq, whpool.lastRequest)

	urls = whpool.pool.GetURLs()
	assert.Len(urls, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.Contains(urls, uri)
	}
}

func TestDeserializeWebhookJSON(t *testing.T) {
	assert := assert.New(t)

	// assert input of webhookResponse address object returns correct address
	resp, _ := json.Marshal(&[]webhookResponse{webhookResponse{Address: "https://127.0.0.1:8936"}})
	urls, err := deserializeWebhookJSON(resp)
	assert.Nil(err)
	assert.Equal("https://127.0.0.1:8936", urls[0].String())

	// assert input of empty byte array returns JSON error
	urls, err = deserializeWebhookJSON([]byte{})
	assert.Contains(err.Error(), "unexpected end of JSON input")
	assert.Nil(urls)

	// assert input of empty byte array returns empty object
	resp, _ = json.Marshal(&[]webhookResponse{webhookResponse{}})
	urls, err = deserializeWebhookJSON(resp)
	assert.Nil(err)
	assert.Empty(urls)

	// assert input of invalid addresses returns invalid JSON error
	urls, err = deserializeWebhookJSON(make([]byte, 64))
	assert.Contains(err.Error(), "invalid character")
	assert.Empty(urls)

	// assert input of invalid JSON returns JSON unmarshal object error
	urls, err = deserializeWebhookJSON([]byte(`{"name":false}`))
	assert.Contains(err.Error(), "cannot unmarshal object")
	assert.Empty(urls)

	// assert input of invalid JSON returns JSON unmarshal number error
	urls, err = deserializeWebhookJSON([]byte(`1112`))
	assert.Contains(err.Error(), "cannot unmarshal number")
	assert.Empty(urls)
}

type orchTest struct {
	EthereumAddr string
	ServiceURI   string
}

func toOrchTest(addr, serviceURI string) orchTest {
	return orchTest{EthereumAddr: addr, ServiceURI: serviceURI}
}
