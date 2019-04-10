package discovery

import (
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"

	"github.com/golang/glog"
)

var cacheRefreshInterval = 1 * time.Hour
var getTicker = func() *time.Ticker {
	return time.NewTicker(cacheRefreshInterval)
}

type DBOrchestratorPoolCache struct {
	node *core.LivepeerNode
}

func NewDBOrchestratorPoolCache(node *core.LivepeerNode) *DBOrchestratorPoolCache {
	if node.Eth == nil {
		glog.Error("Could not refresh DB list of orchestrators: LivepeerNode nil")
		return nil
	}

	_ = cacheRegisteredTranscoders(node)

	ticker := getTicker()
	go func(node *core.LivepeerNode) {
		for _ = range ticker.C {
			err := cacheRegisteredTranscoders(node)
			if err != nil {
				continue
			}
		}
	}(node)

	return &DBOrchestratorPoolCache{node: node}
}

func (dbo *DBOrchestratorPoolCache) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	orchs, err := dbo.node.Database.SelectOrchs()
	if err != nil || len(orchs) <= 0 {
		return nil, err
	}

	var uris []string
	for _, orch := range orchs {
		uri := orch.ServiceURI
		uris = append(uris, uri)
	}

	orchPool := NewOrchestratorPool(dbo.node, uris)

	orchInfos, err := orchPool.GetOrchestrators(numOrchestrators)
	if err != nil || len(orchInfos) <= 0 {
		return nil, err
	}

	return orchInfos, nil
}

func (dbo *DBOrchestratorPoolCache) Size() int {
	orchs, err := dbo.node.Database.SelectOrchs()
	if err != nil {
		return 0
	}
	return len(orchs)
}

func cacheRegisteredTranscoders(node *core.LivepeerNode) error {
	orchestrators, err := node.Eth.RegisteredTranscoders()
	if err != nil {
		glog.Error("Could not refresh DB list of orchestrators: ", err)
		return err
	}
	_, dbOrchErr := cacheDBOrchs(node, orchestrators)
	if dbOrchErr != nil {
		glog.Error("Could not refresh DB list of orchestrators: cacheDBOrchs err")
		return err
	}

	return nil
}

func cacheDBOrchs(node *core.LivepeerNode, orchs []*lpTypes.Transcoder) ([]*common.DBOrch, error) {
	var dbOrchs []*common.DBOrch
	if orchs == nil {
		glog.Error("No new DB orchestrators created: no orchestrators found onchain")
		return dbOrchs, nil
	}

	for _, orch := range orchs {
		if orch == nil {
			continue
		}
		dbOrch := ethOrchToDBOrch(orch)
		if err := node.Database.UpdateOrch(dbOrch); err != nil {
			glog.Error("Error updating Orchestrator in DB: ", err)
			continue
		}
		dbOrchs = append(dbOrchs, dbOrch)
	}
	return dbOrchs, nil

}

func ethOrchToDBOrch(orch *lpTypes.Transcoder) *common.DBOrch {
	if orch == nil {
		return nil
	}
	return common.NewDBOrch(orch.ServiceURI, orch.Address.String())
}
