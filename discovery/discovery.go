package discovery

import (
	"container/heap"
	"context"
	"math"
	"math/rand"
	"net/url"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

var getOrchestratorsTimeoutLoop = 3 * time.Second

var serverGetOrchInfo = server.GetOrchestratorInfo

type orchestratorPool struct {
	uris  []*url.URL
	pred  func(info *net.OrchestratorInfo) bool
	bcast common.Broadcaster
}

func NewOrchestratorPool(bcast common.Broadcaster, uris []*url.URL) *orchestratorPool {
	if len(uris) <= 0 {
		// Should we return here?
		glog.Error("Orchestrator pool does not have any URIs")
	}

	return &orchestratorPool{uris: uris, bcast: bcast}
}

func NewOrchestratorPoolWithPred(bcast common.Broadcaster, addresses []*url.URL, pred func(*net.OrchestratorInfo) bool) *orchestratorPool {
	pool := NewOrchestratorPool(bcast, addresses)
	pool.pred = pred
	return pool
}

func (o *orchestratorPool) GetURLs() []*url.URL {
	return o.uris
}

func (o *orchestratorPool) GetOrchestrators(numOrchestrators int, suspender common.Suspender) ([]*net.OrchestratorInfo, error) {
	numAvailableOrchs := len(o.uris)
	numOrchestrators = int(math.Min(float64(numAvailableOrchs), float64(numOrchestrators)))
	ctx, cancel := context.WithTimeout(context.Background(), getOrchestratorsTimeoutLoop)

	infoCh := make(chan *net.OrchestratorInfo, numAvailableOrchs)
	errCh := make(chan error, numAvailableOrchs)
	getOrchInfo := func(uri *url.URL) {
		info, err := serverGetOrchInfo(ctx, o.bcast, uri)
		if err == nil && (o.pred == nil || o.pred(info)) {
			infoCh <- info
			return
		}
		if err != nil && monitor.Enabled {
			monitor.LogDiscoveryError(err.Error())
		}
		errCh <- err
	}

	// Shuffle into new slice to avoid mutating underlying data
	uris := make([]*url.URL, numAvailableOrchs)
	for i, j := range rand.Perm(numAvailableOrchs) {
		uris[i] = o.uris[j]
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
	return len(o.uris)
}
