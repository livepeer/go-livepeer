package server

import (
	"container/heap"
	"context"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/stretchr/testify/assert"
)

func TestProcessAIRequest_RetryableError(t *testing.T) {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	n.AIProcesssingRetryTimeout = 1 * time.Millisecond //allows processAIRequest to run one time
	s := LivepeerServer{LivepeerNode: n}
	penalty := 3
	cap := core.Capability_AudioToText
	modelID := "audio-model/1"

	orchSess1 := StubBroadcastSession("http://local.host/1")
	orchSess2 := StubBroadcastSession("http://local.host/2")
	orchSess3 := StubBroadcastSession("http://local.host/3")
	orchSess4 := StubBroadcastSession("http://local.host/4")
	orchSessions := []*BroadcastSession{orchSess1, orchSess2, orchSess3, orchSess4}

	n.OrchestratorPool = createOrchestratorPool(orchSessions)

	//warmCaps := newAICapabilities(cap, modelID, true, "")
	//coldCaps := newAICapabilities(cap, modelID, true, "")
	warmSelector := newStubMinLSSelector()
	coldSelector := newStubMinLSSelector()
	suspender := newSuspender()

	warmPool := AISessionPool{
		selector:       warmSelector,
		sessMap:        make(map[string]*BroadcastSession),
		sessionsOnHold: make(map[string][]*BroadcastSession),
		penalty:        penalty,
		suspender:      suspender,
	}

	coldPool := AISessionPool{
		selector:       coldSelector,
		sessMap:        make(map[string]*BroadcastSession),
		sessionsOnHold: make(map[string][]*BroadcastSession),
		penalty:        penalty,
		suspender:      suspender,
	}
	//add sessions to pool
	warmPool.Add(orchSessions)

	selector := &AISessionSelector{
		cap:             cap,
		modelID:         modelID,
		node:            n,
		warmPool:        &warmPool,
		coldPool:        &coldPool,
		ttl:             10 * time.Second,
		lastRefreshTime: time.Now(),
		initialPoolSize: 4,
		suspender:       suspender,
		penalty:         penalty,
		os:              &stubOSSession{},
	}

	s.AISessionManager = NewAISessionManager(n, 10*time.Second)
	selectorKey := strconv.Itoa(int(cap)) + "_" + modelID
	s.AISessionManager.selectors[selectorKey] = selector

	reqID := string(core.RandomManifestID())
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	params := aiRequestParams{
		node:        n,
		os:          drivers.NodeStorage.NewSession(reqID),
		sessManager: s.AISessionManager,
		requestID:   reqID,
	}

	testReq := worker.GenAudioToTextMultipartRequestBody{
		ModelId: &modelID,
	}

	// Mock isRetryableError to return true
	originalIsRetryableError := isRetryableError
	defer func() { isRetryableError = originalIsRetryableError }()
	isRetryableError = mockRetryableErrorTrue

	//select one session
	_, err := processAIRequest(context.TODO(), params, testReq)
	if err != nil {
		t.Logf("Unexpected error: %v", err)
	}
	assert := assert.New(t)
	assert.Less(len(warmSelector.unknownSessions), 4)
	assert.Equal(len(warmPool.sessionsOnHold[reqID]), 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Second)
	}()
	wg.Wait()
	_, sessionStillOnHold := warmPool.sessionsOnHold[reqID]
	assert.False(sessionStillOnHold)

	// now test another error to confirm session is suspended
	isRetryableError = mockRetryableErrorTrue
	_, err = processAIRequest(context.TODO(), params, testReq)
	if err != nil {
		t.Logf("non retryable error: %v", err)
	}
	assert.Greater(len(warmPool.suspender.list), 0)
	assert.Less(len(warmSelector.unknownSessions), 4)
	assert.Equal(0, warmSelector.knownSessions.Len())
}

func TestPutSessionOnHold(t *testing.T) {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	n.AIProcesssingRetryTimeout = 1 * time.Millisecond //allows processAIRequest to run one time
	s := LivepeerServer{LivepeerNode: n}
	penalty := 3
	cap := core.Capability_AudioToText
	modelID := "audio-model/1"

	orchSess1 := StubBroadcastSession("http://local.host/1")
	orchSessions := []*BroadcastSession{orchSess1}

	n.OrchestratorPool = createOrchestratorPool(orchSessions)

	warmSelector := newStubMinLSSelector()
	coldSelector := newStubMinLSSelector()
	suspender := newSuspender()

	warmPool := AISessionPool{
		selector:       warmSelector,
		sessMap:        make(map[string]*BroadcastSession),
		sessionsOnHold: make(map[string][]*BroadcastSession),
		penalty:        penalty,
		suspender:      suspender,
	}

	coldPool := AISessionPool{
		selector:       coldSelector,
		sessMap:        make(map[string]*BroadcastSession),
		sessionsOnHold: make(map[string][]*BroadcastSession),
		penalty:        penalty,
		suspender:      suspender,
	}
	//add sessions to pool
	warmPool.Add(orchSessions)

	selector := &AISessionSelector{
		cap:             cap,
		modelID:         modelID,
		node:            n,
		warmPool:        &warmPool,
		coldPool:        &coldPool,
		ttl:             10 * time.Second,
		lastRefreshTime: time.Now(),
		initialPoolSize: 4,
		suspender:       suspender,
		penalty:         penalty,
		os:              &stubOSSession{},
	}

	s.AISessionManager = NewAISessionManager(n, 10*time.Second)
	selectorKey := strconv.Itoa(int(cap)) + "_" + modelID
	s.AISessionManager.selectors[selectorKey] = selector

	reqID := string(core.RandomManifestID())
	//select one session and put it on hold
	sess := selector.Select(context.TODO())
	s.AISessionManager.PutSessionOnHold(context.TODO(), reqID, sess)

	assert := assert.New(t)
	assert.Equal(1, len(warmPool.sessionsOnHold[reqID]))
	assert.Equal(0, warmPool.selector.Size())

	//sessions should release in 1 millisecond
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
	}()
	wg.Wait()
	assert.Equal(0, len(warmPool.sessionsOnHold))
	assert.Equal(1, warmPool.selector.Size())
	assert.Equal(sess.Transcoder(), warmSelector.unknownSessions[0].Transcoder())

	//select 2 sessions and put on hold for same reqID
	orchSess2 := StubBroadcastSession("http://local.host/2")
	orchSessions = []*BroadcastSession{orchSess2}
	warmPool.Add(orchSessions)
	selector.lastRefreshTime = time.Now()

	sess1 := selector.Select(context.TODO())
	sess2 := selector.Select(context.TODO())
	s.AISessionManager.PutSessionOnHold(context.TODO(), reqID, sess1)
	s.AISessionManager.PutSessionOnHold(context.TODO(), reqID, sess2)
	assert.Equal(2, len(warmPool.sessionsOnHold[reqID]))
	assert.Equal(1, len(warmPool.sessionsOnHold))
	assert.Equal(0, warmPool.selector.Size())

	// sessions should release after 1 millisecond
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
	}()
	wg.Wait()
	assert.Equal(0, len(warmPool.sessionsOnHold))
	assert.Equal(2, warmPool.selector.Size())
}

func createOrchestratorPool(sessions []*BroadcastSession) *stubDiscovery {
	sd := &stubDiscovery{}

	// populate stub discovery
	for idx, sess := range sessions {
		authToken := &net.AuthToken{Token: stubAuthToken.Token, SessionId: string(core.RandomManifestID()), Expiration: stubAuthToken.Expiration}
		sess.OrchestratorInfo = &net.OrchestratorInfo{
			PriceInfo:    &net.PriceInfo{PricePerUnit: int64(idx), PixelsPerUnit: 1},
			TicketParams: &net.TicketParams{},
			AuthToken:    authToken,
			Transcoder:   sess.Transcoder(),
		}

		sd.infos = append(sd.infos, sess.OrchestratorInfo)
	}

	return sd
}

// selector to test behavior of AI session selection that is re-used and moves
// sessions from unknownSessions to knownSessions at completion
type stubMinLSSelector struct {
	knownSessions   *sessHeap
	unknownSessions []*BroadcastSession
}

func newStubMinLSSelector() *stubMinLSSelector {
	knownSessions := &sessHeap{}
	heap.Init(knownSessions)

	return &stubMinLSSelector{
		knownSessions: knownSessions,
	}
}

var tries int

func mockRetryableErrorTrue(err error) bool {
	if tries < 1 {
		tries++
		return true
	} else {
		return false
	}
}

func mockRetryableErrorFalse(err error) bool {
	return false
}

func (s *stubMinLSSelector) Add(sessions []*BroadcastSession) {
	s.unknownSessions = append(s.unknownSessions, sessions...)
}

func (s *stubMinLSSelector) Complete(sess *BroadcastSession) {
	heap.Push(s.knownSessions, sess)
}

func (s *stubMinLSSelector) Select(ctx context.Context) *BroadcastSession {
	sess := s.knownSessions.Peek()
	if sess == nil {
		randSelected := rand.Intn(len(s.unknownSessions))
		sess := s.unknownSessions[randSelected]
		s.removeUnknownSession(randSelected)
		return sess
	}

	//return the known session selected
	return heap.Pop(s.knownSessions).(*BroadcastSession)
}

// Size returns the number of sessions stored by the selector
func (s *stubMinLSSelector) Size() int {
	return len(s.unknownSessions) + s.knownSessions.Len()
}

// Clear resets the selector's state
func (s *stubMinLSSelector) Clear() {
	s.unknownSessions = nil
	s.knownSessions = &sessHeap{}
	//s.stakeRdr = nil //not used in this test
}

func (s *stubMinLSSelector) removeUnknownSession(i int) {
	n := len(s.unknownSessions)
	s.unknownSessions[n-1], s.unknownSessions[i] = s.unknownSessions[i], s.unknownSessions[n-1]
	s.unknownSessions = s.unknownSessions[:n-1]
}
