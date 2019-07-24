package discovery

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/server"

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

func (dbo *DBOrchestratorPoolCache) GetURLs() []*url.URL {
	orchs, err := dbo.node.Database.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	if err != nil || len(orchs) <= 0 {
		return nil
	}

	var uris []*url.URL
	for _, orch := range orchs {
		if uri, err := url.Parse(orch.ServiceURI); err == nil {
			uris = append(uris, uri)
		}
	}
	return uris
}

func (dbo *DBOrchestratorPoolCache) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	orchs, err := dbo.node.Database.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	if err != nil || len(orchs) <= 0 {
		return nil, err
	}

	var uris []string
	for _, orch := range orchs {
		uri := orch.ServiceURI
		uris = append(uris, uri)
	}

	pred := func(info *net.OrchestratorInfo) bool {
		if dbo.node.Sender != nil {
			if err := dbo.node.Sender.ValidateTicketParams(pmTicketParams(info.TicketParams)); err != nil {
				return false
			}
		}

		price := server.BroadcastCfg.MaxPrice()
		if price != nil {
			return big.NewRat(info.PriceInfo.PricePerUnit, info.PriceInfo.PixelsPerUnit).Cmp(price) <= 0
		}
		return true
	}

	orchPool := NewOrchestratorPoolWithPred(dbo.node, uris, pred)

	orchInfos, err := orchPool.GetOrchestrators(numOrchestrators)
	if err != nil || len(orchInfos) <= 0 {
		return nil, err
	}

	return orchInfos, nil
}

func (dbo *DBOrchestratorPoolCache) Size() int {
	return len(dbo.GetURLs())
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

	resc, errc := make(chan *common.DBOrch), make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), getOrchestratorsTimeoutLoop)
	defer cancel()

	getOrchInfo := func(dbOrch *common.DBOrch) {
		uri, err := parseURI(dbOrch.ServiceURI)
		if err != nil {
			errc <- err
			return
		}
		info, err := serverGetOrchInfo(ctx, core.NewBroadcaster(node), uri)
		if err != nil {
			errc <- err
			return
		}
		dbOrch.PricePerPixel, err = common.PriceToFixed(big.NewRat(info.PriceInfo.GetPricePerUnit(), info.PriceInfo.GetPixelsPerUnit()))
		if err != nil {
			errc <- err
			return
		}
		resc <- dbOrch
	}

	numOrchs := 0
	for _, orch := range orchs {
		if orch == nil {
			continue
		}
		dbOrch := ethOrchToDBOrch(orch)
		numOrchs++
		go getOrchInfo(dbOrch)

	}

	var returnDBOrchs []*common.DBOrch

	for i := 0; i < numOrchs; i++ {
		select {
		case res := <-resc:
			if err := node.Database.UpdateOrch(res); err != nil {
				glog.Error("Error updating Orchestrator in DB: ", err)
			}
			returnDBOrchs = append(returnDBOrchs, res)
		case err := <-errc:
			glog.Errorln(err)
		case <-ctx.Done():
			glog.Info("Done fetching orch info for orchestrators, context timeout")
			break
		}
	}
	return returnDBOrchs, nil
}

func parseURI(addr string) (*url.URL, error) {
	if !strings.HasPrefix(addr, "http") {
		addr = "https://" + addr
	}
	uri, err := url.ParseRequestURI(addr)
	if err != nil {
		return nil, fmt.Errorf("Could not parse orchestrator URI: %v", err)
	}
	return uri, nil
}

func ethOrchToDBOrch(orch *lpTypes.Transcoder) *common.DBOrch {
	if orch == nil {
		return nil
	}
	return common.NewDBOrch(orch.ServiceURI, orch.Address.String())
}

func pmTicketParams(params *net.TicketParams) *pm.TicketParams {
	if params == nil {
		return nil
	}

	return &pm.TicketParams{
		Recipient:         ethcommon.BytesToAddress(params.Recipient),
		FaceValue:         new(big.Int).SetBytes(params.FaceValue),
		WinProb:           new(big.Int).SetBytes(params.WinProb),
		RecipientRandHash: ethcommon.BytesToHash(params.RecipientRandHash),
		Seed:              new(big.Int).SetBytes(params.Seed),
	}
}
