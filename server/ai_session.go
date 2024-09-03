package server

import (
	"context"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
)

type AISession struct {
	*BroadcastSession

	// Fields used by AISessionSelector for session lifecycle management
	Cap     core.Capability
	ModelID string
	Warm    bool
}

type AISessionPool struct {
	selector  BroadcastSessionsSelector
	sessMap   map[string]*BroadcastSession
	inUseSess []*BroadcastSession
	suspender *suspender
	penalty   int
	mu        sync.RWMutex
}

func NewAISessionPool(selector BroadcastSessionsSelector, suspender *suspender, penalty int) *AISessionPool {
	return &AISessionPool{
		selector:  selector,
		sessMap:   make(map[string]*BroadcastSession),
		suspender: suspender,
		penalty:   penalty,
		mu:        sync.RWMutex{},
	}
}

func (pool *AISessionPool) Select(ctx context.Context) *BroadcastSession {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for {
		sess := pool.selector.Select(ctx)
		if sess == nil {
			sess = pool.selectInUse()
		} else {
			// Track in-use session the first time it is returned by the selector
			pool.inUseSess = append(pool.inUseSess, sess)
		}

		if sess == nil {
			return nil
		}

		if _, ok := pool.sessMap[sess.Transcoder()]; !ok {
			// If the session is not tracked by sessMap skip it
			continue
		}

		// Track a dummy segment for the session in indicate an in-flight request
		sess.pushSegInFlight(&stream.HLSSegment{})

		return sess
	}
}

func (pool *AISessionPool) Complete(sess *BroadcastSession) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	existingSess, ok := pool.sessMap[sess.Transcoder()]
	if !ok {
		// If the session is not tracked by sessMap, skip returning it to the selector
		return
	}

	if sess != existingSess {
		// If the session is tracked by sessMap AND it is different from what is tracked by sessMap
		// skip returning it to the selector
		return
	}

	// If there are still in-flight requests for the session return early
	// and do not return the session to the selector
	inFlight, _ := sess.popSegInFlight()
	if inFlight > 0 {
		return
	}

	pool.inUseSess = removeSessionFromList(pool.inUseSess, sess)
	pool.selector.Complete(sess)
}

func (pool *AISessionPool) Add(sessions []*BroadcastSession) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	var uniqueSessions []*BroadcastSession
	for _, sess := range sessions {
		if _, ok := pool.sessMap[sess.Transcoder()]; ok {
			// Skip the session if it is already tracked by sessMap
			continue
		}

		pool.sessMap[sess.Transcoder()] = sess
		uniqueSessions = append(uniqueSessions, sess)
	}

	pool.selector.Add(uniqueSessions)
}

func (pool *AISessionPool) Remove(sess *BroadcastSession) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	delete(pool.sessMap, sess.Transcoder())
	pool.inUseSess = removeSessionFromList(pool.inUseSess, sess)

	penalty := 0
	// If this method is called assume that the orch should be suspended
	// as well.  Since AISessionManager re-uses the pools the suspension
	// penalty needs to consider the current suspender count to set the penalty
	last_count, ok := pool.suspender.list[sess.Transcoder()]
	if ok {
		penalty = pool.suspender.count - last_count + pool.penalty
	} else {
		penalty = pool.suspender.count + pool.penalty
	}

	pool.suspender.suspend(sess.Transcoder(), penalty)
}

func (pool *AISessionPool) Size() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return len(pool.sessMap)
}

func (pool *AISessionPool) selectInUse() *BroadcastSession {
	if len(pool.inUseSess) == 0 {
		return nil
	}
	// Select a random in-use session
	return pool.inUseSess[rand.Intn(len(pool.inUseSess))]
}

type AISessionSelector struct {
	// Pool of sessions with orchs that have the requested model warm
	warmPool *AISessionPool
	// Pool of sessions with orchs that have the requested model cold
	coldPool *AISessionPool
	// The time until the pools should be refreshed with orchs from discovery
	ttl             time.Duration
	lastRefreshTime time.Time
	initialPoolSize int

	cap     core.Capability
	modelID string

	node      *core.LivepeerNode
	suspender *suspender
	penalty   int
	os        drivers.OSSession
}

func NewAISessionSelector(cap core.Capability, modelID string, node *core.LivepeerNode, ttl time.Duration) (*AISessionSelector, error) {
	var stakeRdr stakeReader
	if node.Eth != nil {
		stakeRdr = &storeStakeReader{store: node.Database}
	}

	suspender := newSuspender()

	// Create caps for selector to get maxPrice
	warmCaps := newAICapabilities(cap, modelID, true, node.Capabilities.MinVersionConstraint())
	coldCaps := newAICapabilities(cap, modelID, false, node.Capabilities.MinVersionConstraint())

	// The latency score in this context is just the latency of the last completed request for a session
	// The "good enough" latency score is set to 0.0 so the selector will always select unknown sessions first
	minLS := 0.0
	// Session pool suspender starts at 0.  Suspension is 3 requests if there are errors from the orchestrator
	penalty := 3
	warmPool := NewAISessionPool(NewMinLSSelector(stakeRdr, minLS, node.SelectionAlgorithm, node.OrchPerfScore, warmCaps), suspender, penalty)
	coldPool := NewAISessionPool(NewMinLSSelector(stakeRdr, minLS, node.SelectionAlgorithm, node.OrchPerfScore, coldCaps), suspender, penalty)
	sel := &AISessionSelector{
		warmPool:  warmPool,
		coldPool:  coldPool,
		ttl:       ttl,
		cap:       cap,
		modelID:   modelID,
		node:      node,
		suspender: suspender,
		penalty:   penalty,
		os:        drivers.NodeStorage.NewSession(strconv.Itoa(int(cap)) + "_" + modelID),
	}

	if err := sel.Refresh(context.Background()); err != nil {
		return nil, err
	}

	return sel, nil
}

// newAICapabilities creates a new capabilities object with
func newAICapabilities(cap core.Capability, modelID string, warm bool, minVersion string) *core.Capabilities {
	aiCaps := []core.Capability{cap}
	capabilityConstraints := core.PerCapabilityConstraints{
		cap: &core.CapabilityConstraints{
			Models: map[string]*core.ModelConstraint{
				modelID: {Warm: warm},
			},
		},
	}

	caps := core.NewCapabilities(aiCaps, nil)
	caps.SetPerCapabilityConstraints(capabilityConstraints)
	caps.SetMinVersionConstraint(minVersion)

	return caps
}

func (sel *AISessionSelector) Select(ctx context.Context) *AISession {
	shouldRefreshSelector := func() bool {
		// Refresh if the # of sessions across warm and cold pools falls below the smaller of the maxRefreshSessionsThreshold and
		// 1/2 the total # of orchs that can be queried during discovery
		discoveryPoolSize := int(math.Min(float64(sel.node.OrchestratorPool.Size()), float64(sel.initialPoolSize)))

		if (sel.warmPool.Size() + sel.coldPool.Size()) == 0 {
			// release all orchestrators from suspension and try refresh
			// if there are no orchestrators in the pools
			clog.Infof(ctx, "refreshing sessions, no orchestrators in pools")
			for i := 0; i < sel.penalty; i++ {
				sel.suspender.signalRefresh()
			}
		}

		if sel.warmPool.Size()+sel.coldPool.Size() < int(math.Min(maxRefreshSessionsThreshold, math.Ceil(float64(discoveryPoolSize)/2.0))) {
			return true
		}

		// Refresh if the selector has expired
		if time.Now().After(sel.lastRefreshTime.Add(sel.ttl)) {
			return true
		}

		return false
	}

	if shouldRefreshSelector() {
		// Should this call be in a goroutine so the refresh can happen in the background?
		if err := sel.Refresh(ctx); err != nil {
			clog.Infof(ctx, "Error refreshing AISessionSelector err=%v", err)
		}
	}

	sess := sel.warmPool.Select(ctx)
	if sess != nil {
		return &AISession{BroadcastSession: sess, Cap: sel.cap, ModelID: sel.modelID, Warm: true}
	}

	sess = sel.coldPool.Select(ctx)
	if sess != nil {
		return &AISession{BroadcastSession: sess, Cap: sel.cap, ModelID: sel.modelID, Warm: false}
	}

	return nil
}

func (sel *AISessionSelector) Complete(sess *AISession) {
	if sess.Warm {
		sel.warmPool.Complete(sess.BroadcastSession)
	} else {
		sel.coldPool.Complete(sess.BroadcastSession)
	}
}

func (sel *AISessionSelector) Remove(sess *AISession) {
	if sess.Warm {
		sel.warmPool.Remove(sess.BroadcastSession)
	} else {
		sel.coldPool.Remove(sess.BroadcastSession)
	}
}

func (sel *AISessionSelector) Refresh(ctx context.Context) error {
	// If we try to add new sessions to the pool the suspender
	// should treat this as a refresh
	sel.suspender.signalRefresh()

	sessions, err := sel.getSessions(ctx)
	if err != nil {
		return err
	}

	var warmSessions []*BroadcastSession
	var coldSessions []*BroadcastSession

	for _, sess := range sessions {
		// If the constraints are missing for this capability skip this session
		constraints, ok := sess.OrchestratorInfo.Capabilities.Constraints.PerCapability[uint32(sel.cap)]
		if !ok {
			continue
		}

		// If the constraint for the modelID are missing skip this session
		modelConstraint, ok := constraints.Models[sel.modelID]
		if !ok {
			continue
		}

		if modelConstraint.Warm {
			warmSessions = append(warmSessions, sess)
		} else {
			coldSessions = append(coldSessions, sess)
		}
	}

	sel.warmPool.Add(warmSessions)
	sel.coldPool.Add(coldSessions)
	sel.initialPoolSize = len(warmSessions) + len(coldSessions) + len(sel.suspender.list)

	sel.lastRefreshTime = time.Now()

	return nil
}

func (sel *AISessionSelector) getSessions(ctx context.Context) ([]*BroadcastSession, error) {
	// No warm constraints applied here because we don't want to filter out orchs based on warm criteria at discovery time
	// Instead, we want all orchs that support the model and then will filter for orchs that have a warm model separately
	capabilityConstraints := core.PerCapabilityConstraints{
		sel.cap: {
			Models: map[string]*core.ModelConstraint{
				sel.modelID: {
					Warm: false,
				},
			},
		},
	}
	caps := core.NewCapabilities(append(core.DefaultCapabilities(), sel.cap), nil)
	caps.SetPerCapabilityConstraints(capabilityConstraints)
	caps.SetMinVersionConstraint(sel.node.Capabilities.MinVersionConstraint())

	// Set numOrchs to the pool size so that discovery tries to find maximum # of compatible orchs within a timeout
	numOrchs := sel.node.OrchestratorPool.Size()

	// Use a dummy manifestID specific to the capability + modelID
	// Typically, a manifestID would identify a stream
	// In the AI context, a manifestID can identify a capability + modelID and each
	// request for the capability + modelID can be thought of as a part of the same "stream"
	manifestID := strconv.Itoa(int(sel.cap)) + "_" + sel.modelID
	streamParams := &core.StreamParameters{
		ManifestID:   core.ManifestID(manifestID),
		Capabilities: caps,
		OS:           sel.os,
	}
	// TODO: Implement cleanup for AI sessions.
	return selectOrchestrator(ctx, sel.node, streamParams, numOrchs, sel.suspender, common.ScoreAtLeast(0), func(sessionID string) {})
}

type AISessionManager struct {
	node      *core.LivepeerNode
	selectors map[string]*AISessionSelector
	mu        sync.Mutex
	ttl       time.Duration
}

func NewAISessionManager(node *core.LivepeerNode, ttl time.Duration) *AISessionManager {
	return &AISessionManager{
		node:      node,
		selectors: make(map[string]*AISessionSelector),
		mu:        sync.Mutex{},
		ttl:       ttl,
	}
}

func (c *AISessionManager) Select(ctx context.Context, cap core.Capability, modelID string) (*AISession, error) {
	sel, err := c.getSelector(ctx, cap, modelID)
	if err != nil {
		return nil, err
	}

	sess := sel.Select(ctx)
	if sess == nil {
		return nil, nil
	}

	if err := refreshSessionIfNeeded(ctx, sess.BroadcastSession); err != nil {
		return nil, err
	}

	clog.Infof(ctx, "selected orchestrator=%s", sess.Transcoder())

	return sess, nil
}

func (c *AISessionManager) Remove(ctx context.Context, sess *AISession) error {
	sel, err := c.getSelector(ctx, sess.Cap, sess.ModelID)
	if err != nil {
		return err
	}

	sel.Remove(sess)

	return nil
}

func (c *AISessionManager) Complete(ctx context.Context, sess *AISession) error {
	sel, err := c.getSelector(ctx, sess.Cap, sess.ModelID)
	if err != nil {
		return err
	}

	sel.Complete(sess)

	return nil
}

func (c *AISessionManager) getSelector(ctx context.Context, cap core.Capability, modelID string) (*AISessionSelector, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheKey := strconv.Itoa(int(cap)) + "_" + modelID
	sel, ok := c.selectors[cacheKey]
	if !ok {
		// Create the selector
		var err error
		sel, err = NewAISessionSelector(cap, modelID, c.node, c.ttl)
		if err != nil {
			return nil, err
		}

		c.selectors[cacheKey] = sel
	}

	return sel, nil
}
