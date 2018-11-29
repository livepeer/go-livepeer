package discovery

import (
	"net/url"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

type offchainOrchestrator struct {
	uri   *url.URL
	bcast server.Broadcaster
}

func NewOffchainOrchestrator(node *core.LivepeerNode, address string) *offchainOrchestrator {
	addr := "https://" + address
	uri, err := url.ParseRequestURI(addr)
	if err != nil {
		glog.Error("Could not parse orchestrator URI: ", err)
		return nil
	}
	bcast := core.NewBroadcaster(node)
	return &offchainOrchestrator{bcast: bcast, uri: uri}
}

func (o *offchainOrchestrator) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	tinfo, err := server.GetOrchestratorInfo(o.bcast, o.uri)
	return []*net.OrchestratorInfo{tinfo}, err
}
