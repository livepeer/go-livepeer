package server

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"

	"github.com/stretchr/testify/assert"
)

func StubBroadcastSession(transcoder string) *BroadcastSession {
	return &BroadcastSession{
		Broadcaster:      StubBroadcaster2(),
		ManifestID:       core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{Transcoder: transcoder},
	}
}

func StubBroadcastSessionsManager() *BroadcastSessionsManager {
	sess1 := StubBroadcastSession("transcoder1")
	sess2 := StubBroadcastSession("transcoder2")

	return &BroadcastSessionsManager{
		sessList: []*BroadcastSession{sess1, sess2},
		sessMap: map[string]*BroadcastSession{
			sess1.OrchestratorInfo.Transcoder: sess1,
			sess2.OrchestratorInfo.Transcoder: sess2,
		},
		sessLock: &sync.Mutex{},
		createSessions: func() ([]*BroadcastSession, error) {
			return []*BroadcastSession{sess1, sess2}, nil
		},
	}
}

func TestStopSessionErrors(t *testing.T) {

	// check error cases
	errs := []string{
		"Unable to read response body for segment 4 : unexpected EOF",
		"Unable to submit segment 5 Post https://127.0.0.1:8936/segment: dial tcp 127.0.0.1:8936: getsockopt: connection refused",
		core.ErrOrchBusy.Error(),
		core.ErrOrchCap.Error(),
	}

	// Sanity check that we're checking each failure case
	if len(errs) != len(sessionErrStrings) {
		t.Error("Mismatched error cases for stop session")
	}
	for _, v := range errs {
		if !shouldStopSession(errors.New(v)) {
			t.Error("Should have stopped session but didn't: ", v)
		}
	}

	// check non-error cases
	errs = []string{
		"",
		"not really an error",
	}
	for _, v := range errs {
		if shouldStopSession(errors.New(v)) {
			t.Error("Should not have stopped session but did: ", v)
		}
	}

}

func TestNewSessionManager(t *testing.T) {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	assert := assert.New(t)

	mid := core.RandomManifestID()
	storage := drivers.NewMemoryDriver(nil).NewSession(string(mid))
	pl := core.NewBasicPlaylistManager(mid, storage)

	// Check empty pool produces expected numOrchs
	sess := NewSessionManager(n, pl)
	assert.Equal(0, sess.numOrchs)

	// Check numOrchs up to maximum and a bit beyond
	sd := &stubDiscovery{}
	n.OrchestratorPool = sd
	max := int(HTTPTimeout.Seconds()/SegLen.Seconds()) * 2
	for i := 0; i < 10; i++ {
		sess = NewSessionManager(n, pl)
		if i < max {
			assert.Equal(i, sess.numOrchs)
		} else {
			assert.Equal(max, sess.numOrchs)
		}
		sd.infos = append(sd.infos, &net.OrchestratorInfo{})
	}
	// sanity check some expected postconditions
	assert.Equal(sess.numOrchs, max)
	assert.True(sd.Size() > max, "pool should be greater than max numOrchs")
}

func wgWait(wg *sync.WaitGroup) bool {
	c := make(chan struct{})
	go func() { defer close(c); wg.Wait() }()
	select {
	case <-c:
		return true
	case <-time.After(1 * time.Second):
		return false
	}
}

func TestSelectSession(t *testing.T) {
	bsm := StubBroadcastSessionsManager()

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)
	expectedSess1 := bsm.sessList[1]
	expectedSess2 := bsm.sessList[0]

	// assert last session selected and sessList is correct length
	sess := bsm.selectSession()
	assert.Equal(expectedSess1, sess)
	assert.Len(bsm.sessList, 1)

	sess = bsm.selectSession()
	assert.Equal(expectedSess2, sess)
	assert.Len(bsm.sessList, 0)

	// assert no session is selected from empty list
	sess = bsm.selectSession()
	assert.Nil(sess)
	assert.Len(bsm.sessList, 0)
	assert.Len(bsm.sessMap, 2) // map should still track original sessions

	// assert session list gets refreshed if under threshold. check via waitgroup
	bsm.numOrchs = 1
	var wg sync.WaitGroup
	wg.Add(1)
	bsm.createSessions = func() ([]*BroadcastSession, error) { wg.Done(); return nil, fmt.Errorf("err") }
	bsm.selectSession()
	assert.True(wgWait(&wg), "Session refresh timed out")

	// assert the selection retries if session in list doesn't exist in map
	bsm = StubBroadcastSessionsManager()
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)
	// sanity checks then rebuild in order
	firstSess := bsm.selectSession()
	expectedSess := bsm.selectSession()
	assert.Len(bsm.sessList, 0)
	assert.Len(bsm.sessMap, 2)
	bsm.completeSession(expectedSess)
	bsm.completeSession(firstSess)
	// remove first sess from map, but keep in list. check results around that
	bsm.removeSession(firstSess)
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 1)
	assert.Equal(firstSess, bsm.sessList[1]) // ensure removed sess still in list
	// now ensure next selectSession call fixes up sessList as expected
	sess = bsm.selectSession()
	assert.Equal(sess, expectedSess)
	assert.Len(bsm.sessList, 0)
	assert.Len(bsm.sessMap, 1)

	// XXX check refresh condition more precisely - currently numOrchs / 2
}

func TestRemoveSession(t *testing.T) {
	bsm := StubBroadcastSessionsManager()
	sess1 := bsm.sessList[0]
	sess2 := bsm.sessList[1]

	assert := assert.New(t)
	assert.Len(bsm.sessMap, 2)

	// remove session in map
	assert.NotNil(bsm.sessMap[sess1.OrchestratorInfo.Transcoder])
	bsm.removeSession(sess1)
	assert.Nil(bsm.sessMap[sess1.OrchestratorInfo.Transcoder])
	assert.Len(bsm.sessMap, 1)

	// remove nonexistent session
	assert.Nil(bsm.sessMap[sess1.OrchestratorInfo.Transcoder])
	bsm.removeSession(sess1)
	assert.Nil(bsm.sessMap[sess1.OrchestratorInfo.Transcoder])
	assert.Len(bsm.sessMap, 1)

	// remove last session in map
	assert.NotNil(bsm.sessMap[sess2.OrchestratorInfo.Transcoder])
	bsm.removeSession(sess2)
	assert.Nil(bsm.sessMap[sess2.OrchestratorInfo.Transcoder])
	assert.Len(bsm.sessMap, 0)
}

func TestCompleteSessions(t *testing.T) {
	bsm := StubBroadcastSessionsManager()
	sess1 := bsm.selectSession()

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(bsm.sessList, 1)
	assert.Len(bsm.sessMap, 2)

	bsm.completeSession(sess1)

	// assert that session already in sessMap is added back to sessList
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)
	assert.Equal(sess1, bsm.sessMap[sess1.OrchestratorInfo.Transcoder])

	// assert that we get the same session back next time we call select
	newSess := bsm.selectSession()
	assert.Equal(sess1, newSess)
	bsm.completeSession(newSess)

	// assert that session not in sessMap is not added to sessList
	sess3 := StubBroadcastSession("transcoder3")
	bsm.completeSession(sess3)
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)
}

func TestRefreshSessions(t *testing.T) {
	bsm := StubBroadcastSessionsManager()

	assert := assert.New(t)
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)

	sess1 := bsm.sessList[0]
	sess2 := bsm.sessList[1]
	bsm.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{sess1, sess2}, nil
	}

	// asserting that pre-existing sessions are not added to sessList or sessMap
	bsm.refreshSessions()
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)

	sess3 := StubBroadcastSession("transcoder3")
	sess4 := StubBroadcastSession("transcoder4")

	bsm.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{sess3, sess4}, nil
	}

	// asserting that new sessions are added to beginning of sessList and sessMap
	bsm.refreshSessions()
	assert.Len(bsm.sessList, 4)
	assert.Len(bsm.sessMap, 4)
	assert.Equal(bsm.sessList[0], sess3)
	assert.Equal(bsm.sessList[1], sess4)

	// asserting that refreshes stop while another is in-flight
	bsm.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{StubBroadcastSession("5"), StubBroadcastSession("6")}, nil
	}
	bsm.refreshing = true
	bsm.refreshSessions()
	assert.Len(bsm.sessList, 4)
	assert.Len(bsm.sessMap, 4)
	assert.Equal(bsm.sessList[0], sess3)
	assert.Equal(bsm.sessList[1], sess4)
	bsm.refreshing = false

	// Check thread safety, run this under -race
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { bsm.refreshSessions(); wg.Done() }()
	}
	assert.True(wgWait(&wg), "Session refresh timed out")

	// asserting that refreshes stop after a cleanup
	bsm.cleanup()
	assert.Len(bsm.sessList, 0)
	assert.Len(bsm.sessMap, 0)
	bsm.refreshSessions()
	assert.Len(bsm.sessList, 0)
	assert.Len(bsm.sessMap, 0)

	// check exit errors from createSession. Run this under -race
	bsm = StubBroadcastSessionsManager() // reset bsm from previous tests
	bsm.createSessions = func() ([]*BroadcastSession, error) {
		time.Sleep(time.Millisecond)
		return nil, fmt.Errorf("err")
	}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { bsm.refreshSessions(); wg.Done() }()
	}
	assert.True(wgWait(&wg), "Session refresh timed out")

	// check empty returns from createSession. Run this under -race
	bsm.createSessions = func() ([]*BroadcastSession, error) {
		time.Sleep(time.Millisecond)
		return []*BroadcastSession{}, nil
	}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() { bsm.refreshSessions(); wg.Done() }()
	}
	assert.True(wgWait(&wg), "Session refresh timed out")
}

func TestCleanupSessions(t *testing.T) {
	bsm := StubBroadcastSessionsManager()

	// sanity checks
	assert := assert.New(t)
	assert.Len(bsm.sessList, 2)
	assert.Len(bsm.sessMap, 2)

	// check relevant fields are reset
	bsm.cleanup()
	assert.Len(bsm.sessList, 0)
	assert.Len(bsm.sessMap, 0)
}

// Note: Add processSegment tests, including:
//     assert an error from transcoder removes sess from BroadcastSessionManager
//     assert a success re-adds sess to BroadcastSessionManager
