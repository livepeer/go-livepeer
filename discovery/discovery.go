package discovery

import (
	"container/heap"
	"context"
	"encoding/hex"
	"errors"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

var getOrchestratorTimeoutLoop = 3 * time.Second
var maxGetOrchestratorCutoffTimeout = 6 * time.Second

var serverGetOrchInfo = server.GetOrchestratorInfo

type orchestratorPool struct {
	infos            []common.OrchestratorLocalInfo
	pred             func(info *net.OrchestratorInfo) bool
	bcast            common.Broadcaster
	orchBlacklist    []string
	discoveryTimeout time.Duration
}

func NewOrchestratorPool(bcast common.Broadcaster, uris []*url.URL, score float32, orchBlacklist []string, discoveryTimeout time.Duration) *orchestratorPool {
	if len(uris) <= 0 {
		// Should we return here?
		glog.Error("Orchestrator pool does not have any URIs")
	}
	infos := make([]common.OrchestratorLocalInfo, 0, len(uris))
	for _, uri := range uris {
		infos = append(infos, common.OrchestratorLocalInfo{URL: uri, Score: score})
	}
	return &orchestratorPool{infos: infos, bcast: bcast, orchBlacklist: orchBlacklist, discoveryTimeout: discoveryTimeout}
}

func NewOrchestratorPoolWithPred(bcast common.Broadcaster, addresses []*url.URL,
	pred func(*net.OrchestratorInfo) bool, score float32, orchBlacklist []string, discoveryTimeout time.Duration) *orchestratorPool {

	pool := NewOrchestratorPool(bcast, addresses, score, orchBlacklist, discoveryTimeout)
	pool.pred = pred
	return pool
}

func (o *orchestratorPool) GetInfos() []common.OrchestratorLocalInfo {
	return o.infos
}

func (o *orchestratorPool) GetOrchestrators(ctx context.Context, numOrchestrators int, suspender common.Suspender, caps common.CapabilityComparator,
	scorePred common.ScorePred) (common.OrchestratorDescriptors, error) {

	linfos := make([]*common.OrchestratorLocalInfo, 0, len(o.infos))
	for i, _ := range o.infos {
		if scorePred(o.infos[i].Score) {
			linfos = append(linfos, &o.infos[i])
		}
	}

	numAvailableOrchs := len(linfos)
	numOrchestrators = int(math.Min(float64(numAvailableOrchs), float64(numOrchestrators)))

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

	isBlacklisted := func(info *net.OrchestratorInfo) bool {
		for _, blacklisted := range o.orchBlacklist {
			if strings.TrimPrefix(blacklisted, "0x") == strings.ToLower(hex.EncodeToString(info.Address)) {
				return true
			}
		}
		return false
	}

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
	getOrchInfo := func(ctx context.Context, od common.OrchestratorDescriptor, infoCh chan common.OrchestratorDescriptor, errCh chan error) {
		info, err := serverGetOrchInfo(ctx, o.bcast, od.LocalInfo.URL)
		if err == nil && !isBlacklisted(info) && isCompatible(info) {
			od.RemoteInfo = info
			infoCh <- od
			return
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			clog.V(common.DEBUG).Infof(ctx, "err=%q", err)
			if monitor.Enabled {
				monitor.LogDiscoveryError(ctx, od.LocalInfo.URL.String(), err.Error())
			}
		}
		errCh <- err
	}

	var ods common.OrchestratorDescriptors
	suspendedInfos := newSuspensionQueue()
	timedOut := false
	nbResp := 0
	odCh := make(chan common.OrchestratorDescriptor, numAvailableOrchs)
	errCh := make(chan error, numAvailableOrchs)

	ctx, cancel := context.WithTimeout(clog.Clone(context.Background(), ctx), maxGetOrchestratorCutoffTimeout)

	// Shuffle and create O descriptor
	for _, i := range rand.Perm(numAvailableOrchs) {
		go getOrchInfo(ctx, common.OrchestratorDescriptor{linfos[i], nil}, odCh, errCh)
	}

	// try to wait for orchestrators until at least 1 is found (with the exponential backoff timout)
	timeout := o.discoveryTimeout
	timer := time.NewTimer(timeout)

	for nbResp < numAvailableOrchs && len(ods) < numOrchestrators && !timedOut {
		select {
		case od := <-odCh:
			if penalty := suspender.Suspended(od.RemoteInfo.Transcoder); penalty == 0 {
				ods = append(ods, od)
			} else {
				heap.Push(suspendedInfos, &suspension{od.RemoteInfo, &od, penalty})
			}
			nbResp++
		case <-errCh:
			nbResp++
		case <-timer.C:
			if len(ods) > 0 {
				timedOut = true
			}

			// At this point we already waited timeout, so need to wait another timeout to make it the increased 2 * timeout
			timer.Reset(timeout)
			timeout *= 2
			if timeout > maxGetOrchestratorCutoffTimeout {
				timeout = maxGetOrchestratorCutoffTimeout
			}
			clog.V(common.DEBUG).Infof(ctx, "No orchestrators found, increasing discovery timeout to %s", timeout)
		case <-ctx.Done():
			timedOut = true
		}
	}
	cancel()

	// consider suspended orchestrators if we have an insufficient number of non-suspended ones
	if len(ods) < numOrchestrators {
		diff := numOrchestrators - len(ods)
		for i := 0; i < diff && suspendedInfos.Len() > 0; i++ {
			od := heap.Pop(suspendedInfos).(*suspension).od
			ods = append(ods, *od)
		}
	}

	clog.Infof(ctx, "Done fetching orch info numOrch=%d responses=%d/%d timedOut=%t",
		len(ods), nbResp, len(linfos), timedOut)
	return ods, nil
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
