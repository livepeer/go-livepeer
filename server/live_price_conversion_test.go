package server

import (
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
)

func TestLiveRunnerPriceConversionCache_ConvertsUSDToWEI(t *testing.T) {
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()

	cache := &liveRunnerPriceConverterCache{
		converters:    make(map[string]*liveRunnerPriceConverter),
		runnerKeyByID: make(map[string]string),
	}
	req := runner.LiveRunnerHeartbeatRequest{
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		PriceInfo: runner.LiveRunnerPriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	}
	require.NoError(t, cache.upsertRunner("runner-1", req))

	runners := []runner.LiveRunnerDiscoveryRunner{{
		Endpoint:  "https://runner.example.com",
		App:       "live-video-to-video/scope",
		PriceInfo: req.PriceInfo,
	}}
	cache.applyConvertedPrices(runners)

	got := big.NewRat(runners[0].PriceInfo.PricePerUnit, runners[0].PriceInfo.PixelsPerUnit)
	expectedFixed, err := common.PriceToFixed(big.NewRat(5_000_000_000_000_000, liveVideoBillingPixelsPerHour()))
	require.NoError(t, err)
	expected := big.NewRat(expectedFixed, 1)
	require.Zero(t, expected.Cmp(got))
	require.Equal(t, "WEI", runners[0].PriceInfo.Unit)
	cache.unregisterRunner("runner-1")
}

func TestLiveVideoBillingBasisConstants(t *testing.T) {
	require.Equal(t, int64(99_532_800_000), liveVideoBillingPixelsPerHour())
}
