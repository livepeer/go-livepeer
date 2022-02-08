package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

	"github.com/livepeer/go-livepeer/clog"
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

// Max threshold for # of broadcast sessions under which we will refresh the session list
var maxRefreshSessionsThreshold = 8.0

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
	poolSize := math.Max(1, float64(sp.poolSize))
	numOrchs := math.Max(1, float64(sp.numOrchs))
	penalty := int(math.Ceil(poolSize / numOrchs))
	sp.sus.suspend(orch, penalty)
}

func (sp *SessionPool) refreshSessions(ctx context.Context) {
	started := time.Now()
	clog.V(common.DEBUG).Infof(ctx, "Starting session refresh")
	defer func() {
		sp.lock.Lock()
		clog.V(common.DEBUG).Infof(ctx, "Ending session refresh dur=%s orchs=%d", time.Since(started),
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
			// Re-use last session if oldest segment is in-flight for < durMult * segDur
			return session
		}
	}
	return nil
}

func (sp *SessionPool) selectSessions(ctx context.Context, sessionsNum int) []*BroadcastSession {
	sp.lock.Lock()
	defer sp.lock.Unlock()
	if sp.poolSize == 0 {
		return nil
	}

	checkSessions := func(m *SessionPool) bool {
		numSess := m.sel.Size()
		if numSess < int(math.Min(maxRefreshSessionsThreshold, math.Ceil(float64(m.numOrchs)/2.0))) {
			go m.refreshSessions(ctx)
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
			sess = sp.sel.Select(ctx)
		} else {
			gotFromLast = true
		}

		if sess == nil {
			// If no new sessions are available, re-use last session when oldest segment is in-flight for < 2 * segDur
			sess = selectSession(sp.lastSess, selectedSessions, 2)
			if sess != nil {
				gotFromLast = true
				clog.V(common.DEBUG).Infof(ctx, "No sessions in the selector for manifestID=%v re-using orch=%v with acceptable in-flight time",
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
				clog.V(common.DEBUG).Infof(ctx, "Removing orch=%v from manifestID=%s session list", sess.Transcoder(), sp.mid)
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
				clog.V(common.DEBUG).Infof(ctx, "Swapping from orch=%v to orch=%+v for manifestID=%s", ls.Transcoder(),
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

	verifiedSession *BroadcastSession
}

func NewSessionManager(ctx context.Context, node *core.LivepeerNode, params *core.StreamParameters, sel BroadcastSessionsSelectorFactory) *BroadcastSessionsManager {
	var trustedPoolSize, untrustedPoolSize float64
	if node.OrchestratorPool != nil {
		trustedPoolSize = float64(node.OrchestratorPool.SizeWith(common.ScoreAtLeast(common.Score_Trusted)))
		untrustedPoolSize = float64(node.OrchestratorPool.SizeWith(common.ScoreEqualTo(common.Score_Untrusted)))
	}
	maxInflight := common.HTTPTimeout.Seconds() / SegLen.Seconds()
	trustedNumOrchs := int(math.Min(trustedPoolSize, maxInflight*2))
	untrustedNumOrchs := int(untrustedPoolSize)
	susTrusted := newSuspender()
	susUntrusted := newSuspender()
	createSessionsTrusted := func() ([]*BroadcastSession, error) {
		return selectOrchestrator(ctx, node, params, trustedNumOrchs, susTrusted, common.ScoreAtLeast(common.Score_Trusted))
	}
	createSessionsUntrusted := func() ([]*BroadcastSession, error) {
		return selectOrchestrator(ctx, node, params, untrustedNumOrchs, susUntrusted, common.ScoreEqualTo(common.Score_Untrusted))
	}
	var stakeRdr stakeReader
	if node.Eth != nil {
		stakeRdr = &storeStakeReader{store: node.Database}
	}
	bsm := &BroadcastSessionsManager{
		mid:              params.ManifestID,
		VerificationFreq: params.VerificationFreq,
		trustedPool:      NewSessionPool(params.ManifestID, int(trustedPoolSize), trustedNumOrchs, susTrusted, createSessionsTrusted, NewMinLSSelector(stakeRdr, 1.0)),
		untrustedPool:    NewSessionPool(params.ManifestID, int(untrustedPoolSize), untrustedNumOrchs, susUntrusted, createSessionsUntrusted, NewMinLSSelectorWithRandFreq(stakeRdr, 1.0, SelectRandFreq)),
	}
	bsm.trustedPool.refreshSessions(ctx)
	bsm.untrustedPool.refreshSessions(ctx)
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
func (bsm *BroadcastSessionsManager) selectSessions(ctx context.Context) ([]*BroadcastSession, bool, bool) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	var verified bool

	if bsm.VerificationFreq > 0 {
		// Select 1 trusted O and 2 untrusted Os
		sessions := bsm.trustedPool.selectSessions(ctx, 1)
		untrustedSessions := bsm.untrustedPool.selectSessions(ctx, 2)
		sessions = append(sessions, untrustedSessions...)

		// Only return the last verified session if:
		// - It is present in the 3 sessions returned by the selector
		// - With probability 1 - 1/VerificationFrequency
		if bsm.verifiedSession != nil && includesSession(sessions, bsm.verifiedSession) &&
			common.RandomUintUnder(bsm.VerificationFreq) > 0 {
			clog.V(common.DEBUG).Infof(ctx, "Reusing verified orch=%v", bsm.verifiedSession.OrchestratorInfo.Transcoder)
			verified = true
			// Mark remaining unused sessions returned by selector as complete
			remaining := removeSessionFromList(sessions, bsm.verifiedSession)
			for _, sess := range remaining {
				bsm.completeSessionUnsafe(sess)
			}
			sessions = []*BroadcastSession{bsm.verifiedSession}
		} else if bsm.verifiedSession != nil && !includesSession(sessions, bsm.verifiedSession) {
			bsm.verifiedSession = nil
		}

		// Return selected sessions
		return sessions, true, verified
	}

	// Default to selecting from untrusted pool
	sessions := bsm.untrustedPool.selectSessions(ctx, 1)
	if len(sessions) == 0 {
		sessions = bsm.trustedPool.selectSessions(ctx, 1)
	}

	return sessions, false, verified
}

func (bsm *BroadcastSessionsManager) cleanup() {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.finished = true

	bsm.trustedPool.cleanup()
	bsm.untrustedPool.cleanup()
}

func (bsm *BroadcastSessionsManager) chooseResults(ctx context.Context, submitResultsCh chan *SubmitResult,
	submittedCount int) (*BroadcastSession, *ReceivedTranscodeResult, error) {

	trustedResult, untrustedResults, err := bsm.collectResults(submitResultsCh, submittedCount)

	if trustedResult == nil {
		// no results from trusted orch, using anything
		if len(untrustedResults) == 0 {
			// no results at all
			return nil, nil, fmt.Errorf("error transcoding: no results at all err=%w", err)
		}
		return untrustedResults[0].Session, untrustedResults[0].TranscodeResult, untrustedResults[0].Err
	}
	if len(untrustedResults) == 0 {
		// no results from untrusted orch, just using trusted ones
		return trustedResult.Session, trustedResult.TranscodeResult, trustedResult.Err
	}
	segmToCheckIndex := rand.Intn(len(trustedResult.TranscodeResult.Segments))

	// download trusted hashes
	trustedHash, err := drivers.GetSegmentData(ctx, trustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl)
	if err != nil {
		err = fmt.Errorf("error downloading perceptual hash from url=%s err=%w",
			trustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, err)
		return nil, nil, err
	}

	// verify untrusted hashes
	var sessionsToSuspend []*BroadcastSession
	for _, untrustedResult := range untrustedResults {
		untrustedHash, err := drivers.GetSegmentData(ctx, untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl)
		if err != nil {
			err = fmt.Errorf("error downloading perceptual hash from url=%s err=%w",
				untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, err)
			return nil, nil, err
		}
		equal, err := ffmpeg.CompareSignatureByBuffer(trustedHash, untrustedHash)
		if monitor.Enabled {
			monitor.FastVerificationDone()
		}
		if err != nil {
			clog.Errorf(ctx, "error comparing perceptual hashes from url=%s err=%q",
				untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, err)
		}
		clog.Infof(ctx, "Hashes from url=%s and url=%s are equal=%v",
			trustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl,
			untrustedResult.TranscodeResult.Segments[segmToCheckIndex].PerceptualHashUrl, equal)
		if equal {
			// stick to this verified orchestrator for further segments.
			if untrustedResult.Err == nil {
				bsm.sessionVerified(untrustedResult.Session)
			}
			// suspend sessions which returned incorrect results
			for _, s := range sessionsToSuspend {
				bsm.suspendAndRemoveOrch(s)
			}
			return untrustedResult.Session, untrustedResult.TranscodeResult, untrustedResult.Err
		} else {
			sessionsToSuspend = append(sessionsToSuspend, untrustedResult.Session)
			if monitor.Enabled {
				monitor.FastVerificationFailed()
			}
		}
	}

	return trustedResult.Session, trustedResult.TranscodeResult, trustedResult.Err
}

func (bsm *BroadcastSessionsManager) collectResults(submitResultsCh chan *SubmitResult, submittedCount int) (*SubmitResult, []*SubmitResult, error) {
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
			} else if res.Session == bsm.verifiedSession {
				// verified result should always come first and therefore take the priority
				untrustedResults = append([]*SubmitResult{res}, untrustedResults...)
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

	return trustedResults, untrustedResults, err
}

// the caller needs to ensure bsm.sessLock is acquired before calling this.
func (bsm *BroadcastSessionsManager) completeSessionUnsafe(sess *BroadcastSession) {
	if sess.OrchestratorScore == common.Score_Untrusted {
		bsm.untrustedPool.completeSession(sess)
	} else if sess.OrchestratorScore == common.Score_Trusted {
		bsm.trustedPool.completeSession(sess)
	} else {
		panic("shouldn't happen")
	}
}

func (bsm *BroadcastSessionsManager) completeSession(sess *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.completeSessionUnsafe(sess)
}

func (bsm *BroadcastSessionsManager) sessionVerified(sess *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.verifiedSession = sess
}

func (bsm *BroadcastSessionsManager) usingVerified() bool {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	return bsm.verifiedSession != nil
}

func selectOrchestrator(ctx context.Context, n *core.LivepeerNode, params *core.StreamParameters, count int, sus *suspender,
	scorePred common.ScorePred) ([]*BroadcastSession, error) {

	if n.OrchestratorPool == nil {
		clog.Infof(ctx, "No orchestrators specified; not transcoding")
		return nil, errDiscovery
	}

	tinfos, err := n.OrchestratorPool.GetOrchestrators(ctx, count, sus, params.Capabilities, scorePred)
	if len(tinfos) <= 0 {
		clog.InfofErr(ctx, "No orchestrators found; not transcoding", err)
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

		if tinfo.AuthToken == nil {
			clog.Errorf(ctx, "Missing auth token orch=%v", tinfo.Transcoder)
			continue
		}

		if n.Sender != nil {
			if tinfo.TicketParams == nil {
				clog.Errorf(ctx, "Missing ticket params orch=%v", tinfo.Transcoder)
				continue
			}

			ticketParams = pmTicketParams(tinfo.TicketParams)
			sessionID = n.Sender.StartSession(*ticketParams)

			if n.Balances != nil {
				balance = core.NewBalance(ticketParams.Recipient, core.ManifestID(tinfo.AuthToken.SessionId), n.Balances)
			}
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

func processSegment(ctx context.Context, cxn *rtmpConnection, seg *stream.HLSSegment) ([]string, error) {

	rtmpStrm := cxn.stream
	nonce := cxn.nonce
	cpl := cxn.pl
	mid := cxn.mid
	vProfile := cxn.profile

	if seg.Duration > maxDurationSec || seg.Duration < 0 {
		clog.Errorf(ctx, "Invalid duration seqNo=%d dur=%v", seg.SeqNo, seg.Duration)
		return nil, fmt.Errorf("invalid duration %v", seg.Duration)
	}

	clog.V(common.DEBUG).Infof(ctx, "Processing segment dur=%v bytes=%v", seg.Duration, len(seg.Data))
	if monitor.Enabled {
		monitor.SegmentEmerged(ctx, nonce, seg.SeqNo, len(BroadcastJobVideoProfiles), seg.Duration)
	}
	atomic.AddUint64(&cxn.sourceBytes, uint64(len(seg.Data)))

	seg.Name = "" // hijack seg.Name to convey the uploaded URI
	ext, err := common.ProfileFormatExtension(vProfile.Format)
	if err != nil {
		clog.Errorf(ctx, "Unknown format extension err=%s", err)
		return nil, err
	}
	name := fmt.Sprintf("%s/%d%s", vProfile.Name, seg.SeqNo, ext)
	ros := cpl.GetRecordOSSession()
	segDurMs := getSegDurMsString(seg)

	hasZeroVideoFrame := seg.IsZeroFrame
	if ros != nil && !hasZeroVideoFrame {
		go func() {
			now := time.Now()
			uri, err := drivers.SaveRetried(ctx, ros, name, seg.Data, map[string]string{"duration": segDurMs}, 2)
			took := time.Since(now)
			if err != nil {
				clog.Errorf(ctx, "Error saving name=%s bytes=%d to record store err=%q",
					name, len(seg.Data), err)
			} else {
				cpl.InsertHLSSegmentJSON(vProfile, seg.SeqNo, uri, seg.Duration)
				clog.Infof(ctx, "Successfully saved name=%s bytes=%d to record store took=%s",
					name, len(seg.Data), took)
				cpl.FlushRecord()
			}
			if monitor.Enabled {
				monitor.RecordingSegmentSaved(took, err)
			}
		}()
	}
	uri, err := cpl.GetOSSession().SaveData(ctx, name, seg.Data, nil, 0)
	if err != nil {
		clog.Errorf(ctx, "Error saving segment err=%q", err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err, true)
		}
		return nil, err
	}
	if cpl.GetOSSession().IsExternal() {
		seg.Name = uri // hijack seg.Name to convey the uploaded URI
	}
	err = cpl.InsertHLSSegment(vProfile, seg.SeqNo, uri, seg.Duration)
	if monitor.Enabled {
		monitor.SourceSegmentAppeared(ctx, nonce, seg.SeqNo, string(mid), vProfile.Name, ros != nil)
	}
	if err != nil {
		clog.Errorf(ctx, "Error inserting segment err=%q", err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorDuplicateSegment, err, false)
		}
	}

	if hasZeroVideoFrame {
		var urls []string
		for _, profile := range cxn.params.Profiles {
			ext, err := common.ProfileFormatExtension(profile.Format)
			if err != nil {
				clog.Errorf(ctx, "Error getting extension for profile=%v with segment err=%q",
					profile.Format, err)
				return nil, err
			}
			name := fmt.Sprintf("%s/%d%s", profile.Name, seg.SeqNo, ext)
			uri, err := cpl.GetOSSession().SaveData(ctx, name, seg.Data, nil, 0)
			if err != nil {
				clog.Errorf(ctx, "Error saving segment err=%q", err)
				if monitor.Enabled {
					monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err, true)
				}
				return nil, err
			}
			urls = append(urls, uri)
			err = cpl.InsertHLSSegment(&profile, seg.SeqNo, uri, seg.Duration)
			if err != nil {
				clog.Errorf(ctx, "Error inserting segment err=%q", err)
				if monitor.Enabled {
					monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorDuplicateSegment, err, false)
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
		urls, info, err = transcodeSegment(ctx, cxn, seg, name, sv)
		attempts = append(attempts, *info)
		if err == nil {
			break
		}

		if shouldStopStream(err) {
			clog.Warningf(ctx, "Stopping current stream due to err=%q", err)
			rtmpStrm.Close()
			break
		}
		if isNonRetryableError(err) {
			clog.Warningf(ctx, "Not retrying current segment due to non-retryable error err=%q", err)
			break
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
			clog.Warningf(ctx, "Not retrying current segment due to context cancellation err=%q", err)
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
				clog.Errorf(ctx, "Error publishing stream transcode event: err=%q key=%q event=%+v", err, key, evt)
			}
		}()
	}
	if len(attempts) == MaxAttempts && err != nil {
		err = fmt.Errorf("Hit max transcode attempts: %w", err)
	}
	return urls, err
}

func transcodeSegment(ctx context.Context, cxn *rtmpConnection, seg *stream.HLSSegment, name string,
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
	sessions, calcPerceptualHash, verified := cxn.sessManager.selectSessions(ctx)
	// Return early under a few circumstances:
	// View-only (non-transcoded) streams or no sessions available
	if len(sessions) == 0 {
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorNoOrchestrators, nonce, seg.SeqNo, errNoOrchs, true)
		}
		clog.Infof(ctx, "No sessions available for segment")
		// We may want to introduce a "non-retryable" error type here
		// would help error propagation for live ingest.
		// similar to the orchestrator's RemoteTranscoderFatalError
		return nil, info, nil
	}
	info.Orchestrator = data.OrchestratorMetadata{
		TranscoderUri: sessions[0].Transcoder(),
		Address:       sessions[0].Address(),
	}

	clog.Infof(ctx, "Trying to transcode segment using sessions=%d", len(sessions))
	if monitor.Enabled {
		monitor.TranscodeTry(ctx, nonce, seg.SeqNo)
	}
	if len(sessions) == 1 {
		// shortcut for most common path
		sess := sessions[0]
		if seg, err = prepareForTranscoding(ctx, cxn, sess, seg, name); err != nil {
			return nil, info, err
		}
		// cxn.sessManager.pushSegInFlight(sess, seg)
		sess.pushSegInFlight(seg)
		var res *ReceivedTranscodeResult
		res, err = SubmitSegment(ctx, sess.Clone(), seg, nonce, calcPerceptualHash, verified)
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
		// [EXPERIMENTAL] send content detection results to callback webhook
		// for now use detection only in common path
		if DetectionWebhookURL != nil && len(res.Detections) > 0 {
			clog.V(common.DEBUG).Infof(ctx, "Got detection result %v", res.Detections)
			go func(mid core.ManifestID, config core.DetectionConfig, seqNo uint64, detections []*net.DetectData) {
				req := common.DetectionWebhookRequest{ManifestID: string(mid), SeqNo: seqNo}
				for _, detection := range detections {
					switch x := detection.Value.(type) {
					case *net.DetectData_SceneClassification:
						probs := x.SceneClassification.ClassProbs
						// match returned probs (key: class id) with one of the user-selected class names
						for _, name := range config.SelectedClassNames {
							if id, ok := ffmpeg.DetectorClassIDLookup[name]; ok {
								if prob, ok := probs[uint32(id)]; ok {
									req.SceneClassification = append(req.SceneClassification,
										common.SceneClassificationResult{
											Name:        name,
											Probability: prob,
										})
								}
							}
						}
					}
				}
				jsonValue, err := json.Marshal(req)
				if err != nil {
					clog.Errorf(ctx, "Unable to marshal detection result into JSON ")
					return
				}
				resp, err := DetectionWhClient.Post(DetectionWebhookURL.String(), "application/json", bytes.NewBuffer(jsonValue))
				if err != nil {
					clog.Errorf(ctx, "Unable to POST detection result on webhook url=%v err=%q",
						DetectionWebhookURL.Redacted(), err)
				} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					rbody, rerr := ioutil.ReadAll(resp.Body)
					resp.Body.Close()
					if rerr != nil {
						clog.Errorf(ctx, "Detection webhook returned error status=%v with unreadable body err=%q",
							resp.StatusCode, rerr)
					} else {
						clog.Errorf(ctx, "Detection webhook returned error status=%v err=%q",
							resp.StatusCode, string(rbody))
					}
				}
			}(cxn.mid, cxn.params.Detection, seg.SeqNo, res.Detections)
		}
		// Ensure perceptual hash is generated if we ask for it
		if calcPerceptualHash {
			segmToCheckIndex := rand.Intn(len(res.Segments))
			segHash, err := drivers.GetSegmentData(ctx, res.Segments[segmToCheckIndex].PerceptualHashUrl)
			if err != nil || len(segHash) <= 0 {
				err = fmt.Errorf("error downloading perceptual hash from url=%s err=%w",
					res.Segments[segmToCheckIndex].PerceptualHashUrl, err)
				return nil, info, err
			}
		}
		urls, err = downloadResults(ctx, cxn, seg, sess, res, verifier)
		return urls, info, err
	} else {
		resc := make(chan *SubmitResult, len(sessions))
		submittedCount := 0
		for _, sess := range sessions {
			// todo: run it in own goroutine (move to submitSegment?)
			seg2, err := prepareForTranscoding(ctx, cxn, sess, seg, name)
			if err != nil || seg2 == nil {
				continue
			}
			// cxn.sessManager.pushSegInFlight(sess, seg)
			sess.pushSegInFlight(seg2)
			go submitSegment(ctx, sess, seg2, nonce, calcPerceptualHash, resc)
			submittedCount++
		}
		if submittedCount == 0 {
			return nil, info, fmt.Errorf("error: not submitted anything")
		}

		sess, results, err := cxn.sessManager.chooseResults(ctx, resc, submittedCount)
		if err != nil {
			clog.Errorf(ctx, "Error choosing results: err=%q", err)
			return nil, info, err
		}
		for _, usedSession := range sessions {
			if usedSession != sess {
				// return session that we're not using
				cxn.sessManager.completeSession(usedSession)
			}
		}

		urls, err = downloadResults(ctx, cxn, seg, sess, results, verifier)
		return urls, info, err
	}
}

type SubmitResult struct {
	Session         *BroadcastSession
	TranscodeResult *ReceivedTranscodeResult
	Err             error
}

func submitSegment(ctx context.Context, sess *BroadcastSession, seg *stream.HLSSegment, nonce uint64, calcPerceptualHash bool, resc chan *SubmitResult) {
	res, err := SubmitSegment(ctx, sess.Clone(), seg, nonce, calcPerceptualHash, false)
	resc <- &SubmitResult{
		Session:         sess,
		TranscodeResult: res,
		Err:             err,
	}
}

func prepareForTranscoding(ctx context.Context, cxn *rtmpConnection, sess *BroadcastSession, seg *stream.HLSSegment,
	name string) (*stream.HLSSegment, error) {

	// storage the orchestrator prefers
	res := seg
	sess.lock.RLock()
	ios := sess.OrchestratorOS
	sess.lock.RUnlock()
	if ios != nil {
		// XXX handle case when orch expects direct upload
		uri, err := ios.SaveData(ctx, name, seg.Data, nil, 0)
		if err != nil {
			clog.Errorf(ctx, "Error saving segment to OS manifestID=%v nonce=%d seqNo=%d err=%q", cxn.mid, cxn.nonce, seg.SeqNo, err)
			if monitor.Enabled {
				monitor.SegmentUploadFailed(ctx, cxn.nonce, seg.SeqNo, monitor.SegmentUploadErrorOS, err, false)
			}
			cxn.sessManager.suspendAndRemoveOrch(sess)
			return nil, err
		}
		segCopy := *seg
		res = &segCopy
		res.Name = uri // hijack seg.Name to convey the uploaded URI
	}

	refresh, err := shouldRefreshSession(ctx, sess)
	if err != nil {
		clog.Errorf(ctx, "Error checking whether to refresh session manifestID=%s orch=%v err=%q", cxn.mid, sess.Transcoder(), err)
		cxn.sessManager.suspendAndRemoveOrch(sess)
		return nil, err
	}

	if refresh {
		err := refreshSession(ctx, sess)
		if err != nil {
			clog.Errorf(ctx, "Error refreshing session manifestID=%s orch=%v err=%q", cxn.mid, sess.Transcoder(), err)
			cxn.sessManager.suspendAndRemoveOrch(sess)
			return nil, err
		}
	}
	return res, nil
}

func downloadResults(ctx context.Context, cxn *rtmpConnection, seg *stream.HLSSegment, sess *BroadcastSession, res *ReceivedTranscodeResult,
	verifier *verification.SegmentVerifier) ([]string, error) {

	nonce := cxn.nonce
	// download transcoded segments from the transcoder
	gotErr := false // only send one error msg per segment list
	var errCode monitor.SegmentTranscodeError
	errFunc := func(subType monitor.SegmentTranscodeError, url string, err error) {
		clog.Errorf(ctx, "%v error with segment nonce=%d seqNo=%d: %v (URL: %v)", subType, nonce, seg.SeqNo, err, url)
		if monitor.Enabled && !gotErr {
			monitor.SegmentTranscodeFailed(ctx, subType, nonce, seg.SeqNo, err, false)
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
			d, err := downloadSeg(ctx, url)
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
				uri, err := drivers.SaveRetried(ctx, bros, name, data, map[string]string{"duration": segDurMs}, 2)
				took := time.Since(now)
				if err != nil {
					clog.Errorf(ctx, "Error saving nonce=%d manifestID=%s name=%s to record store err=%q", nonce, cxn.mid, name, err)
				} else {
					cpl.InsertHLSSegmentJSON(&profile, seg.SeqNo, uri, seg.Duration)
					clog.Infof(ctx, "Successfully saved nonce=%d manifestID=%s name=%s size=%d bytes to record store took=%s",
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
			newURL, err := bos.SaveData(ctx, name, data, nil, 0)
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
			monitor.TranscodedSegmentAppeared(ctx, nonce, seg.SeqNo, profile.Name, bros != nil)
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
		monitor.SegmentDownloaded(ctx, nonce, seg.SeqNo, downloadDur)
	}

	if verifier != nil {
		// verify potentially can change content of segURLs
		err := verify(verifier, cxn, sess, seg, res.TranscodeData, segURLs, segData)
		if err != nil {
			clog.Errorf(ctx, "Error verifying nonce=%d manifestID=%s seqNo=%d err=%q", nonce, cxn.mid, seg.SeqNo, err)
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
			clog.Errorf(ctx, "Playlist insertion error nonce=%d manifestID=%s seqNo=%d err=%q", nonce, cxn.mid, seg.SeqNo, err)
			if monitor.Enabled {
				monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorDuplicateSegment, nonce, seg.SeqNo, err, false)
			}
		}
	}

	if monitor.Enabled {
		monitor.SegmentFullyTranscoded(nonce, seg.SeqNo, common.ProfilesNames(sess.Params.Profiles), errCode)
	}

	clog.V(common.DEBUG).Infof(ctx, "Successfully validated segment")
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
				_, err := sess.BroadcasterOS.SaveData(context.TODO(), uri, data, nil, 0)
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

func refreshSession(ctx context.Context, sess *BroadcastSession) error {
	uri, err := url.Parse(sess.Transcoder())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
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

func shouldRefreshSession(ctx context.Context, sess *BroadcastSession) (bool, error) {
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
		clog.V(common.VERBOSE).Infof(ctx, "Auth token expired, refreshing for orch=%v", OrchestratorInfo.Transcoder)

		return true, nil
	}

	if sess.Sender != nil {
		if err := sess.Sender.ValidateTicketParams(pmTicketParams(OrchestratorInfo.TicketParams)); err != nil {
			if err != pm.ErrTicketParamsExpired {
				return false, err
			}

			clog.V(common.VERBOSE).Infof(ctx, "Ticket params expired, refreshing for orch=%v", OrchestratorInfo.Transcoder)

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
	if strings.HasPrefix(err.Error(), "Hit max transcode attempts:") {
		return true
	}
	return false
}
