package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/verification"
	"github.com/livepeer/livepeer-data/pkg/data"
	"github.com/livepeer/livepeer-data/pkg/event"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

var refreshTimeout = 2500 * time.Millisecond
var maxDurationSec = common.MaxDuration.Seconds()

var Policy *verification.Policy
var BroadcastCfg = &BroadcastConfig{}
var MaxAttempts = 3

var MetadataQueue event.Producer
var MetadataPublishTimeout = 1 * time.Second

var getOrchestratorInfoRPC = GetOrchestratorInfo
var downloadSeg = drivers.GetSegmentData

type BroadcastConfig struct {
	maxPrice *big.Rat
	mu       sync.RWMutex
}

type SegFlightMetadata struct {
	startTime time.Time
	segDur    time.Duration
}

func (cfg *BroadcastConfig) MaxPrice() *big.Rat {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.maxPrice
}

func (cfg *BroadcastConfig) SetMaxPrice(price *big.Rat) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.maxPrice = price

	if monitor.Enabled {
		monitor.MaxTranscodingPrice(price)
	}
}

type sessionsCreator func() ([]*BroadcastSession, error)
type SessionPool struct {
	mid core.ManifestID

	// Accessing or changing any of the below requires ownership of this mutex
	lock sync.Mutex

	sel      BroadcastSessionsSelector
	lastSess []*BroadcastSession
	sessMap  map[string]*BroadcastSession
	numOrchs int // how many orchs to request at once
	poolSize int

	refreshing bool // only allow one refresh in-flight
	finished   bool // set at stream end

	createSessions sessionsCreator
	sus            *suspender
}

func NewSessionPool(mid core.ManifestID, poolSize, numOrchs int, sus *suspender, createSession sessionsCreator,
	sel BroadcastSessionsSelector) *SessionPool {

	return &SessionPool{
		mid:            mid,
		numOrchs:       numOrchs,
		poolSize:       poolSize,
		sessMap:        make(map[string]*BroadcastSession),
		sel:            sel,
		createSessions: createSession,
		sus:            sus,
	}
}

func (sp *SessionPool) suspend(orch string) {
	sp.sus.suspend(orch, sp.poolSize/sp.numOrchs)
}

func (sp *SessionPool) refreshSessions() {
	started := time.Now()
	glog.V(common.DEBUG).Infof("Starting session refresh manifestID=%s", sp.mid)
	defer func() {
		sp.lock.Lock()
		glog.V(common.DEBUG).Infof("Ending session refresh manifestID=%s dur=%s orchs=%d", sp.mid, time.Since(started),
			sp.sel.Size())
		sp.lock.Unlock()
	}()
	sp.lock.Lock()
	if sp.finished || sp.refreshing {
		sp.lock.Unlock()
		return
	}
	sp.refreshing = true
	sp.lock.Unlock()

	sp.sus.signalRefresh()

	newBroadcastSessions, err := sp.createSessions()
	if err != nil {
		sp.lock.Lock()
		sp.refreshing = false
		sp.lock.Unlock()
		return
	}

	// if newBroadcastSessions is empty, exit without refreshing list
	if len(newBroadcastSessions) <= 0 {
		sp.lock.Lock()
		sp.refreshing = false
		sp.lock.Unlock()
		return
	}

	uniqueSessions := make([]*BroadcastSession, 0, len(newBroadcastSessions))
	sp.lock.Lock()
	defer sp.lock.Unlock()

	sp.refreshing = false
	if sp.finished {
		return
	}

	for _, sess := range newBroadcastSessions {
		if _, ok := sp.sessMap[sess.OrchestratorInfo.Transcoder]; ok {
			continue
		}
		uniqueSessions = append(uniqueSessions, sess)
		sp.sessMap[sess.OrchestratorInfo.Transcoder] = sess
	}

	sp.sel.Add(uniqueSessions)
}

func includesSession(sessions []*BroadcastSession, session *BroadcastSession) bool {
	for _, sess := range sessions {
		if sess == session {
			return true
		}
	}
	return false
}

func getOrchs(sessions []*BroadcastSession) []string {
	res := make([]string, len(sessions))
	for i, sess := range sessions {
		res[i] = sess.Transcoder()
	}
	return res
}

func removeSessionFromList(sessions []*BroadcastSession, sess *BroadcastSession) []*BroadcastSession {
	var res []*BroadcastSession
	for _, ls := range sessions {
		if ls != sess {
			res = append(res, ls)
		}
	}
	return res
}

func selectSession(sessions []*BroadcastSession, exclude []*BroadcastSession, durMult int) *BroadcastSession {
	for _, session := range sessions {
		if len(session.SegsInFlight) > 0 &&
			time.Since(session.SegsInFlight[0].startTime) < time.Duration(durMult)*session.SegsInFlight[0].segDur &&
			!includesSession(exclude, session) {
			// Re-use last session if oldest segment is in-flight for < segDur
			return session
		}
	}
	return nil
}

func (sp *SessionPool) selectSessions(sessionsNum int) []*BroadcastSession {
	sp.lock.Lock()
	defer sp.lock.Unlock()
	if sp.poolSize == 0 {
		return nil
	}

	checkSessions := func(m *SessionPool) bool {
		numSess := m.sel.Size()
		if numSess < int(math.Ceil(float64(m.numOrchs)/2.0)) {
			go m.refreshSessions()
		}
		return (numSess > 0 || len(sp.lastSess) > 0)
	}
	var selectedSessions []*BroadcastSession

	for checkSessions(sp) {
		var sess *BroadcastSession

		// Re-use last session if oldest segment is in-flight for < segDur
		gotFromLast := false
		sess = selectSession(sp.lastSess, selectedSessions, 1)
		if sess == nil {
			// Or try a new session from the available ones
			sess = sp.sel.Select()
		} else {
			gotFromLast = true
		}

		if sess == nil {
			// If no new sessions are available, re-use last session when oldest segment is in-flight for < 2 * segDur
			sess = selectSession(sp.lastSess, selectedSessions, 2)
			if sess != nil {
				gotFromLast = true
				glog.V(common.DEBUG).Infof("No sessions in the selector for manifestID=%v re-using orch=%v with acceptable in-flight time",
					sp.mid, sess.Transcoder())
			}
		}

		// No session found, return nil
		if sess == nil {
			break
		}

		/*
		   Don't select sessions no longer in the map.

		   Retry if the first selected session has been removed from the map.
		   This may occur if the session is removed while still in the list.
		   To avoid a runtime search of the session list under lock, simply
		   fixup the session list at selection time by retrying the selection.
		*/
		if _, ok := sp.sessMap[sess.Transcoder()]; ok {
			selectedSessions = append(selectedSessions, sess)

			if len(selectedSessions) == sessionsNum {
				break
			}
		} else {
			if gotFromLast {
				// Last session got removed from map (possibly due to a failure) so stop tracking its in-flight segments
				sess.SegsInFlight = nil
				sp.lastSess = removeSessionFromList(sp.lastSess, sess)
				glog.V(common.DEBUG).Infof("Removing orch=%v from manifestID=%s session list", sess.Transcoder(), sp.mid)
				if monitor.Enabled {
					monitor.OrchestratorSwapped()
				}
			}
		}
	}
	if len(selectedSessions) == 0 {
		// No session found, return nil
		sp.lastSess = nil
	} else {
		for _, ls := range sp.lastSess {
			if !includesSession(selectedSessions, ls) {
				glog.V(common.DEBUG).Infof("Swapping from orch=%v to orch=%+v for manifestID=%s", ls.Transcoder(),
					getOrchs(selectedSessions), sp.mid)
				if monitor.Enabled {
					monitor.OrchestratorSwapped()
				}
			}
		}
		sp.lastSess = append([]*BroadcastSession{}, selectedSessions...)
	}
	return selectedSessions
}

func (sp *SessionPool) removeSession(session *BroadcastSession) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	delete(sp.sessMap, session.Transcoder())
}

func (sp *SessionPool) cleanup() {
	sp.lock.Lock()
	defer sp.lock.Unlock()
	sp.finished = true
	sp.lastSess = nil
	sp.sel.Clear()
	sp.sessMap = make(map[string]*BroadcastSession) // prevent segfaults
}

func (sp *SessionPool) completeSession(sess *BroadcastSession) {
	sp.lock.Lock()
	defer sp.lock.Unlock()
	if existingSess, ok := sp.sessMap[sess.Transcoder()]; ok {
		if existingSess != sess {
			// that means that sess object was removed from pool and then same
			// Orchestrator was added to the pool again
			return
		}
		sess.lock.Lock()
		defer sess.lock.Unlock()
		if len(sess.SegsInFlight) == 1 {
			sess.SegsInFlight = nil
		} else if len(sess.SegsInFlight) > 1 {
			sess.SegsInFlight = sess.SegsInFlight[1:]
			// skip returning this session back to the selector
			// we will return it later in transcodeSegment() once all in-flight segs downloaded
			return
		}
		sp.sel.Complete(sess)
	}
}

type BroadcastSessionsManager struct {
	mid              core.ManifestID
	VerificationFreq uint

	// Accessing or changing any of the below requires ownership of this mutex
	sessLock sync.Mutex

	finished bool // set at stream end

	trustedPool   *SessionPool
	untrustedPool *SessionPool
}

func NewSessionManager(node *core.LivepeerNode, params *core.StreamParameters, sel BroadcastSessionsSelectorFactory) *BroadcastSessionsManager {
	var trustedPoolSize, untrustedPoolSize float64
	if node.OrchestratorPool != nil {
		trustedPoolSize = float64(node.OrchestratorPool.SizeWithPred(common.SizePredAtLeast(common.Score_Trusted)))
		untrustedPoolSize = float64(node.OrchestratorPool.SizeWithPred(common.SizePredEqual(common.Score_Untrusted)))
	}
	maxInflight := common.HTTPTimeout.Seconds() / SegLen.Seconds()
	trustedNumOrchs := int(math.Min(trustedPoolSize, maxInflight*2))
	untrustedNumOrchs := int(math.Min(untrustedPoolSize, maxInflight*2)) * 2
	susTrusted := newSuspender()
	susUntrusted := newSuspender()
	createSessionsTrusted := func() ([]*BroadcastSession, error) {
		return selectOrchestrator(node, params, trustedNumOrchs, susTrusted, common.SizePredAtLeast(common.Score_Trusted))
	}
	createSessionsUntrusted := func() ([]*BroadcastSession, error) {
		return selectOrchestrator(node, params, untrustedNumOrchs, susUntrusted, common.SizePredEqual(common.Score_Untrusted))
	}
	bsm := &BroadcastSessionsManager{
		mid:              params.ManifestID,
		VerificationFreq: params.VerificationFreq,
		trustedPool:      NewSessionPool(params.ManifestID, int(trustedPoolSize), trustedNumOrchs, susTrusted, createSessionsTrusted, sel()),
		untrustedPool:    NewSessionPool(params.ManifestID, int(untrustedPoolSize), untrustedNumOrchs, susUntrusted, createSessionsUntrusted, sel()),
	}
	bsm.trustedPool.refreshSessions()
	bsm.untrustedPool.refreshSessions()
	return bsm
}

func (bsm *BroadcastSessionsManager) suspendAndRemoveOrch(sess *BroadcastSession) {
	if sess.OrchestratorScore == common.Score_Untrusted {
		bsm.untrustedPool.suspend(sess.OrchestratorInfo.GetTranscoder())
		bsm.untrustedPool.removeSession(sess)
	} else {
		bsm.trustedPool.suspend(sess.OrchestratorInfo.GetTranscoder())
		bsm.trustedPool.removeSession(sess)
	}
}

func (bsm *BroadcastSessionsManager) removeSession(session *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	if session.OrchestratorScore == common.Score_Untrusted {
		bsm.untrustedPool.removeSession(session)
	} else {
		bsm.trustedPool.removeSession(session)
	}
}

func (bs *BroadcastSession) pushSegInFlight(seg *stream.HLSSegment) {
	bs.lock.Lock()
	bs.SegsInFlight = append(bs.SegsInFlight,
		SegFlightMetadata{
			startTime: time.Now(),
			segDur:    time.Duration(seg.Duration * float64(time.Second)),
		})
	bs.lock.Unlock()
}

// selects number of sessions to use according to current algorithm
func (bsm *BroadcastSessionsManager) selectSessions() ([]*BroadcastSession, bool) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	if bsm.VerificationFreq > 0 {

		sessions := bsm.trustedPool.selectSessions(1)
		untrustedSesions := bsm.untrustedPool.selectSessions(2)
		calcPerceptualHash := len(sessions) > 0 && len(untrustedSesions) > 0
		return append(sessions, untrustedSesions...), calcPerceptualHash
	}

	sessions := bsm.trustedPool.selectSessions(1)
	if len(sessions) == 0 {
		sessions = bsm.untrustedPool.selectSessions(1)
	}

	return sessions, false
}

func (bsm *BroadcastSessionsManager) cleanup() {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.finished = true

	bsm.trustedPool.cleanup()
	bsm.untrustedPool.cleanup()
}

func (bsm *BroadcastSessionsManager) chooseResults(submitResultsCh chan *SubmitResult,
	submittedCount int) (*BroadcastSession, *ReceivedTranscodeResult, error) {

	submitResults := make([]*SubmitResult, submittedCount)

	// can have different strategies - for example, just use first one
	// and ignore everything else
	// for now wait for all the results
	for i := 0; i < submittedCount; i++ {
		submitResults[i] = <-submitResultsCh
	}
	// we're here because we're doing verification
	var trustedResults *SubmitResult
	var untrustedResults []*SubmitResult
	var err error
	for _, res := range submitResults {
		if res.Err == nil && res.TranscodeResult != nil {
			if res.Session.OrchestratorScore == common.Score_Trusted {
				trustedResults = res
			} else {
				untrustedResults = append(untrustedResults, res)
			}
		}
		if res.Err != nil {
			err = res.Err
			if isNonRetryableError(err) {
				bsm.completeSession(res.Session)
			} else {
				bsm.suspendAndRemoveOrch(res.Session)
			}
		}
	}
	if trustedResults == nil {
		// no results from trusted orch, using anything
		if len(untrustedResults) == 0 {
			// no results at all
			return nil, nil, fmt.Errorf("error transcoding: no results at all err=%w", err)
		}
		return untrustedResults[0].Session, untrustedResults[0].TranscodeResult, untrustedResults[0].Err
	}
	if len(untrustedResults) == 0 {
		// no results from untrusted orch, just using trusted ones
		return trustedResults.Session, trustedResults.TranscodeResult, trustedResults.Err
	}
	segmToCheckIndex := rand.Intn(len(trustedResults.TranscodeResult.Segments))
	// downloading hashes
	trustedHash, err := drivers.GetSegmentData(trustedResults.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl)
	if err != nil {
		err = fmt.Errorf("error downloading perceptual hash from url=%s err=%w",
			trustedResults.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, err)
		return nil, nil, err
	}
	for _, untrustedResult := range untrustedResults {
		untrustedHash, err := drivers.GetSegmentData(untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl)
		if err != nil {
			err = fmt.Errorf("error downloading perceptual hash from url=%s err=%w",
				untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, err)
			return nil, nil, err
		}
		equal, err := ffmpeg.CompareSignatureByBuffer(trustedHash, untrustedHash)
		if err != nil {
			glog.Errorf("error comparing perceptual hashes from url=%s err=%v",
				untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, err)
		}
		glog.Infof("Hashes from url=%s and url=%s are equal=%v",
			trustedResults.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl,
			untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, equal)
		if equal {
			// tell to use untrusted orch for all transcoding
			return untrustedResult.Session, untrustedResult.TranscodeResult, untrustedResult.Err
		}
	}

	return trustedResults.Session, trustedResults.TranscodeResult, trustedResults.Err
}

func (bsm *BroadcastSessionsManager) completeSession(sess *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	if sess.OrchestratorScore == common.Score_Untrusted {
		bsm.untrustedPool.completeSession(sess)
	} else if sess.OrchestratorScore == common.Score_Trusted {
		bsm.trustedPool.completeSession(sess)
	} else {
		panic("shouldn't happen")
	}
}

func selectOrchestrator(n *core.LivepeerNode, params *core.StreamParameters, count int, sus *suspender,
	scorePred common.ScorePred) ([]*BroadcastSession, error) {

	if n.OrchestratorPool == nil {
		glog.Info("No orchestrators specified; not transcoding")
		return nil, errDiscovery
	}

	tinfos, err := n.OrchestratorPool.GetOrchestrators(count, sus, params.Capabilities, scorePred)
	if len(tinfos) <= 0 {
		glog.Info("No orchestrators found; not transcoding. Error: ", err)
		return nil, errNoOrchs
	}
	if err != nil {
		return nil, err
	}

	var sessions []*BroadcastSession

	for _, tinfo := range tinfos {
		var (
			sessionID    string
			balance      Balance
			ticketParams *pm.TicketParams
		)

		if n.Sender != nil && tinfo.TicketParams != nil {
			ticketParams = pmTicketParams(tinfo.TicketParams)
			sessionID = n.Sender.StartSession(*ticketParams)
		}

		if n.Balances != nil {
			balance = core.NewBalance(ticketParams.Recipient, core.ManifestID(tinfo.AuthToken.SessionId), n.Balances)
		}

		var orchOS drivers.OSSession
		if len(tinfo.Storage) > 0 {
			orchOS = drivers.NewSession(tinfo.Storage[0])
		}

		bcastOS := params.OS
		if bcastOS.IsExternal() {
			// Give each O its own OS session to prevent front running uploads
			pfx := fmt.Sprintf("%v/%v", params.ManifestID, tinfo.AuthToken.SessionId)
			bcastOS = bcastOS.OS().NewSession(pfx)
		}

		session := &BroadcastSession{
			Broadcaster:       core.NewBroadcaster(n),
			Params:            params,
			OrchestratorInfo:  tinfo,
			OrchestratorOS:    orchOS,
			BroadcasterOS:     bcastOS,
			Sender:            n.Sender,
			PMSessionID:       sessionID,
			Balances:          n.Balances,
			Balance:           balance,
			lock:              &sync.RWMutex{},
			OrchestratorScore: n.OrchestratorPool.GetInfo(tinfo.Transcoder).Score, // todo: use score from OrchestratorLocalInfo
		}

		sessions = append(sessions, session)
	}
	return sessions, nil
}

func processSegment(cxn *rtmpConnection, seg *stream.HLSSegment) ([]string, error) {

	rtmpStrm := cxn.stream
	nonce := cxn.nonce
	cpl := cxn.pl
	mid := cxn.mid
	vProfile := cxn.profile

	if seg.Duration > maxDurationSec || seg.Duration < 0 {
		glog.Errorf("Invalid duration nonce=%d manifestID=%s seqNo=%d dur=%v", nonce, mid, seg.SeqNo, seg.Duration)
		return nil, fmt.Errorf("Invalid duration %v", seg.Duration)
	}

	glog.V(common.DEBUG).Infof("Processing segment nonce=%d manifestID=%s seqNo=%d dur=%v bytes=%v", nonce, mid, seg.SeqNo, seg.Duration, len(seg.Data))
	if monitor.Enabled {
		monitor.SegmentEmerged(nonce, seg.SeqNo, len(BroadcastJobVideoProfiles), seg.Duration)
	}
	atomic.AddUint64(&cxn.sourceBytes, uint64(len(seg.Data)))

	seg.Name = "" // hijack seg.Name to convey the uploaded URI
	ext, err := common.ProfileFormatExtension(vProfile.Format)
	if err != nil {
		glog.Errorf("Unknown format extension manifestID=%s seqNo=%d err=%s", mid, seg.SeqNo, err)
		return nil, err
	}
	name := fmt.Sprintf("%s/%d%s", vProfile.Name, seg.SeqNo, ext)
	ros := cpl.GetRecordOSSession()
	segDurMs := getSegDurMsString(seg)

	now := time.Now()
	hasZeroVideoFrame, err := ffmpeg.HasZeroVideoFrameBytes(seg.Data)
	if err != nil {
		glog.Warningf("Error checking for zero video frame manifestID=%s name=%s bytes=%d took=%s err=%v",
			mid, seg.Name, len(seg.Data), time.Since(now), err)
	}
	if ros != nil && !hasZeroVideoFrame {
		go func() {
			now := time.Now()
			uri, err := drivers.SaveRetried(ros, name, seg.Data, map[string]string{"duration": segDurMs}, 2)
			took := time.Since(now)
			if err != nil {
				glog.Errorf("Error saving nonce=%d manifestID=%s name=%s bytes=%d to record store err=%v",
					nonce, mid, name, len(seg.Data), err)
			} else {
				cpl.InsertHLSSegmentJSON(vProfile, seg.SeqNo, uri, seg.Duration)
				glog.Infof("Successfully saved nonce=%d manifestID=%s name=%s bytes=%d to record store took=%s",
					nonce, mid, name, len(seg.Data), took)
				cpl.FlushRecord()
			}
			if monitor.Enabled {
				monitor.RecordingSegmentSaved(took, err)
			}
		}()
	}
	uri, err := cpl.GetOSSession().SaveData(name, seg.Data, nil, 0)
	if err != nil {
		glog.Errorf("Error saving segment nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err, true)
		}
		return nil, err
	}
	if cpl.GetOSSession().IsExternal() {
		seg.Name = uri // hijack seg.Name to convey the uploaded URI
	}
	err = cpl.InsertHLSSegment(vProfile, seg.SeqNo, uri, seg.Duration)
	if monitor.Enabled {
		monitor.SourceSegmentAppeared(nonce, seg.SeqNo, string(mid), vProfile.Name, ros != nil)
	}
	if err != nil {
		glog.Errorf("Error inserting segment manifestID=%s nonce=%d seqNo=%d err=%v", cxn.mid, nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorDuplicateSegment, err, false)
		}
	}

	if hasZeroVideoFrame {
		var urls []string
		for _, profile := range cxn.params.Profiles {
			ext, err := common.ProfileFormatExtension(profile.Format)
			if err != nil {
				glog.Errorf("Error getting extension for profile=%v with segment manifestID=%s nonce=%d seqNo=%d err=%v",
					profile.Format, cxn.mid, nonce, seg.SeqNo, err)
				return nil, err
			}
			name := fmt.Sprintf("%s/%d%s", profile.Name, seg.SeqNo, ext)
			uri, err := cpl.GetOSSession().SaveData(name, seg.Data, nil, 0)
			if err != nil {
				glog.Errorf("Error saving segment manifestID=%s nonce=%d seqNo=%d err=%v", cxn.mid, nonce, seg.SeqNo, err)
				if monitor.Enabled {
					monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err, true)
				}
				return nil, err
			}
			urls = append(urls, uri)
			err = cpl.InsertHLSSegment(&profile, seg.SeqNo, uri, seg.Duration)
			if err != nil {
				glog.Errorf("Error inserting segment manifestID=%s nonce=%d seqNo=%d err=%v", cxn.mid, nonce, seg.SeqNo, err)
				if monitor.Enabled {
					monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorDuplicateSegment, err, false)
				}
			}
		}
		return urls, nil
	}

	var sv *verification.SegmentVerifier
	if Policy != nil {
		sv = verification.NewSegmentVerifier(Policy)
	}

	var (
		startTime = time.Now()
		attempts  []data.TranscodeAttemptInfo
		urls      []string
	)
	for len(attempts) < MaxAttempts {
		// if transcodeSegment fails, retry; rudimentary
		var info *data.TranscodeAttemptInfo
		urls, info, err = transcodeSegment(cxn, seg, name, sv)
		attempts = append(attempts, *info)
		if err == nil {
			break
		}

		if shouldStopStream(err) {
			glog.Warningf("Stopping current stream due to err=%v", err)
			rtmpStrm.Close()
			break
		}
		if isNonRetryableError(err) {
			glog.Warningf("Not retrying current segment nonce=%d seqNo=%d due to non-retryable error err=%v", nonce, seg.SeqNo, err)
			break
		}
		// recoverable error, retry
	}
	if MetadataQueue != nil {
		success := err == nil && len(urls) > 0
		streamID := string(mid)
		if cxn.params != nil && cxn.params.ExternalStreamID != "" {
			streamID = cxn.params.ExternalStreamID
		}
		key := newTranscodeEventKey(mid, streamID)
		evt := newTranscodeEvent(streamID, seg, startTime, success, attempts)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), MetadataPublishTimeout)
			defer cancel()
			if err := MetadataQueue.Publish(ctx, key, evt, false); err != nil {
				glog.Errorf("Error publishing stream transcode event: err=%q manifestID=%q seqNo=%d key=%q event=%+v", err, mid, seg.SeqNo, key, evt)
			}
		}()
	}
	if len(attempts) == MaxAttempts && err != nil {
		err = fmt.Errorf("Hit max transcode attempts: %w", err)
	}
	return urls, err
}

func transcodeSegment(cxn *rtmpConnection, seg *stream.HLSSegment, name string,
	verifier *verification.SegmentVerifier) ([]string, *data.TranscodeAttemptInfo, error) {

	var urls []string
	info := &data.TranscodeAttemptInfo{}
	var err error

	defer func(startTime time.Time) {
		info.LatencyMs = time.Since(startTime).Milliseconds()
		if err != nil {
			errStr := err.Error()
			info.Error = &errStr
		}
	}(time.Now())

	nonce := cxn.nonce
	sessions, calcPerceptualHash := cxn.sessManager.selectSessions()
	// Return early under a few circumstances:
	// View-only (non-transcoded) streams or no sessions available
	if len(sessions) == 0 {
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorNoOrchestrators, nonce, seg.SeqNo, errNoOrchs, true)
		}
		glog.Infof("No sessions available for segment nonce=%d manifestID=%s seqNo=%d", nonce, cxn.mid, seg.SeqNo)
		// We may want to introduce a "non-retryable" error type here
		// would help error propagation for live ingest.
		// similar to the orchestrator's RemoteTranscoderFatalError
		return nil, info, nil
	}
	info.Orchestrator = data.OrchestratorMetadata{
		TranscoderUri: sessions[0].Transcoder(),
		Address:       sessions[0].Address(),
	}

	glog.Infof("Trying to transcode segment manifestID=%v nonce=%d seqNo=%d using sessions=%d", cxn.mid, nonce, seg.SeqNo, len(sessions))
	if monitor.Enabled {
		monitor.TranscodeTry(nonce, seg.SeqNo)
	}
	if len(sessions) == 1 {
		// shortcut for most common path
		sess := sessions[0]
		if seg, err = prepareForTranscoding(cxn, sess, seg, name); err != nil {
			return nil, info, err
		}
		// cxn.sessManager.pushSegInFlight(sess, seg)
		sess.pushSegInFlight(seg)
		var res *ReceivedTranscodeResult
		res, err = SubmitSegment(sess.Clone(), seg, nonce, false)
		if err != nil || res == nil {
			if isNonRetryableError(err) {
				cxn.sessManager.completeSession(sess)
				return nil, info, err
			}
			cxn.sessManager.suspendAndRemoveOrch(sess)
			if res == nil && err == nil {
				err = errors.New("empty response")
			}
			return nil, info, err
		}
		urls, err = downloadResults(cxn, seg, sess, res, verifier)
		return urls, info, err
	} else {
		resc := make(chan *SubmitResult, len(sessions))
		submittedCount := 0
		for _, sess := range sessions {
			// todo: run it in own goroutine (move to submitSegment?)
			if seg, err = prepareForTranscoding(cxn, sess, seg, name); err != nil {
				continue
			}
			// cxn.sessManager.pushSegInFlight(sess, seg)
			sess.pushSegInFlight(seg)
			go submitSegment(sess, seg, nonce, calcPerceptualHash, resc)
			submittedCount++
		}
		if submittedCount == 0 {
			return nil, info, fmt.Errorf("error: not submitted anything")
		}

		sess, results, err := cxn.sessManager.chooseResults(resc, submittedCount)
		if err != nil {
			glog.Errorf("Error choosing results: err=%v", err)
			return nil, info, err
		}
		for _, usedSession := range sessions {
			if usedSession != sess {
				// return session that we're not using
				cxn.sessManager.completeSession(usedSession)
			}
		}

		urls, err = downloadResults(cxn, seg, sess, results, verifier)
		return urls, info, err
	}
}

type SubmitResult struct {
	Session         *BroadcastSession
	TranscodeResult *ReceivedTranscodeResult
	Err             error
}

func submitSegment(sess *BroadcastSession, seg *stream.HLSSegment, nonce uint64, calcPerceptualHash bool, resc chan *SubmitResult) {
	res, err := SubmitSegment(sess.Clone(), seg, nonce, calcPerceptualHash)
	resc <- &SubmitResult{
		Session:         sess,
		TranscodeResult: res,
		Err:             err,
	}
}

func prepareForTranscoding(cxn *rtmpConnection, sess *BroadcastSession, seg *stream.HLSSegment,
	name string) (*stream.HLSSegment, error) {

	// storage the orchestrator prefers
	res := seg
	sess.lock.RLock()
	ios := sess.OrchestratorOS
	sess.lock.RUnlock()
	if ios != nil {
		// XXX handle case when orch expects direct upload
		uri, err := ios.SaveData(name, seg.Data, nil, 0)
		if err != nil {
			glog.Errorf("Error saving segment to OS manifestID=%v nonce=%d seqNo=%d err=%v", cxn.mid, cxn.nonce, seg.SeqNo, err)
			if monitor.Enabled {
				monitor.SegmentUploadFailed(cxn.nonce, seg.SeqNo, monitor.SegmentUploadErrorOS, err, false)
			}
			cxn.sessManager.suspendAndRemoveOrch(sess)
			return nil, err
		}
		segCopy := *seg
		res = &segCopy
		res.Name = uri // hijack seg.Name to convey the uploaded URI
	}

	refresh, err := shouldRefreshSession(sess)
	if err != nil {
		glog.Errorf("Error checking whether to refresh session manifestID=%s orch=%v err=%v", cxn.mid, sess.Transcoder(), err)
		cxn.sessManager.suspendAndRemoveOrch(sess)
		return nil, err
	}

	if refresh {
		err := refreshSession(sess)
		if err != nil {
			glog.Errorf("Error refreshing session manifestID=%s orch=%v err=%v", cxn.mid, sess.Transcoder(), err)
			cxn.sessManager.suspendAndRemoveOrch(sess)
			return nil, err
		}
	}
	return res, nil
}

func downloadResults(cxn *rtmpConnection, seg *stream.HLSSegment, sess *BroadcastSession, res *ReceivedTranscodeResult,
	verifier *verification.SegmentVerifier) ([]string, error) {

	nonce := cxn.nonce
	// download transcoded segments from the transcoder
	gotErr := false // only send one error msg per segment list
	var errCode monitor.SegmentTranscodeError
	errFunc := func(subType monitor.SegmentTranscodeError, url string, err error) {
		glog.Errorf("%v error with segment nonce=%d seqNo=%d: %v (URL: %v)", subType, nonce, seg.SeqNo, err, url)
		if monitor.Enabled && !gotErr {
			monitor.SegmentTranscodeFailed(subType, nonce, seg.SeqNo, err, false)
			gotErr = true
			errCode = subType
		}
	}
	cpl := cxn.pl

	var dlErr error
	segData := make([][]byte, len(res.Segments))
	n := len(res.Segments)
	segURLs := make([]string, len(res.Segments))
	segLock := &sync.Mutex{}
	cond := sync.NewCond(segLock)
	var recordWG sync.WaitGroup

	dlFunc := func(url string, pixels int64, i int) {
		defer func() {
			cond.L.Lock()
			n--
			if n == 0 {
				cond.Signal()
			}
			cond.L.Unlock()
		}()

		bos := sess.BroadcasterOS
		profile := sess.Params.Profiles[i]

		bros := cpl.GetRecordOSSession()
		var data []byte
		// Download segment data in the following cases:
		// - A verification policy is set. The segment data is needed for signature verification and/or pixel count verification
		// - The segment data needs to be uploaded to the broadcaster's own OS
		if verifier != nil || bros != nil || bos != nil && !bos.IsOwn(url) {
			d, err := downloadSeg(url)
			if err != nil {
				errFunc(monitor.SegmentTranscodeErrorDownload, url, err)
				segLock.Lock()
				dlErr = err
				segLock.Unlock()
				cxn.sessManager.suspendAndRemoveOrch(sess)
				return
			}

			data = d
			atomic.AddUint64(&cxn.transcodedBytes, uint64(len(data)))
		}

		if bros != nil {
			go func() {
				ext, _ := common.ProfileFormatExtension(profile.Format)
				name := fmt.Sprintf("%s/%d%s", profile.Name, seg.SeqNo, ext)
				segDurMs := getSegDurMsString(seg)
				now := time.Now()
				uri, err := drivers.SaveRetried(bros, name, data, map[string]string{"duration": segDurMs}, 2)
				took := time.Since(now)
				if err != nil {
					glog.Errorf("Error saving nonce=%d manifestID=%s name=%s to record store err=%v", nonce, cxn.mid, name, err)
				} else {
					cpl.InsertHLSSegmentJSON(&profile, seg.SeqNo, uri, seg.Duration)
					glog.Infof("Successfully saved nonce=%d manifestID=%s name=%s size=%d bytes to record store took=%s",
						nonce, cxn.mid, name, len(data), took)
				}
				recordWG.Done()
				if monitor.Enabled {
					monitor.RecordingSegmentSaved(took, err)
				}
			}()
		}

		if bos != nil && !bos.IsOwn(url) {
			ext, err := common.ProfileFormatExtension(profile.Format)
			if err != nil {
				errFunc(monitor.SegmentTranscodeErrorSaveData, url, err)
				return
			}
			name := fmt.Sprintf("%s/%d%s", profile.Name, seg.SeqNo, ext)
			newURL, err := bos.SaveData(name, data, nil, 0)
			if err != nil {
				switch err.Error() {
				case "Session ended":
					errFunc(monitor.SegmentTranscodeErrorSessionEnded, url, err)
				default:
					errFunc(monitor.SegmentTranscodeErrorSaveData, url, err)
				}
				return
			}
			url = newURL
		}

		// Store URLs for the verifier. Be aware that the segment is
		// already within object storage  at this point, whether local or
		// external. If a client were to ignore the playlist and
		// preemptively fetch segments, they could be reading tampered
		// data. Not an issue if the delivery protocol is being obeyed.
		segLock.Lock()
		segURLs[i] = url
		segData[i] = data
		segLock.Unlock()

		if monitor.Enabled {
			monitor.TranscodedSegmentAppeared(nonce, seg.SeqNo, profile.Name, bros != nil)
		}
	}

	dlStart := time.Now()
	if cpl.GetRecordOSSession() != nil && len(res.Segments) > 0 {
		recordWG.Add(len(res.Segments))
	}
	for i, v := range res.Segments {
		go dlFunc(v.Url, v.Pixels, i)
	}
	if cpl.GetRecordOSSession() != nil && len(res.Segments) > 0 {
		go func() {
			recordWG.Wait()
			cpl.FlushRecord()
		}()
	}

	cond.L.Lock()
	for n != 0 {
		cond.Wait()
	}
	cond.L.Unlock()
	if dlErr != nil {
		return nil, dlErr
	}
	updateSession(sess, res)
	cxn.sessManager.completeSession(sess)

	downloadDur := time.Since(dlStart)
	if monitor.Enabled {
		monitor.SegmentDownloaded(nonce, seg.SeqNo, downloadDur)
	}

	if verifier != nil {
		// verify potentially can change content of segURLs
		err := verify(verifier, cxn, sess, seg, res.TranscodeData, segURLs, segData)
		if err != nil {
			glog.Errorf("Error verifying nonce=%d manifestID=%s seqNo=%d err=%s", nonce, cxn.mid, seg.SeqNo, err)
			return nil, err
		}
	}

	for i, url := range segURLs {
		err := cpl.InsertHLSSegment(&sess.Params.Profiles[i], seg.SeqNo, url, seg.Duration)
		if err != nil {
			// InsertHLSSegment only returns ErrSegmentAlreadyExists error
			// Right now InsertHLSSegment call is atomic regarding transcoded segments - we either inserting
			// all the transcoded segments or none, so we shouldn't hit this error
			// But report in case that InsertHLSSegment changed or something wrong is going on in other parts of workflow
			glog.Errorf("Playlist insertion error nonce=%d manifestID=%s seqNo=%d err=%s", nonce, cxn.mid, seg.SeqNo, err)
			if monitor.Enabled {
				monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorDuplicateSegment, nonce, seg.SeqNo, err, false)
			}
		}
	}

	if monitor.Enabled {
		monitor.SegmentFullyTranscoded(nonce, seg.SeqNo, common.ProfilesNames(sess.Params.Profiles), errCode)
	}

	glog.V(common.DEBUG).Infof("Successfully validated segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
	return segURLs, nil
}

var sessionErrStrings = []string{"dial tcp", "unexpected EOF", core.ErrOrchBusy.Error(), core.ErrOrchCap.Error()}

var sessionErrRegex = common.GenErrRegex(sessionErrStrings)

func shouldStopSession(err error) bool {
	return sessionErrRegex.MatchString(err.Error())
}

func verify(verifier *verification.SegmentVerifier, cxn *rtmpConnection,
	sess *BroadcastSession, source *stream.HLSSegment,
	res *net.TranscodeData, URIs []string, segData [][]byte) error {

	sess.lock.RLock()
	OrchestratorInfo := sess.OrchestratorInfo
	sess.lock.RUnlock()
	// Cache segment contents in params.Renditions
	// If we need to retry transcoding because verification fails,
	// the the segments' OS location will be overwritten.
	// Cache the segments so we can restore them in OS if necessary.
	params := &verification.Params{
		ManifestID:   sess.Params.ManifestID,
		Source:       source,
		Profiles:     sess.Params.Profiles,
		Orchestrator: OrchestratorInfo,
		Results:      res,
		URIs:         URIs,
		Renditions:   segData,
		OS:           cxn.pl.GetOSSession(),
	}

	// The return value from the verifier, if any, are the *accepted* params.
	// The accepted params are not necessarily the same as `params` sent here.
	// The accepted params may be from an earlier iteration if max retries hit.
	accepted, err := verifier.Verify(params)
	if verification.IsRetryable(err) {
		// If retryable, means tampering was detected from this O
		// Remove the O from the working set for now
		// Error falls through towards end if necessary
		cxn.sessManager.removeSession(sess)
	}
	if accepted != nil {
		// The returned set of results has been accepted by the verifier

		// Check if an earlier verification attempt was the one accepted.
		// If so, reset the local OS if we're using that since it's been
		// overwritten with this rendition.
		for i, data := range accepted.Renditions {
			if accepted != params && !sess.BroadcasterOS.IsExternal() {
				// Sanity check that we actually have the rendition data?
				if len(data) <= 0 {
					return errors.New("MissingLocalData")
				}
				// SaveData only takes the /<rendition>/<seqNo> part of the URI
				// However, it returns /stream/<manifestID>/<rendition>/<seqNo>
				// The incoming URI is likely to be in the longer format.
				// Hence, trim the /stream/<manifestID> prefix if it exists.
				pfx := fmt.Sprintf("/stream/%s/", sess.Params.ManifestID)
				uri := strings.TrimPrefix(accepted.URIs[i], pfx)
				_, err := sess.BroadcasterOS.SaveData(uri, data, nil, 0)
				if err != nil {
					return err
				}
			} else {
				// Normally we don't need to reset the URI here, but we do
				// if an external OS is used and an earlier attempt is accepted
				// (Recall that each O uploads segments to a different location
				// if a B-supplied external OS is used)
				URIs[i] = accepted.URIs[i]
			}
		}

		// Ignore any errors from the Verify call; don't need to retry anymore
		return nil
	}
	return err // possibly nil
}

// Return an updated copy of the given session using the received transcode result
func updateSession(sess *BroadcastSession, res *ReceivedTranscodeResult) {
	sess.lock.Lock()
	defer sess.lock.Unlock()
	sess.LatencyScore = res.LatencyScore

	if res.Info == nil {
		// Return early if we do not need to update OrchestratorInfo
		return
	}

	oInfo := res.Info
	oldInfo := sess.OrchestratorInfo
	sess.OrchestratorInfo = oInfo

	if len(oInfo.Storage) > 0 {
		sess.OrchestratorOS = drivers.NewSession(oInfo.Storage[0])
	}

	if sess.Sender != nil && oInfo.TicketParams != nil {
		// Note: We do not validate the ticket params included in the OrchestratorInfo
		// message here. Instead, we store the ticket params with the current BroadcastSession
		// and the next time this BroadcastSession is used, the ticket params will be validated
		// during ticket creation in genPayment(). If ticket params validation during ticket
		// creation fails, then this BroadcastSession will be removed
		sess.PMSessionID = sess.Sender.StartSession(*pmTicketParams(oInfo.TicketParams))

		// Session ID changed so we need to make sure the balance tracks the new session ID
		if oldInfo.AuthToken.SessionId != oInfo.AuthToken.SessionId {
			sess.Balance = core.NewBalance(ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient),
				core.ManifestID(sess.OrchestratorInfo.AuthToken.SessionId), sess.Balances)
		}
	}
}

func refreshSession(sess *BroadcastSession) error {
	uri, err := url.Parse(sess.Transcoder())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	oInfo, err := getOrchestratorInfoRPC(ctx, sess.Broadcaster, uri)
	if err != nil {
		return err
	}

	// Create dummy result
	sess.lock.RLock()
	res := &ReceivedTranscodeResult{
		LatencyScore: sess.LatencyScore,
		Info:         oInfo,
	}
	sess.lock.RUnlock()

	updateSession(sess, res)
	return nil
}

func shouldRefreshSession(sess *BroadcastSession) (bool, error) {
	sess.lock.RLock()
	OrchestratorInfo := sess.OrchestratorInfo
	sess.lock.RUnlock()
	if OrchestratorInfo.AuthToken == nil {
		return false, errors.New("missing auth token")
	}

	// Refresh auth token if we are within the last 10% of the token's valid period
	authTokenExpireBuffer := 0.1
	refreshPoint := OrchestratorInfo.AuthToken.Expiration - int64(authTokenValidPeriod.Seconds()*authTokenExpireBuffer)
	if time.Now().After(time.Unix(refreshPoint, 0)) {
		glog.V(common.VERBOSE).Infof("Auth token expired, refreshing for orch=%v", OrchestratorInfo.Transcoder)

		return true, nil
	}

	if sess.Sender != nil {
		if err := sess.Sender.ValidateTicketParams(pmTicketParams(OrchestratorInfo.TicketParams)); err != nil {
			if err != pm.ErrTicketParamsExpired {
				return false, err
			}

			glog.V(common.VERBOSE).Infof("Ticket params expired, refreshing for orch=%v", OrchestratorInfo.Transcoder)

			return true, nil
		}
	}

	return false, nil
}

func newTranscodeEventKey(mid core.ManifestID, streamID string) string {
	shardKey := string(mid[0])
	return fmt.Sprintf("stream_health.transcode.%s.%s", shardKey, streamID)
}

func newTranscodeEvent(streamID string, seg *stream.HLSSegment, startTime time.Time, success bool, attempts []data.TranscodeAttemptInfo) *data.TranscodeEvent {
	segMeta := data.SegmentMetadata{
		Name:     seg.Name,
		SeqNo:    seg.SeqNo,
		Duration: seg.Duration,
		ByteSize: len(seg.Data),
	}
	return data.NewTranscodeEvent(monitor.NodeID, streamID, segMeta, startTime, success, attempts)
}

func getSegDurMsString(seg *stream.HLSSegment) string {
	return strconv.Itoa(int(seg.Duration * 1000))
}

func nonRetryableErrMapInit() map[string]bool {
	errs := make(map[string]bool)
	for _, v := range ffmpeg.NonRetryableErrs {
		errs[v] = true
	}
	return errs
}

var NonRetryableErrMap = nonRetryableErrMapInit()

func isNonRetryableError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if NonRetryableErrMap[e.Error()] {
			return true
		}
	}
	return false
}
