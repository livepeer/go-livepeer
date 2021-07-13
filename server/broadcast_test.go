package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/verification"
	"github.com/livepeer/livepeer-data/pkg/data"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/m3u8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func StubBroadcastSession(transcoder string) *BroadcastSession {
	return &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params:      &core.StreamParameters{ManifestID: core.RandomManifestID()},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: transcoder,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
			AuthToken: stubAuthToken,
		},
	}
}

func StubBroadcastSessionsManager() *BroadcastSessionsManager {
	sess1 := StubBroadcastSession("transcoder1")
	sess2 := StubBroadcastSession("transcoder2")

	return bsmWithSessList([]*BroadcastSession{sess1, sess2})
}

func bsmWithSessList(sessList []*BroadcastSession) *BroadcastSessionsManager {
	sessMap := make(map[string]*BroadcastSession)
	for _, sess := range sessList {
		sessMap[sess.OrchestratorInfo.Transcoder] = sess
	}

	sel := &LIFOSelector{}
	sel.Add(sessList)

	return &BroadcastSessionsManager{
		sel:      sel,
		sessMap:  sessMap,
		sessLock: &sync.Mutex{},
		createSessions: func() ([]*BroadcastSession, error) {
			return sessList, nil
		},
		sus:      newSuspender(),
		numOrchs: 1,
		poolSize: len(sessList),
	}
}

func stubSessionList(ctx context.Context, sessionCount int, handler http.HandlerFunc) []*BroadcastSession {
	sessions := []*BroadcastSession{}
	for i := 0; i < sessionCount; i++ {
		sess := StubBroadcastSession(stubTestTranscoder(ctx, handler))
		sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
		sessions = append(sessions, sess)
	}
	return sessions
}

type sessionsManagerLIFO struct {
	*BroadcastSessionsManager
}

func newSessionsManagerLIFO(bsm *BroadcastSessionsManager) *sessionsManagerLIFO {
	return &sessionsManagerLIFO{bsm}
}

func (bsm *sessionsManagerLIFO) sessList() []*BroadcastSession {
	sessList, _ := bsm.sel.(*LIFOSelector)
	return *sessList
}

type stubVerifier struct {
	retries int
	calls   int
	params  *verification.Params
	err     error
	results []verification.Results
}

func (v *stubVerifier) Verify(params *verification.Params) (*verification.Results, error) {
	var res *verification.Results
	if v.calls < len(v.results) {
		res = &v.results[v.calls]
	}
	v.calls++
	v.params = params
	return res, v.err
}
func newStubSegmentVerifier(v *stubVerifier) *verification.SegmentVerifier {
	return verification.NewSegmentVerifier(&verification.Policy{Retries: v.retries, Verifier: v})
}

type stubOSSession struct {
	external bool
	host     string
	saved    []string
	err      error
}

func (s *stubOSSession) SaveData(name string, data []byte, meta map[string]string, timeout time.Duration) (string, error) {
	s.saved = append(s.saved, name)
	return "saved_" + name, s.err
}
func (s *stubOSSession) EndSession() {
}
func (s *stubOSSession) GetInfo() *net.OSInfo {
	return nil
}
func (s *stubOSSession) IsExternal() bool {
	return s.external
}
func (s *stubOSSession) IsOwn(url string) bool {
	return strings.HasPrefix(url, s.host)
}
func (s *stubOSSession) ListFiles(ctx context.Context, prefix, delim string) (drivers.PageInfo, error) {
	return nil, nil
}
func (s *stubOSSession) ReadData(ctx context.Context, name string) (*drivers.FileInfoReader, error) {
	return nil, nil
}
func (s *stubOSSession) OS() drivers.OSDriver {
	return nil
}

type stubPlaylistManager struct {
	manifestID core.ManifestID
	seq        uint64
	profile    ffmpeg.VideoProfile
	uri        string
	os         drivers.OSSession
	lock       sync.Mutex
}

func (pm *stubPlaylistManager) ManifestID() core.ManifestID {
	return pm.manifestID
}

func (pm *stubPlaylistManager) InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64) error {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.profile = *profile
	pm.seq = seqNo
	pm.uri = uri
	return nil
}

func (pm *stubPlaylistManager) GetHLSMasterPlaylist() *m3u8.MasterPlaylist {
	return nil
}

func (pm *stubPlaylistManager) GetHLSMediaPlaylist(rendition string) *m3u8.MediaPlaylist {
	return nil
}

func (pm *stubPlaylistManager) GetOSSession() drivers.OSSession {
	return pm.os
}

func (pm *stubPlaylistManager) Cleanup()     {}
func (pm *stubPlaylistManager) FlushRecord() {}
func (pm *stubPlaylistManager) GetRecordOSSession() drivers.OSSession {
	return nil
}
func (pm *stubPlaylistManager) InsertHLSSegmentJSON(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64) {
}

type stubSelector struct {
	sess *BroadcastSession
	size int
}

func (s *stubSelector) Add(sessions []*BroadcastSession) {}
func (s *stubSelector) Complete(sess *BroadcastSession)  {}
func (s *stubSelector) Select() *BroadcastSession        { return s.sess }
func (s *stubSelector) Size() int                        { return s.size }
func (s *stubSelector) Clear()                           {}

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
	params := &core.StreamParameters{OS: storage}

	// Check empty pool produces expected numOrchs
	sess := NewSessionManager(n, params, &LIFOSelector{})
	assert.Equal(0, sess.numOrchs)

	// Check numOrchs equals poolSize
	sd := &stubDiscovery{}
	n.OrchestratorPool = sd
	for i := 0; i < 10; i++ {
		sd.infos = append(sd.infos, &net.OrchestratorInfo{PriceInfo: &net.PriceInfo{}})
		sess = NewSessionManager(n, params, &LIFOSelector{})
		assert.Equal(i+1, sess.numOrchs)
	}
	// sanity check some expected postconditions
	assert.Equal(sess.numOrchs, sd.Size())
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
	bsm := newSessionsManagerLIFO(StubBroadcastSessionsManager())

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)
	expectedSess1 := bsm.sessList()[1]
	expectedSess2 := bsm.sessList()[0]

	// assert last session selected and sessList is correct length
	sess := bsm.selectSession()
	assert.Equal(expectedSess1, sess)
	assert.Equal(sess, bsm.lastSess)
	assert.Len(bsm.sessList(), 1)

	sess = bsm.selectSession()
	assert.Equal(expectedSess2, sess)
	assert.Equal(sess, bsm.lastSess)
	assert.Len(bsm.sessList(), 0)

	// assert no session is selected from empty list
	sess = bsm.selectSession()
	assert.Nil(sess)
	assert.Nil(bsm.lastSess)
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 2) // map should still track original sessions

	// assert session list gets refreshed if under threshold. check via waitgroup
	bsm = newSessionsManagerLIFO(bsmWithSessList([]*BroadcastSession{}))
	bsm.numOrchs = 1
	var wg sync.WaitGroup
	wg.Add(1)
	bsm.createSessions = func() ([]*BroadcastSession, error) { wg.Done(); return nil, fmt.Errorf("err") }
	bsm.selectSession()
	assert.True(wgWait(&wg), "Session refresh timed out")

	// assert the selection retries if session in list doesn't exist in map
	bsm = newSessionsManagerLIFO(StubBroadcastSessionsManager())

	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)
	// sanity checks then rebuild in order
	firstSess := bsm.selectSession()
	expectedSess := bsm.selectSession()
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 2)
	bsm.completeSession(expectedSess)
	bsm.completeSession(firstSess)
	// remove first sess from map, but keep in list. check results around that
	bsm.removeSession(firstSess)
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 1)
	assert.Equal(firstSess, bsm.sessList()[1]) // ensure removed sess still in list
	// now ensure next selectSession call fixes up sessList as expected
	sess = bsm.selectSession()
	assert.Equal(sess, expectedSess)
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 1)

	// XXX check refresh condition more precisely - currently numOrchs / 2
}

func TestSelectSession_MultipleInFlight2(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	tr := &net.TranscodeResult{
		Info: &net.OrchestratorInfo{
			Transcoder:   ts.URL,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7},
			TicketParams: &net.TicketParams{ExpirationBlock: big.NewInt(100).Bytes()},
			AuthToken:    stubAuthToken,
		},
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{{Url: "test.flv"}},
				Sig:      []byte("bar"),
			},
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	segDone := make(chan interface{})
	segStarted := make(chan interface{})
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		segStarted <- nil
		<-segDone
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	successOrchInfoUpdate := &net.OrchestratorInfo{
		Transcoder: ts.URL,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
		TicketParams: &net.TicketParams{},
		AuthToken:    stubAuthToken,
	}

	oldGetOrchestratorInfoRPC := getOrchestratorInfoRPC
	defer func() { getOrchestratorInfoRPC = oldGetOrchestratorInfoRPC }()

	orchInfoCalled := 0
	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		orchInfoCalled++
		return successOrchInfoUpdate, nil
	}

	sess := StubBroadcastSession(ts.URL)
	sender := &pm.MockSender{}
	sender.On("StartSession", mock.Anything).Return("foo").Times(3)
	sender.On("EV", mock.Anything).Return(big.NewRat(1000000, 1), nil)
	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(defaultTicketBatch(), nil)
	sender.On("ValidateTicketParams", mock.Anything).Return(nil)
	sess.Sender = sender
	balance := &mockBalance{}
	sess.Balance = balance
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(1, big.NewRat(100, 1), big.NewRat(100, 1))
	balance.On("Credit", mock.Anything)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.OrchestratorInfo = &net.OrchestratorInfo{
		Transcoder: ts.URL,
		AuthToken:  stubAuthToken,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  2,
			PixelsPerUnit: 2,
		},
	}

	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}

	// Refresh session for expired auth token
	sess.OrchestratorInfo.AuthToken = &net.AuthToken{Token: []byte("foo"), SessionId: "bar", Expiration: time.Now().Add(-1 * time.Hour).Unix()}
	errC := make(chan error)
	go func() {
		res, _, err := transcodeSegment(cxn, &stream.HLSSegment{Name: "s1", Duration: 900}, "dummy", nil)
		assert.Len(res, 1)
		errC <- err
	}()
	<-segStarted
	assert.Len(cxn.sessManager.lastSess.SegsInFlight, 1)
	go func() {
		res, _, err := transcodeSegment(cxn, &stream.HLSSegment{Name: "s2", Duration: 900}, "dummy", nil)
		assert.Nil(err)
		assert.Len(res, 1)
		errC <- err
	}()
	<-segStarted
	assert.Len(cxn.sessManager.lastSess.SegsInFlight, 2, "wrong length %d", len(cxn.sessManager.lastSess.SegsInFlight))

	segDone <- nil
	segDone <- nil
	err = <-errC
	assert.Nil(err)
	err = <-errC
	assert.Nil(err)
	assert.Equal(1, orchInfoCalled)
}

func TestSelectSession_MultipleInFlight(t *testing.T) {
	bsm := newSessionsManagerLIFO(StubBroadcastSessionsManager())

	sendSegStub := func() *BroadcastSession {
		sess := bsm.selectSession()
		bsm.sessLock.Lock()
		if sess == nil {
			bsm.sessLock.Unlock()
			return nil
		}
		bsm.sessLock.Unlock()
		seg := &stream.HLSSegment{Data: []byte("dummy"), Duration: 0.100}
		bsm.pushSegInFlight(sess, seg)
		return sess
	}

	completeSegStub := func(sess *BroadcastSession) {
		// Create dummy result
		res := &ReceivedTranscodeResult{
			LatencyScore: sess.LatencyScore,
			Info:         sess.OrchestratorInfo,
		}
		bsm.completeSession(updateSession(sess, res))
	}

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)

	expectedSess0 := &BroadcastSession{}
	expectedSess1 := &BroadcastSession{}
	*expectedSess0 = *(bsm.sessList()[1])
	*expectedSess1 = *(bsm.sessList()[0])

	// send in multiple segments at the same time and verify SegsInFlight & lastSess are updated
	sess0 := sendSegStub()
	assert.Equal(bsm.lastSess, sess0)
	assert.Equal(expectedSess0.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(bsm.lastSess.SegsInFlight, 1)

	sess1 := sendSegStub()
	assert.Equal(bsm.lastSess, sess1)
	assert.Equal(sess0, sess1)
	assert.Len(bsm.lastSess.SegsInFlight, 2)

	completeSegStub(sess0)
	assert.Len(bsm.lastSess.SegsInFlight, 1)
	assert.Len(bsm.sessList(), 1)

	completeSegStub(sess1)
	assert.Len(bsm.lastSess.SegsInFlight, 0)
	assert.Len(bsm.sessList(), 2)

	// Same as above but to check thread safety, run this under -race
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { sess0 = sendSegStub(); wg.Done() }()
	go func() { sess1 = sendSegStub(); wg.Done() }()
	assert.True(wgWait(&wg), "Segment sending timed out")
	assert.Equal(bsm.lastSess, sess0)
	assert.Equal(sess0, sess1)
	assert.Len(bsm.lastSess.SegsInFlight, 2)
	wg.Add(2)
	go func() { completeSegStub(sess0); wg.Done() }()
	go func() { completeSegStub(sess1); wg.Done() }()
	assert.True(wgWait(&wg), "Segment completion timed out")
	assert.Len(bsm.lastSess.SegsInFlight, 0)

	// send in multiple segments with delay > segDur to trigger O switch
	sess0 = sendSegStub()
	assert.Equal(bsm.lastSess, sess0)
	assert.Equal(expectedSess0.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(bsm.lastSess.SegsInFlight, 1)

	time.Sleep(110 * time.Millisecond)

	sess1 = sendSegStub()
	assert.Equal(bsm.lastSess, sess1)
	assert.Equal(expectedSess1.OrchestratorInfo, sess1.OrchestratorInfo)
	assert.Len(bsm.lastSess.SegsInFlight, 1)

	completeSegStub(sess0)
	completeSegStub(sess1)

	// send in multiple segments with delay > segDur but < 2*segDur and only a single session available
	bsm.suspendOrch(expectedSess0)
	bsm.removeSession(expectedSess0)
	assert.Len(bsm.sessMap, 1)

	sess0 = sendSegStub()
	assert.Equal(bsm.lastSess, sess0)
	assert.Equal(expectedSess1.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(bsm.lastSess.SegsInFlight, 1)

	time.Sleep(110 * time.Millisecond)

	sess1 = sendSegStub()
	assert.Equal(bsm.lastSess, sess1)
	assert.Equal(expectedSess1.OrchestratorInfo, sess1.OrchestratorInfo)
	assert.Len(bsm.lastSess.SegsInFlight, 2)

	completeSegStub(sess0)
	completeSegStub(sess1)
	assert.Len(bsm.lastSess.SegsInFlight, 0)

	// send in multiple segments with delay > 2*segDur and only a single session available
	bsm.suspendOrch(expectedSess0)
	bsm.removeSession(expectedSess0)
	assert.Len(bsm.sessMap, 1)

	sess0 = sendSegStub()
	assert.Equal(bsm.lastSess, sess0)
	assert.Equal(expectedSess1.OrchestratorInfo, sess0.OrchestratorInfo)
	assert.Len(bsm.lastSess.SegsInFlight, 1)

	time.Sleep(210 * time.Millisecond)

	sess1 = sendSegStub()
	assert.Nil(sess1)
	assert.Nil(bsm.lastSess)

	completeSegStub(sess0)

	// remove both session and check if selector returns nil and sets lastSession to nil
	bsm.suspendOrch(expectedSess0)
	bsm.suspendOrch(expectedSess1)
	bsm.removeSession(expectedSess0)
	bsm.removeSession(expectedSess1)
	bsm.sessLock.Lock() // refresh session could be running in parallel and modifying sessMap
	assert.Len(bsm.sessMap, 0)
	bsm.sessLock.Unlock()

	sess0 = sendSegStub()
	assert.Nil(sess0)
	assert.Nil(bsm.lastSess)
}

func TestSelectSession_NilSession(t *testing.T) {
	bsm := StubBroadcastSessionsManager()
	// Replace selector with stubSelector that will return nil for Select(), but 1 for Size()
	bsm.sel = &stubSelector{size: 1}

	assert.Nil(t, bsm.selectSession())
}

func TestRemoveSession(t *testing.T) {
	bsm := newSessionsManagerLIFO(StubBroadcastSessionsManager())

	sess1 := bsm.sessList()[0]
	sess2 := bsm.sessList()[1]

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
	bsm := newSessionsManagerLIFO(StubBroadcastSessionsManager())

	sess1 := bsm.selectSession()

	// assert that initial lengths are as expected
	assert := assert.New(t)
	assert.Len(bsm.sessList(), 1)
	assert.Len(bsm.sessMap, 2)

	bsm.completeSession(sess1)

	// assert that session already in sessMap is added back to sessList
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)
	assert.Equal(sess1, bsm.sessMap[sess1.OrchestratorInfo.Transcoder])

	// assert that we get the same session back next time we call select
	newSess := bsm.selectSession()
	assert.Equal(sess1, newSess)
	bsm.completeSession(newSess)

	// assert that session not in sessMap is not added to sessList
	sess3 := StubBroadcastSession("transcoder3")
	bsm.completeSession(sess3)
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)

	sess1 = bsm.selectSession()

	copiedSess := &BroadcastSession{}
	*copiedSess = *sess1
	copiedSess.LatencyScore = 2.7
	bsm.completeSession(copiedSess)

	// assert that existing session with same key in sessMap is replaced
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)
	assert.NotEqual(sess1, bsm.sessMap[copiedSess.OrchestratorInfo.Transcoder])
	assert.Equal(copiedSess, bsm.sessMap[copiedSess.OrchestratorInfo.Transcoder])
}

func TestRefreshSessions(t *testing.T) {
	bsm := newSessionsManagerLIFO(StubBroadcastSessionsManager())

	assert := assert.New(t)
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)

	sess1 := bsm.sessList()[0]
	sess2 := bsm.sessList()[1]
	bsm.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{sess1, sess2}, nil
	}

	// asserting that pre-existing sessions are not added to sessList or sessMap
	bsm.refreshSessions()
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)

	sess3 := StubBroadcastSession("transcoder3")
	sess4 := StubBroadcastSession("transcoder4")

	bsm.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{sess3, sess4}, nil
	}

	// asserting that new sessions are added to beginning of sessList and sessMap
	bsm.refreshSessions()
	assert.Len(bsm.sessList(), 4)
	assert.Len(bsm.sessMap, 4)
	assert.Equal(bsm.sessList()[0], sess3)
	assert.Equal(bsm.sessList()[1], sess4)

	// asserting that refreshes stop while another is in-flight
	bsm.createSessions = func() ([]*BroadcastSession, error) {
		return []*BroadcastSession{StubBroadcastSession("5"), StubBroadcastSession("6")}, nil
	}
	bsm.refreshing = true
	bsm.refreshSessions()
	assert.Len(bsm.sessList(), 4)
	assert.Len(bsm.sessMap, 4)
	assert.Equal(bsm.sessList()[0], sess3)
	assert.Equal(bsm.sessList()[1], sess4)
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
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 0)
	bsm.refreshSessions()
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 0)

	// check exit errors from createSession. Run this under -race
	bsm = newSessionsManagerLIFO(StubBroadcastSessionsManager()) // reset bsm from previous tests

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
	bsm := newSessionsManagerLIFO(StubBroadcastSessionsManager())

	// sanity checks
	assert := assert.New(t)
	assert.Len(bsm.sessList(), 2)
	assert.Len(bsm.sessMap, 2)

	// check relevant fields are reset
	bsm.cleanup()
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 0)
}

// Note: Add processSegment tests, including:
//     assert an error from transcoder removes sess from BroadcastSessionManager
//     assert a success re-adds sess to BroadcastSessionManager

func TestTranscodeSegment_UploadFailed_SuspendAndRemove(t *testing.T) {
	assert := assert.New(t)
	mid := core.ManifestID("foo")
	pl := &stubPlaylistManager{manifestID: mid}
	// drivers.S3BUCKET = "livepeer"
	mem := &stubOSSession{err: errors.New("some error")}
	assert.NotNil(mem)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseURL := "https://livepeer.s3.amazonaws.com"
	sess := genBcastSess(ctx, t, baseURL, mem, mid)
	sess.OrchestratorOS = mem

	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn := &rtmpConnection{
		mid:         mid,
		pl:          pl,
		profile:     &ffmpeg.P240p30fps16x9,
		sessManager: bsm,
	}
	seg := &stream.HLSSegment{}
	_, _, err := transcodeSegment(cxn, seg, "dummy", nil)
	assert.EqualError(err, "some error")
	_, ok := cxn.sessManager.sessMap[sess.OrchestratorInfo.GetTranscoder()]
	assert.False(ok)
	assert.Greater(cxn.sessManager.sus.Suspended(sess.OrchestratorInfo.GetTranscoder()), 0)
}

func TestTranscodeSegment_RefreshSession(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	tr := &net.TranscodeResult{
		Info: &net.OrchestratorInfo{
			Transcoder:   ts.URL,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7},
			TicketParams: &net.TicketParams{ExpirationBlock: big.NewInt(100).Bytes()},
			AuthToken:    stubAuthToken,
		},
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{{Url: "test.flv"}},
				Sig:      []byte("bar"),
			},
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	sender := &pm.MockSender{}
	sess.Sender = sender
	balance := &mockBalance{}
	sess.Balance = balance
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.OrchestratorInfo = &net.OrchestratorInfo{
		Transcoder: ts.URL,
		AuthToken:  stubAuthToken,
	}

	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}

	// Validate TicketParams error (not ErrTicketParamsExpired) -> Don't refresh, remove session & suspend orch
	sender.On("ValidateTicketParams", mock.Anything).Return(errors.New("some error")).Once()
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.True(strings.Contains(err.Error(), "some error"))
	_, ok := cxn.sessManager.sessMap[ts.URL]
	assert.False(ok)
	assert.Greater(cxn.sessManager.sus.Suspended(ts.URL), 0)

	cxn = &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}
	// Expired ticket params -> GetOrchestratorInfo error -> Error
	sender.On("ValidateTicketParams", mock.Anything).Return(pm.ErrTicketParamsExpired)
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.True(strings.Contains(err.Error(), "Could not get orchestrator"))
	_, ok = cxn.sessManager.sessMap[ts.URL]
	assert.False(ok)
	assert.Greater(cxn.sessManager.sus.Suspended(ts.URL), 0)

	// Expired ticket params -> GetOrchestratorInfo -> Still Expired -> Error
	cxn = &rtmpConnection{
		mid:         core.ManifestID("bar"),
		nonce:       8,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("bar")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}

	successOrchInfoUpdate := &net.OrchestratorInfo{
		Transcoder: ts.URL,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
		TicketParams: &net.TicketParams{},
		AuthToken:    stubAuthToken,
	}

	oldGetOrchestratorInfoRPC := getOrchestratorInfoRPC
	defer func() { getOrchestratorInfoRPC = oldGetOrchestratorInfoRPC }()

	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return successOrchInfoUpdate, nil
	}

	sender.On("StartSession", mock.Anything).Return(mock.Anything)
	sender.On("EV", mock.Anything).Return(big.NewRat(1000000, 1), nil)
	balance.On("StageUpdate", mock.Anything, mock.Anything).Return(1, big.NewRat(100, 1), big.NewRat(100, 1))
	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(nil, pm.ErrTicketParamsExpired).Once()
	balance.On("Credit", mock.Anything)
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.EqualError(err, pm.ErrTicketParamsExpired.Error())
	_, ok = cxn.sessManager.sessMap[ts.URL]
	assert.False(ok)
	assert.Greater(cxn.sessManager.sus.Suspended(ts.URL), 0)

	// Expired ticket params -> GetOrchestratorInfo -> No Longer Expired -> Complete Session
	cxn = &rtmpConnection{
		mid:         core.ManifestID("baz"),
		nonce:       9,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("baz")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)
	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(defaultTicketBatch(), nil)
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.Nil(err)

	completedSess := cxn.sessManager.sessMap[ts.URL]
	assert.NotEqual(completedSess, sess)
	assert.NotZero(completedSess.LatencyScore)

	// Check that BroadcastSession.OrchestratorInfo was updated
	completedSessInfo := cxn.sessManager.sessMap[tr.Info.Transcoder].OrchestratorInfo
	assert.Equal(tr.Info.Transcoder, completedSessInfo.Transcoder)
	assert.Equal(tr.Info.PriceInfo.PixelsPerUnit, completedSessInfo.PriceInfo.PixelsPerUnit)
	assert.Equal(tr.Info.PriceInfo.PricePerUnit, completedSessInfo.PriceInfo.PricePerUnit)
	assert.Equal(tr.Info.TicketParams.ExpirationBlock, completedSessInfo.TicketParams.ExpirationBlock)

	// Missing auth token
	sess.OrchestratorInfo.AuthToken = nil
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{}, "dummy", nil)
	assert.Equal("missing auth token", err.Error())

	// Refresh session for expired auth token
	sess.OrchestratorInfo.AuthToken = &net.AuthToken{Token: []byte("foo"), SessionId: "bar", Expiration: time.Now().Add(-1 * time.Hour).Unix()}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{}, "dummy", nil)
	assert.Nil(err)

	completedSessInfo = cxn.sessManager.sessMap[tr.Info.Transcoder].OrchestratorInfo
	assert.True(time.Now().Before(time.Unix(completedSessInfo.AuthToken.Expiration, 0)))
	assert.True(proto.Equal(completedSessInfo.AuthToken, stubAuthToken))

	// Refresh session for almost expired auth token
	sess.OrchestratorInfo.AuthToken = &net.AuthToken{Token: []byte("foo"), SessionId: "bar", Expiration: time.Now().Add(30 * time.Second).Unix()}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})
	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{}, "dummy", nil)
	assert.Nil(err)

	completedSessInfo = cxn.sessManager.sessMap[tr.Info.Transcoder].OrchestratorInfo
	assert.True(time.Now().Before(time.Unix(completedSessInfo.AuthToken.Expiration, 0)))
	assert.True(proto.Equal(completedSessInfo.AuthToken, stubAuthToken))
}

func TestTranscodeSegment_SuspendOrchestrator(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()

	tr := &net.TranscodeResult{
		Info: &net.OrchestratorInfo{
			Transcoder:   ts.URL,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7},
			TicketParams: &net.TicketParams{ExpirationBlock: big.NewInt(100).Bytes()},
		},
		Result: &net.TranscodeResult_Error{
			Error: "OrchestratorBusy",
		},
	}

	buf, err := proto.Marshal(tr)
	require.Nil(err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	bsm.poolSize = 40
	bsm.numOrchs = 8
	selSize := bsm.sel.Size()
	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)

	assert.EqualError(err, "OrchestratorBusy")
	assert.Equal(bsm.sus.Suspended(ts.URL), bsm.poolSize/selSize)
}

func TestTranscodeSegment_CompleteSession(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{{Url: "test.flv"}},
				Sig:      []byte("bar"),
			},
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.Nil(err)

	completedSess := bsm.sessMap[ts.URL]
	assert.NotEqual(completedSess.LatencyScore, sess.LatencyScore)
	assert.NotZero(completedSess.LatencyScore)

	// Check that the completed session is just the original session with a different LatencyScore
	copiedSess := &BroadcastSession{}
	*copiedSess = *completedSess
	copiedSess.LatencyScore = 0.0
	assert.Equal(copiedSess.Broadcaster, sess.Broadcaster)
	assert.Equal(copiedSess.LatencyScore, sess.LatencyScore)
	assert.Equal(copiedSess.OrchestratorInfo, sess.OrchestratorInfo)

	tr.Info = &net.OrchestratorInfo{Transcoder: ts.URL, PriceInfo: &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7}}
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.Nil(err)

	// Check that BroadcastSession.OrchestratorInfo was updated
	completedSessInfo := bsm.sessMap[ts.URL].OrchestratorInfo
	assert.Equal(tr.Info.Transcoder, completedSessInfo.Transcoder)
	assert.Equal(tr.Info.PriceInfo.PricePerUnit, completedSessInfo.PriceInfo.PricePerUnit)
	assert.Equal(tr.Info.PriceInfo.PixelsPerUnit, completedSessInfo.PriceInfo.PixelsPerUnit)
}

func TestProcessSegment_MaxAttempts(t *testing.T) {
	assert := assert.New(t)

	// Preliminaries and test setup
	oldAttempts := MaxAttempts
	defer func() {
		MaxAttempts = oldAttempts
	}()
	transcodeCalls := 0
	resp := func(w http.ResponseWriter, r *http.Request) {
		transcodeCalls++
	}
	ts1, mux1 := stubTLSServer()
	defer ts1.Close()
	ts2, mux2 := stubTLSServer()
	defer ts2.Close()
	mux1.HandleFunc("/segment", resp)
	mux2.HandleFunc("/segment", resp)
	sess1 := StubBroadcastSession(ts1.URL)
	sess2 := StubBroadcastSession(ts2.URL)
	bsm := bsmWithSessList([]*BroadcastSession{sess1, sess2})
	pl := &stubPlaylistManager{os: &stubOSSession{}}
	cxn := &rtmpConnection{
		profile:     &ffmpeg.VideoProfile{Name: "unused"},
		sessManager: bsm,
		pl:          pl,
	}
	seg := &stream.HLSSegment{}

	// Sanity check: zero attempts should not transcode
	MaxAttempts = 0
	_, err := processSegment(cxn, seg)
	assert.Nil(err)
	assert.Equal(0, transcodeCalls, "Unexpectedly submitted segment")
	assert.Len(bsm.sessMap, 2)

	// One failed transcode attempt. Should leave another in the map
	MaxAttempts = 1
	_, err = processSegment(cxn, seg)
	assert.NotNil(err)
	assert.Equal("Hit max transcode attempts: UnknownResponse", err.Error())
	assert.Equal(1, transcodeCalls, "Segment submission calls did not match")
	assert.Len(bsm.sessMap, 1)

	// Drain the swamp! Empty out the session list
	_, err = processSegment(cxn, seg)
	assert.NotNil(err)
	assert.Equal("Hit max transcode attempts: UnknownResponse", err.Error())
	assert.Equal(2, transcodeCalls, "Segment submission calls did not match")
	assert.Len(bsm.sessMap, 0) // Now empty

	// The session list is empty. TODO Should return an error indicating such
	// (This test should fail and be corrected once this is actually implemented)
	_, err = processSegment(cxn, seg)
	assert.Nil(err)
	assert.Equal(2, transcodeCalls, "Segment submission calls did not match")
	assert.Len(bsm.sessMap, 0)
}

type queueEvent struct {
	key  string
	data interface{}
}

type producerChan struct {
	C   chan queueEvent
	err error
}

func (p producerChan) Publish(ctx context.Context, key string, body interface{}, persistent bool) error {
	select {
	case p.C <- queueEvent{key, body}:
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p producerChan) receive(ctx context.Context) (queueEvent, bool) {
	select {
	case evt, ok := <-p.C:
		return evt, ok
	case <-ctx.Done():
		return queueEvent{}, false
	}
}

func TestProcessSegment_MetadataQueueTranscodeEvent(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Preliminaries and test setup
	oldQueue := MetadataQueue
	defer func() {
		MetadataQueue = oldQueue
	}()
	queue := producerChan{make(chan queueEvent, 1), nil}
	MetadataQueue = queue

	dummyRes := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{{Url: "test.flv", Pixels: 100}},
				Sig:      []byte("bar"),
			},
		},
	}
	transcodeResps := make(chan *net.TranscodeResult, 10)
	handler := func(w http.ResponseWriter, r *http.Request) {
		select {
		default:
			require.FailNow("must setup all responses")
		case resp := <-transcodeResps:
			if resp == nil {
				return // Cause UnknownResponse error
			}
			buf, err := proto.Marshal(resp)
			require.Nil(err)
			_, err = w.Write(buf)
			require.Nil(err)
		}
	}
	cxn := &rtmpConnection{
		mid:     "dummy1",
		params:  &core.StreamParameters{ManifestID: "dummy1", ExternalStreamID: "ext_dummy"},
		profile: &ffmpeg.VideoProfile{Name: "unused"},
		pl:      &stubPlaylistManager{os: &stubOSSession{}},
	}
	seg := &stream.HLSSegment{Data: []byte("dummy"), SeqNo: 123}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calls producer once with transcode event
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 2, handler))
	transcodeResps <- dummyRes
	_, err := processSegment(cxn, seg)
	assert.Nil(err)
	assert.Len(cxn.sessManager.sessMap, 2)
	evt, ok := queue.receive(ctx)
	require.True(ok)
	assert.Equal("stream_health.transcode.d.ext_dummy", evt.key)
	require.IsType(&data.TranscodeEvent{}, evt.data)
	transEvt := evt.data.(*data.TranscodeEvent)
	assert.Equal(true, transEvt.Success)
	assert.EqualValues(cxn.params.ExternalStreamID, transEvt.StreamID())
	assert.Equal(seg.SeqNo, transEvt.Segment.SeqNo)
	assert.Equal(1, len(transEvt.Attempts))
	assert.Nil(transEvt.Attempts[0].Error)

	// One failed transcode attempt. Failed attempt should be in transcode event
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 2, handler))
	transcodeResps <- nil
	transcodeResps <- dummyRes
	_, err = processSegment(cxn, seg)
	assert.Nil(err)
	assert.Len(cxn.sessManager.sessMap, 1)
	evt, ok = queue.receive(ctx)
	require.True(ok)
	require.IsType(&data.TranscodeEvent{}, evt.data)
	transEvt = evt.data.(*data.TranscodeEvent)
	assert.Equal(true, transEvt.Success)
	assert.Equal(2, len(transEvt.Attempts))
	assert.NotNil(transEvt.Attempts[0].Error)
	assert.Equal("UnknownResponse", *transEvt.Attempts[0].Error)

	// All failed transcode attempts. Transcode event should be sent with success=false
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 3, handler))
	transcodeResps <- nil
	transcodeResps <- nil
	transcodeResps <- nil
	_, err = processSegment(cxn, seg)
	assert.NotNil(err)
	assert.Len(cxn.sessManager.sessMap, 0)
	evt, ok = queue.receive(ctx)
	require.True(ok)
	require.IsType(&data.TranscodeEvent{}, evt.data)
	transEvt = evt.data.(*data.TranscodeEvent)
	assert.Equal(false, transEvt.Success)
	assert.Equal(3, len(transEvt.Attempts))
	for _, attempt := range transEvt.Attempts {
		assert.NotNil(attempt.Error)
		assert.Equal("UnknownResponse", *attempt.Error)
	}

	// Empty session list. Transcode event should still have success=false
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{})
	_, err = processSegment(cxn, seg)
	assert.Nil(err)
	assert.Len(cxn.sessManager.sessMap, 0)
	evt, ok = queue.receive(ctx)
	require.True(ok)
	require.IsType(&data.TranscodeEvent{}, evt.data)
	transEvt = evt.data.(*data.TranscodeEvent)
	assert.Equal(false, transEvt.Success)
	assert.Equal(1, len(transEvt.Attempts))
	assert.Nil(transEvt.Attempts[0].Error)

	// Should not fail processing on queue error
	queue.err = errors.New("publish failure")
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 1, handler))
	transcodeResps <- dummyRes
	_, err = processSegment(cxn, seg)
	assert.Nil(err)
	assert.Len(cxn.sessManager.sessMap, 1)
	evt, ok = queue.receive(ctx)
	require.True(ok)
	require.IsType(&data.TranscodeEvent{}, evt.data)
	assert.Equal(true, evt.data.(*data.TranscodeEvent).Success)

	// Uses manifest ID if external stream ID or params not present
	testMissingStreamID := func() {
		transcodeResps <- dummyRes
		_, err = processSegment(cxn, seg)
		assert.Nil(err)
		evt, ok = queue.receive(ctx)
		require.True(ok)
		assert.Equal("stream_health.transcode.d.dummy1", evt.key)
		require.IsType(&data.TranscodeEvent{}, evt.data)
		assert.EqualValues(cxn.mid, evt.data.(*data.TranscodeEvent).StreamID())
	}
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 1, handler))
	cxn.params.ExternalStreamID = ""
	testMissingStreamID()
	cxn.params = nil
	testMissingStreamID()

	// ensure no left-overs
	assert.Zero(len(transcodeResps))
	assert.Zero(len(queue.C))
}

func TestTranscodeSegment_VerifyPixels(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dummyRes := func(tSegData []*net.TranscodedSegmentData) *net.TranscodeResult {
		return &net.TranscodeResult{
			Result: &net.TranscodeResult_Data{
				Data: &net.TranscodeData{
					Segments: tSegData,
					Sig:      []byte("bar"),
				},
			},
		}
	}

	// Create stub response with incorrect reported pixels
	tSegData := []*net.TranscodedSegmentData{
		{Url: "test.flv", Pixels: 100},
	}
	tr := dummyRes(tSegData)
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	urls, _, err := transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil)
	assert.Nil(err)
	assert.NotNil(urls)
	assert.Len(urls, 1)
	assert.Equal("test.flv", urls[0])

	// Wait for async pixels verification to finish (or in this case we are just making sure that it did NOT run)
	time.Sleep(1 * time.Second)

	// Check that the session was NOT removed because we are in off-chain mode
	_, ok := bsm.sessMap[ts.URL]
	assert.True(ok)

	sess.OrchestratorInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}
	sender := &pm.MockSender{}
	sess.Sender = sender
	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)

	urls, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil)
	assert.Nil(err)
	assert.Equal("test.flv", urls[0])

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	_, _, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil)
	assert.Nil(err)

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	// Check that the session was not removed
	_, ok = bsm.sessMap[ts.URL]
	assert.True(ok)
}

func TestUpdateSession(t *testing.T) {
	assert := assert.New(t)

	balances := core.NewAddressBalances(5 * time.Minute)
	defer balances.StopCleanup()
	sess := &BroadcastSession{PMSessionID: "foo", LatencyScore: 1.1, Balances: balances}
	res := &ReceivedTranscodeResult{
		LatencyScore: 2.1,
	}
	newSess := updateSession(sess, res)
	assert.Equal(res.LatencyScore, newSess.LatencyScore)
	// Check that LatencyScore of old session is not mutated
	assert.Equal(1.1, sess.LatencyScore)

	info := &net.OrchestratorInfo{
		Storage: []*net.OSInfo{
			{
				StorageType: 1,
				S3Info:      &net.S3OSInfo{Host: "http://apple.com"},
			},
		},
	}
	res.Info = info

	newSess = updateSession(sess, res)
	assert.Equal(info, newSess.OrchestratorInfo)
	// Check that BroadcastSession.OrchestratorOS is updated when len(info.Storage) > 0
	assert.Equal(info.Storage[0], newSess.OrchestratorOS.GetInfo())
	// Check that a new PM session is not created because BroadcastSession.Sender = nil
	assert.Equal("foo", newSess.PMSessionID)
	// Check that OrchestratorInfo of old session is not mutated
	assert.Nil(sess.OrchestratorInfo)

	sender := &pm.MockSender{}
	sess.Sender = sender
	sess.OrchestratorInfo = &net.OrchestratorInfo{AuthToken: stubAuthToken}
	res.Info = &net.OrchestratorInfo{
		TicketParams: &net.TicketParams{},
		PriceInfo:    &net.PriceInfo{},
		AuthToken:    stubAuthToken,
	}
	sender.On("StartSession", mock.Anything).Return("foo").Once()
	newSess = updateSession(sess, res)
	// Check that a new PM session is not created because OrchestratorInfo.TicketParams = nil
	assert.Equal("foo", newSess.PMSessionID)

	sender.On("StartSession", mock.Anything).Return("bar")
	newSess = updateSession(sess, res)
	// Check that a new PM session is created
	assert.Equal("bar", newSess.PMSessionID)
	// Check that PMSessionID of old session is not mutated
	assert.Equal("foo", sess.PMSessionID)
	// Check that Balance of new session is not different because auth token sessionID did not change
	assert.Equal(newSess.Balance, sess.Balance)

	res.Info.AuthToken = &net.AuthToken{SessionId: "diffdiff"}
	newSess = updateSession(sess, res)
	// Check that Balance of new session is different because auth token sessionID did change
	assert.NotEqual(newSess.Balance, sess.Balance)
	// Check that new Balance initialized with new auth token sessionID
	assert.Nil(balances.Balance(ethcommon.Address{}, core.ManifestID("diffdiff")))
	newSess.Balance.Credit(big.NewRat(5, 1))
	assert.Equal(balances.Balance(ethcommon.Address{}, core.ManifestID("diffdiff")), big.NewRat(5, 1))
}

func TestHLSInsertion(t *testing.T) {
	assert := assert.New(t)

	segData := []*net.TranscodedSegmentData{
		{Url: "/path/to/video", Pixels: 100},
	}

	buf, err := proto.Marshal(&net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{Segments: segData},
		},
	})
	assert.Nil(err)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.Params.ManifestID = core.ManifestID("foo")
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	pl := &stubPlaylistManager{manifestID: core.ManifestID("foo")}
	cxn := &rtmpConnection{
		mid:         sess.Params.ManifestID,
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P240p30fps16x9,
		sessManager: bsm,
	}

	seg := &stream.HLSSegment{SeqNo: 93}
	_, _, err = transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)

	// some sanity checks
	assert.Greater(len(sess.Params.Profiles), 0)
	assert.NotEqual(pl.profile, *cxn.profile, "HLS profile matched")

	// Check HLS insertion
	assert.Equal(seg.SeqNo, pl.seq, "HLS insertion failed")
	assert.Equal(pl.profile, sess.Params.Profiles[0], "HLS profile mismatch")
}

func TestVerifier_Invocation(t *testing.T) {
	// Various tests around ensuring that the verifier itself is invoked within
	// transcodeSegment, as well as various unusual verifier configurations

	require := require.New(t)
	assert := assert.New(t)

	// Stub verifier
	verifier := &stubVerifier{}
	policy := &verification.Policy{Verifier: verifier}
	segmentVerifier := verification.NewSegmentVerifier(policy)

	// Create stub server
	ts, mux := stubTLSServer()
	defer ts.Close()
	buf, err := proto.Marshal(&net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{},
		},
	})
	require.Nil(err)
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.Params.ManifestID = core.ManifestID("foo")
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	pl := &stubPlaylistManager{manifestID: core.ManifestID("foo")}
	cxn := &rtmpConnection{
		mid:         sess.Params.ManifestID,
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	seg := &stream.HLSSegment{}
	_, _, err = transcodeSegment(cxn, seg, "dummy", segmentVerifier)
	assert.Nil(err)
	assert.Equal(1, verifier.calls)
	require.NotNil(verifier.params)
	assert.Equal(cxn.mid, verifier.params.ManifestID)
	assert.Equal(seg, verifier.params.Source)
	// Do it again for good measure
	_, _, err = transcodeSegment(cxn, seg, "dummy", segmentVerifier)
	assert.Nil(err)
	assert.Equal(2, verifier.calls)

	// now "disable" the verifier and ensure no calls
	_, _, err = transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)
	assert.Equal(2, verifier.calls)

	// Pass in a nil policy
	_, _, err = transcodeSegment(cxn, seg, "dummy", verification.NewSegmentVerifier(nil))
	assert.Nil(err)

	// Pass in a policy but no verifier specified
	policy = &verification.Policy{}
	_, _, err = transcodeSegment(cxn, seg, "dummy", verification.NewSegmentVerifier(policy))
	assert.Nil(err)
}

func TestVerifier_Verify(t *testing.T) {
	assert := assert.New(t)
	os := drivers.NewMemoryDriver(nil).NewSession("")
	oldjpqt := core.JsonPlaylistQuitTimeout
	defer func() {
		core.JsonPlaylistQuitTimeout = oldjpqt
	}()
	core.JsonPlaylistQuitTimeout = 0 * time.Second
	c := core.NewBasicPlaylistManager(core.ManifestID("streamName"), os, nil)
	cxn := &rtmpConnection{
		pl: c,
	}
	sess := &BroadcastSession{Params: &core.StreamParameters{}}
	source := &stream.HLSSegment{}
	res := &net.TranscodeData{}
	verifier := verification.NewSegmentVerifier(&verification.Policy{})
	URIs := []string{}
	renditionData := [][]byte{}
	err := verify(verifier, cxn, sess, source, res, URIs, renditionData)
	assert.Nil(err)

	sess.Params.ManifestID = core.ManifestID("streamName")
	URIs = append(URIs, "filename")
	renditionData = [][]byte{[]byte("foo")}

	// Check non-retryable errors
	sess.OrchestratorInfo = &net.OrchestratorInfo{Transcoder: "asdf"}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm
	sv := &stubVerifier{err: errors.New("NonRetryable")}
	verifier = newStubSegmentVerifier(sv)
	assert.Equal(0, sv.calls)  // sanity check initial call count
	assert.Len(bsm.sessMap, 1) // sanity check initial bsm map
	err = verify(verifier, cxn, sess, source, res, URIs, renditionData)
	assert.NotNil(err)
	assert.Equal(1, sv.calls)
	assert.Equal(sv.err, err)
	assert.Len(bsm.sessMap, 1) // No effect on map for now

	// Check retryable errors, esp broadcast session removal from manager
	sv.err = verification.ErrTampered
	sv.retries = 10 // Do this to ensure we get a nil result
	_, retryable := sv.err.(verification.Retryable)
	assert.True(retryable)
	verifier = newStubSegmentVerifier(sv)
	err = verify(verifier, cxn, sess, source, res, URIs, renditionData)
	assert.NotNil(err)
	assert.Equal(2, sv.calls)
	assert.Equal(sv.err, err)
	assert.Len(bsm.sessMap, 0)

	// When retries are set to 0, results are returned anyway in case of error
	// (and more generally, when attempts > retries)

	// Check data gets re-saved into OS if retry succeeds with earlier params
	sv = &stubVerifier{
		retries: 1,
		err:     verification.ErrTampered,
		results: []verification.Results{
			{Score: 9},
			{Score: 1},
		},
	}
	mem, ok := drivers.NewMemoryDriver(nil).NewSession("streamName").(*drivers.MemorySession)
	assert.True(ok)
	name, err := mem.SaveData("/rendition/seg/1", []byte("attempt1"), nil, 0)
	assert.Nil(err)
	assert.Equal([]byte("attempt1"), mem.GetData(name))
	sess.BroadcasterOS = mem
	verifier = newStubSegmentVerifier(sv)
	URIs[0] = name
	renditionData = [][]byte{[]byte("attempt1")}
	err = verify(verifier, cxn, sess, source, res, URIs, renditionData)
	assert.Equal(sv.err, err)

	// Now "insert" 2nd attempt into OS
	// and ensure 1st attempt is what remains after verification
	_, err = mem.SaveData("/rendition/seg/1", []byte("attempt2"), nil, 0)
	assert.Nil(err)
	assert.Equal([]byte("attempt2"), mem.GetData(name))
	renditionData = [][]byte{[]byte("attempt2")}
	err = verify(verifier, cxn, sess, source, res, URIs, renditionData)
	assert.Nil(err)
	assert.Equal([]byte("attempt1"), mem.GetData(name))
	c.Cleanup()
}

func TestVerifier_HLSInsertion(t *testing.T) {
	assert := assert.New(t)

	// Ensure that the playlist has the correct URL after verification
	// Following this sequence:
	//   1. Verify seg1. Verification fails.
	//   2. Verify seg2. Verification fails.
	//   3. Verify seg3. Verification fails
	//   4. Max retries hit, assume false positives
	//   5. Return seg2 as highest scoring result
	//   6. Insert seg2 into playlist
	mid := core.ManifestID("foo")
	pl := &stubPlaylistManager{manifestID: mid}
	// drivers.S3BUCKET = "livepeer"
	S3BUCKET := "livepeer"
	mem := drivers.NewS3Driver("", S3BUCKET, "", "", false).NewSession(string(mid))
	assert.NotNil(mem)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseURL := "https://livepeer.s3.amazonaws.com"
	bsm := bsmWithSessList([]*BroadcastSession{
		genBcastSess(ctx, t, baseURL+"/resp1", mem, mid),
		genBcastSess(ctx, t, baseURL+"/resp2", mem, mid),
		genBcastSess(ctx, t, baseURL+"/resp3", mem, mid),
	})
	cxn := &rtmpConnection{
		mid:         mid,
		pl:          pl,
		profile:     &ffmpeg.P240p30fps16x9,
		sessManager: bsm,
	}
	seg := &stream.HLSSegment{}
	verifier := newStubSegmentVerifier(&stubVerifier{
		retries: 2,
		err:     verification.ErrTampered,
		results: []verification.Results{
			{Score: 5, Pixels: []int64{100}},
			{Score: 9, Pixels: []int64{100}},
			{Score: 1, Pixels: []int64{100}},
		},
	})

	oldDownloadSeg := downloadSeg
	defer func() { downloadSeg = oldDownloadSeg }()
	downloadSeg = func(url string) ([]byte, error) { return []byte("foo"), nil }

	_, _, err := transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Equal(verification.ErrTampered, err)
	assert.Empty(pl.uri) // sanity check that no insertion happened

	_, _, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Equal(verification.ErrTampered, err)
	assert.Empty(pl.uri)

	_, _, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Nil(err)
	assert.Equal(baseURL+"/resp2", pl.uri)
}

func TestDownloadSegError_SuspendAndRemove(t *testing.T) {
	assert := assert.New(t)
	mid := core.ManifestID("foo")
	pl := &stubPlaylistManager{manifestID: mid}
	S3BUCKET := "livepeer"
	mem := drivers.NewS3Driver("", S3BUCKET, "", "", false).NewSession(string(mid))
	assert.NotNil(mem)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseURL := "https://livepeer.s3.amazonaws.com"
	sess := genBcastSess(ctx, t, baseURL, mem, mid)

	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn := &rtmpConnection{
		mid:         mid,
		pl:          pl,
		profile:     &ffmpeg.P240p30fps16x9,
		sessManager: bsm,
	}
	seg := &stream.HLSSegment{}
	verifier := newStubSegmentVerifier(&stubVerifier{
		retries: 2,
		err:     verification.ErrTampered,
		results: []verification.Results{
			{Score: 5, Pixels: []int64{100}},
			{Score: 9, Pixels: []int64{100}},
			{Score: 1, Pixels: []int64{100}},
		},
	})
	oldDownloadSeg := downloadSeg
	defer func() { downloadSeg = oldDownloadSeg }()
	downloadSeg = func(url string) ([]byte, error) { return nil, errors.New("some error") }
	_, _, err := transcodeSegment(cxn, seg, "dummy", verifier)
	assert.EqualError(err, "some error")
	_, ok := cxn.sessManager.sessMap[sess.OrchestratorInfo.GetTranscoder()]
	assert.False(ok)
	assert.Greater(cxn.sessManager.sus.Suspended(sess.OrchestratorInfo.GetTranscoder()), 0)
}

func TestRefreshSession(t *testing.T) {
	assert := assert.New(t)
	successOrchInfoUpdate := &net.OrchestratorInfo{
		Transcoder: "foo",
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
		TicketParams: &net.TicketParams{},
	}

	oldGetOrchestratorInfoRPC := getOrchestratorInfoRPC
	defer func() { getOrchestratorInfoRPC = oldGetOrchestratorInfoRPC }()

	// trigger parse URL error
	sess := StubBroadcastSession(string(rune(0x7f)))
	newSess, err := refreshSession(sess)
	assert.Nil(newSess)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid control character in URL")

	// trigger getOrchestratorInfo error
	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return nil, errors.New("some error")
	}
	sess = StubBroadcastSession("foo")
	newSess, err = refreshSession(sess)
	assert.Nil(newSess)
	assert.EqualError(err, "some error")

	// trigger update
	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return successOrchInfoUpdate, nil
	}
	newSess, err = refreshSession(sess)
	assert.Equal(newSess.OrchestratorInfo, successOrchInfoUpdate)
	assert.Nil(err)

	// trigger timeout
	oldRefreshTimeout := refreshTimeout
	defer func() { refreshTimeout = oldRefreshTimeout }()
	refreshTimeout = 10 * time.Millisecond
	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, serv *url.URL) (*net.OrchestratorInfo, error) {
		// Wait until the refreshTimeout has elapsed
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
			return nil, errors.New("wrong error")
		}

		return nil, errors.New("context timeout")
	}
	newSess, err = refreshSession(sess)
	assert.Nil(newSess)
	assert.EqualError(err, "context timeout")
}

func defaultTicketBatch() *pm.TicketBatch {
	return &pm.TicketBatch{
		TicketParams: &pm.TicketParams{
			Recipient:       pm.RandAddress(),
			FaceValue:       big.NewInt(1234),
			WinProb:         big.NewInt(5678),
			Seed:            big.NewInt(7777),
			ExpirationBlock: big.NewInt(1000),
		},
		TicketExpirationParams: &pm.TicketExpirationParams{},
		Sender:                 pm.RandAddress(),
		SenderParams:           []*pm.TicketSenderParams{{SenderNonce: 777, Sig: pm.RandBytes(42)}},
	}
}

func TestVerifier_SegDownload(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mid := core.ManifestID("foo")

	externalOS := &stubOSSession{
		external: true,
		host:     "https://livepeer.s3.amazonaws.com",
	}
	bsm := bsmWithSessList([]*BroadcastSession{})
	cxn := &rtmpConnection{
		mid:         mid,
		pl:          &stubPlaylistManager{manifestID: mid},
		profile:     &ffmpeg.P240p30fps16x9,
		sessManager: bsm,
	}
	seg := &stream.HLSSegment{}

	oldDownloadSeg := downloadSeg
	defer func() { downloadSeg = oldDownloadSeg }()

	downloaded := make(map[string]bool)
	downloadSeg = func(url string) ([]byte, error) {
		downloaded[url] = true

		return []byte("foo"), nil
	}

	//
	// Tests when there is no verification policy
	//

	// When there is no broadcaster OS, segments should not be downloaded
	url := "somewhere1"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, nil, mid)})
	_, _, err := transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)
	assert.False(downloaded[url])

	// When segments are in the broadcaster's external OS, segments should not be downloaded
	url = "https://livepeer.s3.amazonaws.com/resp1"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)
	assert.False(downloaded[url])

	// When segments are not in the broadcaster's external OS, segments should be downloaded
	url = "somewhere2"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)
	assert.True(downloaded[url])

	//
	// Tests when there is a verification policy
	//

	verifier := newStubSegmentVerifier(&stubVerifier{retries: 100})

	// When there is no broadcaster OS, segments should be downloaded
	url = "somewhere3"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, nil, mid)})
	_, _, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Nil(err)
	assert.True(downloaded[url])

	// When segments are in the broadcaster's external OS, segments should be downloaded
	url = "https://livepeer.s3.amazonaws.com/resp2"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Nil(err)
	assert.True(downloaded[url])

	// When segments are not in the broadcaster's exernal OS, segments should be downloaded
	url = "somewhere4"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Nil(err)
	assert.True(downloaded[url])
}

func TestProcessSegment_VideoFormat(t *testing.T) {
	// Test format from saving "transcoder" data into broadcaster/transcoder OS.
	// For each rendition, check extension based on format (none, mp4, mpegts).
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bcastOS := &stubOSSession{host: "test://broad.com"}
	orchOS := &stubOSSession{host: "test://orch.com"}
	sess := genBcastSess(ctx, t, "", bcastOS, "")
	sess.OrchestratorOS = orchOS
	sess.Params.Profiles = append([]ffmpeg.VideoProfile{}, sess.Params.Profiles...)
	sourceProfile := ffmpeg.P240p30fps16x9 // make copy bc we mutate the preset
	cxn := &rtmpConnection{
		pl:          &stubPlaylistManager{os: bcastOS},
		profile:     &sourceProfile,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}
	seg := &stream.HLSSegment{}

	oldDownloadSeg := downloadSeg
	defer func() { downloadSeg = oldDownloadSeg }()
	downloadSeg = func(url string) ([]byte, error) { return []byte(url), nil }

	// processSegment will also call transcodeSegment; also check that behavior
	_, err := processSegment(cxn, seg)

	assert.Nil(err)
	assert.Equal(ffmpeg.FormatNone, cxn.profile.Format)
	for _, p := range sess.Params.Profiles {
		assert.Equal(ffmpeg.FormatNone, p.Format)
	}
	assert.Equal([]string{"P240p30fps16x9/0.ts"}, orchOS.saved)
	assert.Equal([]string{"P240p30fps16x9/0.ts", "P144p30fps16x9/0.ts"}, bcastOS.saved)
	assert.Equal("saved_P240p30fps16x9/0.ts", seg.Name)

	// Check MP4. Reset OS for simplicity
	bcastOS = &stubOSSession{host: "test://broad.com"}
	orchOS = &stubOSSession{host: "test://orch.com"}
	sess.BroadcasterOS = bcastOS
	sess.OrchestratorOS = orchOS
	cxn.pl = &stubPlaylistManager{os: bcastOS}
	cxn.profile.Format = ffmpeg.FormatMP4
	for i := range sess.Params.Profiles {
		sess.Params.Profiles[i].Format = ffmpeg.FormatMP4
	}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})

	_, err = processSegment(cxn, seg)

	assert.Nil(err)
	for _, p := range sess.Params.Profiles {
		assert.Equal(ffmpeg.FormatMP4, p.Format)
	}
	assert.Equal(ffmpeg.FormatMP4, cxn.profile.Format)
	assert.Equal([]string{"P240p30fps16x9/0.mp4"}, orchOS.saved)
	assert.Equal([]string{"P240p30fps16x9/0.mp4", "P144p30fps16x9/0.mp4"}, bcastOS.saved)
	assert.Equal("saved_P240p30fps16x9/0.mp4", seg.Name)

	// Check mpegts. Reset OS for simplicity
	bcastOS = &stubOSSession{host: "test://broad.com"}
	orchOS = &stubOSSession{host: "test://orch.com"}
	sess.BroadcasterOS = bcastOS
	sess.OrchestratorOS = orchOS
	cxn.pl = &stubPlaylistManager{os: bcastOS}
	cxn.profile.Format = ffmpeg.FormatMPEGTS
	for i := range sess.Params.Profiles {
		sess.Params.Profiles[i].Format = ffmpeg.FormatMPEGTS
	}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})

	_, err = processSegment(cxn, seg)

	assert.Nil(err)
	for _, p := range sess.Params.Profiles {
		assert.Equal(ffmpeg.FormatMPEGTS, p.Format)
	}
	assert.Equal(ffmpeg.FormatMPEGTS, cxn.profile.Format)
	assert.Equal([]string{"P240p30fps16x9/0.ts"}, orchOS.saved)
	assert.Equal([]string{"P240p30fps16x9/0.ts", "P144p30fps16x9/0.ts"}, bcastOS.saved)
	assert.Equal("saved_P240p30fps16x9/0.ts", seg.Name)
}

func TestProcessSegment_CheckDuration(t *testing.T) {
	assert := assert.New(t)
	seg := &stream.HLSSegment{Duration: -1.0}
	cxn := &rtmpConnection{}

	// Check less-than-zero
	_, err := processSegment(cxn, seg)
	assert.Equal("Invalid duration -1", err.Error())

	// CHeck greater than max duration
	seg.Duration = maxDurationSec + 0.01
	_, err = processSegment(cxn, seg)
	assert.Equal("Invalid duration 300.01", err.Error())
}

func genBcastSess(ctx context.Context, t *testing.T, url string, os drivers.OSSession, mid core.ManifestID) *BroadcastSession {
	segData := []*net.TranscodedSegmentData{
		{Url: url, Pixels: 100},
	}

	buf, err := proto.Marshal(&net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{Segments: segData},
		},
	})
	require.Nil(t, err, fmt.Sprintf("Could not marshal results for %s", url))
	transcoderURL := stubTestTranscoder(ctx, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	return &BroadcastSession{
		Broadcaster:      stubBroadcaster2(),
		Params:           &core.StreamParameters{ManifestID: mid, Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}, OS: os},
		BroadcasterOS:    os,
		OrchestratorInfo: &net.OrchestratorInfo{Transcoder: transcoderURL, AuthToken: stubAuthToken},
	}
}

func stubTestTranscoder(ctx context.Context, handler http.HandlerFunc) string {
	ts, mux := stubTLSServer()
	go func() { <-ctx.Done(); ts.Close() }()
	mux.HandleFunc("/segment", handler)
	return ts.URL
}
