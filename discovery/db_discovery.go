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
	"github.com/livepeer/go-livepeer/eth"
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

type ticketParamsValidator interface {
	ValidateTicketParams(ticketParams *pm.TicketParams) error
}

type DBOrchestratorPoolCache struct {
	store                 common.OrchestratorStore
	lpEth                 eth.LivepeerEthClient
	ticketParamsValidator ticketParamsValidator
	rm                    common.RoundsManager
	bcast                 common.Broadcaster
}

func NewDBOrchestratorPoolCache(ctx context.Context, node *core.LivepeerNode, rm common.RoundsManager) (*DBOrchestratorPoolCache, error) {
	if node.Eth == nil {
		return nil, fmt.Errorf("could not create DBOrchestratorPoolCache: LivepeerEthClient is nil")
	}

	dbo := &DBOrchestratorPoolCache{
		store:                 node.Database,
		lpEth:                 node.Eth,
		ticketParamsValidator: node.Sender,
		rm:                    rm,
		bcast:                 core.NewBroadcaster(node),
	}

	if err := dbo.cacheTranscoderPool(); err != nil {
		return nil, err
	}

	if err := dbo.cacheOrchestratorStake(); err != nil {
		return nil, err
	}

	if err := dbo.pollOrchestratorInfo(ctx); err != nil {
		return nil, err
	}

	return dbo, nil
}

func (dbo *DBOrchestratorPoolCache) getURLs() ([]*url.URL, error) {
	orchs, err := dbo.store.SelectOrchs(
		&common.DBOrchFilter{
			MaxPrice:     server.BroadcastCfg.MaxPrice(),
			CurrentRound: dbo.rm.LastInitializedRound(),
		},
	)
	if err != nil || len(orchs) <= 0 {
		return nil, err
	}

	var uris []*url.URL
	for _, orch := range orchs {
		if uri, err := url.Parse(orch.ServiceURI); err == nil {
			uris = append(uris, uri)
		}
	}
	return uris, nil
}

func (dbo *DBOrchestratorPoolCache) GetURLs() []*url.URL {
	uris, _ := dbo.getURLs()
	return uris
}

func (dbo *DBOrchestratorPoolCache) GetOrchestrators(numOrchestrators int, suspender common.Suspender) ([]*net.OrchestratorInfo, error) {
	uris, err := dbo.getURLs()
	if err != nil || len(uris) <= 0 {
		return nil, err
	}

	pred := func(info *net.OrchestratorInfo) bool {

		if err := dbo.ticketParamsValidator.ValidateTicketParams(pmTicketParams(info.TicketParams)); err != nil {
			glog.V(common.DEBUG).Infof("invalid ticket params - orch=%v err=%v",
				info.GetTranscoder(),
				err,
			)
			return false
		}

		// check if O's price is below B's max price
		maxPrice := server.BroadcastCfg.MaxPrice()
		price := big.NewRat(info.PriceInfo.PricePerUnit, info.PriceInfo.PixelsPerUnit)
		if maxPrice != nil && price.Cmp(maxPrice) > 0 {
			glog.V(common.DEBUG).Infof("orchestrator's price is too high - orch=%v price=%v wei/pixel maxPrice=%v wei/pixel",
				info.GetTranscoder(),
				price.FloatString(3),
				maxPrice.FloatString(3),
			)
			return false
		}
		return true
	}

	orchPool := NewOrchestratorPoolWithPred(dbo.bcast, uris, pred)
	orchInfos, err := orchPool.GetOrchestrators(numOrchestrators, suspender)
	if err != nil || len(orchInfos) <= 0 {
		return nil, err
	}

	return orchInfos, nil
}

func (dbo *DBOrchestratorPoolCache) Size() int {
	count, _ := dbo.store.OrchCount(
		&common.DBOrchFilter{
			MaxPrice:     server.BroadcastCfg.MaxPrice(),
			CurrentRound: dbo.rm.LastInitializedRound(),
		},
	)
	return count
}

func (dbo *DBOrchestratorPoolCache) cacheTranscoderPool() error {
	orchestrators, err := dbo.lpEth.TranscoderPool()
	if err != nil {
		return fmt.Errorf("Could not refresh DB list of orchestrators: %v", err)
	}

	for _, o := range orchestrators {
		if err := dbo.store.UpdateOrch(ethOrchToDBOrch(o)); err != nil {
			glog.Errorf("Unable to update orchestrator %v in DB: %v", o.Address.Hex(), err)
		}
	}

	return nil
}

func (dbo *DBOrchestratorPoolCache) cacheOrchestratorStake() error {
	orchs, err := dbo.store.SelectOrchs(
		&common.DBOrchFilter{
			CurrentRound: dbo.rm.LastInitializedRound(),
		},
	)
	if err != nil {
		return fmt.Errorf("could not retrieve orchestrators from DB: %v", err)
	}

	resc, errc := make(chan *common.DBOrch), make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), getOrchestratorsTimeoutLoop)
	defer cancel()

	currentRound := dbo.rm.LastInitializedRound()

	getStake := func(o *common.DBOrch) {
		ep, err := dbo.lpEth.GetTranscoderEarningsPoolForRound(ethcommon.HexToAddress(o.EthereumAddr), currentRound)
		if err != nil {
			errc <- err
			return
		}

		stakeFp, err := common.BaseTokenAmountToFixed(ep.TotalStake)
		if err != nil {
			errc <- err
			return
		}
		o.Stake = stakeFp

		resc <- o
	}

	for _, o := range orchs {
		go getStake(o)
	}

	for i := 0; i < len(orchs); i++ {
		select {
		case res := <-resc:
			if err := dbo.store.UpdateOrch(res); err != nil {
				glog.Error("Error updating Orchestrator in DB: ", err)
			}
		case err := <-errc:
			glog.Errorln(err)
		case <-ctx.Done():
			glog.Info("Done fetching stake for orchestrators, context timeout")
			break
		}
	}

	return nil
}

func (dbo *DBOrchestratorPoolCache) pollOrchestratorInfo(ctx context.Context) error {
	if err := dbo.cacheDBOrchs(); err != nil {
		return err
	}

	ticker := getTicker()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := dbo.cacheDBOrchs(); err != nil {
					glog.Errorf("unable to poll orchestrator info: %v", err)
				}
			}
		}
	}()

	return nil
}

func (dbo *DBOrchestratorPoolCache) cacheDBOrchs() error {
	orchs, err := dbo.store.SelectOrchs(
		&common.DBOrchFilter{
			CurrentRound: dbo.rm.LastInitializedRound(),
		},
	)
	if err != nil {
		return fmt.Errorf("could not retrieve orchestrators from DB: %v", err)
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
		info, err := serverGetOrchInfo(ctx, dbo.bcast, uri)
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
		numOrchs++
		go getOrchInfo(orch)

	}

	for i := 0; i < numOrchs; i++ {
		select {
		case res := <-resc:
			if err := dbo.store.UpdateOrch(res); err != nil {
				glog.Error("Error updating Orchestrator in DB: ", err)
			}
		case err := <-errc:
			glog.Errorln(err)
		case <-ctx.Done():
			glog.Info("Done fetching orch info for orchestrators, context timeout")
			break
		}
	}

	return nil
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

	dbo := &common.DBOrch{
		ServiceURI:        orch.ServiceURI,
		EthereumAddr:      orch.Address.String(),
		ActivationRound:   common.ToInt64(orch.ActivationRound),
		DeactivationRound: common.ToInt64(orch.DeactivationRound),
	}

	return dbo
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
		ExpirationBlock:   new(big.Int).SetBytes(params.ExpirationBlock),
		ExpirationParams: &pm.TicketExpirationParams{
			CreationRound:          params.ExpirationParams.GetCreationRound(),
			CreationRoundBlockHash: ethcommon.BytesToHash(params.ExpirationParams.GetCreationRoundBlockHash()),
		},
	}
}
