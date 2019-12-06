package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
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

type stubPlaylistManager struct {
	manifestID core.ManifestID
}

func (pm *stubPlaylistManager) ManifestID() core.ManifestID {
	return pm.manifestID
}

func (pm *stubPlaylistManager) InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64) error {
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
		pl:          &stubPlaylistManager{core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	assert.Nil(transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy"))

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

	assert.Nil(transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy"), Duration: 2.0}, "dummy"))

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
		pl:          &stubPlaylistManager{core.ManifestID("foo")},
		profile:     &ffmpeg.P144p30fps16x9,
		sessManager: bsm,
	}

	err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy")
	assert.Nil(err)

	// Wait for async pixels verification to finish (or in this case we are just making sure that it did NOT run)
	time.Sleep(1 * time.Second)

	// Check that the session was NOT removed because we are in off-chain mode
	_, ok := bsm.sessMap[ts.URL]
	assert.True(ok)

	sess.OrchestratorInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}
	sess.Sender = &pm.MockSender{}
	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy")
	assert.Nil(err)

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	// Check that the session was removed because we are in on-chain mode
	_, ok = bsm.sessMap[ts.URL]
	assert.False(ok)

	// Create stub response with correct reported pixels
	p, err := pixels("test.flv")
	require.Nil(err)
	tSegData = []*net.TranscodedSegmentData{
		&net.TranscodedSegmentData{Url: "test.flv", Pixels: p},
	}
	tr = dummyRes(tSegData)
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	bsm = bsmWithSessList([]*BroadcastSession{sess})
	cxn.sessManager = bsm

	err = transcodeSegment(cxn, &stream.HLSSegment{Data: []byte("dummy")}, "dummy")
	assert.Nil(err)

	// Wait for async pixels verification to finish
	time.Sleep(1 * time.Second)

	// Check that the session was not removed
	_, ok = bsm.sessMap[ts.URL]
	assert.True(ok)
}

func TestPixels(t *testing.T) {
	ffmpeg.InitFFmpeg()

	assert := assert.New(t)

	p, err := pixels("foo")
	assert.EqualError(err, "No such file or directory")
	assert.Equal(int64(0), p)

	// Assume that ffmpeg.Transcode3() returns the correct pixel count so we just
	// check that no error is returned
	p, err = pixels("test.flv")
	assert.Nil(err)
	assert.NotZero(p)
}

func TestVerifyPixels(t *testing.T) {
	ffmpeg.InitFFmpeg()

	require := require.New(t)
	assert := assert.New(t)

	// Create memory session and save a test file
	bos := drivers.NewMemoryDriver(nil).NewSession("foo")
	data, err := ioutil.ReadFile("test.flv")
	require.Nil(err)
	fname, err := bos.SaveData("test.ts", data)
	require.Nil(err)

	// Test error for relative URI and no memory storage if the file does not exist on disk
	// Will try to use the relative URI to read the file from disk and fail
	err = verifyPixels(fname, nil, 50)
	assert.EqualError(err, "No such file or directory")

	// Test error for relative URI and local memory storage if the file does not exist in storage
	// Will try to use the relative URI to read the file from storage and fail
	err = verifyPixels("/stream/bar/dne.ts", bos, 50)
	assert.EqualError(err, "error fetching data from local memory storage")

	// Test writing temp file for relative URI and local memory storage with incorrect pixels
	err = verifyPixels(fname, bos, 50)
	assert.EqualError(err, "mismatch between calculated and reported pixels")

	// Test writing temp file for relative URI and local memory storage with correct pixels
	// Make sure that verifyPixels() checks against the output of pixels()
	p, err := pixels("test.flv")
	require.Nil(err)
	err = verifyPixels(fname, bos, p)
	assert.Nil(err)

	// Test no writing temp file with incorrect pixels
	err = verifyPixels("test.flv", nil, 50)
	assert.EqualError(err, "mismatch between calculated and reported pixels")

	// Test no writing temp file with correct pixels
	err = verifyPixels("test.flv", nil, p)
	assert.Nil(err)
}
