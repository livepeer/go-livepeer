package discovery

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
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

	ticker := time.NewTicker(1 * time.Hour)
	// quit := make(chan struct{})
	var orchestrators []*lpTypes.Transcoder
	go func(node *core.LivepeerNode) {
		for {
			select {
			case <-ticker.C:
				orchestrators, err := node.Eth.RegisteredTranscoders()
				if err != nil {
					glog.Error("Could not refresh DB list of orchestrators: ", err)
					return
				}
				_, dbOrchErr := NewDBOrchestrators(node, orchestrators)
				if dbOrchErr != nil {
					glog.Error("Could not refresh DB list of orchestrators: NewDBOrchestrators err")
					return
				}
				// case <-quit:
				// 	ticker.Stop()
				// 	return
			}
		}
	}(node)

	var addresses []string
	for _, orch := range orchestrators {
		addresses = append(addresses, orch.ServiceURI)
	}

	return NewOrchestratorPool(node, addresses)
}

func NewDBOrchestrators(node *core.LivepeerNode, orchs []*lpTypes.Transcoder) ([]*common.DBOrch, error) {
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

func GetOrchestratorsFromDB(node *core.LivepeerNode, numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	// Get orchestrators that have been updated in the past 24h
	orchs, err := node.Database.SelectOrchs()
	if err != nil {
		return nil, err
	}
	var addresses []string
	for _, orch := range orchs {
		addr := orch.ServiceURI
		addresses = append(addresses, addr)
	}
	offchainOrchList := NewOrchestratorPool(node, addresses)
	return offchainOrchList.GetOrchestrators(numOrchestrators)
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
