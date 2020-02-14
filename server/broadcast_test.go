package server

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
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
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/m3u8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func StubBroadcastSession(transcoder string) *BroadcastSession {
	return &BroadcastSession{
		Broadcaster:      stubBroadcaster2(),
		ManifestID:       core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{Transcoder: transcoder},
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
	}
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

type stubPlaylistManager struct {
	manifestID core.ManifestID
	seq        uint64
	profile    ffmpeg.VideoProfile
	uri        string
}

func (pm *stubPlaylistManager) ManifestID() core.ManifestID {
	return pm.manifestID
}

func (pm *stubPlaylistManager) InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64) error {
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
	return nil
}

func (pm *stubPlaylistManager) Cleanup() {}

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
	params := &streamParameters{}
	storage := drivers.NewMemoryDriver(nil).NewSession(string(mid))
	pl := core.NewBasicPlaylistManager(mid, storage)

	// Check empty pool produces expected numOrchs
	sess := NewSessionManager(n, params, pl, &LIFOSelector{})
	assert.Equal(0, sess.numOrchs)

	// Check numOrchs up to maximum and a bit beyond
	sd := &stubDiscovery{}
	n.OrchestratorPool = sd
	max := int(common.HTTPTimeout.Seconds()/SegLen.Seconds()) * 2
	for i := 0; i < 10; i++ {
		sess = NewSessionManager(n, params, pl, &LIFOSelector{})
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
	assert.Len(bsm.sessList(), 1)

	sess = bsm.selectSession()
	assert.Equal(expectedSess2, sess)
	assert.Len(bsm.sessList(), 0)

	// assert no session is selected from empty list
	sess = bsm.selectSession()
	assert.Nil(sess)
	assert.Len(bsm.sessList(), 0)
	assert.Len(bsm.sessMap, 2) // map should still track original sessions

	// assert session list gets refreshed if under threshold. check via waitgroup
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

func TestTranscodeSegment_CompleteSession(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{&net.TranscodedSegmentData{Url: "test.flv"}},
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
	sess.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	_, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.Nil(err)

	completedSess := bsm.sessMap[ts.URL]
	assert.NotEqual(completedSess, sess)
	assert.NotZero(completedSess.LatencyScore)

	// Check that the completed session is just the original session with a different LatencyScore
	copiedSess := &BroadcastSession{}
	*copiedSess = *completedSess
	copiedSess.LatencyScore = 0.0
	assert.Equal(copiedSess, sess)

	tr.Info = &net.OrchestratorInfo{Transcoder: ts.URL, PriceInfo: &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7}}
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	_, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy", nil)
	assert.Nil(err)

	// Check that BroadcastSession.OrchestratorInfo was updated
	completedSessInfo := bsm.sessMap[ts.URL].OrchestratorInfo
	assert.Equal(tr.Info.Transcoder, completedSessInfo.Transcoder)
	assert.Equal(tr.Info.PriceInfo.PricePerUnit, completedSessInfo.PriceInfo.PricePerUnit)
	assert.Equal(tr.Info.PriceInfo.PixelsPerUnit, completedSessInfo.PriceInfo.PixelsPerUnit)
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
		&net.TranscodedSegmentData{Url: "test.flv", Pixels: 100},
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
	sess.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn := &rtmpConnection{
		mid:         core.ManifestID("foo"),
		nonce:       7,
		pl:          &stubPlaylistManager{manifestID: core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	urls, err := transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil)
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
	sess.Sender = &pm.MockSender{}
	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	urls, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil)
	assert.Nil(err)
	assert.Equal("test.flv", urls[0])

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	_, err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy", nil)
	assert.Nil(err)

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	// Check that the session was not removed
	_, ok = bsm.sessMap[ts.URL]
	assert.True(ok)
}

func TestUpdateSession(t *testing.T) {
	assert := assert.New(t)

	sess := &BroadcastSession{PMSessionID: "foo", LatencyScore: 1.1}
	res := &ReceivedTranscodeResult{
		LatencyScore: 2.1,
	}
	newSess := updateSession(sess, res)
	assert.Equal(res.LatencyScore, newSess.LatencyScore)
	// Check that LatencyScore of old session is not mutated
	assert.Equal(1.1, sess.LatencyScore)

	info := &net.OrchestratorInfo{
		Storage: []*net.OSInfo{
			&net.OSInfo{
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
	res.Info = &net.OrchestratorInfo{}

	newSess = updateSession(sess, res)
	// Check that a new PM session is not created because OrchestratorInfo.TicketParams = nil
	assert.Equal("foo", newSess.PMSessionID)

	params := pm.TicketParams{
		Recipient:         ethcommon.Address{},
		FaceValue:         big.NewInt(100),
		WinProb:           big.NewInt(100),
		RecipientRandHash: ethcommon.BytesToHash([]byte{}),
		Seed:              big.NewInt(100),
	}
	res.Info = &net.OrchestratorInfo{
		TicketParams: &net.TicketParams{
			Recipient:         params.Recipient.Bytes(),
			FaceValue:         params.FaceValue.Bytes(),
			WinProb:           params.WinProb.Bytes(),
			RecipientRandHash: params.RecipientRandHash.Bytes(),
			Seed:              params.Seed.Bytes(),
		},
	}

	sender.On("StartSession", params).Return("bar")

	newSess = updateSession(sess, res)
	// Check that a new PM session is created
	assert.Equal("bar", newSess.PMSessionID)
	// Check that PMSessionID of old session is not mutated
	assert.Equal("foo", sess.PMSessionID)
}

func TestHLSInsertion(t *testing.T) {
	assert := assert.New(t)

	segData := []*net.TranscodedSegmentData{
		&net.TranscodedSegmentData{Url: "/path/to/video", Pixels: 100},
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
	sess.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.ManifestID = core.ManifestID("foo")
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	pl := &stubPlaylistManager{manifestID: core.ManifestID("foo")}
	cxn := &rtmpConnection{
		mid:         sess.ManifestID,
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P240p30fps16x9,
		sessManager: bsm,
	}

	seg := &stream.HLSSegment{SeqNo: 93}
	_, err = transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)

	// some sanity checks
	assert.Greater(len(sess.Profiles), 0)
	assert.NotEqual(pl.profile, *cxn.profile, "HLS profile matched")

	// Check HLS insertion
	assert.Equal(seg.SeqNo, pl.seq, "HLS insertion failed")
	assert.Equal(pl.profile, sess.Profiles[0], "HLS profile mismatch")
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
	sess.Profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess.ManifestID = core.ManifestID("foo")
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	pl := &stubPlaylistManager{manifestID: core.ManifestID("foo")}
	cxn := &rtmpConnection{
		mid:         sess.ManifestID,
		nonce:       7,
		pl:          pl,
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	seg := &stream.HLSSegment{}
	_, err = transcodeSegment(cxn, seg, "dummy", segmentVerifier)
	assert.Nil(err)
	assert.Equal(1, verifier.calls)
	require.NotNil(verifier.params)
	assert.Equal(cxn.mid, verifier.params.ManifestID)
	assert.Equal(seg, verifier.params.Source)
	// Do it again for good measure
	_, err = transcodeSegment(cxn, seg, "dummy", segmentVerifier)
	assert.Nil(err)
	assert.Equal(2, verifier.calls)

	// now "disable" the verifier and ensure no calls
	_, err = transcodeSegment(cxn, seg, "dummy", nil)
	assert.Nil(err)
	assert.Equal(2, verifier.calls)

	// Pass in a nil policy
	_, err = transcodeSegment(cxn, seg, "dummy", verification.NewSegmentVerifier(nil))
	assert.Nil(err)

	// Pass in a policy but no verifier specified
	policy = &verification.Policy{}
	_, err = transcodeSegment(cxn, seg, "dummy", verification.NewSegmentVerifier(policy))
	assert.Nil(err)
}

func TestVerifier_Verify(t *testing.T) {
	assert := assert.New(t)

	cxn := &rtmpConnection{}
	sess := &BroadcastSession{}
	source := &stream.HLSSegment{}
	res := &net.TranscodeData{}
	verifier := verification.NewSegmentVerifier(&verification.Policy{})
	URIs := []string{}
	err := verify(verifier, cxn, sess, source, res, URIs)
	assert.Nil(err)

	// Test local OS: Should fail with an invalid path
	sess.ManifestID = core.ManifestID("streamName")
	sess.BroadcasterOS = drivers.NewMemoryDriver(nil).NewSession("streamName")
	URIs = append(URIs, "filename")
	err = verify(verifier, cxn, sess, source, res, URIs)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid URI for request")

	// Test local OS : Should fail if data does not exist in OS
	URIs[0] = "/filename"
	err = verify(verifier, cxn, sess, source, res, URIs)
	assert.NotNil(err)
	assert.Contains(err.Error(), "Missing Local Data")

	// Test for segment not in broadcaster's own OS - "livepeer" S3 bucket
	drivers.S3BUCKET = "livepeer"
	sess.BroadcasterOS = drivers.NewS3Driver("", drivers.S3BUCKET, "", "").NewSession("")
	err = verify(verifier, cxn, sess, source, res, URIs)
	assert.NotNil(err)
	assert.Contains(err.Error(), "Expected local storage but did not have it")

	// Set broadcaster's OS to "livepeer" S3 bucket and fix the URL
	URIs[0] = "https://livepeer.s3.amazonaws.com"
	err = verify(verifier, cxn, sess, source, res, URIs)
	assert.Nil(err)

	// Check non-retryable errors
	sess.OrchestratorInfo = &net.OrchestratorInfo{Transcoder: "asdf"}
	bsm := bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm
	sv := &stubVerifier{err: errors.New("NonRetryable")}
	verifier = newStubSegmentVerifier(sv)
	assert.Equal(0, sv.calls)  // sanity check initial call count
	assert.Len(bsm.sessMap, 1) // sanity check initial bsm map
	err = verify(verifier, cxn, sess, source, res, URIs)
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
	err = verify(verifier, cxn, sess, source, res, URIs)
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
			verification.Results{Score: 9},
			verification.Results{Score: 1},
		},
	}
	mem, ok := drivers.NewMemoryDriver(nil).NewSession("streamName").(*drivers.MemorySession)
	assert.True(ok)
	name, err := mem.SaveData("/rendition/seg/1", []byte("attempt1"))
	assert.Nil(err)
	assert.Equal([]byte("attempt1"), mem.GetData(name))
	sess.BroadcasterOS = mem
	verifier = newStubSegmentVerifier(sv)
	URIs[0] = name
	err = verify(verifier, cxn, sess, source, res, URIs)
	assert.Equal(sv.err, err)

	// Now "insert" 2nd attempt into OS
	// and ensure 1st attempt is what remains after verification
	_, err = mem.SaveData("/rendition/seg/1", []byte("attempt2"))
	assert.Nil(err)
	assert.Equal([]byte("attempt2"), mem.GetData(name))
	err = verify(verifier, cxn, sess, source, res, URIs)
	assert.Nil(err)
	assert.Equal([]byte("attempt1"), mem.GetData(name))
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
	drivers.S3BUCKET = "livepeer"
	mem := drivers.NewS3Driver("", drivers.S3BUCKET, "", "").NewSession(string(mid))
	assert.NotNil(mem)
	genBcastSess := func(url string) *BroadcastSession {
		segData := []*net.TranscodedSegmentData{
			&net.TranscodedSegmentData{Url: url, Pixels: 100},
		}

		buf, err := proto.Marshal(&net.TranscodeResult{
			Result: &net.TranscodeResult_Data{
				Data: &net.TranscodeData{Segments: segData},
			},
		})
		assert.Nil(err, fmt.Sprintf("Could not marshal results for %s", url))
		ts, mux := stubTLSServer()
		mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(buf)
		})
		defer func() {
			// Work around a weird timing issue. Tests fail if the server closes
			// in-scope (prob leads to something like the client being unable
			// to read the response?), so we delay the close for a little bit
			go func() {
				// We assume this test doesn't take more than 1s
				// But if it does (eg, we get a POST error), then bump this up
				time.Sleep(1 * time.Second)
				ts.Close()
			}()
		}()
		return &BroadcastSession{
			Broadcaster:      stubBroadcaster2(),
			ManifestID:       mid,
			Profiles:         []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9},
			BroadcasterOS:    mem,
			OrchestratorInfo: &net.OrchestratorInfo{Transcoder: ts.URL},
		}
	}

	baseURL := "https://livepeer.s3.amazonaws.com"
	bsm := bsmWithSessList([]*BroadcastSession{
		genBcastSess(baseURL + "/resp1"),
		genBcastSess(baseURL + "/resp2"),
		genBcastSess(baseURL + "/resp3"),
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

	_, err := transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Equal(verification.ErrTampered, err)
	assert.Empty(pl.uri) // sanity check that no insertion happened

	_, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Equal(verification.ErrTampered, err)
	assert.Empty(pl.uri)

	_, err = transcodeSegment(cxn, seg, "dummy", verifier)
	assert.Nil(err)
	assert.Equal(baseURL+"/resp2", pl.uri)
}
