package discovery

import (
	"context"
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

func TestNewOrchestratorPoolWithPred_NilEthClient_ReturnsNil_LogsError(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	errorLogsBefore := glog.Stats.Error.Lines()
	pool := NewOrchestratorPoolWithPred(node, nil, nil)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)
	assert.Nil(t, pool)
}

func TestNewOnchainOrchestratorPool_NilEthClient_ReturnsNil_LogsError(t *testing.T) {
	node, _ := core.NewLivepeerNode(nil, "", nil)
	errorLogsBefore := glog.Stats.Error.Lines()
	pool := NewOnchainOrchestratorPool(node)
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
	assert := assert.New(t)
	pool := NewOrchestratorPool(nil, addresses)
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

	pool := NewOrchestratorPoolWithPred(node, addresses, pred)
	infos, err := pool.GetOrchestrators(1)

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 1, "Should return one orchestrator")
	assert.Equal("transcoderfromtestserver", infos[0].Transcoder)
}

func TestPoolSize(t *testing.T) {
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}

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
	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)

		assert.Contains(orchs, dbO)
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

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.Eth = &eth.StubClient{Orchestrators: orchestrators}

	pool := NewOrchestratorPoolWithPred(node, addresses, pred)

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
	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)
		dbO.PricePerPixel, _ = common.PriceToFixed(big.NewRat(999, 1))

		assert.Contains(cachedOrchs, dbO)
	}

	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)

		assert.Contains(orchs, dbO)
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
	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)
		dbO.PricePerPixel, _ = common.PriceToFixed(big.NewRat(999, 1))

		assert.Contains(cachedOrchs, dbO)
	}

	// ensuring orchs exist in DB
	orchs, err := node.Database.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)

		assert.Contains(orchs, dbO)
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

	for _, o := range orchestrators {
		dbO := ethOrchToDBOrch(o)

		assert.Contains(orchs, dbO)
	}
	orchs, err = node.Database.SelectOrchs(&common.DBOrchFilter{server.BroadcastCfg.MaxPrice()})
	require.Nil(err)
	assert.Len(orchs, 25)
	for _, o := range orchestrators[25:] {
		dbO := ethOrchToDBOrch(o)

		assert.Contains(orchs, dbO)
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
