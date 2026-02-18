package server

import (
	"context"
	"math/big"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
)

var remoteDiscoveryRefreshInterval = common.WebhookDiscoveryRefreshInterval

type RemoteDiscoveryConfig struct {
	Pool     common.OrchestratorPool
	Node     *core.LivepeerNode
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
		node:         cfg.Node,
		refreshEvery: refreshEvery,
		ctx:          ctx,
		cancel:       cancel,
	}
	go p.loop()
	return p
}

type remoteDiscoveryPool struct {
	pool common.OrchestratorPool
	node *core.LivepeerNode

	mu     sync.RWMutex
	cached []remoteDiscoveryOrchestrator
	caps   map[string][]remoteDiscoveryOrchestrator

	refreshEvery time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
}

type remoteDiscoveryOrchestrator struct {
	OD           common.OrchestratorDescriptor
	Capabilities []string
}

func (o remoteDiscoveryOrchestrator) clone() remoteDiscoveryOrchestrator {
	orch := o
	orch.Capabilities = append([]string(nil), o.Capabilities...)
	return orch
}

func (p *remoteDiscoveryPool) Orchestrators(caps []string) []remoteDiscoveryOrchestrator {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(caps) == 0 {
		return cloneRemoteDiscoveryOrchestrators(p.cached)
	}

	var cached []remoteDiscoveryOrchestrator
	seen := make(map[string]bool)
	for _, key := range caps {
		for _, orch := range p.caps[key] {
			u := orch.OD.LocalInfo.URL.String()
			if seen[u] {
				continue
			}
			seen[u] = true
			cached = append(cached, orch.clone())
		}
	}
	return cached
}

func (p *remoteDiscoveryPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cached)
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
	var ods common.OrchestratorDescriptors
	if p.node != nil {
		networkCaps := p.node.GetNetworkCapabilities()
		ods = make(common.OrchestratorDescriptors, 0, len(networkCaps))
		for _, orchCaps := range networkCaps {
			if orchCaps == nil || orchCaps.OrchURI == "" {
				continue
			}
			orchURL, err := url.ParseRequestURI(orchCaps.OrchURI)
			if err != nil {
				continue
			}
			ods = append(ods, common.OrchestratorDescriptor{
				LocalInfo: &common.OrchestratorLocalInfo{
					URL:   orchURL,
					Score: common.Score_Trusted,
				},
				RemoteInfo: &net.OrchestratorInfo{
					Transcoder:         orchCaps.OrchURI,
					Capabilities:       orchCaps.Capabilities,
					PriceInfo:          orchCaps.PriceInfo,
					CapabilitiesPrices: orchCaps.CapabilitiesPrices,
					Hardware:           orchCaps.Hardware,
				},
			})
		}
	}

	cached := make([]remoteDiscoveryOrchestrator, 0, len(ods))
	caps := make(map[string][]remoteDiscoveryOrchestrator)
	for _, od := range ods {
		if od.LocalInfo == nil || od.LocalInfo.URL == nil {
			continue
		}
		entry := remoteDiscoveryOrchestrator{
			OD: od,
		}
		eligibleCaps := make([]string, 0)
		// Capabilities in discovery are price-eligible capabilities only.
		forEachRemoteDiscoveryCapability(od.RemoteInfo, func(key string, capability core.Capability, modelID string) {
			price := capabilityPrice(od.RemoteInfo, capability, modelID)
			maxPrice := capabilityMaxPrice(capability, modelID)
			if maxPrice != nil && price != nil && price.Cmp(maxPrice) > 0 {
				return
			}
			eligibleCaps = append(eligibleCaps, key)
		})
		// Drop orchestrators if it doesn't have any eligible capabilities
		if len(eligibleCaps) == 0 {
			continue
		}
		sort.Strings(eligibleCaps)
		entry.Capabilities = eligibleCaps
		// Keep the capability index aligned with the orchestrator capability list.
		for _, key := range eligibleCaps {
			caps[key] = append(caps[key], entry.clone())
		}
		cached = append(cached, entry)
	}

	// Publish the cache/index snapshot atomically.
	p.mu.Lock()
	p.cached = cached
	p.caps = caps
	p.mu.Unlock()
}

func cloneRemoteDiscoveryOrchestrators(cached []remoteDiscoveryOrchestrator) []remoteDiscoveryOrchestrator {
	cloned := make([]remoteDiscoveryOrchestrator, 0, len(cached))
	for _, orch := range cached {
		cloned = append(cloned, orch.clone())
	}
	return cloned
}

func forEachRemoteDiscoveryCapability(info *net.OrchestratorInfo, f func(key string, capability core.Capability, modelID string)) {
	if info == nil || info.Capabilities == nil || info.Capabilities.Constraints == nil {
		return
	}

	for capabilityInt, constraints := range info.Capabilities.Constraints.PerCapability {
		if constraints == nil {
			continue
		}
		pipeline := capabilityToPipeline(core.Capability(capabilityInt))
		if pipeline == "" {
			continue
		}
		for modelID := range constraints.Models {
			if modelID == "" {
				continue
			}
			f(pipeline+"/"+modelID, core.Capability(capabilityInt), modelID)
		}
	}
}

func capabilityToPipeline(capability core.Capability) string {
	name, err := core.CapabilityToName(capability)
	if err != nil || len(name) == 0 {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

func capabilityPrice(info *net.OrchestratorInfo, capability core.Capability, modelID string) *big.Rat {
	if info != nil {
		// Check per-capability price if it exists
		for _, capPrice := range info.CapabilitiesPrices {
			if capPrice == nil || capPrice.PixelsPerUnit <= 0 || core.Capability(capPrice.Capability) != capability {
				continue
			}
			price := new(big.Rat).SetFrac64(capPrice.PricePerUnit, capPrice.PixelsPerUnit)
			if capPrice.Constraint == modelID {
				return price
			}
		}
	}
	// Global fallback if no per-capability price is available.
	if info == nil || info.PriceInfo == nil || info.PriceInfo.PixelsPerUnit <= 0 {
		return nil
	}
	return new(big.Rat).SetFrac64(info.PriceInfo.PricePerUnit, info.PriceInfo.PixelsPerUnit)
}

func capabilityMaxPrice(capability core.Capability, modelID string) *big.Rat {
	defer func() {
		_ = recover()
	}()

	caps := core.NewCapabilities([]core.Capability{capability}, nil)
	caps.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
		capability: &core.CapabilityConstraints{
			Models: map[string]*core.ModelConstraint{
				modelID: {},
			},
		},
	})
	// Broadcast config applies model-specific max price with "default" fallback.
	return BroadcastCfg.GetCapabilitiesMaxPrice(caps)
}
