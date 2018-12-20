package discovery

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

const GetOrchestratorsTimeoutLoop = 3 * time.Second

type orchestratorPool struct {
	uri   []*url.URL
	bcast server.Broadcaster
}

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

	bcast := core.NewBroadcaster(node)
	return &orchestratorPool{bcast: bcast, uri: uris}
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
		}
		if numSuccessResp >= numOrchestrators || numResp >= len(o.uri) {
			orchChan <- struct{}{}
		}
	}

	for _, uri := range o.uri {
		go getOrchInfo(uri)
	}

	select {
	case <-ctx.Done():
		glog.Info("Done fetching orch info for orchestrators, context timeout: ", orchInfos)
		cancel()
		return orchInfos, nil
	case <-orchChan:
		glog.Info("Done fetching orch info for orchestrators, numResponses fetched: ", orchInfos)
		cancel()
		return orchInfos, nil
	}
}
