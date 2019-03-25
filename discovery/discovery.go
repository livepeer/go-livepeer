package discovery

import (
	"context"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

const GetOrchestratorsTimeoutLoop = 1 * time.Hour

type orchestratorPool struct {
	uris  []*url.URL
	bcast server.Broadcaster
}

var perm = func(len int) []int { return rand.Perm(len) }

func NewOrchestratorPool(node *core.LivepeerNode, addresses []string) *orchestratorPool {
	var uris []*url.URL

	for _, addr := range addresses {
		if !strings.HasPrefix(addr, "http") {
			addr = "https://" + addr
		}
		uri, err := url.ParseRequestURI(addr)
		if err != nil {
			glog.Error("Could not parse orchestrator URI: ", err)
			continue
		}
		uris = append(uris, uri)
	}

	if len(uris) <= 0 {
		glog.Error("Could not parse orchAddresses given - no URIs returned ")
	}

	var randomizedUris []*url.URL
	for _, i := range perm(len(uris)) {
		uri := uris[i]
		randomizedUris = append(randomizedUris, uri)
	}

	bcast := core.NewBroadcaster(node)
	return &orchestratorPool{bcast: bcast, uris: randomizedUris}
}

func NewOnchainOrchestratorPool(node *core.LivepeerNode) *orchestratorPool {
	// if livepeer running in offchain mode, return nil
	if node.Eth == nil {
		glog.Error("Could not refresh DB list of orchestrators: LivepeerNode nil")
		return nil
	}

	orchestrators, err := node.Eth.RegisteredTranscoders()
	if err != nil {
		glog.Error("Could not refresh DB list of orchestrators: ", err)
		return nil
	}

	var addresses []string
	for _, orch := range orchestrators {
		addresses = append(addresses, orch.ServiceURI)
	}

	return NewOrchestratorPool(node, addresses)
}

func (o *orchestratorPool) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	numAvailableOrchs := len(o.uris)
	numOrchestrators = int(math.Min(float64(numAvailableOrchs), float64(numOrchestrators)))
	ctx, cancel := context.WithTimeout(context.Background(), GetOrchestratorsTimeoutLoop)
	orchInfos := []*net.OrchestratorInfo{}
	orchChan := make(chan struct{})
	numResp := 0
	numSuccessResp := 0
	respLock := sync.Mutex{}

	getOrchInfo := func(uri *url.URL) {
		info, err := server.GetOrchestratorInfo(ctx, o.bcast, uri)
		respLock.Lock()
		defer respLock.Unlock()
		numResp++
		if err == nil {
			orchInfos = append(orchInfos, info)
			numSuccessResp++
		} else if monitor.Enabled {
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
		glog.Info("Done fetching orch info for orchestrators, context timeout: ", returnOrchs)
		cancel()
		return returnOrchs, nil
	case <-orchChan:
		respLock.Lock()
		if len(orchInfos) < numOrchestrators {
			numOrchestrators = len(orchInfos)
		}
		returnOrchs := orchInfos[:numOrchestrators]
		respLock.Unlock()
		glog.Info("Done fetching orch info for orchestrators, numResponses fetched: ", returnOrchs)
		cancel()
		return returnOrchs, nil
	}
}

func (o *orchestratorPool) Size() int {
	return len(o.uris)
}
