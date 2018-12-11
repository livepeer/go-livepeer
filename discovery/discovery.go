package discovery

import (
	"context"
	"net/url"
	"strings"
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

func NewOffchainOrchestrator(node *core.LivepeerNode, address string) *offchainOrchestrators {
	addr := "https://" + address
	uri, err := url.ParseRequestURI(addr)
	if err != nil {
		glog.Error("Could not parse orchestrator URI: ", err)
		return nil
	}
	bcast := core.NewBroadcaster(node)
	return &offchainOrchestrators{bcast: bcast, uri: []*url.URL{uri}}
}

func NewOffchainOrchestratorFromOnchainList(node *core.LivepeerNode) *offchainOrchestrators {
	orchestrators, err := node.Eth.RegisteredTranscoders()
	if err != nil {
		glog.Error(err)
		return nil
	}

	var uris []*url.URL

	for _, orch := range orchestrators {
		serviceURI := orch.ServiceURI
		if !strings.HasPrefix(serviceURI, "http") {
			serviceURI = "https://" + serviceURI
		}
		uri, err := url.ParseRequestURI(serviceURI)
		if err != nil {
			glog.Error("Could not parse orchestrator URI: ", err)
			return nil
		}
		uris = append(uris, uri)
	}

	bcast := core.NewBroadcaster(node)
	return &offchainOrchestrators{bcast: bcast, uri: uris}
}

func (o *offchainOrchestrators) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), GetOrchestratorsTimeoutLoop)

	var orchInfo []*net.OrchestratorInfo
	orchChan := make(chan struct{})

	getOrchInfo := func(uri *url.URL, bcast server.Broadcaster, i int) {
		tinfo, err := server.GetOrchestratorInfo(bcast, uri)
		if err != nil {
			glog.Errorf("Error getting OrchestratorInfo for uri: %v", uri)
			return
		}
		orchInfo = append(orchInfo, tinfo)
		if numOrchestrators <= i {
			orchChan <- struct{}{}
		}
	}

	for i, uri := range o.uri {
		go getOrchInfo(uri, o.bcast, i)
	}

	select {
	case <-ctx.Done():
		glog.Info("Done fetching orch info for %v orchestrators", numOrchestrators)
		cancel()
		return orchInfo, nil
	case <-orchChan:
		glog.Info("Done fetching orch info for %v orchestrators", numOrchestrators)
		cancel()
		return orchInfo, nil
	}
}
