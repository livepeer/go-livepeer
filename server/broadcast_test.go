package server

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/verification"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/livepeer-data/pkg/data"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/m3u8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
			AuthToken:    stubAuthToken,
			TicketParams: &net.TicketParams{Recipient: pm.RandAddress().Bytes()},
		},
		OrchestratorScore: common.Score_Trusted,
		lock:              &sync.RWMutex{},
		CleanupSession:    func(sessionId string) {},
	}
}

func bsmWithSessList(sessList []*BroadcastSession) *BroadcastSessionsManager {
	return bsmWithSessListExt(sessList, nil, false)
}

func cloneSessions(sessions []*BroadcastSession) []*BroadcastSession {
	res := make([]*BroadcastSession, len(sessions))
	for i, sess := range sessions {
		res[i] = sess.Clone()
	}
	return res
}

func bsmWithSessListExt(sessList, untrustedSessList []*BroadcastSession, noRefresh bool) *BroadcastSessionsManager {
	sessMap := make(map[string]*BroadcastSession)
	for _, sess := range sessList {
		sessMap[sess.OrchestratorInfo.Transcoder] = sess
	}

	sel := &LIFOSelector{}
	sel.Add(sessList)

	var createSessions = func() ([]*BroadcastSession, error) {
		// should return new sessions
		return cloneSessions(sessList), nil
	}

	var deleteSessions = func(sessionID string) {}

	untrustedSessMap := make(map[string]*BroadcastSession)
	for _, sess := range untrustedSessList {
		untrustedSessMap[sess.OrchestratorInfo.Transcoder] = sess
	}
	unsel := &LIFOSelector{}
	unsel.Add(untrustedSessList)
	var createSessionsUntrusted = func() ([]*BroadcastSession, error) {
		// should return new sessions
		return cloneSessions(untrustedSessList), nil
	}

	var createSessionsEmpty = func() ([]*BroadcastSession, error) {
		return nil, nil
	}
	if noRefresh {
		createSessions = createSessionsEmpty
		createSessionsUntrusted = createSessionsEmpty
	}
	trustedPool := NewSessionPool("test", len(sessList), 1, newSuspender(), createSessions, deleteSessions, sel)
	trustedPool.sessMap = sessMap
	untrustedPool := NewSessionPool("test", len(untrustedSessList), 1, newSuspender(), createSessionsUntrusted, deleteSessions, unsel)
	untrustedPool.sessMap = untrustedSessMap

	return &BroadcastSessionsManager{
		trustedPool:   trustedPool,
		untrustedPool: untrustedPool,
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
	sessList, _ := bsm.trustedPool.sel.(*LIFOSelector)
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

func (s *stubOSSession) SaveData(ctx context.Context, name string, data io.Reader, meta map[string]string, timeout time.Duration) (string, error) {
	s.saved = append(s.saved, name)
	return "saved_" + name, s.err
}
func (s *stubOSSession) EndSession() {
}
func (s *stubOSSession) GetInfo() *drivers.OSInfo {
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

func (s *stubSelector) Add(sessions []*BroadcastSession)         {}
func (s *stubSelector) Complete(sess *BroadcastSession)          {}
func (s *stubSelector) Select(context.Context) *BroadcastSession { return s.sess }
func (s *stubSelector) Size() int                                { return s.size }
func (s *stubSelector) Clear()                                   {}
func (s *stubSelector) Remove(session *BroadcastSession) bool    { return false }

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

	sess := NewSessionManager(context.TODO(), n, params)
	assert.Equal(0, sess.trustedPool.numOrchs)
	assert.Equal(0, sess.untrustedPool.numOrchs)

	// Check numOrchs up to maximum and a bit beyond
	sd := &stubDiscovery{}
	n.OrchestratorPool = sd
	max := int(common.HTTPTimeout.Seconds()/SegLen.Seconds()) * 2
	for i := 0; i < 10; i++ {
		sess = NewSessionManager(context.TODO(), n, params)
		if i < max {
			assert.Equal(i, sess.trustedPool.numOrchs)
		} else {
			assert.Equal(max, sess.trustedPool.numOrchs)
		}
		sd.infos = append(sd.infos, &net.OrchestratorInfo{PriceInfo: &net.PriceInfo{}})
	}
	// sanity check some expected postconditions
	assert.Equal(sess.trustedPool.numOrchs, max)
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
	sender.On("StopSession", mock.Anything).Times(3)
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
		res, _, err := transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Name: "s1", Duration: 900}, "dummy", nil, nil)
		assert.Len(res, 1)
		errC <- err
	}()
	<-segStarted
	assert.Len(cxn.sessManager.trustedPool.lastSess[0].SegsInFlight, 1)
	go func() {
		res, _, err := transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Name: "s2", Duration: 900}, "dummy", nil, nil)
		assert.Nil(err)
		assert.Len(res, 1)
		errC <- err
	}()
	<-segStarted
	assert.Len(cxn.sessManager.trustedPool.lastSess[0].SegsInFlight, 2, "wrong length %d", len(cxn.sessManager.trustedPool.lastSess[0].SegsInFlight))

	segDone <- nil
	segDone <- nil
	err = <-errC
	assert.Nil(err)
	err = <-errC
	assert.Nil(err)
	assert.Equal(1, orchInfoCalled)
}

func TestSelectSession_NoSegsInFlight(t *testing.T) {
	assert := require.New(t)
	ctx := context.Background()

	sess := &BroadcastSession{}
	sessList := []*BroadcastSession{sess}

	// Session has segs in flight
	sess.SegsInFlight = []SegFlightMetadata{
		{startTime: time.Now().Add(time.Duration(-2) * time.Second), segDur: 1 * time.Second},
	}
	s := selectSession(ctx, sessList, nil, 1)
	assert.Nil(s)

	// Session has no segs in flight, latency score = 0
	sess.SegsInFlight = nil
	s = selectSession(ctx, sessList, nil, 1)
	assert.Nil(s)

	// Session has no segs in flight, latency score > SELECTOR_LATENCY_SCORE_THRESHOLD
	sess.LatencyScore = SELECTOR_LATENCY_SCORE_THRESHOLD + 0.001
	s = selectSession(ctx, sessList, nil, 1)
	assert.Nil(s)

	// Session has no segs in flight, latency score > 0 and < SELECTOR_LATENCY_SCORE_THRESHOLD
	sess.LatencyScore = SELECTOR_LATENCY_SCORE_THRESHOLD - 0.001
	s = selectSession(ctx, sessList, nil, 1)
	assert.Equal(sess, s)
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
	_, _, err := transcodeSegment(context.TODO(), cxn, seg, "dummy", nil, nil)
	assert.EqualError(err, "some error")
	_, ok := cxn.sessManager.trustedPool.sessMap[sess.OrchestratorInfo.GetTranscoder()]
	assert.False(ok)
	assert.Greater(cxn.sessManager.trustedPool.sus.Suspended(sess.OrchestratorInfo.GetTranscoder()), 0)
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
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil, nil)
	assert.True(strings.Contains(err.Error(), "some error"))
	_, ok := cxn.sessManager.trustedPool.sessMap[ts.URL]
	assert.False(ok)
	assert.Greater(cxn.sessManager.trustedPool.sus.Suspended(ts.URL), 0)

	cxn = &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsmWithSessList([]*BroadcastSession{sess}),
	}
	// Expired ticket params -> GetOrchestratorInfo error -> Error
	sender.On("ValidateTicketParams", mock.Anything).Return(pm.ErrTicketParamsExpired)
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil, nil)
	assert.True(strings.Contains(err.Error(), "Could not get orchestrator"))
	_, ok = cxn.sessManager.trustedPool.sessMap[ts.URL]
	assert.False(ok)
	assert.Greater(cxn.sessManager.trustedPool.sus.Suspended(ts.URL), 0)

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
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil, nil)
	assert.EqualError(err, pm.ErrTicketParamsExpired.Error())
	_, ok = cxn.sessManager.trustedPool.sessMap[ts.URL]
	assert.False(ok)
	assert.Greater(cxn.sessManager.trustedPool.sus.Suspended(ts.URL), 0)

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
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil, nil)
	assert.Nil(err)

	completedSess := cxn.sessManager.trustedPool.sessMap[ts.URL]
	assert.Equal(completedSess, sess)
	assert.NotZero(completedSess.LatencyScore)

	// Check that BroadcastSession.OrchestratorInfo was updated
	completedSessInfo := cxn.sessManager.trustedPool.sessMap[tr.Info.Transcoder].OrchestratorInfo
	assert.Equal(tr.Info.Transcoder, completedSessInfo.Transcoder)
	assert.Equal(tr.Info.PriceInfo.PixelsPerUnit, completedSessInfo.PriceInfo.PixelsPerUnit)
	assert.Equal(tr.Info.PriceInfo.PricePerUnit, completedSessInfo.PriceInfo.PricePerUnit)
	assert.Equal(tr.Info.TicketParams.ExpirationBlock, completedSessInfo.TicketParams.ExpirationBlock)

	// Missing auth token
	sess.OrchestratorInfo.AuthToken = nil
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{}, "dummy", nil, nil)
	assert.Equal("missing auth token", err.Error())

	// Refresh session for expired auth token
	sess.OrchestratorInfo.AuthToken = &net.AuthToken{Token: []byte("foo"), SessionId: "bar", Expiration: time.Now().Add(-1 * time.Hour).Unix()}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{}, "dummy", nil, nil)
	assert.Nil(err)

	completedSessInfo = cxn.sessManager.trustedPool.sessMap[tr.Info.Transcoder].OrchestratorInfo
	assert.True(time.Now().Before(time.Unix(completedSessInfo.AuthToken.Expiration, 0)))
	assert.True(proto.Equal(completedSessInfo.AuthToken, stubAuthToken))

	// Refresh session for almost expired auth token
	sess.OrchestratorInfo.AuthToken = &net.AuthToken{Token: []byte("foo"), SessionId: "bar", Expiration: time.Now().Add(30 * time.Second).Unix()}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{}, "dummy", nil, nil)
	assert.Nil(err)

	completedSessInfo = cxn.sessManager.trustedPool.sessMap[tr.Info.Transcoder].OrchestratorInfo
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
	bsm.trustedPool.poolSize = 40
	bsm.trustedPool.numOrchs = 8
	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil, nil)

	assert.EqualError(err, "OrchestratorBusy")
	assert.Equal(bsm.trustedPool.sus.Suspended(ts.URL), bsm.trustedPool.poolSize/bsm.trustedPool.numOrchs)
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
	transcodeDelay := 1500 * time.Millisecond
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(transcodeDelay)
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

	segDurMultiplier := 100.0
	// The seg duration that meets the latency score threshold
	// A larger seg duration gives a smaller/better latency score
	// A smaller seg duration gives a larger/worse latency score
	segDurLatencyScoreThreshold := transcodeDelay.Seconds() / SELECTOR_LATENCY_SCORE_THRESHOLD
	// The seg duration that is better than the threshold (i.e. "fast")
	segDurFastLatencyScore := segDurLatencyScoreThreshold * segDurMultiplier
	// The seg duration that is worse than the threshold (i.e "slow")
	segDurSlowLatencyScore := segDurLatencyScoreThreshold / segDurMultiplier

	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: segDurSlowLatencyScore}, "dummy", nil, nil)
	assert.Nil(err)

	completedSess := bsm.trustedPool.sessMap[ts.URL]
	assert.Equal(sess, completedSess)
	assert.NotZero(completedSess.LatencyScore)
	// Check that session is returned to the selector because it did not pass the latency score threshold check
	assert.Equal(1, bsm.trustedPool.sel.Size())

	newInfo := StubBroadcastSession(ts.URL).OrchestratorInfo
	newInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7}
	tr.Info = newInfo
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: segDurFastLatencyScore}, "dummy", nil, nil)
	assert.Nil(err)

	// Check that BroadcastSession.OrchestratorInfo was updated
	completedSessInfo := bsm.trustedPool.sessMap[ts.URL].OrchestratorInfo
	assert.Equal(tr.Info.Transcoder, completedSessInfo.Transcoder)
	assert.Equal(tr.Info.PriceInfo.PricePerUnit, completedSessInfo.PriceInfo.PricePerUnit)
	assert.Equal(tr.Info.PriceInfo.PixelsPerUnit, completedSessInfo.PriceInfo.PixelsPerUnit)
	// Check that session is not returned to the selector because it passed the latency score threshold check
	assert.Zero(bsm.trustedPool.sel.Size())

	// Check that we can re-use the last session that previously passed the latency score threshold check
	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: segDurFastLatencyScore}, "dummy", nil, nil)
	assert.Nil(err)
	completedSessInfo = bsm.trustedPool.sessMap[ts.URL].OrchestratorInfo
	assert.Equal(sess.OrchestratorInfo.Transcoder, completedSessInfo.Transcoder)
	assert.Zero(bsm.trustedPool.sel.Size())
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
	bsm := bsmWithSessListExt([]*BroadcastSession{sess1, sess2}, nil, true)
	pl := &stubPlaylistManager{os: &stubOSSession{}}
	cxn := &rtmpConnection{
		profile:     &ffmpeg.VideoProfile{Name: "unused"},
		sessManager: bsm,
		pl:          pl,
	}
	seg := &stream.HLSSegment{}

	// Sanity check: zero attempts should not transcode
	MaxAttempts = 0
	_, err := processSegment(context.Background(), cxn, seg, nil)
	assert.Nil(err)
	assert.Equal(0, transcodeCalls, "Unexpectedly submitted segment")
	assert.Len(bsm.trustedPool.sessMap, 2)

	// One failed transcode attempt. Should leave another in the map
	MaxAttempts = 1
	transcodeCalls = 0
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.NotNil(err)
	assert.True(errors.Is(err, maxTranscodeAttempts))
	assert.Equal("hit max transcode attempts: UnknownResponse", err.Error())
	assert.Equal(1, transcodeCalls, "Segment submission calls did not match")
	assert.Len(bsm.trustedPool.sessMap, 1)

	// Context canceled. Execute once and not retry
	MaxAttempts = 10
	transcodeCalls = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = processSegment(ctx, cxn, seg, nil)
	assert.NotNil(err)
	assert.Contains("context canceled", err.Error())
	assert.Equal(1, transcodeCalls, "Segment submission calls did not match")
	assert.Len(bsm.trustedPool.sessMap, 0)

	// The session list is empty. TODO Should return an error indicating such
	// (This test should fail and be corrected once this is actually implemented)
	transcodeCalls = 0
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.Nil(err)
	assert.Equal(0, transcodeCalls, "Segment submission calls did not match")
	assert.Len(bsm.trustedPool.sessMap, 0)
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
		params:  &core.StreamParameters{ManifestID: "dummy1", ExternalStreamID: "ext_dummy", Profiles: BroadcastJobVideoProfiles},
		profile: &ffmpeg.VideoProfile{Name: "unused"},
		pl:      &stubPlaylistManager{os: &stubOSSession{}},
	}
	seg := &stream.HLSSegment{Data: []byte("dummy"), SeqNo: 123}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calls producer once with transcode event
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 2, handler))
	transcodeResps <- dummyRes
	_, err := processSegment(context.Background(), cxn, seg, nil)
	assert.Nil(err)
	assert.Len(cxn.sessManager.trustedPool.sessMap, 2)
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

	require.Equal(0, len(transcodeResps))
	// One failed transcode attempt. Failed attempt should be in transcode event
	cxn.sessManager = bsmWithSessList(stubSessionList(ctx, 2, handler))
	transcodeResps <- nil
	transcodeResps <- dummyRes
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.Nil(err)
	assert.Len(cxn.sessManager.trustedPool.sessMap, 1)
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
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.NotNil(err)
	assert.Len(cxn.sessManager.trustedPool.sessMap, 0)
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
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.Nil(err)
	assert.Len(cxn.sessManager.trustedPool.sessMap, 0)
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
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.Nil(err)
	assert.Len(cxn.sessManager.trustedPool.sessMap, 1)
	evt, ok = queue.receive(ctx)
	require.True(ok)
	require.IsType(&data.TranscodeEvent{}, evt.data)
	assert.Equal(true, evt.data.(*data.TranscodeEvent).Success)

	// Uses manifest ID if external stream ID or params not present
	testMissingStreamID := func() {
		transcodeResps <- dummyRes
		_, err = processSegment(context.Background(), cxn, seg, nil)
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

	urls, _, err := transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil, nil)
	assert.Nil(err)
	assert.NotNil(urls)
	assert.Len(urls, 1)
	assert.Equal("test.flv", urls[0])

	// Wait for async pixels verification to finish (or in this case we are just making sure that it did NOT run)
	time.Sleep(1 * time.Second)

	// Check that the session was NOT removed because we are in off-chain mode
	_, ok := bsm.trustedPool.sessMap[ts.URL]
	assert.True(ok)

	sess.OrchestratorInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}
	sender := &pm.MockSender{}
	sess.Sender = sender
	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	sender.On("ValidateTicketParams", mock.Anything).Return(nil)

	urls, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil, nil)
	assert.Nil(err)
	assert.Equal("test.flv", urls[0])

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	_, _, err = transcodeSegment(context.TODO(), cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil, nil)
	assert.Nil(err)

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	// Check that the session was not removed
	_, ok = bsm.trustedPool.sessMap[ts.URL]
	assert.True(ok)
}

func TestUpdateSession(t *testing.T) {
	assert := assert.New(t)

	balances := core.NewAddressBalances(5 * time.Minute)
	defer balances.StopCleanup()
	sess := &BroadcastSession{PMSessionID: "foo", LatencyScore: 1.1, Balances: balances, lock: &sync.RWMutex{}, CleanupSession: func(sessionID string) {

	}}
	res := &ReceivedTranscodeResult{
		LatencyScore: 2.1,
	}
	updateSession(sess, res)
	assert.Equal(res.LatencyScore, sess.LatencyScore)

	info := net.OrchestratorInfo{
		Storage: []*net.OSInfo{
			{
				StorageType: 1,
				S3Info:      &net.S3OSInfo{Host: "http://apple.com"},
			},
		},
	}
	res.Info = &info

	updateSession(sess, res)
	assert.Equal(info, *sess.OrchestratorInfo)
	// Check that BroadcastSession.OrchestratorOS is updated when len(info.Storage) > 0
	assert.Equal(info.Storage[0], core.ToNetOSInfo(sess.OrchestratorOS.GetInfo()))
	// Check that a new PM session is not created because BroadcastSession.Sender = nil
	assert.Equal("foo", sess.PMSessionID)
	assert.Equal(info.Transcoder, sess.Transcoder())

	sender := &pm.MockSender{}
	sess.Sender = sender
	sess.OrchestratorInfo = &net.OrchestratorInfo{AuthToken: stubAuthToken}
	res.Info = &net.OrchestratorInfo{
		TicketParams: &net.TicketParams{},
		PriceInfo:    &net.PriceInfo{},
		AuthToken:    stubAuthToken,
	}
	sender.On("StartSession", mock.Anything).Return("foo").Once()
	updateSession(sess, res)
	// Check that a new PM session is not created because OrchestratorInfo.TicketParams = nil
	assert.Equal("foo", sess.PMSessionID)

	sender.On("StartSession", mock.Anything).Return("bar")
	updateSession(sess, res)
	// Check that a new PM session is created
	assert.Equal("bar", sess.PMSessionID)

	info2 := *res.Info
	res.Info = &info2
	res.Info.AuthToken = &net.AuthToken{SessionId: "diffdiff"}
	updateSession(sess, res)
	// Check that new Balance initialized with new auth token sessionID
	assert.Nil(balances.Balance(ethcommon.Address{}, core.ManifestID("diffdiff")))
	sess.Balance.Credit(big.NewRat(5, 1))
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
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", nil, nil)
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
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", segmentVerifier, nil)
	assert.Nil(err)
	assert.Equal(1, verifier.calls)
	require.NotNil(verifier.params)
	assert.Equal(cxn.mid, verifier.params.ManifestID)
	assert.Equal(seg, verifier.params.Source)
	// Do it again for good measure
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", segmentVerifier, nil)
	assert.Nil(err)
	assert.Equal(2, verifier.calls)

	// now "disable" the verifier and ensure no calls
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", nil, nil)
	assert.Nil(err)
	assert.Equal(2, verifier.calls)

	// Pass in a nil policy
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verification.NewSegmentVerifier(nil), nil)
	assert.Nil(err)

	// Pass in a policy but no verifier specified
	policy = &verification.Policy{}
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verification.NewSegmentVerifier(policy), nil)
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
	sess := &BroadcastSession{Params: &core.StreamParameters{}, OrchestratorScore: common.Score_Trusted, lock: &sync.RWMutex{}}
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
	assert.Equal(0, sv.calls)              // sanity check initial call count
	assert.Len(bsm.trustedPool.sessMap, 1) // sanity check initial bsm map
	err = verify(verifier, cxn, sess, source, res, URIs, renditionData)
	assert.NotNil(err)
	assert.Equal(1, sv.calls)
	assert.Equal(sv.err, err)
	assert.Len(bsm.trustedPool.sessMap, 1) // No effect on map for now

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
	assert.Len(bsm.trustedPool.sessMap, 0)

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
	name, err := mem.SaveData(context.TODO(), "/rendition/seg/1", strings.NewReader("attempt1"), nil, 0)
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
	_, err = mem.SaveData(context.TODO(), "/rendition/seg/1", strings.NewReader("attempt2"), nil, 0)
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
	driver, err := drivers.NewS3Driver("", S3BUCKET, "", "", "", false)
	assert.Nil(err)
	mem := driver.NewSession(string(mid))
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
	downloadSeg = func(ctx context.Context, url string) ([]byte, error) { return []byte("foo"), nil }

	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.Equal(verification.ErrTampered, err)
	assert.Empty(pl.uri) // sanity check that no insertion happened

	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.Equal(verification.ErrTampered, err)
	assert.Empty(pl.uri)

	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.Nil(err)
	assert.Equal(baseURL+"/resp2", pl.uri)
}

func TestDownloadSegError_SuspendAndRemove(t *testing.T) {
	assert := assert.New(t)
	mid := core.ManifestID("foo")
	pl := &stubPlaylistManager{manifestID: mid}
	S3BUCKET := "livepeer"
	driver, err := drivers.NewS3Driver("", S3BUCKET, "", "", "", false)
	assert.Nil(err)
	mem := driver.NewSession(string(mid))
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
	downloadSeg = func(ctx context.Context, url string) ([]byte, error) { return nil, errors.New("some error") }
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.EqualError(err, "some error")
	_, ok := cxn.sessManager.trustedPool.sessMap[sess.OrchestratorInfo.GetTranscoder()]
	assert.False(ok)
	assert.Greater(cxn.sessManager.trustedPool.sus.Suspended(sess.OrchestratorInfo.GetTranscoder()), 0)
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
	err := refreshSession(context.TODO(), sess)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid control character in URL")

	// trigger getOrchestratorInfo error
	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return nil, errors.New("some error")
	}
	sess = StubBroadcastSession("foo")
	err = refreshSession(context.TODO(), sess)
	assert.EqualError(err, "some error")

	// trigger update
	getOrchestratorInfoRPC = func(ctx context.Context, bcast common.Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
		return successOrchInfoUpdate, nil
	}
	err = refreshSession(context.TODO(), sess)
	assert.Nil(err)
	assert.Equal(sess.OrchestratorInfo, successOrchInfoUpdate)

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
	err = refreshSession(context.TODO(), sess)
	assert.EqualError(err, "context timeout")
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
	downloadSeg = func(ctx context.Context, url string) ([]byte, error) {
		downloaded[url] = true

		return []byte("foo"), nil
	}

	//
	// Tests when there is no verification policy
	//

	// When there is no broadcaster OS, segments should not be downloaded
	url := "somewhere1"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, nil, mid)})
	_, _, err := transcodeSegment(context.TODO(), cxn, seg, "dummy", nil, nil)
	assert.Nil(err)
	assert.False(downloaded[url])

	// When segments are in the broadcaster's external OS, segments should not be downloaded
	url = "https://livepeer.s3.amazonaws.com/resp1"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", nil, nil)
	assert.Nil(err)
	assert.False(downloaded[url])

	// When segments are not in the broadcaster's external OS, segments should be downloaded
	url = "somewhere2"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", nil, nil)
	assert.Nil(err)
	assert.True(downloaded[url])

	//
	// Tests when there is a verification policy
	//

	verifier := newStubSegmentVerifier(&stubVerifier{retries: 100})

	// When there is no broadcaster OS, segments should be downloaded
	url = "somewhere3"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, nil, mid)})
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.Nil(err)
	assert.True(downloaded[url])

	// When segments are in the broadcaster's external OS, segments should be downloaded
	url = "https://livepeer.s3.amazonaws.com/resp2"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.Nil(err)
	assert.True(downloaded[url])

	// When segments are not in the broadcaster's exernal OS, segments should be downloaded
	url = "somewhere4"
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{genBcastSess(ctx, t, url, externalOS, mid)})
	_, _, err = transcodeSegment(context.TODO(), cxn, seg, "dummy", verifier, nil)
	assert.Nil(err)
	assert.True(downloaded[url])
}

func TestProcessSegment_VideoFormat(t *testing.T) {
	// Test format from saving "transcoder" data into broadcaster/transcoder OS.
	// For each rendition, check extension based on format (none, mp4, mpegts).
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bcastOS := &stubOSSession{host: "test://broad.com", external: true}
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
	downloadSeg = func(ctx context.Context, url string) ([]byte, error) { return []byte(url), nil }

	// processSegment will also call transcodeSegment; also check that behavior
	_, err := processSegment(context.Background(), cxn, seg, nil)

	assert.Nil(err)
	assert.Equal(ffmpeg.FormatNone, cxn.profile.Format)
	for _, p := range sess.Params.Profiles {
		assert.Equal(ffmpeg.FormatNone, p.Format)
	}
	assert.Equal([]string{"P240p30fps16x9/0.ts"}, orchOS.saved)
	assert.Equal([]string{"P240p30fps16x9/0.ts", "P144p30fps16x9/0.ts"}, bcastOS.saved)
	assert.Equal("saved_P240p30fps16x9/0.ts", seg.Name)

	// Check MP4. Reset OS for simplicity
	bcastOS = &stubOSSession{host: "test://broad.com", external: true}
	orchOS = &stubOSSession{host: "test://orch.com"}
	sess.BroadcasterOS = bcastOS
	sess.OrchestratorOS = orchOS
	cxn.pl = &stubPlaylistManager{os: bcastOS}
	cxn.profile.Format = ffmpeg.FormatMP4
	for i := range sess.Params.Profiles {
		sess.Params.Profiles[i].Format = ffmpeg.FormatMP4
	}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})

	_, err = processSegment(context.Background(), cxn, seg, nil)

	assert.Nil(err)
	for _, p := range sess.Params.Profiles {
		assert.Equal(ffmpeg.FormatMP4, p.Format)
	}
	assert.Equal(ffmpeg.FormatMP4, cxn.profile.Format)
	assert.Equal([]string{"P240p30fps16x9/0.mp4"}, orchOS.saved)
	assert.Equal([]string{"P240p30fps16x9/0.mp4", "P144p30fps16x9/0.mp4"}, bcastOS.saved)
	assert.Equal("saved_P240p30fps16x9/0.mp4", seg.Name)

	// Check mpegts. Reset OS for simplicity
	bcastOS = &stubOSSession{host: "test://broad.com", external: true}
	orchOS = &stubOSSession{host: "test://orch.com"}
	sess.BroadcasterOS = bcastOS
	sess.OrchestratorOS = orchOS
	cxn.pl = &stubPlaylistManager{os: bcastOS}
	cxn.profile.Format = ffmpeg.FormatMPEGTS
	for i := range sess.Params.Profiles {
		sess.Params.Profiles[i].Format = ffmpeg.FormatMPEGTS
	}
	cxn.sessManager = bsmWithSessList([]*BroadcastSession{sess})

	_, err = processSegment(context.Background(), cxn, seg, nil)

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
	_, err := processSegment(context.Background(), cxn, seg, nil)
	assert.Equal("invalid duration -1", err.Error())

	// CHeck greater than max duration
	seg.Duration = maxDurationSec + 0.01
	_, err = processSegment(context.Background(), cxn, seg, nil)
	assert.Equal("invalid duration 300.01", err.Error())
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
		Broadcaster:       stubBroadcaster2(),
		Params:            &core.StreamParameters{ManifestID: mid, Profiles: []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}, OS: os},
		BroadcasterOS:     os,
		OrchestratorInfo:  &net.OrchestratorInfo{Transcoder: transcoderURL, AuthToken: stubAuthToken, TicketParams: &net.TicketParams{Recipient: pm.RandAddress().Bytes()}},
		OrchestratorScore: common.Score_Trusted,
		lock:              &sync.RWMutex{},
	}
}

func stubTestTranscoder(ctx context.Context, handler http.HandlerFunc) string {
	ts, mux := stubTLSServer()
	go func() { <-ctx.Done(); ts.Close() }()
	mux.HandleFunc("/segment", handler)
	return ts.URL
}

func TestCollectResults(t *testing.T) {
	assert := assert.New(t)

	trustedSess := StubBroadcastSession("trustedTranscoder")
	trustedSess.OrchestratorScore = common.Score_Trusted
	untrustedSess1 := StubBroadcastSession("untrustedTranscoder1")
	untrustedSess1.OrchestratorScore = common.Score_Untrusted
	untrustedSess2 := StubBroadcastSession("untrustedTranscoder2")
	untrustedSess2.OrchestratorScore = common.Score_Untrusted
	untrustedSessVerified := StubBroadcastSession("untrustedTranscoderVerified")
	untrustedSessVerified.OrchestratorScore = common.Score_Untrusted
	bsm := bsmWithSessList([]*BroadcastSession{trustedSess, untrustedSess1, untrustedSess2, untrustedSessVerified})
	bsm.sessionVerified(untrustedSessVerified)

	resChan := make(chan *SubmitResult, 4)
	resChan <- &SubmitResult{Session: untrustedSess1, TranscodeResult: &ReceivedTranscodeResult{}}
	resChan <- &SubmitResult{Session: untrustedSessVerified, TranscodeResult: &ReceivedTranscodeResult{}}
	resChan <- &SubmitResult{Session: untrustedSess2, TranscodeResult: &ReceivedTranscodeResult{}}
	resChan <- &SubmitResult{Session: trustedSess, TranscodeResult: &ReceivedTranscodeResult{}}

	trustedResult, untrustedResults, err := bsm.collectResults(resChan, 4)

	assert.NoError(err)
	assert.Equal(trustedSess, trustedResult.Session)
	assert.Len(untrustedResults, 3)
	// the first result should always come from the verified session
	assert.Equal(untrustedSessVerified, untrustedResults[0].Session)
}

func TestVerifcationEnabledWhenFreqGreaterThanZero(t *testing.T) {
	b := BroadcastSessionsManager{}
	require.False(t, b.isVerificationEnabled())

	b.VerificationFreq = 1
	require.True(t, b.isVerificationEnabled())
}

func TestVerifcationDoesntRunWhenNoVerifiedSessionPresent(t *testing.T) {
	b := BroadcastSessionsManager{
		VerificationFreq: 1,
	}
	require.False(t, b.shouldSkipVerification(nil))

	b.verifiedSession = &BroadcastSession{
		LatencyScore: 1.23,
	}

	require.False(t, b.shouldSkipVerification([]*BroadcastSession{}))
}

func TestVerifcationRunsBasedOnVerificationFrequency(t *testing.T) {
	verificationFreq := 5
	b := BroadcastSessionsManager{
		VerificationFreq: uint(verificationFreq), // Verification should run approximately 1 in 5 times
	}

	var verifiedSession = &BroadcastSession{
		LatencyScore: 1.23,
	}

	b.verifiedSession = verifiedSession

	var shouldSkipCount int
	numTests := 10000
	for i := 0; i < numTests; i++ {
		if b.shouldSkipVerification([]*BroadcastSession{verifiedSession}) {
			shouldSkipCount++
		}
	}

	require.Greater(t, float32(shouldSkipCount), float32(numTests)*(1-2/float32(verificationFreq)))
	require.Less(t, float32(shouldSkipCount), float32(numTests)*(1-0.5/float32(verificationFreq)))
}
