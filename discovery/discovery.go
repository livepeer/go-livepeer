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

type offchainOrchestrators struct {
	uri   []*url.URL
	bcast server.Broadcaster
}

func NewOffchainOrchestrator(node *core.LivepeerNode, addresses []string) *offchainOrchestrators {
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
	return &offchainOrchestrators{bcast: bcast, uri: uris}
}

func NewOffchainOrchestratorFromOnchainList(node *core.LivepeerNode) *offchainOrchestrators {
	// if livepeer running in offchain mode, return nil
	if node.Eth == nil {
		return nil
	}

	orchestrators, err := node.Eth.RegisteredTranscoders()
	if err != nil {
		glog.Error(err)
		return nil
	}

	var addresses []string
	for _, orch := range orchestrators {
		addresses = append(addresses, orch.ServiceURI)
	}

	return NewOffchainOrchestrator(node, addresses)
}

func (o *offchainOrchestrators) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
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
