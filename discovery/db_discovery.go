package discovery

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"

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
	orchBlacklist         []string
	discoveryTimeout      time.Duration
}

func NewDBOrchestratorPoolCache(ctx context.Context, node *core.LivepeerNode, rm common.RoundsManager, orchBlacklist []string, discoveryTimeout time.Duration) (*DBOrchestratorPoolCache, error) {
	if node.Eth == nil {
		return nil, fmt.Errorf("could not create DBOrchestratorPoolCache: LivepeerEthClient is nil")
	}

	dbo := &DBOrchestratorPoolCache{
		store:                 node.Database,
		lpEth:                 node.Eth,
		ticketParamsValidator: node.Sender,
		rm:                    rm,
		bcast:                 core.NewBroadcaster(node),
		orchBlacklist:         orchBlacklist,
		discoveryTimeout:      discoveryTimeout,
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
			CurrentRound:   dbo.rm.LastInitializedRound(),
			UpdatedLastDay: true,
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

func (dbo *DBOrchestratorPoolCache) GetInfos() []common.OrchestratorLocalInfo {
	uris, _ := dbo.getURLs()
	infos := make([]common.OrchestratorLocalInfo, 0, len(uris))
	for _, uri := range uris {
		infos = append(infos, common.OrchestratorLocalInfo{URL: uri, Score: common.Score_Untrusted})
	}
	return infos
}

func (dbo *DBOrchestratorPoolCache) GetOrchestrators(ctx context.Context, numOrchestrators int, suspender common.Suspender, caps common.CapabilityComparator,
	scorePred common.ScorePred) (common.OrchestratorDescriptors, error) {

	uris, err := dbo.getURLs()
	if err != nil || len(uris) <= 0 {
		return nil, err
	}

	pred := func(info *net.OrchestratorInfo) bool {
		// Return early if no ETH address is specified
		if len(info.Address) == 0 {
			return false
		}

		if err := dbo.ticketParamsValidator.ValidateTicketParams(pmTicketParams(info.TicketParams)); err != nil {
			clog.V(common.DEBUG).Infof(ctx, "invalid ticket params orch=%v err=%q",
				info.GetTranscoder(),
				err,
			)
			return false
		}

		// check if O has a valid price
		price, err := common.RatPriceInfo(info.PriceInfo)
		if err != nil {
			clog.V(common.DEBUG).Infof(ctx, "invalid price info orch=%v err=%q", info.GetTranscoder(), err)
			return false
		}
		if price == nil {
			clog.V(common.DEBUG).Infof(ctx, "no price info received for orch=%v", info.GetTranscoder())
			return false
		}
		if price.Sign() < 0 {
			clog.V(common.DEBUG).Infof(ctx, "invalid price received for orch=%v price=%v", info.GetTranscoder(), price.RatString())
			return false
		}
		return true
	}

	orchPool := NewOrchestratorPoolWithPred(dbo.bcast, uris, pred, common.Score_Untrusted, dbo.orchBlacklist, dbo.discoveryTimeout)
	orchInfos, err := orchPool.GetOrchestrators(ctx, numOrchestrators, suspender, caps, scorePred)
	if err != nil || len(orchInfos) <= 0 {
		return nil, err
	}

	return orchInfos, nil
}

func (dbo *DBOrchestratorPoolCache) Size() int {
	count, _ := dbo.store.OrchCount(
		&common.DBOrchFilter{
			CurrentRound:   dbo.rm.LastInitializedRound(),
			UpdatedLastDay: true,
		},
	)
	return count
}

func (dbo *DBOrchestratorPoolCache) SizeWith(scorePred common.ScorePred) int {
	if scorePred(common.Score_Untrusted) {
		return dbo.Size()
	}
	return 0
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

	resc, errc := make(chan *common.DBOrch, len(orchs)), make(chan error, len(orchs))
	timeout := getOrchestratorTimeoutLoop //needs to be same or longer than GRPCConnectTimeout in server/rpc.go
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
			return nil
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

	resc, errc := make(chan *common.DBOrch, len(orchs)), make(chan error, len(orchs))
	timeout := getOrchestratorTimeoutLoop //needs to be same or longer than GRPCConnectTimeout in server/rpc.go
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

		// Return early if no ETH address is specified
		if len(info.Address) == 0 {
			errc <- fmt.Errorf("missing ETH address orch=%v", info.GetTranscoder())
			return
		}

		price, err := common.RatPriceInfo(info.PriceInfo)
		if err != nil {
			errc <- fmt.Errorf("invalid price info orch=%v err=%q", info.GetTranscoder(), err)
			return
		}

		// PriceToFixed also checks if the input is nil, but this check tells us
		// which orch was missing price info
		if price == nil {
			errc <- fmt.Errorf("missing price info orch=%v", info.GetTranscoder())
			return
		}

		dbOrch.PricePerPixel, err = common.PriceToFixed(price)
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
			return nil
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
