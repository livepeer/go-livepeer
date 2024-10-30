package discovery

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
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
	"go.uber.org/goleak"
)

func TestNewDBOrchestratorPoolCache_NilEthClient_ReturnsError(t *testing.T) {
	assert := assert.New(t)
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)
	node := &core.LivepeerNode{
		Database: dbh,
		Eth:      nil,
		Sender:   &pm.MockSender{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	assert.Nil(pool)
	assert.EqualError(err, "could not create DBOrchestratorPoolCache: LivepeerEthClient is nil")
}

func TestDeadLock(t *testing.T) {
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	first := true
	oldOrchInfo := serverGetOrchInfo
	defer func() { wg.Wait(); serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		defer wg.Done()
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
	wg.Add(len(uris))
	pool := NewOrchestratorPool(nil, uris, common.Score_Trusted, []string{}, 50*time.Millisecond)
	infos, err := pool.GetOrchestrators(context.TODO(), 1, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	assert.Nil(err, "Should not be error")
	assert.Len(infos, 1, "Should return one orchestrator")
	assert.Equal("transcoderfromtestserver", infos[0].RemoteInfo.Transcoder)
}

func TestDeadLock_NewOrchestratorPoolWithPred(t *testing.T) {
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	first := true
	oldOrchInfo := serverGetOrchInfo
	defer func() { wg.Wait(); serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		defer wg.Done()
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

	wg.Add(len(uris))
	pool := NewOrchestratorPoolWithPred(nil, uris, pred, common.Score_Trusted, []string{}, 50*time.Millisecond)
	infos, err := pool.GetOrchestrators(context.TODO(), 1, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 1, "Should return one orchestrator")
	assert.Equal("transcoderfromtestserver", infos[0].RemoteInfo.Transcoder)
}

func TestPoolSize(t *testing.T) {
	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})

	assert := assert.New(t)
	pool := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{}, 50*time.Millisecond)
	assert.Equal(3, pool.Size())

	// will results in len(uris) <= 0 -> log Error
	errorLogsBefore := glog.Stats.Error.Lines()
	pool = NewOrchestratorPool(nil, nil, common.Score_Trusted, []string{}, 50*time.Millisecond)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(0, pool.Size())
	assert.NotZero(t, errorLogsAfter-errorLogsBefore)
}

func TestDBOrchestratorPoolCacheSize(t *testing.T) {
	assert := assert.New(t)
	dbh, dbraw, err := common.TempDB(t)
	require := require.New(t)
	require.Nil(err)

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth:      &eth.StubClient{TotalStake: big.NewInt(0)},
		Sender:   sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		dbh.Close()
		dbraw.Close()
		goleak.VerifyNone(t, common.IgnoreRoutines()...)
	}()

	emptyPool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)
	require.NotNil(emptyPool)
	assert.Equal(0, emptyPool.Size())

	addresses := []string{"stub1", "stub2", "stub3"}
	orchestrators := StubOrchestrators(addresses)
	for _, o := range orchestrators {
		dbh.UpdateOrch(ethOrchToDBOrch(o))
	}

	nonEmptyPool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)
	require.NotNil(nonEmptyPool)
	assert.Equal(len(addresses), nonEmptyPool.Size())
}

func TestNewDBOrchestorPoolCache_NoEthAddress(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	oldServerGetOrchInfo := serverGetOrchInfo
	defer func() { serverGetOrchInfo = oldServerGetOrchInfo }()
	var mu sync.Mutex
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		defer mu.Unlock()

		return &net.OrchestratorInfo{
			Transcoder: "transcoder",
			PriceInfo: &net.PriceInfo{
				PixelsPerUnit: 1000,
				PricePerUnit:  5000,
			},
		}, nil
	}

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require.Nil(err)

	// Populate test DB with a stub orch so that serverGetOrchInfo will be called in cacheDBOrchs
	orch := common.NewDBOrch("foo", "foo", 1, 0, 1000000000000, 100)
	require.Nil(dbh.UpdateOrch(orch))

	node := &core.LivepeerNode{
		Database: dbh,
		Eth:      &eth.StubClient{TotalStake: big.NewInt(0)},
	}
	// LastInitializedRound() will ensure the stub orch is always returned from the DB
	rm := &stubRoundsManager{round: big.NewInt(100)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := NewDBOrchestratorPoolCache(ctx, node, rm, []string{}, 500*time.Millisecond)
	require.Nil(err)

	// Check that serverGetOrchInfo returns early and the orchestrator isn't updated
	assert.Nil(pool.cacheDBOrchs())
	orchs, err := dbh.SelectOrchs(&common.DBOrchFilter{})
	assert.Nil(err)
	assert.Len(orchs, 1)
	assert.Equal(orchs[0].PricePerPixel, int64(1))
	falsePrice, _ := common.PriceToFixed(big.NewRat(5000, 1000))
	assert.NotEqual(orchs[0].PricePerPixel, falsePrice)
	assert.Equal(orchs[0].ServiceURI, "foo")
}

func TestNewDBOrchestratorPoolCache_InvalidPrices(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	priceInfo := &net.PriceInfo{
		PricePerUnit:  0,
		PixelsPerUnit: 0,
	}

	oldServerGetOrchInfo := serverGetOrchInfo
	defer func() { serverGetOrchInfo = oldServerGetOrchInfo }()
	var mu sync.Mutex
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		defer mu.Unlock()

		return &net.OrchestratorInfo{
			Transcoder: "transcoder",
			PriceInfo:  priceInfo,
		}, nil
	}

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require.Nil(err)

	// Populate test DB with a stub orch so that serverGetOrchInfo will be called in cacheDBOrchs
	orch := common.NewDBOrch("foo", "foo", 0, 0, 1000000000000, 100)
	require.Nil(dbh.UpdateOrch(orch))

	node := &core.LivepeerNode{
		Database: dbh,
		Eth:      &eth.StubClient{TotalStake: big.NewInt(0)},
	}
	// LastInitializedRound() will ensure the stub orch is always returned from the DB
	rm := &stubRoundsManager{round: big.NewInt(100)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := NewDBOrchestratorPoolCache(ctx, node, rm, []string{}, 500*time.Millisecond)
	require.Nil(err)

	// priceInfo.PixelsPerUnit = 0
	// Check that this does not trigger a division by zero
	assert.Nil(pool.cacheDBOrchs())

	// priceInfo = nil
	// Check that this does not trigger a nil pointer error
	priceInfo = nil
	assert.Nil(pool.cacheDBOrchs())
}

func TestNewDBOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	expPriceInfo := &net.PriceInfo{
		PricePerUnit:  999,
		PixelsPerUnit: 1,
	}
	expTranscoder := "transcoderFromTest"
	expPricePerPixel, _ := common.PriceToFixed(big.NewRat(999, 1))
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Address:    pm.RandBytes(20),
			Transcoder: expTranscoder,
			PriceInfo:  expPriceInfo,
		}, nil
	}

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	// adding orchestrators to DB
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)

	testOrchs := make([]orchTest, 0)
	for _, o := range orchestrators {
		to := orchTest{
			EthereumAddr:  o.Address.String(),
			ServiceURI:    o.ServiceURI,
			PricePerPixel: expPricePerPixel,
		}
		testOrchs = append(testOrchs, to)
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
			TotalStake:    new(big.Int).Mul(big.NewInt(5000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender.On("ValidateTicketParams", mock.Anything).Return(nil).Times(3)

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)
	assert.Equal(pool.Size(), 3)
	orchs, err := pool.GetOrchestrators(context.TODO(), pool.Size(), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	for _, o := range orchs {
		assert.Equal(o.RemoteInfo.PriceInfo, expPriceInfo)
		assert.Equal(o.RemoteInfo.Transcoder, expTranscoder)
	}

	// ensuring orchs exist in DB
	dbOrchs, err := pool.store.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(dbOrchs, 3)
	for _, o := range dbOrchs {
		test := toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel)
		assert.Contains(testOrchs, test)
		assert.Equal(o.Stake, int64(500000000))
	}

	infos := pool.GetInfos()
	assert.Len(infos, 3)
	for _, info := range infos {
		assert.Contains(addresses, info.URL.String())
	}
}

func TestNewDBOrchestratorPoolCache_TestURLs(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	addresses := []string{"badUrl\\://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)
	orchs := make([]*common.DBOrch, 0)
	for _, o := range orchestrators {
		orchs = append(orchs, ethOrchToDBOrch(o))
	}

	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Transcoder: "transcoderfromtestserver",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
		}, nil
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)
	// bad URLs are inserted in the database but are not included in the working set, as there is no returnable query for getting their priceInfo
	// And if URL is updated it won't be picked up until next cache update
	assert.Equal(3, pool.Size())
	infos := pool.GetInfos()
	assert.Len(infos, 2)
}

func TestNewDBOrchestratorPoolCache_TestURLs_Empty(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	addresses := []string{}
	// Addresses is empty slice -> No orchestrators
	orchestrators := StubOrchestrators(addresses)
	for _, o := range orchestrators {
		dbh.UpdateOrch(ethOrchToDBOrch(o))
	}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: &pm.MockSender{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)
	assert.Equal(0, pool.Size())
	infos := pool.GetInfos()
	assert.Len(infos, 0)
}

func TestNewDBOrchestorPoolCache_PollOrchestratorInfo(t *testing.T) {
	addr := pm.RandBytes(20)
	cachedOrchInfo := &net.OrchestratorInfo{
		Address:    addr,
		Transcoder: "transcoderFromTest",
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  999,
			PixelsPerUnit: 1,
		},
	}
	polledOrchInfo := &net.OrchestratorInfo{
		Address:    addr,
		Transcoder: "transcoderFromTest",
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
	}
	returnInfo := cachedOrchInfo

	var mu sync.Mutex
	callCount := 0
	first := true
	wg := sync.WaitGroup{}
	oldOrchInfo := serverGetOrchInfo
	defer func() { wg.Wait(); serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		defer mu.Unlock()
		// slightly unsafe to be adding to the wg counter here
		// but we invoke this function rather often / unpredictably due to low
		// cache interval, so we can't reliably set the counter outside the fn
		wg.Add(1)
		defer wg.Done()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		callCount++
		return returnInfo, nil
	}

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	// adding orchestrators to DB
	addresses := []string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"}
	orchestrators := StubOrchestrators(addresses)

	testOrchs := make([]orchTest, 0)
	expPrice, _ := common.PriceToFixed(big.NewRat(999, 1))
	for _, o := range orchestrators {
		to := orchTest{
			EthereumAddr:  o.Address.String(),
			ServiceURI:    o.ServiceURI,
			PricePerPixel: expPrice,
		}
		testOrchs = append(testOrchs, to)
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	origCacheRefreshInterval := cacheRefreshInterval
	cacheRefreshInterval = 200 * time.Millisecond
	defer func() { cacheRefreshInterval = origCacheRefreshInterval }()
	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)

	// Ensure orchestrators exist in DB
	require.Equal(pool.Size(), 3)
	dbOrchs, err := pool.store.SelectOrchs(nil)
	require.Nil(err)
	require.Len(dbOrchs, 3)
	for _, o := range dbOrchs {
		test := toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel)
		require.Contains(testOrchs, test)
	}

	// reset callCount to 0 which is now 3 after calling CacheTranscoderPool()
	mu.Lock()
	callCount = 0
	returnInfo = polledOrchInfo
	mu.Unlock()
	time.Sleep(1100 * time.Millisecond)
	dbOrchs, err = pool.store.SelectOrchs(nil)
	require.Nil(err)
	require.Len(dbOrchs, 3)
	expPrice, _ = common.PriceToFixed(big.NewRat(1, 1))
	for _, o := range dbOrchs {
		assert.Equal(o.PricePerPixel, expPrice)
	}
	// called serverGetOrchInfo 1100 / 200 * 3 = 15 times
	mu.Lock()
	assert.GreaterOrEqual(callCount, 14)
	assert.LessOrEqual(callCount, 16)
	mu.Unlock()
}

func TestNewOrchestratorPoolCache_GivenListOfOrchs_CreatesPoolCacheCorrectly(t *testing.T) {
	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})
	assert := assert.New(t)

	// creating NewOrchestratorPool with orch addresses
	offchainOrch := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{}, 50*time.Millisecond)

	for i, info := range offchainOrch.infos {
		assert.Equal(info.URL.String(), addresses[i].String())
	}
}

func TestNewOrchestratorPoolWithPred_TestPredicate(t *testing.T) {
	pred := func(info *net.OrchestratorInfo) bool {
		price := server.BroadcastCfg.MaxPrice()
		if price == nil {
			return true
		}
		ratPriceInfo, err := common.RatPriceInfo(info.PriceInfo)
		if err != nil {
			return false
		}
		return ratPriceInfo.Cmp(price) <= 0
	}

	addresses := []string{}
	for i := 0; i < 50; i++ {
		addresses = append(addresses, "https://127.0.0.1:8936")
	}
	uris := stringsToURIs(addresses)

	pool := NewOrchestratorPoolWithPred(nil, uris, pred, common.Score_Trusted, []string{}, 50*time.Millisecond)

	oInfo := &net.OrchestratorInfo{
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  5,
			PixelsPerUnit: 1,
		},
	}

	// server.BroadcastCfg.maxPrice not yet set, predicate should return true
	assert.True(t, pool.pred(oInfo))

	// Set server.BroadcastCfg.maxPrice higher than PriceInfo , should return true
	server.BroadcastCfg.SetMaxPrice(core.NewFixedPrice(big.NewRat(10, 1)))
	assert.True(t, pool.pred(oInfo))

	// Set MaxBroadcastPrice lower than PriceInfo, should return false
	server.BroadcastCfg.SetMaxPrice(core.NewFixedPrice(big.NewRat(1, 1)))
	assert.False(t, pool.pred(oInfo))

	// PixelsPerUnit is 0 , return false
	oInfo.PriceInfo.PixelsPerUnit = 0
	assert.False(t, pool.pred(oInfo))
}

func TestCachedPool_AllOrchestratorsTooExpensive_ReturnsAllOrchestrators(t *testing.T) {
	// Test setup
	expPriceInfo := &net.PriceInfo{
		PricePerUnit:  999,
		PixelsPerUnit: 1,
	}
	expTranscoder := "transcoderFromTest"
	expPricePerPixel, _ := common.PriceToFixed(big.NewRat(999, 1))

	server.BroadcastCfg.SetMaxPrice(core.NewFixedPrice(big.NewRat(1, 1)))
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Address:    pm.RandBytes(20),
			Transcoder: expTranscoder,
			PriceInfo:  expPriceInfo,
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
	testOrchs := make([]orchTest, 0)
	for _, o := range orchestrators {
		to := orchTest{
			EthereumAddr:  o.Address.String(),
			ServiceURI:    o.ServiceURI,
			PricePerPixel: expPricePerPixel,
		}
		testOrchs = append(testOrchs, to)
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)

	// ensuring orchs exist in DB
	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	for _, o := range orchs {
		test := toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel)
		assert.Contains(testOrchs, test)
	}

	// check size
	assert.Equal(50, pool.Size())

	urls := pool.GetInfos()
	assert.Len(urls, 50)

	infos, err := pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 50)
}

func TestCachedPool_GetOrchestrators_MaxBroadcastPriceNotSet(t *testing.T) {
	// Test setup
	expPriceInfo := &net.PriceInfo{
		PricePerUnit:  999,
		PixelsPerUnit: 1,
	}
	expTranscoder := "transcoderFromTest"
	expPricePerPixel, _ := common.PriceToFixed(big.NewRat(999, 1))

	server.BroadcastCfg.SetMaxPrice(nil)
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Address:    pm.RandBytes(20),
			Transcoder: expTranscoder,
			PriceInfo:  expPriceInfo,
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
	testOrchs := make([]orchTest, 0)
	for _, o := range orchestrators {
		to := orchTest{
			EthereumAddr:  o.Address.String(),
			ServiceURI:    o.ServiceURI,
			PricePerPixel: expPricePerPixel,
		}
		testOrchs = append(testOrchs, to)
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)

	// ensuring orchs exist in DB
	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	for _, o := range orchs {
		test := toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel)
		assert.Contains(testOrchs, test)
	}

	// check size
	assert.Equal(50, pool.Size())

	infos := pool.GetInfos()
	assert.Len(infos, 50)
	for _, info := range infos {
		assert.Contains(addresses, info.URL.String())
	}
	oinfos, err := pool.GetOrchestrators(context.TODO(), 50, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	for _, info := range oinfos {
		assert.Equal(info.RemoteInfo.PriceInfo, expPriceInfo)
		assert.Equal(info.RemoteInfo.Transcoder, expTranscoder)
	}

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 50)
}

func TestCachedPool_N_OrchestratorsGoodPricing_ReturnsNOrchestrators(t *testing.T) {
	// Test setup
	goodTranscoder := &net.OrchestratorInfo{
		Address:    pm.RandBytes(20),
		Transcoder: "goodPriceTranscoder",
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
	}
	badTranscoder := &net.OrchestratorInfo{
		Address:    pm.RandBytes(20),
		Transcoder: "badPriceTranscoder",
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  999,
			PixelsPerUnit: 1,
		},
	}

	server.BroadcastCfg.SetMaxPrice(core.NewFixedPrice(big.NewRat(10, 1)))
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		if i, _ := strconv.Atoi(orchestratorServer.Port()); i > 8960 {
			// Return valid pricing
			return goodTranscoder, nil
		}
		// Return invalid pricing
		return badTranscoder, nil
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
	testOrchs := make([]orchTest, 0)
	for _, o := range orchestrators[:25] {
		price, _ := common.PriceToFixed(big.NewRat(999, 1))
		testOrchs = append(testOrchs, toOrchTest(o.Address.String(), o.ServiceURI, price))
	}
	for _, o := range orchestrators[25:] {
		price, _ := common.PriceToFixed(big.NewRat(1, 1))
		testOrchs = append(testOrchs, toOrchTest(o.Address.String(), o.ServiceURI, price))
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)

	// ensuring orchs exist in DB
	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)

	for _, o := range orchs {
		assert.Contains(testOrchs, toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel))
	}

	orchs, err = dbh.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	require.Nil(err)
	assert.Len(orchs, 25)
	for _, o := range orchs {
		assert.Contains(testOrchs[25:], toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel))
	}

	// check pool returns all Os, not filtering by max price
	assert.Equal(50, pool.Size())

	infos := pool.GetInfos()
	assert.Len(infos, 50)
	for _, info := range infos {
		assert.Contains(addresses, info.URL.String())
	}

	oinfos, err := pool.GetOrchestrators(context.TODO(), len(orchestrators), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))

	assert.Nil(err, "Should not be error")
	assert.Len(oinfos, 50)

	seenAddrs := make(map[string]bool)
	for _, info := range oinfos {
		addr := info.LocalInfo.URL.String()
		assert.Contains(addresses, addr)
		seenAddrs[addr] = true
	}
	assert.Len(seenAddrs, 50)
}

func TestCachedPool_GetOrchestrators_TicketParamsValidation(t *testing.T) {
	// Test setup
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	// Disable retrying discovery with extended timeout
	maxGetOrchestratorCutoffTimeout = 500 * time.Millisecond

	server.BroadcastCfg.SetMaxPrice(nil)

	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return &net.OrchestratorInfo{
			Address:      pm.RandBytes(20),
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

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{}, []string{}, 500*time.Millisecond)
	require.NoError(err)

	// Test 25 out of 50 orchs pass ticket params validation
	sender.On("ValidateTicketParams", mock.Anything).Return(errors.New("ValidateTicketParams error")).Times(25)
	sender.On("ValidateTicketParams", mock.Anything).Return(nil).Times(25)

	infos, err := pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(infos, 25)
	sender.AssertNumberOfCalls(t, "ValidateTicketParams", 50)

	// Test 0 out of 50 orchs pass ticket params validation
	sender.On("ValidateTicketParams", mock.Anything).Return(errors.New("ValidateTicketParams error")).Times(50)

	infos, err = pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(infos, 0)
	sender.AssertNumberOfCalls(t, "ValidateTicketParams", 100)
}

func TestCachedPool_GetOrchestrators_OnlyActiveOrchestrators(t *testing.T) {
	// Test setup
	expPriceInfo := &net.PriceInfo{
		PricePerUnit:  1,
		PixelsPerUnit: 1,
	}
	expTranscoder := "transcoderFromTest"
	expPricePricePixel, _ := common.PriceToFixed(big.NewRat(1, 1))

	server.BroadcastCfg.SetMaxPrice(nil)
	gmp := runtime.GOMAXPROCS(50)
	defer runtime.GOMAXPROCS(gmp)
	var mu sync.Mutex
	first := true
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		if first {
			time.Sleep(100 * time.Millisecond)
			first = false
		}
		mu.Unlock()
		return &net.OrchestratorInfo{
			Address:    pm.RandBytes(20),
			Transcoder: expTranscoder,
			PriceInfo:  expPriceInfo,
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
	testOrchs := make([]orchTest, 0)
	for i, o := range orchestrators {
		to := orchTest{
			EthereumAddr:  o.Address.String(),
			ServiceURI:    o.ServiceURI,
			PricePerPixel: expPricePricePixel,
		}
		// Active O's will be addresses[:25]
		o.ActivationRound = big.NewInt(int64(i))
		o.DeactivationRound = big.NewInt(int64(i + 26))
		testOrchs = append(testOrchs, to)

		dbO := ethOrchToDBOrch(o)
		dbO.PricePerPixel = expPricePricePixel
		dbh.UpdateOrch(dbO)
	}

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth: &eth.StubClient{
			Orchestrators: orchestrators,
		},
		Sender: sender,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)

	pool, err := NewDBOrchestratorPoolCache(ctx, node, &stubRoundsManager{round: big.NewInt(24)}, []string{}, 500*time.Millisecond)
	require.NoError(err)

	// ensuring orchs exist in DB
	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 50)
	for _, o := range orchs {
		test := toOrchTest(o.EthereumAddr, o.ServiceURI, o.PricePerPixel)
		assert.Contains(testOrchs, test)
	}

	// check size
	assert.Equal(25, pool.Size())

	infos := pool.GetInfos()
	assert.Len(infos, 25)
	for _, info := range infos {
		assert.Contains(addresses[:25], info.URL.String())
	}
	oinfos, err := pool.GetOrchestrators(context.TODO(), 50, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	for _, info := range oinfos {
		assert.Equal(info.RemoteInfo.PriceInfo, expPriceInfo)
		assert.Equal(info.RemoteInfo.Transcoder, expTranscoder)
	}

	assert.Nil(err, "Should not be error")
	assert.Len(infos, 25)
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

	wg := sync.WaitGroup{}
	oldOrchInfo := serverGetOrchInfo
	defer func() { wg.Wait(); serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(c context.Context, b common.Broadcaster, s *url.URL) (*net.OrchestratorInfo, error) {
		defer wg.Done()
		return &net.OrchestratorInfo{Transcoder: "transcoder"}, nil
	}

	// assert created webhook pool is correct length
	whURL, _ := url.ParseRequestURI("https://livepeer.live/api/orchestrator")
	whpool := NewWebhookPool(nil, whURL, 500*time.Millisecond)
	assert.Equal(3, whpool.Size())

	// assert that list is not refreshed if lastRequest is less than 1 min ago and hash is the same
	wg.Add(whpool.Size())
	whpool.mu.Lock()
	lastReq := whpool.lastRequest
	whpool.mu.Unlock()
	orchInfo, err := whpool.GetOrchestrators(context.TODO(), 2, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())

	infos := whpool.pool.GetInfos()
	assert.Len(infos, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.Contains(infos, common.OrchestratorLocalInfo{URL: uri})
	}

	//  assert that list is not refreshed if lastRequest is more than 1 min ago and hash is the same
	wg.Add(whpool.Size())
	lastReq = time.Now().Add(-2 * time.Minute)
	whpool.mu.Lock()
	whpool.lastRequest = lastReq
	whpool.mu.Unlock()
	orchInfo, err = whpool.GetOrchestrators(context.TODO(), 2, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())
	assert.NotEqual(lastReq, whpool.lastRequest)

	infos = whpool.pool.GetInfos()
	assert.Len(infos, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.Contains(infos, common.OrchestratorLocalInfo{URL: uri})
	}

	// mock a change in webhook addresses
	addresses = []string{"https://127.0.0.1:8932", "https://127.0.0.1:8933", "https://127.0.0.1:8934"}

	//  assert that list is not refreshed if lastRequest is less than 1 min ago and hash is not the same
	wg.Add(whpool.Size())
	lastReq = time.Now()
	whpool.mu.Lock()
	whpool.lastRequest = lastReq
	whpool.mu.Unlock()
	orchInfo, err = whpool.GetOrchestrators(context.TODO(), 2, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())
	assert.Equal(lastReq, whpool.lastRequest)

	infos = whpool.GetInfos()
	assert.Len(infos, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.NotContains(infos, common.OrchestratorLocalInfo{URL: uri})
	}

	//  assert that list is refreshed if lastRequest is longer than 1 min ago and hash is not the same
	wg.Add(whpool.Size())
	lastReq = time.Now().Add(-2 * time.Minute)
	whpool.mu.Lock()
	whpool.lastRequest = lastReq
	whpool.mu.Unlock()
	orchInfo, err = whpool.GetOrchestrators(context.TODO(), 2, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	require.Nil(err)
	assert.Len(orchInfo, 2)
	assert.Equal(3, whpool.Size())
	assert.NotEqual(lastReq, whpool.lastRequest)

	infos = whpool.pool.GetInfos()
	assert.Len(infos, 3)

	for _, addr := range addresses {
		uri, _ := url.ParseRequestURI(addr)
		assert.Contains(infos, common.OrchestratorLocalInfo{URL: uri})
	}
}

func TestDeserializeWebhookJSON(t *testing.T) {
	assert := assert.New(t)

	// assert input of webhookResponse address object returns correct address
	resp, _ := json.Marshal(&[]webhookResponse{{Address: "https://127.0.0.1:8936"}})
	urls, err := deserializeWebhookJSON(resp)
	assert.Nil(err)
	assert.Equal("https://127.0.0.1:8936", urls[0].URL.String())

	// assert input of empty byte array returns JSON error
	urls, err = deserializeWebhookJSON([]byte{})
	assert.Contains(err.Error(), "unexpected end of JSON input")
	assert.Nil(urls)

	// assert input of empty byte array returns empty object
	resp, _ = json.Marshal(&[]webhookResponse{{}})
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

func TestEthOrchToDBOrch(t *testing.T) {
	assert := assert.New(t)
	o := &lpTypes.Transcoder{
		ServiceURI:        "hello livepeer",
		ActivationRound:   big.NewInt(5),
		DeactivationRound: big.NewInt(100),
		Address:           ethcommon.HexToAddress("0x79f709b01033dfDBf065cfF7a1Abe7C72011D3EB"),
	}

	dbo := ethOrchToDBOrch(o)

	assert.Equal(dbo.ServiceURI, o.ServiceURI)
	assert.Equal(dbo.EthereumAddr, o.Address.Hex())
	assert.Equal(dbo.ActivationRound, o.ActivationRound.Int64())
	assert.Equal(dbo.DeactivationRound, o.DeactivationRound.Int64())

	// If DeactivationRound > maxInt64 => DeactivationRound = maxInt64
	o.DeactivationRound, _ = new(big.Int).SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10)
	dbo = ethOrchToDBOrch(o)
	assert.Equal(dbo.ServiceURI, o.ServiceURI)
	assert.Equal(dbo.EthereumAddr, o.Address.Hex())
	assert.Equal(dbo.ActivationRound, o.ActivationRound.Int64())
	assert.Equal(dbo.DeactivationRound, int64(math.MaxInt64))
}

func TestOrchestratorPool_GetOrchestrators(t *testing.T) {
	assert := assert.New(t)

	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})
	orchTimeout := 500 * time.Millisecond

	wg := sync.WaitGroup{}
	orchCb := func() error { return nil }
	oldOrchInfo := serverGetOrchInfo
	defer func() { wg.Wait(); serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, server *url.URL) (*net.OrchestratorInfo, error) {
		defer wg.Done()
		err := orchCb()
		return &net.OrchestratorInfo{
			Transcoder: server.String(),
		}, err
	}

	pool := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{}, orchTimeout)

	// Check that we receive everything
	wg.Add(len(addresses))
	res, err := pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, len(addresses))

	// Check that partial results are received if requested
	wg.Add(len(addresses))
	assert.Greater(len(addresses), 1) // sanity
	res, err = pool.GetOrchestrators(context.TODO(), 1, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 1)
	wg.Wait() // prevents races on remaining responses

	// Check error handling: all errors
	wg.Add(len(addresses))
	orchCb = func() error { return errors.New("Error") }
	res, err = pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 0)

	// Check error handling: mixed errors
	mu := &sync.Mutex{}
	ctr := 0
	orchCb = func() error {
		mu.Lock()
		defer mu.Unlock()
		ctr++
		if ctr == 1 {
			return errors.New("Error")
		}
		return nil
	}
	wg.Add(len(addresses))
	start := time.Now()
	res, err = pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	end := time.Now()
	assert.Nil(err)
	assert.Len(res, len(addresses)-1)
	// Ensure that the timeout did not fire
	assert.Less(end.Sub(start).Milliseconds(),
		pool.discoveryTimeout.Milliseconds())

}

func TestOrchestratorPool_GetOrchestrators_SuspendedOrchs(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})

	wg := sync.WaitGroup{}

	orchCb := func() error { return nil }
	oldOrchInfo := serverGetOrchInfo
	defer func() { wg.Wait(); serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, server *url.URL) (*net.OrchestratorInfo, error) {
		defer wg.Done()
		err := orchCb()
		return &net.OrchestratorInfo{
			Transcoder: server.String(),
		}, err
	}

	pool := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{}, 50*time.Millisecond)

	// suspend https://127.0.0.1:8938
	sus := newStubSuspender()
	sus.list["https://127.0.0.1:8938"] = 5
	require.Greater(sus.Suspended("https://127.0.0.1:8938"), 0)

	caps := newStubCapabilities()

	// don't include suspended orchestrators if enough orchestrators are available
	wg.Add(len(addresses))
	res, err := pool.GetOrchestrators(context.TODO(), 2, sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 2)
	assert.NotEqual(res[0].RemoteInfo.GetTranscoder(), "https://127.0.0.1:8938")
	assert.NotEqual(res[1].RemoteInfo.GetTranscoder(), "https://127.0.0.1:8938")

	// include suspended O's if not enough non-suspended O's available
	wg.Add(len(addresses))
	require.Greater(sus.Suspended("https://127.0.0.1:8938"), 0)
	res, err = pool.GetOrchestrators(context.TODO(), 3, sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 3)
	// suspended Os are added last
	assert.Equal(res[2].RemoteInfo.Transcoder, "https://127.0.0.1:8938")

	// no suspended O's, insufficient non-suspended O's
	sus = newStubSuspender()
	wg.Add(len(addresses))
	res, err = pool.GetOrchestrators(context.TODO(), 4, sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 3)

	// insufficient non-suspended O's, insufficient suspended O's
	wg.Add(len(addresses))
	sus.list["https://127.0.0.1:8938"] = 5
	require.Greater(sus.Suspended("https://127.0.0.1:8938"), 0)
	res, err = pool.GetOrchestrators(context.TODO(), 4, sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 3)
	// suspended Os are added last
	assert.Equal(res[2].RemoteInfo.Transcoder, "https://127.0.0.1:8938")

	// lower penalty is included before a higher penalty
	wg.Add(len(addresses))
	sus.list["https://127.0.0.1:8937"] = 2
	require.Greater(sus.Suspended("https://127.0.0.1:8937"), 0)
	// https://127.0.0.1:8937 should be a lower index than https://127.0.0.1:8938
	res, err = pool.GetOrchestrators(context.TODO(), 4, sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(res, 3)
	assert.Equal(res[1].RemoteInfo.Transcoder, "https://127.0.0.1:8937")
	assert.Equal(res[2].RemoteInfo.Transcoder, "https://127.0.0.1:8938")
}

func TestOrchestratorPool_ShuffleGetOrchestrators(t *testing.T) {
	assert := assert.New(t)

	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})

	ch := make(chan *url.URL, len(addresses))

	oldOrchInfo := serverGetOrchInfo
	defer func() { serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, server *url.URL) (*net.OrchestratorInfo, error) {
		ch <- server
		return &net.OrchestratorInfo{Transcoder: server.String()}, nil
	}

	pool := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{}, 50*time.Millisecond)

	// Check that randomization happens: check for elements in a different order
	// Could fail sometimes due to scheduling; the order of execution is undefined
	// Try a couple times to mitigate this effect
	iters := 0
	for j := 0; j < 10; j++ {
		iters++
		_, err := pool.GetOrchestrators(context.TODO(), len(addresses), newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
		responses := []*url.URL{}
		for i := 0; i < len(addresses); i++ {
			select {
			case url := <-ch:
				responses = append(responses, url)
			case <-time.After(1 * time.Second):
				t.Error("Timed out on receiving responses")
			}
		}
		assert.Nil(err)
		assert.Len(responses, len(addresses), "Timed out")

		// We actually want some variation here to ensure randomization.
		// So check that things are *not* equal. If they are equal, then retry
		// (Can't use assert.NotEqual immediately because the test would fail)
		isAllEqual := true
		for i := 0; i < len(addresses); i++ {
			if addresses[i] != responses[i] {
				isAllEqual = false
				break
			}
		}
		if isAllEqual {
			// Proabably a spurious match, so try again.
			continue
		}
		assert.NotEqual(addresses, responses) // sanity check the above logic

		// This is the bit that ignores ordering, but checks elements are present
		assert.ElementsMatch(addresses, responses)

		// Sanity check that the ordering of orchestrators within the pool is intact
		for i, addr := range addresses {
			assert.Equal(addr, pool.infos[i].URL)
		}
		break
	}
	assert.NotEqual(10, iters, "Shuffling probably did not happen")
}

func TestOrchestratorPool_GetOrchestratorTimeout(t *testing.T) {
	defer goleak.VerifyNone(t, common.IgnoreRoutines()...)
	assert := assert.New(t)

	addresses := stringsToURIs([]string{"https://127.0.0.1:8936", "https://127.0.0.1:8937", "https://127.0.0.1:8938"})

	ch := make(chan struct{})
	oldOrchInfo := serverGetOrchInfo
	defer func() { serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, server *url.URL) (*net.OrchestratorInfo, error) {
		ch <- struct{}{} // this will block if necessary to simulate a timeout
		return &net.OrchestratorInfo{}, nil
	}

	timeout := 1 * time.Millisecond

	pool := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{}, timeout)

	timedOut := func(start, end time.Time) bool {
		return end.Sub(start).Milliseconds() >= pool.discoveryTimeout.Milliseconds()
	}

	// We may only return a subset of responses for a given test
	// Use a waitgroup to ensure we drain all pending responses
	// Keeps things pristine for follow-on tests
	wg := sync.WaitGroup{}
	getOrchestrators := func(nb int) (common.OrchestratorDescriptors, error) {
		// requests go out to all Os in the pool, regardless of number requested
		wg.Add(pool.Size())
		return pool.GetOrchestrators(context.TODO(), nb, newStubSuspender(), newStubCapabilities(), common.ScoreAtLeast(0))
	}
	drainOrchResponses := func(nb int) {
		for i := 0; i < nb; i++ {
			select {
			case <-ch:
				wg.Done()
			}
		}
	}
	responsesDrained := func() bool {
		c := make(chan struct{})
		go func() { defer close(c); wg.Wait() }()
		select {
		case <-c:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}

	// Force a timeout, check that results are empty
	start := time.Now()
	res, err := getOrchestrators(len(addresses))
	end := time.Now()
	assert.Nil(err)
	assert.Empty(res)
	assert.True(timedOut(start, end), "Did not time out")
	drainOrchResponses(len(addresses))
	assert.True(responsesDrained(), "Did not drain responses in time")

	// Sanity check we get addresses with a reasonable timeout and no forced delay
	pool.discoveryTimeout = 25 * time.Millisecond
	go drainOrchResponses(len(addresses))
	start = time.Now()
	res, err = getOrchestrators(len(addresses))
	end = time.Now()
	assert.Nil(err)
	assert.Len(res, len(addresses))
	assert.False(timedOut(start, end), "Timed out")
	assert.True(responsesDrained(), "Did not drain responses in time")

	// Check timeout when we've received partial results
	assert.Greater(len(addresses), 1) // sanity check
	go drainOrchResponses(1)
	start = time.Now()
	res, err = getOrchestrators(len(addresses))
	end = time.Now()
	assert.Nil(err)
	assert.Len(res, 1)
	assert.True(timedOut(start, end), "Did not time out")
	go drainOrchResponses(len(addresses) - 1) // clean up remaining bits
	assert.True(responsesDrained(), "Did not drain responses in time")

	// We shouldn't time out here, but still receive 1 result
	go drainOrchResponses(len(addresses))
	start = time.Now()
	res, err = getOrchestrators(1)
	end = time.Now()
	assert.Nil(err)
	assert.Len(res, 1)
	assert.False(timedOut(start, end), "Timed out")
	assert.True(responsesDrained(), "Did not drain responses in time")
}

func TestOrchestratorPool_Capabilities(t *testing.T) {
	assert := assert.New(t)

	// should succeed: legacy caps only
	i1 := &net.OrchestratorInfo{}
	// should fail: incompatible caps
	i2 := &net.OrchestratorInfo{Capabilities: &net.Capabilities{}}
	i3 := &net.OrchestratorInfo{Capabilities: &net.Capabilities{Bitstring: []uint64{1}}}
	// should succeed: compatible caps
	i4 := &net.OrchestratorInfo{Capabilities: &net.Capabilities{Bitstring: capCompatString}}
	// should be blacklisted
	address, err := hex.DecodeString("40B28ee755260ae2735950Fe1BD0a64326ce58b0")
	assert.NoError(err)
	i5 := &net.OrchestratorInfo{Capabilities: &net.Capabilities{Bitstring: capCompatString}, Address: address}

	responses := []*net.OrchestratorInfo{i1, i2, i3, i4, i5}
	addresses := stringsToURIs([]string{"a://b", "a://b", "a://b", "a://b", "a://b"})
	pool := NewOrchestratorPool(nil, addresses, common.Score_Trusted, []string{hex.EncodeToString(address)}, 50*time.Millisecond)

	// some sanity checks
	assert.Len(addresses, len(responses))
	assert.NotEqual(capCompatString, i2.Capabilities.Bitstring)
	assert.NotEqual(capCompatString, i3.Capabilities.Bitstring)
	assert.Equal(capCompatString, i4.Capabilities.Bitstring)
	assert.True(newStubCapabilities().CompatibleWith(i4.Capabilities))

	mu := &sync.Mutex{}
	calls := 0
	oldOrchInfo := serverGetOrchInfo
	defer func() { serverGetOrchInfo = oldOrchInfo }()
	serverGetOrchInfo = func(ctx context.Context, bcast common.Broadcaster, server *url.URL) (*net.OrchestratorInfo, error) {
		mu.Lock()
		defer func() {
			calls = (calls + 1) % len(responses)
			mu.Unlock()
		}()
		return responses[calls], nil
	}
	sus := newStubSuspender()

	// weird golang behavior: interface values do not check as nil
	// even though they may actually be nil. cf. common.CapabilityComparator
	// Following will segfault unless an additional nil check is placed within
	// the CapabilityComparator methods.
	// Having params.Capabilities == nil would indicate a bug elsewhere.
	// So this should fail to return any orchestrators.
	params := core.StreamParameters{}
	assert.Nil(params.Capabilities)
	infos, err := pool.GetOrchestrators(context.TODO(), len(responses), sus, params.Capabilities, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(infos, 0)

	// stub (legacy) capability for broadcaster
	caps := newStubCapabilities()
	assert.True(caps.LegacyOnly()) // sanity check
	infos, err = pool.GetOrchestrators(context.TODO(), len(responses), sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.ElementsMatch(infos.GetRemoteInfos(), []*net.OrchestratorInfo{i1, i4})

	// non-legacy. only one should pass the filter
	caps.isLegacy = false
	assert.False(caps.LegacyOnly()) // sanity check
	infos, err = pool.GetOrchestrators(context.TODO(), len(responses), sus, caps, common.ScoreAtLeast(0))
	assert.Nil(err)
	assert.Len(infos, 1)
	assert.Equal(i4, infos[0].RemoteInfo)
}

func TestSetGetOrchestratorTimeout(t *testing.T) {
	assert := assert.New(t)
	dbh, _, err := common.TempDB(t)
	require := require.New(t)
	require.Nil(err)

	sender := &pm.MockSender{}
	node := &core.LivepeerNode{
		Database: dbh,
		Eth:      &eth.StubClient{TotalStake: big.NewInt(0)},
		Sender:   sender,
	}

	//set timeout to 1000ms
	poolCache, err := NewDBOrchestratorPoolCache(context.TODO(), node, &stubRoundsManager{}, []string{}, 1000*time.Millisecond)
	assert.Nil(err)
	//confirm the timeout is now 1000ms
	assert.Equal(poolCache.discoveryTimeout, 1000*time.Millisecond)

}
