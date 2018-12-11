package discovery

import (
	"net/url"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

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

func (o *offchainOrchestrators) GetOrchestrators(numOrchestrators int) ([][]*net.OrchestratorInfo, error) {
	glog.Infof("\n\nwe're in GET ORCHESTRATORS!\n\n")
	var orchInfo [][]*net.OrchestratorInfo
	for _, uri := range o.uri {
		glog.Infof("\n\nwe're in GET ORCHESTRATORS.....looping time!\n\n")
		tinfo, err := server.GetOrchestratorInfo(o.bcast, uri)
		if err != nil {
			return nil, nil
		}
		orchInfo = append(orchInfo, []*net.OrchestratorInfo{tinfo})
	}
	return orchInfo, nil
}
