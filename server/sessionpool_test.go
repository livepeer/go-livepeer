package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/lpms/stream"
	"github.com/stretchr/testify/assert"
)

type sessionPoolLIFO struct {
	*SessionPool
}

func newSessionPoolLIFO(pool *SessionPool) *sessionPoolLIFO {
	return &sessionPoolLIFO{pool}
}

func (pool *sessionPoolLIFO) sessList() []*BroadcastSession {
	sessList, _ := pool.sel.(*LIFOSelector)
	return *sessList
}

func stubPool() *sessionPoolLIFO {
	return stubPoolExt(2)
}

func stubPoolExt(num int) *sessionPoolLIFO {
	var sessList []*BroadcastSession
	for i := 0; i < num; i++ {
		sess := StubBroadcastSession(fmt.Sprintf("transcoder%d", i+1))
		sessList = append(sessList, sess)
	}

	return poolWithSessList(sessList)
}

func poolWithSessList(sessList []*BroadcastSession) *sessionPoolLIFO {
	sessMap := make(map[string]*BroadcastSession)
	for _, sess := range sessList {
		sessMap[sess.OrchestratorInfo.Transcoder] = sess
	}

	sel := &LIFOSelector{}
	sel.Add(sessList)

	var createSessions = func() ([]*BroadcastSession, error) {
		// return sessList, nil
		return nil, nil
	}
	var deleteSessions = func(sessionID string) {}
	pool := NewSessionPool("test", len(sessList), 1, newSuspender(), createSessions, deleteSessions, sel)
	pool.sessMap = sessMap
	return newSessionPoolLIFO(pool)
}

func TestSelectSession(t *testing.T) {
	pool := stubPool()

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)
	expectedSess1 := pool.sessList()[1]
	expectedSess2 := pool.sessList()[0]

	// assert last session selected and sessList is correct length
	sess := pool.selectSessions(context.TODO(), 1)[0]
	assert.Equal(expectedSess1, sess)
	assert.Equal(sess, pool.lastSess[0])
	assert.Len(pool.sessList(), 1)

	sess = pool.selectSessions(context.TODO(), 1)[0]
	assert.Equal(expectedSess2, sess)
	assert.Equal(sess, pool.lastSess[0])
	assert.Len(pool.sessList(), 0)

	// assert no session is selected from empty list
	sesss := pool.selectSessions(context.TODO(), 1)
	assert.Nil(sesss)
	assert.Nil(pool.lastSess)
	assert.Len(pool.sessList(), 0)
	assert.Len(pool.sessMap, 2) // map should still track original sessions

	// assert session list gets refreshed if under threshold. check via waitgroup
	pool = poolWithSessList([]*BroadcastSession{})
	pool.numOrchs = 1
	pool.poolSize = 1
	var wg sync.WaitGroup
	wg.Add(1)
	pool.createSessions = func() ([]*BroadcastSession, error) { wg.Done(); return nil, fmt.Errorf("err") }
	pool.selectSessions(context.TODO(), 1)
	assert.True(wgWait(&wg), "Session refresh timed out")

	// assert the selection retries if session in list doesn't exist in map
	pool = stubPool()

	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)
	// sanity checks then rebuild in order
	firstSess := pool.selectSessions(context.TODO(), 1)
	expectedSess := pool.selectSessions(context.TODO(), 1)[0]
	assert.Len(pool.sessList(), 0)
	assert.Len(pool.sessMap, 2)
	pool.completeSession(expectedSess)
	pool.completeSession(firstSess[0])
	// remove first sess from map, but keep in list. check results around that
	pool.removeSession(firstSess[0])
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 1)
	assert.Equal(firstSess[0], pool.sessList()[1]) // ensure removed sess still in list
	// now ensure next selectSession call fixes up sessList as expected
	sess = pool.selectSessions(context.TODO(), 1)[0]
	assert.Equal(sess, expectedSess)
	assert.Len(pool.sessList(), 0)
	assert.Len(pool.sessMap, 1)

	// XXX check refresh condition more precisely - currently numOrchs / 2
}

func TestSelectSession_NilSession(t *testing.T) {
	pool := stubPool()
	// Replace selector with stubSelector that will return nil for Select(), but 1 for Size()
	pool.sel = &stubSelector{size: 1}

	assert.Nil(t, pool.selectSessions(context.TODO(), 1))
}

func TestRemoveSession(t *testing.T) {
	pool := stubPool()

	sess1 := pool.sessList()[0]
	sess2 := pool.sessList()[1]

	assert := assert.New(t)
	assert.Len(pool.sessMap, 2)

	// remove session in map
	assert.NotNil(pool.sessMap[sess1.OrchestratorInfo.Transcoder])
	pool.removeSession(sess1)
	assert.Nil(pool.sessMap[sess1.OrchestratorInfo.Transcoder])
	assert.Len(pool.sessMap, 1)

	// remove nonexistent session
	assert.Nil(pool.sessMap[sess1.OrchestratorInfo.Transcoder])
	pool.removeSession(sess1)
	assert.Nil(pool.sessMap[sess1.OrchestratorInfo.Transcoder])
	assert.Len(pool.sessMap, 1)

	// remove last session in map
	assert.NotNil(pool.sessMap[sess2.OrchestratorInfo.Transcoder])
	pool.removeSession(sess2)
	assert.Nil(pool.sessMap[sess2.OrchestratorInfo.Transcoder])
	assert.Len(pool.sessMap, 0)
}

func TestCompleteSessions(t *testing.T) {
	pool := stubPool()

	sess1 := pool.selectSessions(context.TODO(), 1)[0]

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(pool.sessList(), 1)
	assert.Len(pool.sessMap, 2)

	pool.completeSession(sess1)

	// assert that session already in sessMap is added back to sessList
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)
	assert.Equal(sess1, pool.sessMap[sess1.OrchestratorInfo.Transcoder])

	// assert that we get the same session back next time we call select
	newSess := pool.selectSessions(context.TODO(), 1)[0]
	assert.Equal(sess1, newSess)
	pool.completeSession(newSess)

	// assert that session not in sessMap is not added to sessList
	sess3 := StubBroadcastSession("transcoder3")
	pool.completeSession(sess3)
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)

	sess1 = pool.selectSessions(context.TODO(), 1)[0]

	sess1.LatencyScore = 2.7
	pool.completeSession(sess1)

	// assert that existing session with same key in sessMap is replaced
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)
	assert.Equal(2.7, pool.sessMap[sess1.OrchestratorInfo.Transcoder].LatencyScore)
}

func TestRefreshSessions(t *testing.T) {
	pool := stubPool()

	assert := assert.New(t)
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)

	sess1 := pool.sessList()[0]
	sess2 := pool.sessList()[1]
	pool.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{sess1, sess2}, nil
	}

	// asserting that pre-existing sessions are not added to sessList or sessMap
	pool.refreshSessions(context.TODO())
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)

	sess3 := StubBroadcastSession("transcoder3")
	sess4 := StubBroadcastSession("transcoder4")

	pool.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{sess3, sess4}, nil
	}

	// asserting that new sessions are added to beginning of sessList and sessMap
	pool.refreshSessions(context.TODO())
	assert.Len(pool.sessList(), 4)
	assert.Len(pool.sessMap, 4)
	assert.Equal(pool.sessList()[0], sess3)
	assert.Equal(pool.sessList()[1], sess4)

	// asserting that refreshes stop while another is in-flight
	pool.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{StubBroadcastSession("5"), StubBroadcastSession("6")}, nil
	}
	pool.refreshing = true
	pool.refreshSessions(context.TODO())
	assert.Len(pool.sessList(), 4)
	assert.Len(pool.sessMap, 4)
	assert.Equal(pool.sessList()[0], sess3)
	assert.Equal(pool.sessList()[1], sess4)
	pool.refreshing = false

	// Check thread safety, run this under -race
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { pool.refreshSessions(context.TODO()); wg.Done() }()
	}
	assert.True(wgWait(&wg), "Session refresh timed out")

	// asserting that refreshes stop after a cleanup
	pool.cleanup()
	assert.Len(pool.sessList(), 0)
	assert.Len(pool.sessMap, 0)
	pool.refreshSessions(context.TODO())
	assert.Len(pool.sessList(), 0)
	assert.Len(pool.sessMap, 0)

	// check exit errors from createSession. Run this under -race
	pool = stubPool() // reset pool from previous tests

	pool.createSessions = func() ([]*BroadcastSession, error) {
		time.Sleep(time.Millisecond)
		return nil, fmt.Errorf("err")
	}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { pool.refreshSessions(context.TODO()); wg.Done() }()
	}
	assert.True(wgWait(&wg), "Session refresh timed out")

	// check empty returns from createSession. Run this under -race
	pool.createSessions = func() ([]*BroadcastSession, error) {
		time.Sleep(time.Millisecond)
		return []*BroadcastSession{}, nil
	}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { pool.refreshSessions(context.TODO()); wg.Done() }()
	}
	assert.True(wgWait(&wg), "Session refresh timed out")
}

func TestCleanupSessions(t *testing.T) {
	pool := stubPool()

	// sanity checks
	assert := assert.New(t)
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)

	// check relevant fields are reset
	pool.cleanup()
	assert.Len(pool.sessList(), 0)
	assert.Len(pool.sessMap, 0)
}

func TestSelectSession_MultipleInFlight(t *testing.T) {
	pool := stubPool()

	sendSegStub := func() *BroadcastSession {
		sesss := pool.selectSessions(context.TODO(), 1)
		pool.lock.Lock()
		if sesss == nil {
			pool.lock.Unlock()
			return nil
		}
		seg := &stream.HLSSegment{Data: []byte("dummy"), Duration: 0.100}
		sesss[0].pushSegInFlight(seg)
		pool.lock.Unlock()
		return sesss[0]
	}

	completeSegStub := func(sess *BroadcastSession) {
		// Create dummy result
		sess.lock.RLock()
		res := &ReceivedTranscodeResult{
			LatencyScore: sess.LatencyScore,
			Info:         sess.OrchestratorInfo,
		}
		sess.lock.RUnlock()
		updateSession(sess, res)
		pool.completeSession(sess)
	}

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(pool.sessList(), 2)
	assert.Len(pool.sessMap, 2)

	expectedSess0 := &BroadcastSession{}
	expectedSess1 := &BroadcastSession{}
	*expectedSess0 = *(pool.sessList()[1])
	*expectedSess1 = *(pool.sessList()[0])

	// send in multiple segments at the same time and verify SegsInFlight & lastSess are updated
	sess0 := sendSegStub()
	assert.Equal(pool.lastSess[0], sess0)
	assert.Equal(expectedSess0.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)

	sess1 := sendSegStub()
	assert.Equal(pool.lastSess[0], sess1)
	assert.Equal(sess0, sess1)
	assert.Len(pool.lastSess[0].SegsInFlight, 2)

	completeSegStub(sess0)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)
	assert.Len(pool.sessList(), 1)

	completeSegStub(sess1)
	assert.Len(pool.lastSess, 1)
	assert.Len(pool.lastSess[0].SegsInFlight, 0)
	assert.Len(pool.sessList(), 2)

	// Same as above but to check thread safety, run this under -race
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { sess0 = sendSegStub(); wg.Done() }()
	go func() { sess1 = sendSegStub(); wg.Done() }()
	assert.True(wgWait(&wg), "Segment sending timed out")

	assert.Len(pool.lastSess, 1)
	assert.Equal(pool.lastSess[0], sess0)
	assert.Equal(sess0, sess1)
	assert.Len(pool.lastSess[0].SegsInFlight, 2)
	assert.Len(pool.sessList(), 1)
	wg.Add(2)
	go func() { completeSegStub(sess0); wg.Done() }()
	go func() { completeSegStub(sess1); wg.Done() }()
	assert.True(wgWait(&wg), "Segment completion timed out")
	assert.Len(pool.lastSess[0].SegsInFlight, 0)

	// send in multiple segments with delay > segDur to trigger O switch
	sess0 = sendSegStub()
	assert.Equal(pool.lastSess[0], sess0)
	assert.Equal(expectedSess0.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)

	time.Sleep(1600 * time.Millisecond)

	sess1 = sendSegStub()
	assert.Equal(pool.lastSess[0], sess1)
	assert.Equal(expectedSess1.OrchestratorInfo, sess1.OrchestratorInfo)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)

	completeSegStub(sess0)
	completeSegStub(sess1)

	// send in multiple segments with delay > segDur but < 2*segDur and only a single session available
	pool.suspend(expectedSess0.OrchestratorInfo.GetTranscoder())
	pool.removeSession(expectedSess0)
	assert.Len(pool.sessMap, 1)

	sess0 = sendSegStub()
	assert.Equal(pool.lastSess[0], sess0)
	assert.Equal(expectedSess1.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)

	time.Sleep(1000 * time.Millisecond)

	sess1 = sendSegStub()
	assert.Equal(pool.lastSess[0], sess1)
	assert.Equal(expectedSess1.OrchestratorInfo, sess1.OrchestratorInfo)
	assert.Len(pool.lastSess[0].SegsInFlight, 2)

	completeSegStub(sess0)
	completeSegStub(sess1)
	assert.Len(pool.lastSess[0].SegsInFlight, 0)

	// send in multiple segments with delay > 2*segDur and only a single session available
	pool.suspend(expectedSess0.OrchestratorInfo.Transcoder)
	pool.removeSession(expectedSess0)
	assert.Len(pool.sessMap, 1)

	sess0 = sendSegStub()
	assert.Equal(pool.lastSess[0], sess0)
	assert.Equal(expectedSess1.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)

	time.Sleep(1600 * time.Millisecond)

	sess1 = sendSegStub()
	assert.Nil(sess1)
	assert.Nil(pool.lastSess)

	completeSegStub(sess0)

	// remove both session and check if selector returns nil and sets lastSession to nil
	pool.suspend(expectedSess0.OrchestratorInfo.Transcoder)
	pool.suspend(expectedSess1.OrchestratorInfo.Transcoder)
	pool.removeSession(expectedSess0)
	pool.removeSession(expectedSess1)
	pool.lock.Lock() // refresh session could be running in parallel and modifying sessMap
	assert.Len(pool.sessMap, 0)
	pool.lock.Unlock()
	//
	sess0 = sendSegStub()
	assert.Nil(sess0)

	assert.Nil(pool.lastSess)
}

func TestSelectSessionMoreThanOne(t *testing.T) {
	pool := stubPoolExt(3)
	sendSegStub := func(num int) []*BroadcastSession {
		sesss := pool.selectSessions(context.TODO(), num)
		pool.lock.Lock()
		if sesss == nil {
			pool.lock.Unlock()
			return nil
		}
		for _, ses := range sesss {
			seg := &stream.HLSSegment{Data: []byte("dummy"), Duration: 0.100}
			ses.pushSegInFlight(seg)
		}
		pool.lock.Unlock()
		return sesss
	}

	completeSegStub := func(sess *BroadcastSession) {
		// Create dummy result
		res := &ReceivedTranscodeResult{
			LatencyScore: sess.LatencyScore,
			Info:         sess.OrchestratorInfo,
		}
		updateSession(sess, res)
		pool.completeSession(sess)
	}

	completeSegStubs := func(sess []*BroadcastSession) {
		for _, ses := range sess {
			completeSegStub(ses)
		}
	}

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(pool.sessList(), 3)
	assert.Len(pool.sessMap, 3)
	expectedSess1 := pool.sessList()[2]
	expectedSess2 := pool.sessList()[1]
	// expectedSess3 := pool.sessList()[0]

	// assert last session selected and sessList is correct length
	sessions := sendSegStub(2)
	assert.Len(sessions, 2)
	assert.Equal(expectedSess1, sessions[0])
	assert.Equal(sessions[0], pool.lastSess[0])
	assert.Equal(expectedSess2, sessions[1])
	assert.Equal(sessions[1], pool.lastSess[1])
	assert.Len(pool.sessList(), 1)
	completeSegStubs(sessions)
	assert.Len(pool.sessList(), 3)
	sessions = sendSegStub(4)
	assert.Len(sessions, 3)
	assert.Len(pool.sessList(), 0)
	completeSegStubs(sessions)
	assert.Len(pool.sessList(), 3)
	assert.Len(pool.sessList()[0].SegsInFlight, 0)
	assert.Len(pool.sessList()[1].SegsInFlight, 0)
	assert.Len(pool.sessList()[2].SegsInFlight, 0)
	sessions = sendSegStub(3)
	assert.Len(sessions, 3)
	assert.Len(pool.sessList(), 0)
	assert.Len(sessions[0].SegsInFlight, 1)
	assert.Len(pool.lastSess[0].SegsInFlight, 1)
	time.Sleep(800 * time.Millisecond)
	sessions2 := sendSegStub(3)
	assert.Len(sessions2, 3)
	assert.Len(pool.sessList(), 0)
	assert.Len(sessions2[0].SegsInFlight, 2)
	assert.Len(pool.lastSess[1].SegsInFlight, 2)
	assert.Equal(sessions, sessions2)
	time.Sleep(800 * time.Millisecond)
	sessions3 := sendSegStub(3)
	assert.Len(sessions3, 0)
	assert.Len(sessions2[0].SegsInFlight, 2)
	assert.Len(sessions[0].SegsInFlight, 2)
	completeSegStubs(sessions)
	assert.Len(pool.sessList(), 0)
	assert.Len(sessions2[0].SegsInFlight, 1)
	assert.Len(sessions[0].SegsInFlight, 1)
	completeSegStubs(sessions2)
	assert.Len(pool.sessList(), 3)
}
