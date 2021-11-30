package discovery

import (
	"container/heap"
	"context"
	"errors"
	"math"
	"math/rand"
	"net/url"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

var getOrchestratorsTimeoutLoop = 3 * time.Second
var getOrchestratorsCutoffTimeout = 500 * time.Millisecond

var serverGetOrchInfo = server.GetOrchestratorInfo

type orchestratorPool struct {
	infos []common.OrchestratorLocalInfo
	pred  func(info *net.OrchestratorInfo) bool
	bcast common.Broadcaster
}

func NewOrchestratorPool(bcast common.Broadcaster, uris []*url.URL, score float32) *orchestratorPool {
	if len(uris) <= 0 {
		// Should we return here?
		glog.Error("Orchestrator pool does not have any URIs")
	}
	infos := make([]common.OrchestratorLocalInfo, 0, len(uris))
	for _, uri := range uris {
		infos = append(infos, common.OrchestratorLocalInfo{URL: uri, Score: score})
	}

	return &orchestratorPool{infos: infos, bcast: bcast}
}

func NewOrchestratorPoolWithPred(bcast common.Broadcaster, addresses []*url.URL,
	pred func(*net.OrchestratorInfo) bool, score float32) *orchestratorPool {

	pool := NewOrchestratorPool(bcast, addresses, score)
	pool.pred = pred
	return pool
}

func (o *orchestratorPool) GetInfos() []common.OrchestratorLocalInfo {
	return o.infos
}

func (o *orchestratorPool) GetInfo(uri string) common.OrchestratorLocalInfo {
	var res common.OrchestratorLocalInfo
	for _, info := range o.infos {
		if info.URL.String() == uri {
			res = info
			break
		}
	}
	return res
}

func (o *orchestratorPool) GetOrchestrators(logCtx context.Context, numOrchestrators int, suspender common.Suspender, caps common.CapabilityComparator,
	scorePred common.ScorePred) ([]*net.OrchestratorInfo, error) {

	linfos := make([]common.OrchestratorLocalInfo, 0, len(o.infos))
	for _, info := range o.infos {
		if scorePred(info.Score) {
			linfos = append(linfos, info)
		}
	}

	numAvailableOrchs := len(linfos)
	numOrchestrators = int(math.Min(float64(numAvailableOrchs), float64(numOrchestrators)))
	ctx, cancel := context.WithTimeout(context.Background(), getOrchestratorsCutoffTimeout)

	infoCh := make(chan *net.OrchestratorInfo, numAvailableOrchs)
	errCh := make(chan error, numAvailableOrchs)

	// The following allows us to avoid capability check for jobs that only
	// depend on "legacy" features, since older orchestrators support these
	// features without capability discovery. This enables interop between
	// older orchestrators and newer orchestrators as long as the job only
	// requires the legacy feature set.
	//
	// When / if it's justified to completely break interop with older
	// orchestrators, then we can probably remove this check and work with
	// the assumption that all orchestrators support capability discovery.
	legacyCapsOnly := caps.LegacyOnly()

	isCompatible := func(info *net.OrchestratorInfo) bool {
		if o.pred != nil && !o.pred(info) {
			return false
		}
		// Legacy features already have support on the orchestrator.
		// Capabilities can be omitted in this case for older orchestrators.
		// Otherwise, capabilities are required to be present.
		if info.Capabilities == nil {
			if legacyCapsOnly {
				return true
			}
			return false
		}
		return caps.CompatibleWith(info.Capabilities)
	}
	getOrchInfo := func(uri *url.URL) {
		info, err := serverGetOrchInfo(ctx, o.bcast, uri)
		if err == nil && isCompatible(info) {
			infoCh <- info
			return
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			clog.Errorf(logCtx, "err=%q", err)
			if monitor.Enabled {
				monitor.LogDiscoveryError(err.Error())
			}
		}
		errCh <- err
	}

	// Shuffle into new slice to avoid mutating underlying data
	uris := make([]*url.URL, numAvailableOrchs)
	for i, j := range rand.Perm(numAvailableOrchs) {
		uris[i] = linfos[j].URL
	}

	for _, uri := range uris {
		go getOrchInfo(uri)
	}

	timeout := false
	infos := []*net.OrchestratorInfo{}
	suspendedInfos := newSuspensionQueue()
	nbResp := 0
	for i := 0; i < numAvailableOrchs && len(infos) < numOrchestrators && !timeout; i++ {
		select {
		case info := <-infoCh:
			if penalty := suspender.Suspended(info.Transcoder); penalty == 0 {
				infos = append(infos, info)
			} else {
				heap.Push(suspendedInfos, &suspension{info, penalty})
			}
			nbResp++
		case <-errCh:
			nbResp++
		case <-ctx.Done():
			timeout = true
		}
	}
	cancel()

	if len(infos) < numOrchestrators {
		diff := numOrchestrators - len(infos)
		for i := 0; i < diff && suspendedInfos.Len() > 0; i++ {
			info := heap.Pop(suspendedInfos).(*suspension).orch
			infos = append(infos, info)
		}
	}

	glog.Infof("Done fetching orch info numOrch=%d responses=%d/%d timeout=%t",
		len(infos), nbResp, len(uris), timeout)
	return infos, nil
}

func (o *orchestratorPool) Size() int {
	return len(o.infos)
}

func (o *orchestratorPool) SizeWith(scorePred common.ScorePred) int {
	var size int
	for _, info := range o.infos {
		if scorePred(info.Score) {
			size++
		}
	}
	return size
}
