package server

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/url"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
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

	p := &remoteDiscoveryPool{
		node:         cfg.Node,
		refreshEvery: refreshEvery,
	}
	return p
}

type remoteDiscoveryPool struct {
	node *core.LivepeerNode

	mu          sync.RWMutex
	cached      []remoteDiscoveryOrchestrator
	caps        map[string][]remoteDiscoveryOrchestrator
	lastRefresh time.Time

	refreshEvery time.Duration
}

type remoteDiscoveryOrchestrator struct {
	URL          *url.URL
	Capabilities []string
	Runners      []runner.LiveRunnerDiscoveryRunner
}

type remoteDiscoveryMergeEntry struct {
	orch       remoteDiscoveryOrchestrator
	runners    map[string]runner.LiveRunnerDiscoveryRunner
	runnerURLs []string
}

type remoteDiscoveryEntry struct {
	Address string                             `json:"address"`
	Runners []runner.LiveRunnerDiscoveryRunner `json:"runners,omitempty"`
}

func (o remoteDiscoveryOrchestrator) clone() remoteDiscoveryOrchestrator {
	orch := o
	orch.Capabilities = append([]string(nil), o.Capabilities...)
	orch.Runners = append([]runner.LiveRunnerDiscoveryRunner(nil), o.Runners...)
	return orch
}

func (p *remoteDiscoveryPool) Orchestrators(caps []string) []remoteDiscoveryOrchestrator {
	p.refresh()

	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(caps) == 0 {
		return cloneRemoteDiscoveryOrchestrators(p.cached)
	}

	var cached []remoteDiscoveryOrchestrator
	seen := make(map[string]bool)
	for _, key := range caps {
		for _, orch := range p.caps[key] {
			u := orch.URL.String()
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
	p.refresh()

	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cached)
}

func (p *remoteDiscoveryPool) refresh() {
	if p == nil {
		return
	}

	// This can run on the request path when cache is stale, so keep it fast.
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	// Rate-limit non-empty snapshots.
	if len(p.cached) > 0 && !p.lastRefresh.IsZero() && now.Sub(p.lastRefresh) <= p.refreshEvery {
		return
	}

	var networkCaps []*common.OrchNetworkCapabilities
	if p.node != nil {
		networkCaps = p.node.GetNetworkCapabilities()
	}

	merged := make(map[string]*remoteDiscoveryMergeEntry)
	var order []string
	// A node is one discovery `address` value. A single orchestrator may expose
	// multiple nodes/addresses through its /discovery response.
	validNodes := make(map[string]bool)
	// Create or reuse the accumulator for a node. The same node address can be
	// seen from normal polling and from multiple /discovery responses, so merge
	// everything for that address into one final /discover-orchestrators entry.
	ensureMerged := func(addr string, orchURL *url.URL) *remoteDiscoveryMergeEntry {
		// Treat root service URIs with and without a trailing slash as the same address.
		key := strings.TrimRight(addr, "/")
		if entry := merged[key]; entry != nil {
			return entry
		}
		entry := &remoteDiscoveryMergeEntry{
			orch: remoteDiscoveryOrchestrator{
				URL: orchURL,
			},
			runners: make(map[string]runner.LiveRunnerDiscoveryRunner),
		}
		merged[key] = entry
		order = append(order, key)
		return entry
	}

	for _, orchCaps := range networkCaps {
		if orchCaps == nil || orchCaps.OrchURI == "" {
			continue
		}
		orchURL, err := url.ParseRequestURI(orchCaps.OrchURI)
		if err != nil {
			continue
		}
		// Treat root service URIs with and without a trailing slash as the same address.
		key := strings.TrimRight(orchURL.String(), "/")
		eligibleCaps := make(map[string]bool)
		// Capabilities in discovery are price-eligible capabilities only.
		forEachRemoteDiscoveryCapability(orchCaps, func(key string, capability core.Capability, modelID string) {
			price := capabilityPrice(orchCaps, capability, modelID)
			if price == nil {
				return
			}
			maxPrice := capabilityMaxPrice(capability, modelID)
			if maxPrice != nil && price.Cmp(maxPrice) > 0 {
				return
			}
			eligibleCaps[key] = true
		})
		// Drop orchestrators if it doesn't have any eligible capabilities
		if len(eligibleCaps) == 0 {
			continue
		}
		validNodes[key] = true
		entry := ensureMerged(orchURL.String(), orchURL)
		for cap := range eligibleCaps {
			entry.orch.Capabilities = append(entry.orch.Capabilities, cap)
		}
		sort.Strings(entry.orch.Capabilities)
	}

	for _, orchCaps := range networkCaps {
		if orchCaps == nil {
			continue
		}
		for _, discovery := range remoteDiscoveryEntries(orchCaps.Discovery) {
			if discovery.Address == "" {
				continue
			}
			// Treat root service URIs with and without a trailing slash as the same address.
			key := strings.TrimRight(discovery.Address, "/")
			if validNodes[key] {
				mergeDiscoveryRunners(merged[key], discovery)
				continue
			}
			orchURL, err := url.ParseRequestURI(discovery.Address)
			if err != nil {
				continue
			}
			entry := ensureMerged(discovery.Address, orchURL)
			mergeDiscoveryRunners(entry, discovery)
		}
	}

	cached := make([]remoteDiscoveryOrchestrator, 0, len(order))
	caps := make(map[string][]remoteDiscoveryOrchestrator)
	for _, key := range order {
		entry := merged[key]
		if entry == nil || entry.orch.URL == nil || len(entry.orch.Capabilities) == 0 {
			continue
		}
		sort.Strings(entry.orch.Capabilities)
		entry.orch.Runners = make([]runner.LiveRunnerDiscoveryRunner, 0, len(entry.runnerURLs))
		for _, url := range entry.runnerURLs {
			entry.orch.Runners = append(entry.orch.Runners, entry.runners[url])
		}
		for _, cap := range entry.orch.Capabilities {
			caps[cap] = append(caps[cap], entry.orch.clone())
		}
		cached = append(cached, entry.orch)
	}

	// Publish the cache/index snapshot atomically.
	p.cached = cached
	p.caps = caps
	p.lastRefresh = now
}

func cloneRemoteDiscoveryOrchestrators(cached []remoteDiscoveryOrchestrator) []remoteDiscoveryOrchestrator {
	cloned := make([]remoteDiscoveryOrchestrator, 0, len(cached))
	for _, orch := range cached {
		cloned = append(cloned, orch.clone())
	}
	return cloned
}

func mergeDiscoveryRunners(entry *remoteDiscoveryMergeEntry, discovery remoteDiscoveryEntry) {
	// For each address, merge runners by URL. First wins; matching duplicates are
	// ignored and conflicting duplicates are logged before keeping the first.
	for _, r := range discovery.Runners {
		if err := entry.validateRunner(r); err != nil {
			orchURL := ""
			if entry != nil && entry.orch.URL != nil {
				orchURL = entry.orch.URL.String()
			}
			clog.Info(context.Background(), "Filtered remote discovery runner", "orch", orchURL, "runner", r.URL, "app", r.App, "reason", err.Error())
			continue
		}
		if !slices.Contains(entry.orch.Capabilities, r.App) {
			entry.orch.Capabilities = append(entry.orch.Capabilities, r.App)
		}
		if existing, ok := entry.runners[r.URL]; ok {
			if !sameRemoteDiscoveryRunner(existing, r) {
				clog.Warningf(context.Background(), "Conflicting remote discovery runner for orchestrator=%s runner=%s; keeping first", entry.orch.URL.String(), r.URL)
			}
			continue
		}
		entry.runners[r.URL] = r
		entry.runnerURLs = append(entry.runnerURLs, r.URL)
	}
}

func sameRemoteDiscoveryRunner(a, b runner.LiveRunnerDiscoveryRunner) bool {
	// Capacity usage is expected to move between discovery polls. Ignore it
	// when deciding whether duplicate runner URLs represent conflicting metadata.
	a.CapacityUsed = 0
	a.CapacityAvailable = 0
	b.CapacityUsed = 0
	b.CapacityAvailable = 0
	return reflect.DeepEqual(a, b)
}

func remoteDiscoveryEntries(raw json.RawMessage) []remoteDiscoveryEntry {
	if len(raw) == 0 {
		return nil
	}
	var entries []remoteDiscoveryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	return entries
}

func (entry *remoteDiscoveryMergeEntry) validateRunner(runner runner.LiveRunnerDiscoveryRunner) error {
	if runner.URL == "" {
		return errors.New("missing URL")
	}
	if strings.TrimSpace(runner.App) == "" {
		return errors.New("missing app")
	}
	price, err := validateRunnerPrice(runner.PriceInfo)
	if err != nil {
		return err
	}
	maxPrice := BroadcastCfg.MaxPrice()
	if maxPrice != nil {
		if price.Cmp(maxPrice) > 0 {
			return errors.New("above global max price")
		}
	}
	return nil
}

func validateRunnerPrice(priceInfo *runner.LiveRunnerPriceInfo) (*big.Rat, error) {
	price, ok := runnerPrice(priceInfo)
	if !ok {
		return nil, errors.New("invalid price")
	}
	currency := strings.ToLower(strings.TrimSpace(priceInfo.Currency))
	if currency != "wei" {
		return nil, errors.New("unsupported currency")
	}
	unit := strings.ToLower(strings.TrimSpace(priceInfo.Unit))
	if unit != "seconds" && unit != "720p-pixel-seconds" {
		return nil, errors.New("unsupported unit")
	}
	return price, nil
}

func runnerPrice(priceInfo *runner.LiveRunnerPriceInfo) (*big.Rat, bool) {
	if priceInfo == nil {
		return nil, false
	}
	price, ok := new(big.Rat).SetString(strings.TrimSpace(priceInfo.Price.String()))
	if !ok || price.Sign() <= 0 {
		return nil, false
	}
	return price, true
}

func forEachRemoteDiscoveryCapability(info *common.OrchNetworkCapabilities, f func(key string, capability core.Capability, modelID string)) {
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

func capabilityPrice(info *common.OrchNetworkCapabilities, capability core.Capability, modelID string) *big.Rat {
	if info == nil {
		return nil
	}
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
	// Global fallback if no per-capability price is available.
	if info.PriceInfo == nil || info.PriceInfo.PixelsPerUnit <= 0 {
		return nil
	}
	return new(big.Rat).SetFrac64(info.PriceInfo.PricePerUnit, info.PriceInfo.PixelsPerUnit)
}

func capabilityMaxPrice(capability core.Capability, modelID string) *big.Rat {
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
