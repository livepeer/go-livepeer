package discovery

import (
	"container/heap"
	"context"
	"encoding/hex"
	"errors"
	"math/rand"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"
)

var getOrchestratorTimeoutLoop = 3 * time.Second
var maxGetOrchestratorCutoffTimeout = 6 * time.Second

// TODO remove this hack and use orchestratorPool.getOrchInfo
var serverGetOrchInfo = server.GetOrchestratorInfo

// OrchestratorPoolConfig groups options used to construct an orchestratorPool.
type OrchestratorPoolConfig struct {
	Broadcaster      common.Broadcaster
	URIs             []*url.URL
	Pred             func(*net.OrchestratorInfo) bool
	Score            float32
	OrchBlacklist    []string
	DiscoveryTimeout time.Duration

	// Limits the number of additional nodes an orchestrator
	// can advertise within the GetOrchestratorInfo response.
	// Default 0.
	ExtraNodes int
}

type orchestratorPool struct {
	infos            []common.OrchestratorLocalInfo
	pred             func(info *net.OrchestratorInfo) bool
	bcast            common.Broadcaster
	orchBlacklist    []string
	discoveryTimeout time.Duration
	node             core.LivepeerNode
	extraNodes       int
	getOrchInfo      func(context.Context, common.Broadcaster, *url.URL, server.GetOrchestratorInfoParams) (*net.OrchestratorInfo, error)
}

func NewOrchestratorPool(bcast common.Broadcaster, uris []*url.URL, score float32, orchBlacklist []string, discoveryTimeout time.Duration) *orchestratorPool {
	pool, err := NewOrchestratorPoolWithConfig(OrchestratorPoolConfig{
		Broadcaster:      bcast,
		URIs:             uris,
		Score:            score,
		OrchBlacklist:    orchBlacklist,
		DiscoveryTimeout: discoveryTimeout,
		ExtraNodes:       bcast.ExtraNodes(),
	})
	if err != nil {
		glog.Error(err.Error())
		return &orchestratorPool{}
	}
	return pool
}

func NewOrchestratorPoolWithPred(bcast common.Broadcaster, addresses []*url.URL,
	pred func(*net.OrchestratorInfo) bool, score float32, orchBlacklist []string, discoveryTimeout time.Duration) *orchestratorPool {
	pool, err := NewOrchestratorPoolWithConfig(OrchestratorPoolConfig{
		Broadcaster:      bcast,
		URIs:             addresses,
		Pred:             pred,
		Score:            score,
		OrchBlacklist:    orchBlacklist,
		DiscoveryTimeout: discoveryTimeout,
		ExtraNodes:       bcast.ExtraNodes(),
	})
	if err != nil {
		glog.Error(err.Error())
		return &orchestratorPool{}
	}
	return pool
}

func NewOrchestratorPoolWithConfig(cfg OrchestratorPoolConfig) (*orchestratorPool, error) {
	if len(cfg.URIs) == 0 {
		return nil, errors.New("orchestrator pool config must contain at least one URI")
	}

	infos := make([]common.OrchestratorLocalInfo, 0, len(cfg.URIs))
	for _, uri := range cfg.URIs {
		infos = append(infos, common.OrchestratorLocalInfo{URL: uri, Score: cfg.Score})
	}

	return &orchestratorPool{
		infos:            infos,
		pred:             cfg.Pred,
		bcast:            cfg.Broadcaster,
		orchBlacklist:    cfg.OrchBlacklist,
		discoveryTimeout: cfg.DiscoveryTimeout,
		extraNodes:       cfg.ExtraNodes,
		getOrchInfo:      serverGetOrchInfo,
	}, nil
}

func (o *orchestratorPool) GetInfos() []common.OrchestratorLocalInfo {
	return o.infos
}

func (o *orchestratorPool) GetOrchestrators(ctx context.Context, numOrchestrators int, suspender common.Suspender, caps common.CapabilityComparator,
	scorePred common.ScorePred) (common.OrchestratorDescriptors, error) {

	var seenMu sync.Mutex
	nodesPerOrch := o.extraNodes
	seen := make(map[string]bool, len(o.infos)*nodesPerOrch)
	linfos := make([]*common.OrchestratorLocalInfo, 0, len(o.infos))
	for i, _ := range o.infos {
		if scorePred(o.infos[i].Score) {
			linfos = append(linfos, &o.infos[i])
			seen[o.infos[i].URL.String()] = true
		}
	}

	numAvailableOrchs := len(linfos)
	maxOrchNodes := numAvailableOrchs * (nodesPerOrch + 1)
	numOrchestrators = min(maxOrchNodes, numOrchestrators)

	if numOrchestrators < 0 {
		return common.OrchestratorDescriptors{}, nil
	}

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
	// Pre-declare for recursion
	var getOrchInfo func(ctx context.Context, od common.OrchestratorDescriptor, level int, infoCh chan common.OrchestratorDescriptor, errCh chan error, allOrchInfoCh chan common.OrchestratorDescriptor)

	getOrchInfo = func(ctx context.Context, od common.OrchestratorDescriptor, level int, infoCh chan common.OrchestratorDescriptor, errCh chan error, allOrchInfoCh chan common.OrchestratorDescriptor) {
		start := time.Now()
		info, err := o.getOrchInfo(ctx, o.bcast, od.LocalInfo.URL, server.GetOrchestratorInfoParams{Caps: caps.ToNetCapabilities()})
		latency := time.Since(start)
		clog.V(common.DEBUG).Infof(ctx, "Received GetOrchInfo RPC Response from uri=%v, latency=%v", od.LocalInfo.URL, latency)
		doingWork := info != nil && info.Transcoder != ""
		orchDescr := common.OrchestratorDescriptor{
			LocalInfo: &common.OrchestratorLocalInfo{
				URL:     od.LocalInfo.URL,
				Score:   od.LocalInfo.Score,
				Latency: &latency,
			},
			RemoteInfo: info,
		}
		if doingWork {
			allOrchInfoCh <- orchDescr
		}

		// discover newly advertised nodes. only recurse the first level for now.
		if level == 0 && info != nil && len(info.Nodes) > 0 {
			for i, inst := range info.Nodes {
				if i >= nodesPerOrch {
					break
				}
				seenMu.Lock()
				alreadySeen := seen[inst]
				if !alreadySeen {
					seen[inst] = true
				}
				seenMu.Unlock()
				if alreadySeen {
					continue
				}
				// haven't seen this one yet so lets continue
				u, err := url.Parse(inst)
				if err != nil {
					clog.Info(ctx, "Invalid node URL", "orch", od.LocalInfo.URL, "node", inst)
					continue
				}
				newOd := common.OrchestratorDescriptor{
					LocalInfo: &common.OrchestratorLocalInfo{URL: u, Score: od.LocalInfo.Score},
				}
				go getOrchInfo(ctx, newOd, level+1, infoCh, errCh, allOrchInfoCh)
			}
		}

		if err == nil && !isBlacklisted(info) && isCompatible(info) && doingWork {
			infoCh <- orchDescr
			return
		}

		clog.V(common.DEBUG).Infof(ctx, "Discovery unsuccessful for orchestrator %s, err=%v", od.LocalInfo.URL.String(), err)
		if err != nil && !errors.Is(err, context.Canceled) {
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
	odCh := make(chan common.OrchestratorDescriptor, maxOrchNodes)
	allOrchDescrCh := make(chan common.OrchestratorDescriptor, maxOrchNodes)
	errCh := make(chan error, maxOrchNodes)

	// Shuffle and create O descriptor
	for _, i := range rand.Perm(numAvailableOrchs) {
		go getOrchInfo(ctx, common.OrchestratorDescriptor{linfos[i], nil}, 0, odCh, errCh, allOrchDescrCh)
	}
	// TODO revert the changes from https://github.com/livepeer/go-livepeer/commit/905c3144707f1b682667828f350006c5a2e0a3a8#diff-f032101b11930379f2dd15d9a82631fa0a38d87f16760d7bc5423d25c56d3b4f
	// go reportLiveAICapacity(allOrchDescrCh, caps)

	// use a timer to time out the entire get info loop below
	cutoffTimer := time.NewTimer(maxGetOrchestratorCutoffTimeout)
	defer cutoffTimer.Stop()

	// try to wait for orchestrators until at least 1 is found (with the exponential backoff timeout)
	timeout := o.discoveryTimeout
	timer := time.NewTimer(timeout)

	// nbResp < maxOrchNodes : responses expected, whether successful or not
	// len(ods) < numOrchestrator: successful responses needed
	for nbResp < maxOrchNodes && len(ods) < numOrchestrators && !timedOut {
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
		case <-cutoffTimer.C:
			timedOut = true
		}
	}

	// Sort available orchestrators by LocalInfo.Latency ascending.
	sort.SliceStable(ods, func(i, j int) bool {
		li := ods[i].LocalInfo
		lj := ods[j].LocalInfo
		if li == nil || li.Latency == nil {
			// treat as "large" - sort to the end
			return false
		}
		if lj == nil || lj.Latency == nil {
			return true
		}
		return *li.Latency < *lj.Latency
	})

	// consider suspended orchestrators if we have an insufficient number of non-suspended ones
	if len(ods) < numOrchestrators {
		diff := numOrchestrators - len(ods)
		for i := 0; i < diff && suspendedInfos.Len() > 0; i++ {
			od := heap.Pop(suspendedInfos).(*suspension).od
			ods = append(ods, *od)
		}
	}

	if monitor.Enabled && len(ods) > 0 {
		var discoveryResults []map[string]string
		for _, o := range ods {
			discoveryResults = append(discoveryResults, map[string]string{
				"address":    hexutil.Encode(o.RemoteInfo.Address),
				"url":        o.RemoteInfo.Transcoder,
				"latency_ms": strconv.FormatInt(o.LocalInfo.Latency.Milliseconds(), 10),
			})
		}
		monitor.SendQueueEventAsync("discovery_results", discoveryResults)
	}
	clog.Infof(ctx, "Done fetching orch info orchs=%d/%d responses=%d/%d timedOut=%t",
		len(ods), numOrchestrators, nbResp, maxOrchNodes, timedOut)
	return ods, nil
}

func getModelCaps(caps *net.Capabilities) map[string]*net.Capabilities_CapabilityConstraints_ModelConstraint {
	if caps == nil || caps.Constraints == nil || caps.Constraints.PerCapability == nil {
		return nil
	}
	liveAI, ok := caps.Constraints.PerCapability[uint32(core.Capability_LiveVideoToVideo)]
	if !ok {
		return nil
	}

	return liveAI.Models
}

func reportLiveAICapacity(ch chan common.OrchestratorDescriptor, caps common.CapabilityComparator) {
	if !monitor.Enabled {
		return
	}
	modelsReq := getModelCaps(caps.ToNetCapabilities())

	var allOrchInfo []common.OrchestratorDescriptor
	var done bool
	for {
		select {
		case od := <-ch:
			allOrchInfo = append(allOrchInfo, od)
		case <-time.After(maxGetOrchestratorCutoffTimeout):
			done = true
		}
		if done {
			break
		}
	}

	idleContainersByModelAndOrchestrator := make(map[string]map[string]int)
	for _, od := range allOrchInfo {
		var models map[string]*net.Capabilities_CapabilityConstraints_ModelConstraint
		if od.RemoteInfo != nil {
			models = getModelCaps(od.RemoteInfo.Capabilities)
		}

		for modelID := range modelsReq {
			idle := 0
			if models != nil {
				if model, ok := models[modelID]; ok {
					idle = int(model.Capacity)
				}
			}

			if _, exists := idleContainersByModelAndOrchestrator[modelID]; !exists {
				idleContainersByModelAndOrchestrator[modelID] = make(map[string]int)
			}
			idleContainersByModelAndOrchestrator[modelID][od.LocalInfo.URL.String()] = idle
		}
	}
	monitor.AIContainersIdleAfterGatewayDiscovery(idleContainersByModelAndOrchestrator)
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

func (o *orchestratorPool) Broadcaster() common.Broadcaster {
	return o.bcast
}

func (o *orchestratorPool) pollOrchestratorInfo(ctx context.Context) {

}
