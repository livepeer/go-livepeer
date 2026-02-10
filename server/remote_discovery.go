package server

import (
	"context"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
)

const remoteDiscoveryRefreshTimeout = 15 * time.Second

var remoteDiscoveryRefreshInterval = common.WebhookDiscoveryRefreshInterval

type RemoteDiscoveryConfig struct {
	Pool     common.OrchestratorPool
	Interval time.Duration
}

func (cfg RemoteDiscoveryConfig) New() *remoteDiscoveryPool {
	refreshEvery := cfg.Interval
	if refreshEvery <= 0 {
		refreshEvery = remoteDiscoveryRefreshInterval
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &remoteDiscoveryPool{
		pool:         cfg.Pool,
		refreshEvery: refreshEvery,
		ctx:          ctx,
		cancel:       cancel,
	}
	go p.loop()
	return p
}

type remoteDiscoveryPool struct {
	pool common.OrchestratorPool

	mu     sync.RWMutex
	cached common.OrchestratorDescriptors

	refreshEvery time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
}

func (p *remoteDiscoveryPool) Orchestrators() common.OrchestratorDescriptors {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ods := make(common.OrchestratorDescriptors, len(p.cached))
	copy(ods, p.cached)
	return ods
}

func (p *remoteDiscoveryPool) Stop() {
	p.cancel()
}

func (p *remoteDiscoveryPool) loop() {
	p.refresh()

	t := time.NewTicker(p.refreshEvery)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			p.refresh()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *remoteDiscoveryPool) refresh() {
	ctx, cancel := context.WithTimeout(p.ctx, remoteDiscoveryRefreshTimeout)
	defer cancel()

	ods, err := p.pool.GetOrchestrators(
		ctx,
		p.pool.Size(),
		newSuspender(),
		core.NewCapabilities(nil, nil),
		common.ScoreAtLeast(common.Score_Untrusted),
	)

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		return
	}

	p.cached = ods
}
