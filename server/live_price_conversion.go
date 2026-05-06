package server

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
)

type liveRunnerPriceConverter struct {
	source    runner.LiveRunnerPriceInfo
	converter *core.AutoConvertedPrice
}

type liveRunnerPriceConverterCache struct {
	mu            sync.Mutex
	converters    map[string]*liveRunnerPriceConverter
	runnerKeyByID map[string]string
}

var discoveryPriceConverters = &liveRunnerPriceConverterCache{
	converters:    make(map[string]*liveRunnerPriceConverter),
	runnerKeyByID: make(map[string]string),
}

func (c *liveRunnerPriceConverterCache) upsertRunner(runnerID string, req runner.LiveRunnerHeartbeatRequest) error {
	key := discoveryRunnerPriceKey(req.RunnerURL, req.App)
	priceInfo := req.PriceInfo

	c.mu.Lock()
	defer c.mu.Unlock()

	oldKey, hadOld := c.runnerKeyByID[runnerID]
	if hadOld && oldKey != key {
		c.stopAndDeleteByKey(oldKey)
	}

	entry, exists := c.converters[key]
	if exists && entry.source == priceInfo {
		c.runnerKeyByID[runnerID] = key
		return nil
	}

	next, err := newConverterForRunner(priceInfo)
	if err != nil {
		return err
	}
	if exists && entry.converter != nil {
		entry.converter.Stop()
	}
	c.converters[key] = &liveRunnerPriceConverter{
		source:    priceInfo,
		converter: next,
	}
	c.runnerKeyByID[runnerID] = key
	return nil
}

func (c *liveRunnerPriceConverterCache) unregisterRunner(runnerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key, ok := c.runnerKeyByID[runnerID]
	if !ok {
		return
	}
	delete(c.runnerKeyByID, runnerID)
	c.stopAndDeleteByKey(key)
}

func (c *liveRunnerPriceConverterCache) applyConvertedPrices(runners []runner.LiveRunnerDiscoveryRunner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range runners {
		key := discoveryRunnerPriceKey(runners[i].Endpoint, runners[i].App)
		entry := c.converters[key]
		if entry == nil || entry.converter == nil {
			continue
		}
		converted, err := convertedPriceInfo(entry.converter)
		if err != nil {
			glog.Errorf("error reading converted live runner price app=%v endpoint=%v err=%v", runners[i].App, runners[i].Endpoint, err)
			continue
		}
		runners[i].PriceInfo = converted
	}
}

func (c *liveRunnerPriceConverterCache) cleanupStale(runners []runner.LiveRunnerDiscoveryRunner) {
	alive := make(map[string]struct{}, len(runners))
	for _, r := range runners {
		alive[discoveryRunnerPriceKey(r.Endpoint, r.App)] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.converters {
		if _, ok := alive[key]; ok {
			continue
		}
		c.stopAndDeleteByKey(key)
		for runnerID, runnerKey := range c.runnerKeyByID {
			if runnerKey == key {
				delete(c.runnerKeyByID, runnerID)
			}
		}
	}
}

func (c *liveRunnerPriceConverterCache) stopAndDeleteByKey(key string) {
	if entry := c.converters[key]; entry != nil && entry.converter != nil {
		entry.converter.Stop()
	}
	delete(c.converters, key)
}

func discoveryRunnerPriceKey(endpoint, app string) string {
	return endpoint + "\x1f" + app
}

func newConverterForRunner(priceInfo runner.LiveRunnerPriceInfo) (*core.AutoConvertedPrice, error) {
	if priceInfo.PricePerUnit <= 0 || priceInfo.PixelsPerUnit <= 0 {
		return nil, nil
	}
	if strings.ToUpper(priceInfo.Unit) != "USD" {
		return nil, fmt.Errorf("unsupported runner price unit=%s expected USD", priceInfo.Unit)
	}

	usdPerPixel, err := usdPerPixelFromUSDPerHour(priceInfo)
	if err != nil {
		return nil, err
	}
	return core.NewAutoConvertedPrice("USD", usdPerPixel, nil)
}

func usdPerPixelFromUSDPerHour(priceInfo runner.LiveRunnerPriceInfo) (*big.Rat, error) {
	if strings.ToUpper(priceInfo.Unit) != "USD" {
		return nil, fmt.Errorf("unsupported runner price unit=%s expected USD", priceInfo.Unit)
	}

	pixelsPerHour := liveVideoBillingPixelsPerHour()
	if pixelsPerHour <= 0 {
		return nil, fmt.Errorf("invalid live video billing basis")
	}

	usdPerHour := new(big.Rat).SetFrac64(priceInfo.PricePerUnit, priceInfo.PixelsPerUnit)
	return new(big.Rat).Quo(usdPerHour, new(big.Rat).SetInt64(int64(pixelsPerHour))), nil
}

func liveVideoBillingPixelsPerHour() int64 {
	// 720p @ 30fps
	return int64(1280 * 720 * 30 * 3600)
}

func convertedPriceInfo(converter *core.AutoConvertedPrice) (runner.LiveRunnerPriceInfo, error) {
	if converter == nil {
		return runner.LiveRunnerPriceInfo{}, nil
	}
	priceFixed, err := common.PriceToFixed(converter.Value())
	if err != nil {
		return runner.LiveRunnerPriceInfo{}, err
	}
	return runner.LiveRunnerPriceInfo{
		PricePerUnit:  priceFixed,
		PixelsPerUnit: 1,
		Unit:          "WEI",
	}, nil
}
