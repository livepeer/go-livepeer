package discovery

import (
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"

	"github.com/golang/glog"
)

type DBOrchestratorPoolCache struct {
	node *core.LivepeerNode
}

func NewDBOrchestratorPoolCache(node *core.LivepeerNode) *DBOrchestratorPoolCache {
	if node.Eth == nil {
		glog.Error("Could not refresh DB list of orchestrators: LivepeerNode nil")
		return nil
	}

	ticker := time.NewTicker(1 * time.Hour)
	go func(node *core.LivepeerNode) {
		for {
			select {
			case <-ticker.C:
				orchestrators, err := node.Eth.RegisteredTranscoders()
				if err != nil {
					glog.Error("Could not refresh DB list of orchestrators: ", err)
					return
				}
				_, dbOrchErr := CacheDBOrchs(node, orchestrators)
				if dbOrchErr != nil {
					glog.Error("Could not refresh DB list of orchestrators: CacheDBOrchs err")
					return
				}
			}
		}
	}(node)

	return &DBOrchestratorPoolCache{node: node}
}

func CacheDBOrchs(node *core.LivepeerNode, orchs []*lpTypes.Transcoder) ([]*common.DBOrch, error) {
	var dbOrchs []*common.DBOrch
	if orchs == nil {
		glog.Error("No new DB orchestrators created: no orchestrators found onchain")
		return dbOrchs, nil
	}

	for _, orch := range orchs {
		dbOrch := EthOrchToDBOrch(orch)
		if err := node.Database.UpdateOrchs(dbOrch); err != nil {
			glog.Error("Error updating Orchestrator in DB: ", err)
			return nil, err
		}
		dbOrchs = append(dbOrchs, dbOrch)
	}
	return dbOrchs, nil

}

func EthOrchToDBOrch(orch *lpTypes.Transcoder) *common.DBOrch {
	if orch == nil {
		return nil
	}
	return common.NewDBOrch(orch.ServiceURI, orch.Address.String())
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

	orchInfo, err := orchPool.GetOrchestrators(numOrchestrators)
	if err != nil || len(orchInfo) <= 0 {
		return nil, err
	}

	return orchInfo, nil
}
