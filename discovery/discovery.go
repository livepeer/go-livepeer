package discovery

import (
	"context"
	"math"
	"math/rand"
	"net/url"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

const getOrchestratorsTimeoutLoop = 1 * time.Hour

var serverGetOrchInfo = server.GetOrchestratorInfo

type orchestratorPool struct {
	uris  []*url.URL
	bcast server.Broadcaster
	pred  func(info *net.OrchestratorInfo) bool
}

var perm = func(len int) []int { return rand.Perm(len) }

func NewOrchestratorPool(node *core.LivepeerNode, uris []*url.URL) *orchestratorPool {

	if len(uris) <= 0 {
		// Should we return here?
		glog.Error("Orchestrator pool does not have any URIs")
	}

	var randomizedUris []*url.URL
	for _, i := range perm(len(uris)) {
		uri := uris[i]
		randomizedUris = append(randomizedUris, uri)
	}

	bcast := core.NewBroadcaster(node)
	return &orchestratorPool{bcast: bcast, uris: randomizedUris}
}

func NewOrchestratorPoolWithPred(node *core.LivepeerNode, addresses []*url.URL, pred func(*net.OrchestratorInfo) bool) *orchestratorPool {
	// if livepeer running in offchain mode, return nil
	if node.Eth == nil {
		glog.Error("Could not refresh DB list of orchestrators: LivepeerNode nil")
		return nil
	}
	pool := NewOrchestratorPool(node, addresses)
	pool.pred = pred
	return pool
}

func (o *orchestratorPool) GetURLs() []*url.URL {
	return o.uris
}

func (o *orchestratorPool) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	numAvailableOrchs := len(o.uris)
	numOrchestrators = int(math.Min(float64(numAvailableOrchs), float64(numOrchestrators)))
	ctx, cancel := context.WithTimeout(context.Background(), getOrchestratorsTimeoutLoop)
	orchInfos := []*net.OrchestratorInfo{}
	orchChan := make(chan struct{}, len(o.uris))
	numResp := 0
	numSuccessResp := 0
	respLock := sync.Mutex{}

	getOrchInfo := func(uri *url.URL) {
		info, err := serverGetOrchInfo(ctx, o.bcast, uri)
		respLock.Lock()
		defer respLock.Unlock()
		numResp++
		if err == nil && (o.pred == nil || o.pred(info)) {
			orchInfos = append(orchInfos, info)
			numSuccessResp++
		}
		if err != nil && monitor.Enabled {
			monitor.LogDiscoveryError(err.Error())
		}
		if numSuccessResp >= numOrchestrators || numResp >= len(o.uris) {
			orchChan <- struct{}{}
		}
	}

	for _, uri := range o.uris {
		go getOrchInfo(uri)
	}

	select {
	case <-ctx.Done():
		respLock.Lock()
		if len(orchInfos) < numOrchestrators {
			numOrchestrators = len(orchInfos)
		}
		returnOrchs := orchInfos[:numOrchestrators]
		respLock.Unlock()
		glog.Info("Done fetching orch info for orchestrators, context timeout: ", len(returnOrchs))
		cancel()
		return returnOrchs, nil
	case <-orchChan:
		respLock.Lock()
		if len(orchInfos) < numOrchestrators {
			numOrchestrators = len(orchInfos)
		}
		returnOrchs := orchInfos[:numOrchestrators]
		respLock.Unlock()
		glog.Info("Done fetching orch info for orchestrators, numResponses fetched: ", len(returnOrchs))
		cancel()
		return returnOrchs, nil
	}
}

func (o *orchestratorPool) Size() int {
	return len(o.uris)
}
