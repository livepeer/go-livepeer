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
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
)

const aiLiveVideoToVideoPenalty = 5

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

	maxTryCount := 20
	for try := 1; try <= maxTryCount; try++ {
		clog.V(common.DEBUG).Infof(ctx, "Selecting orchestrator, try=%d", try)
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
	clog.Warningf(ctx, "Selecting orchestrator failed, max tries number reached")
	return nil
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
		if existingSess, ok := pool.sessMap[sess.Transcoder()]; ok {
			// For existing sessions we only update its fields
			existingSess.OrchestratorInfo = sess.OrchestratorInfo
			existingSess.InitialLatency = sess.InitialLatency
			existingSess.InitialPrice = sess.InitialPrice
			existingSess.PMSessionID = sess.PMSessionID
			continue
		}

		pool.sessMap[sess.Transcoder()] = sess
		uniqueSessions = append(uniqueSessions, sess)
	}

	pool.selector.Add(uniqueSessions)
}

// Clear clears the session that does not exist in newSessions
func (pool *AISessionPool) Clear(newSessions []*BroadcastSession) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Clear the sessions that are not in newSessions
	for k, _ := range pool.sessMap {
		toRemove := true
		for _, sess := range newSessions {
			if k == sess.Transcoder() {
				// This session is in the new sessions list
				toRemove = false
				break
			}
		}
		if toRemove {
			pool.selector.Remove(pool.sessMap[k])
			pool.inUseSess = removeSessionFromList(pool.inUseSess, pool.sessMap[k])
			delete(pool.sessMap, k)
		}
	}

}

func (pool *AISessionPool) Remove(sess *BroadcastSession) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	delete(pool.sessMap, sess.Transcoder())
	pool.inUseSess = removeSessionFromList(pool.inUseSess, sess)

	// If this method is called assume that the orch should be suspended
	// as well.  Since AISessionManager re-uses the pools the suspension
	// penalty needs to consider the current suspender count to set the penalty
	lastCount, ok := pool.suspender.list[sess.Transcoder()]
	penalty := pool.suspender.count + pool.penalty
	if ok {
		penalty -= lastCount
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
	autoClear       bool

	cap     core.Capability
	modelID string

	node      *core.LivepeerNode
	suspender *suspender
	penalty   int
	os        drivers.OSSession
}

func NewAISessionSelector(ctx context.Context, cap core.Capability, modelID string, node *core.LivepeerNode, ttl time.Duration) (*AISessionSelector, error) {
	var stakeRdr stakeReader
	if node.Eth != nil {
		stakeRdr = &storeStakeReader{store: node.Database}
	}

	suspender := newSuspender()

	// Create caps for selector to get maxPrice
	warmCaps := newAICapabilities(cap, modelID, true, node.Capabilities)
	coldCaps := newAICapabilities(cap, modelID, false, node.Capabilities)

	// Session pool suspender starts at 0.  Suspension is 3 requests if there are errors from the orchestrator
	penalty := 3
	var warmSel, coldSel BroadcastSessionsSelector
	var autoClear bool
	if cap == core.Capability_LiveVideoToVideo {
		// For Realtime Video AI, we use a dedicated selection algorithm
		selAlg := LiveSelectionAlgorithm{}
		warmSel = NewSelector(stakeRdr, selAlg, node.OrchPerfScore, warmCaps)
		coldSel = NewSelector(stakeRdr, selAlg, node.OrchPerfScore, coldCaps)
		// we don't use penalties for not in Realtime Video AI
		penalty = 0
		// Automatically clear the session pool from old sessions during the discovery
		autoClear = true
	} else {
		// sort sessions based on current latency score
		warmSel = NewSelectorOrderByLatencyScore(stakeRdr, node.SelectionAlgorithm, node.OrchPerfScore, warmCaps)
		coldSel = NewSelectorOrderByLatencyScore(stakeRdr, node.SelectionAlgorithm, node.OrchPerfScore, coldCaps)
	}

	warmPool := NewAISessionPool(warmSel, suspender, penalty)
	coldPool := NewAISessionPool(coldSel, suspender, penalty)
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
		autoClear: autoClear,
	}

	if err := sel.Refresh(ctx); err != nil {
		return nil, err
	}

	// Periodically refresh sessions for Live Video to Video in order to minimize the necessity of refreshing sessions
	// when the AI process is started
	if cap == core.Capability_LiveVideoToVideo {
		startPeriodicRefresh(sel)
	}

	return sel, nil
}

func startPeriodicRefresh(sel *AISessionSelector) {
	clog.Infof(context.Background(), "Starting periodic refresh for Live Video to Video")
	go func() {
		// 6 min to avoid Ticket Params Expired and to avoid getting TTL
		refreshInterval := 6 * time.Minute
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), refreshInterval)
				if err := sel.Refresh(ctx); err != nil {
					clog.Infof(context.Background(), "Error refreshing AISessionSelector err=%v", err)
				}
				cancel()
			}
		}
	}()
}

// newAICapabilities creates a new capabilities object with
func newAICapabilities(cap core.Capability, modelID string, warm bool, constraints *core.Capabilities) *core.Capabilities {
	aiCaps := []core.Capability{cap}
	capabilityConstraints := core.PerCapabilityConstraints{
		cap: &core.CapabilityConstraints{
			Models: map[string]*core.ModelConstraint{
				modelID: {
					Warm:          warm,
					RunnerVersion: constraints.MinRunnerVersionConstraint(cap, modelID),
				},
			},
		},
	}

	caps := core.NewCapabilities(aiCaps, nil)
	caps.SetPerCapabilityConstraints(capabilityConstraints)
	caps.SetMinVersionConstraint(constraints.MinVersionConstraint())

	return caps
}

// SelectorIsEmpty returns true if no orchestrators are in the warm or cold pools.
func (sel *AISessionSelector) SelectorIsEmpty() bool {
	return sel.warmPool.Size() == 0 && sel.coldPool.Size() == 0
}

func (sel *AISessionSelector) Select(ctx context.Context) *AISession {
	shouldRefreshSelector := func() bool {
		discoveryPoolSize := int(math.Min(float64(sel.node.OrchestratorPool.Size()), float64(sel.initialPoolSize)))

		// If the selector is empty, release all orchestrators from suspension and
		// try refresh.
		if sel.SelectorIsEmpty() {
			clog.Infof(ctx, "refreshing sessions, no orchestrators in pools")
			penalty := sel.penalty
			if sel.cap == core.Capability_LiveVideoToVideo {
				// For AI Live Video to Video, we don't store penalty in the selector
				penalty = aiLiveVideoToVideoPenalty
			}
			for i := 0; i < penalty; i++ {
				sel.suspender.signalRefresh()
			}
		}

		if sel.cap == core.Capability_LiveVideoToVideo {
			return sel.SelectorIsEmpty()
		}

		// Refresh if the # of sessions across warm and cold pools falls below the smaller of the maxRefreshSessionsThreshold and
		// 1/2 the total # of orchs that can be queried during discovery
		if sel.warmPool.Size()+sel.coldPool.Size() < int(math.Min(maxRefreshSessionsThreshold, math.Ceil(float64(discoveryPoolSize)/2.0))) {
			return true
		}

		// Refresh if the selector has expired
		sel.warmPool.mu.Lock()
		sel.coldPool.mu.Lock()
		defer sel.coldPool.mu.Unlock()
		defer sel.warmPool.mu.Unlock()
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

		// We request 100 orchestrators in getSessions above so all Orchestrators are returned with refreshed information
		// This keeps the suspended Orchestrators out of the pool until the selector is empty or 30 minutes has passed (refresh happens every 10 minutes)
		if sel.suspender.Suspended(sess.Transcoder()) > 0 {
			clog.V(common.DEBUG).Infof(ctx, "skipping suspended orchestrator=%s", sess.Transcoder())
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
	if sel.autoClear {
		sel.warmPool.Clear(warmSessions)
		sel.coldPool.Clear(coldSessions)
	}

	sel.warmPool.mu.Lock()
	sel.coldPool.mu.Lock()
	defer sel.coldPool.mu.Unlock()
	defer sel.warmPool.mu.Unlock()
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
					Warm:          false,
					RunnerVersion: sel.node.Capabilities.MinRunnerVersionConstraint(sel.cap, sel.modelID),
				},
			},
		},
	}
	caps := core.NewCapabilities(append(core.DefaultCapabilities(), sel.cap), nil)
	caps.SetPerCapabilityConstraints(capabilityConstraints)
	caps.SetMinVersionConstraint(sel.node.Capabilities.MinVersionConstraint())

	// Set numOrchs to the pool size so that discovery tries to find maximum # of compatible orchs within a timeout
	numOrchs := sel.node.OrchestratorPool.Size()

	monitor.AINumOrchestrators(numOrchs, sel.modelID)

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

type noopSus struct{}

func (n noopSus) Suspended(orch string) int {
	return 0
}

func (c *AISessionManager) refreshOrchCapacity(modelIDs []string) {
	if len(modelIDs) < 1 {
		return
	}

	pool := c.node.OrchestratorPool
	if pool == nil {
		return
	}
	clog.Infof(context.Background(), "Starting periodic orchestrator refresh for capacity reporting")

	modelsReq := make(map[string]*core.ModelConstraint)
	for _, modelID := range modelIDs {
		modelsReq[modelID] = &core.ModelConstraint{
			Warm:          false,
			RunnerVersion: c.node.Capabilities.MinRunnerVersionConstraint(core.Capability_LiveVideoToVideo, modelID),
		}
	}
	go func() {
		refreshInterval := 10 * time.Second
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), refreshInterval)
				capabilityConstraints := core.PerCapabilityConstraints{
					core.Capability_LiveVideoToVideo: {Models: modelsReq},
				}
				caps := core.NewCapabilities(append(core.DefaultCapabilities(), core.Capability_LiveVideoToVideo), nil)
				caps.SetPerCapabilityConstraints(capabilityConstraints)
				caps.SetMinVersionConstraint(c.node.Capabilities.MinVersionConstraint())

				pool.GetOrchestrators(ctx, pool.Size(), noopSus{}, caps, common.ScoreAtLeast(0))

				cancel()
			}
		}
	}()
}

type AISessionManager struct {
	node      *core.LivepeerNode
	selectors map[string]*AISessionSelector
	mu        sync.Mutex
	ttl       time.Duration
}

func NewAISessionManager(node *core.LivepeerNode, ttl time.Duration) *AISessionManager {
	sessionManager := &AISessionManager{
		node:      node,
		selectors: make(map[string]*AISessionSelector),
		mu:        sync.Mutex{},
		ttl:       ttl,
	}
	sessionManager.refreshOrchCapacity(node.LiveAICapRefreshModels)
	return sessionManager
}

func (c *AISessionManager) Select(ctx context.Context, cap core.Capability, modelID string) (*AISession, error) {
	clog.V(common.DEBUG).Infof(ctx, "selecting orchestrator for modelID=%s", modelID)
	sel, err := c.getSelector(ctx, cap, modelID)
	if err != nil {
		return nil, err
	}

	sess := sel.Select(ctx)
	if sess == nil {
		return nil, nil
	}

	if err := refreshSessionIfNeeded(ctx, sess.BroadcastSession, false); err != nil {
		return sess, err
	}

	clog.V(common.DEBUG).Infof(ctx, "selected orchestrator=%s", sess.Transcoder())

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
		sel, err = NewAISessionSelector(ctx, cap, modelID, c.node, c.ttl)
		if err != nil {
			return nil, err
		}

		c.selectors[cacheKey] = sel
	}

	return sel, nil
}

func (s *AISession) Clone() *AISession {
	bSess := s.BroadcastSession.Clone()
	s.lock.RLock()
	defer s.lock.RUnlock()

	newSess := *s
	newSess.BroadcastSession = bSess
	return &newSess
}
