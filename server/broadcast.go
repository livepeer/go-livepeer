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
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

var minWorkingSetRatio = 0.1

var refreshTimeout = 2500 * time.Millisecond
var maxDurationSec = common.MaxDuration.Seconds()

var Policy *verification.Policy
var BroadcastCfg = &BroadcastConfig{}
var MaxAttempts = 3

var MetadataQueue event.Producer
var MetadataPublishTimeout = 1 * time.Second

var getOrchestratorInfoRPC = GetOrchestratorInfo
var downloadSeg = drivers.GetSegmentData

const MIN_SIZE = 1

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

type BroadcastSessionsManager struct {
	// Accessing or changing any of the below requires ownership of this mutex
	sessLock *sync.Mutex

	mid      core.ManifestID
	sel      BroadcastSessionsSelector
	sessMap  map[string]*BroadcastSession
	lastSess *BroadcastSession
	numOrchs int // how many orchs to request at once
	poolSize int

	refreshing bool // only allow one refresh in-flight
	finished   bool // set at stream end

	createSessions func() ([]*BroadcastSession, error)
	sus            *suspender
}

func (bsm *BroadcastSessionsManager) selectSession() *BroadcastSession {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	checkSessions := func(m *BroadcastSessionsManager) bool {
		numSess := m.sel.Size()
		if numSess < int(math.Ceil(float64(m.numOrchs)/5.0)) {
			go m.refreshSessions()
		}
		return (numSess > 0 || bsm.lastSess != nil)
	}
	for checkSessions(bsm) {
		var sess *BroadcastSession

		if bsm.lastSess != nil && len(bsm.lastSess.SegsInFlight) > 0 &&
			time.Since(bsm.lastSess.SegsInFlight[0].startTime) < bsm.lastSess.SegsInFlight[0].segDur {
			// Re-use last session if oldest segment is in-flight for < segDur
			sess = bsm.lastSess
		} else {
			// Or try a new session from the available ones
			sess = bsm.sel.Select()
		}

		// If no new sessions are available, re-use last session when oldest segment is in-flight for < 2 * segDur
		if sess == nil && bsm.lastSess != nil && len(bsm.lastSess.SegsInFlight) > 0 &&
			time.Since(bsm.lastSess.SegsInFlight[0].startTime) < 2*bsm.lastSess.SegsInFlight[0].segDur {
			glog.V(common.DEBUG).Infof("No sessions in the selector for manifestID=%v re-using orch=%v with acceptable in-flight time", bsm.mid, bsm.lastSess.OrchestratorInfo.Transcoder)
			sess = bsm.lastSess
		}

		// No session found, return nil
		if sess == nil {
			if bsm.lastSess != nil {
				bsm.lastSess.SegsInFlight = nil
				bsm.lastSess = nil
			}
			return nil
		}

		/*
		   Don't select sessions no longer in the map.

		   Retry if the first selected session has been removed from the map.
		   This may occur if the session is removed while still in the list.
		   To avoid a runtime search of the session list under lock, simply
		   fixup the session list at selection time by retrying the selection.
		*/
		if _, ok := bsm.sessMap[sess.OrchestratorInfo.Transcoder]; ok {
			if bsm.lastSess != nil && bsm.lastSess.OrchestratorInfo.Transcoder != sess.OrchestratorInfo.Transcoder {
				glog.V(common.DEBUG).Infof("Swapping from orch=%v to orch=%v for manifestID=%s", bsm.lastSess.OrchestratorInfo.Transcoder, sess.OrchestratorInfo.Transcoder, bsm.mid)
				if monitor.Enabled {
					monitor.OrchestratorSwapped()
				}
			}
			bsm.lastSess = sess
			return sess
		}

		// Last session got removed from map (possibly due to a failure) so stop tracking its in-flight segments
		if bsm.lastSess != nil && sess.OrchestratorInfo.Transcoder == bsm.lastSess.OrchestratorInfo.Transcoder {
			glog.V(common.DEBUG).Infof("Removing orch=%v from manifestID=%s session list", bsm.lastSess.OrchestratorInfo.Transcoder, bsm.mid)
			if monitor.Enabled {
				monitor.OrchestratorSwapped()
			}
			bsm.lastSess.SegsInFlight = nil
			bsm.lastSess = nil
		}
	}
	// No session found, return nil
	if bsm.lastSess != nil {
		bsm.lastSess.SegsInFlight = nil
		bsm.lastSess = nil
	}
	return nil
}

func (bsm *BroadcastSessionsManager) removeSession(session *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	delete(bsm.sessMap, session.OrchestratorInfo.Transcoder)
}

func (bsm *BroadcastSessionsManager) completeSession(sess *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	if existingSess, ok := bsm.sessMap[sess.OrchestratorInfo.Transcoder]; ok {
		// If the new session and the existing session share the same key in sessMap replace
		// the existing session with the new session
		if existingSess != sess {
			bsm.sessMap[sess.OrchestratorInfo.Transcoder] = sess
		}
		if bsm.lastSess != nil && bsm.lastSess.OrchestratorInfo.Transcoder == sess.OrchestratorInfo.Transcoder && sess != bsm.lastSess {
			sess.SegsInFlight = bsm.lastSess.SegsInFlight
			bsm.lastSess = sess
		}
		if len(sess.SegsInFlight) == 1 {
			sess.SegsInFlight = nil
		} else if len(sess.SegsInFlight) > 1 {
			sess.SegsInFlight = sess.SegsInFlight[1:]
			// skip returning this session back to the selector
			// we will return it later in transcodeSegment() once all in-flight segs downloaded
			return
		}
		bsm.sel.Complete(sess)
	}
}

func (bsm *BroadcastSessionsManager) refreshSessions() {

	started := time.Now()
	glog.V(common.DEBUG).Info("Starting session refresh manifestID=", bsm.mid)
	defer glog.V(common.DEBUG).Infof("Ending session refresh manifestID=%s dur=%s", bsm.mid, time.Since(started))
	bsm.sessLock.Lock()
	if bsm.finished || bsm.refreshing {
		bsm.sessLock.Unlock()
		return
	}
	bsm.refreshing = true
	bsm.sessLock.Unlock()

	bsm.sus.signalRefresh()

	newBroadcastSessions, err := bsm.createSessions()
	if err != nil {
		bsm.sessLock.Lock()
		bsm.refreshing = false
		bsm.sessLock.Unlock()
		return
	}

	// if newBroadcastSessions is empty, exit without refreshing list
	if len(newBroadcastSessions) <= 0 {
		bsm.sessLock.Lock()
		bsm.refreshing = false
		bsm.sessLock.Unlock()
		return
	}

	uniqueSessions := make([]*BroadcastSession, 0, len(newBroadcastSessions))
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	bsm.refreshing = false
	if bsm.finished {
		return
	}

	for _, sess := range newBroadcastSessions {
		if _, ok := bsm.sessMap[sess.OrchestratorInfo.Transcoder]; ok {
			continue
		}
		uniqueSessions = append(uniqueSessions, sess)
		bsm.sessMap[sess.OrchestratorInfo.Transcoder] = sess
	}

	bsm.sel.Add(uniqueSessions)
}

func (bsm *BroadcastSessionsManager) pushSegInFlight(sess *BroadcastSession, seg *stream.HLSSegment) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	sess.SegsInFlight = append(sess.SegsInFlight,
		SegFlightMetadata{
			startTime: time.Now(),
			segDur:    time.Duration(seg.Duration * float64(time.Second)),
		})
}

func (bsm *BroadcastSessionsManager) updateLastSession(oldSess, newSess *BroadcastSession) {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	if bsm.lastSess == oldSess {
		bsm.lastSess = newSess
	}
}

func (bsm *BroadcastSessionsManager) cleanup() {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.finished = true
	bsm.lastSess = nil
	bsm.sel.Clear()
	bsm.sessMap = make(map[string]*BroadcastSession) // prevent segfaults
}

func (bsm *BroadcastSessionsManager) suspendOrch(sess *BroadcastSession) {
	poolSize := math.Max(MIN_SIZE, float64(bsm.poolSize))
	selSize := math.Max(MIN_SIZE, float64(bsm.sel.Size()))
	penalty := int(math.Ceil(poolSize / selSize))
	bsm.sus.suspend(sess.OrchestratorInfo.GetTranscoder(), penalty)
}

func NewSessionManager(node *core.LivepeerNode, params *core.StreamParameters, sel BroadcastSessionsSelector) *BroadcastSessionsManager {
	var poolSize int
	if node.OrchestratorPool != nil {
		poolSize = node.OrchestratorPool.Size()
	}

	sus := newSuspender()
	bsm := &BroadcastSessionsManager{
		mid:     params.ManifestID,
		sel:     sel,
		sessMap: make(map[string]*BroadcastSession),
		createSessions: func() ([]*BroadcastSession, error) {
			return selectOrchestrator(node, params, poolSize, sus)
		},
		sessLock: &sync.Mutex{},
		numOrchs: poolSize,
		poolSize: poolSize,
		sus:      sus,
	}
	bsm.refreshSessions()
	glog.Infof("Created new broadcast session manager with pool size %v", poolSize)
	return bsm
}

func selectOrchestrator(n *core.LivepeerNode, params *core.StreamParameters, count int, sus *suspender) ([]*BroadcastSession, error) {
	if n.OrchestratorPool == nil {
		glog.Info("No orchestrators specified; not transcoding")
		return nil, errDiscovery
	}

	tinfos, err := n.OrchestratorPool.GetOrchestrators(count, sus, params.Capabilities)
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
			Broadcaster:      core.NewBroadcaster(n),
			Params:           params,
			OrchestratorInfo: tinfo,
			OrchestratorOS:   orchOS,
			BroadcasterOS:    bcastOS,
			Sender:           n.Sender,
			PMSessionID:      sessionID,
			Balances:         n.Balances,
			Balance:          balance,
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
		var info data.TranscodeAttemptInfo
		urls, info, err = transcodeSegment(cxn, seg, name, sv)
		attempts = append(attempts, info)
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
	verifier *verification.SegmentVerifier) (urls []string, info data.TranscodeAttemptInfo, err error) {
	defer func(startTime time.Time) {
		info.LatencyMs = time.Since(startTime).Milliseconds()
		if err != nil {
			errStr := err.Error()
			info.Error = &errStr
		}
	}(time.Now())

	nonce := cxn.nonce
	cpl := cxn.pl
	sess := cxn.sessManager.selectSession()
	// Return early under a few circumstances:
	// View-only (non-transcoded) streams or no sessions available
	if sess == nil {
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
		TranscoderUri: sess.OrchestratorInfo.Transcoder,
		Address:       hexutil.Encode(sess.OrchestratorInfo.Address),
	}

	glog.Infof("Trying to transcode segment manifestID=%v nonce=%d seqNo=%d", cxn.mid, nonce, seg.SeqNo)
	if monitor.Enabled {
		monitor.TranscodeTry(nonce, seg.SeqNo)
	}

	// storage the orchestrator prefers
	if ios := sess.OrchestratorOS; ios != nil {
		// XXX handle case when orch expects direct upload
		uri, err := ios.SaveData(name, seg.Data, nil, 0)
		if err != nil {
			glog.Errorf("Error saving segment to OS manifestID=%v nonce=%d seqNo=%d err=%v", cxn.mid, nonce, seg.SeqNo, err)
			if monitor.Enabled {
				monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorOS, err, false)
			}
			cxn.sessManager.suspendOrch(sess)
			cxn.sessManager.removeSession(sess)
			return nil, info, err
		}
		seg.Name = uri // hijack seg.Name to convey the uploaded URI
	}

	refresh, err := shouldRefreshSession(sess)
	if err != nil {
		glog.Errorf("Error checking whether to refresh session manifestID=%s orch=%v err=%v", cxn.mid, sess.OrchestratorInfo.Transcoder, err)
		cxn.sessManager.suspendOrch(sess)
		cxn.sessManager.removeSession(sess)
		return nil, info, err
	}

	if refresh {
		newSess, err := refreshSession(sess)
		if err != nil {
			glog.Errorf("Error refreshing session manifestID=%s orch=%v err=%v", cxn.mid, sess.OrchestratorInfo.Transcoder, err)
			cxn.sessManager.suspendOrch(sess)
			cxn.sessManager.removeSession(sess)
			return nil, info, err
		}
		// if sess was lastSess, we need to update lastSess,
		// or else content of SegsInFlight will be lost
		cxn.sessManager.updateLastSession(sess, newSess)
		sess = newSess
	}

	cxn.sessManager.pushSegInFlight(sess, seg)
	res, err := SubmitSegment(sess, seg, nonce)
	if err != nil || res == nil {
		if isNonRetryableError(err) {
			cxn.sessManager.completeSession(sess)
			return nil, info, err
		}
		cxn.sessManager.suspendOrch(sess)
		cxn.sessManager.removeSession(sess)
		if res == nil && err == nil {
			err = errors.New("empty response")
		}
		return nil, info, err
	}

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

	// [EXPERIMENTAL] send content detection results to callback webhook
	if DetectionWebhookURL != nil && len(res.Detections) > 0 {
		glog.V(common.DEBUG).Infof("Got detection result %v", res.Detections)
		go func(mid core.ManifestID, config core.DetectionConfig, seqNo uint64, detections []*net.DetectData) {
			type SceneClassificationResult struct {
				Name        string  `json:"name"`
				Probability float64 `json:"probability"`
			}
			type DetectionWebhookRequest struct {
				ManifestID          core.ManifestID             `json:"manifestID"`
				SeqNo               uint64                      `json:"seqNo"`
				SceneClassification []SceneClassificationResult `json:"sceneClassification"`
			}
			req := DetectionWebhookRequest{ManifestID: mid, SeqNo: seqNo}
			for _, detection := range detections {
				switch x := detection.Value.(type) {
				case *net.DetectData_SceneClassification:
					probs := x.SceneClassification.ClassProbs
					// match returned probs (key: class id) with one of the user-selected class names
					for _, name := range config.SelectedClassNames {
						if id, ok := ffmpeg.DetectorClassIDLookup[name]; ok {
							if prob, ok := probs[uint32(id)]; ok {
								req.SceneClassification = append(req.SceneClassification,
									SceneClassificationResult{
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
				glog.Errorf("Unable to marshal detection result into JSON manifestID=%v seqNo=%v", mid, seqNo)
				return
			}
			resp, err := DetectionWhClient.Post(DetectionWebhookURL.String(), "application/json", bytes.NewBuffer(jsonValue))
			if err != nil {
				glog.Errorf("Unable to POST detection result on webhook url=%v manifestID=%v seqNo=%v err=%v",
					DetectionWebhookURL.Redacted(), mid, seqNo, err)
			} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				rbody, rerr := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if rerr != nil {
					glog.Errorf("Detection webhook returned error status=%v manifestID=%v seqNo=%v with unreadable body err=%v",
						resp.StatusCode, mid, seqNo, rerr)
				} else {
					glog.Errorf("Detection webhook returned error status=%v err=%v manifestID=%v seqNo=%v",
						resp.StatusCode, string(rbody), mid, seqNo)
				}
			}
		}(cxn.mid, cxn.params.Detection, seg.SeqNo, res.Detections)
	}

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
				cxn.sessManager.suspendOrch(sess)
				cxn.sessManager.removeSession(sess)
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
		return nil, info, dlErr
	}

	cxn.sessManager.completeSession(updateSession(sess, res))

	downloadDur := time.Since(dlStart)
	if monitor.Enabled {
		monitor.SegmentDownloaded(nonce, seg.SeqNo, downloadDur)
	}

	if verifier != nil {
		// verify potentially can change content of segURLs
		err := verify(verifier, cxn, sess, seg, res.TranscodeData, segURLs, segData)
		if err != nil {
			glog.Errorf("Error verifying nonce=%d manifestID=%s seqNo=%d err=%s", nonce, cxn.mid, seg.SeqNo, err)
			return nil, info, err
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
	return segURLs, info, nil
}

var sessionErrStrings = []string{"dial tcp", "unexpected EOF", core.ErrOrchBusy.Error(), core.ErrOrchCap.Error()}

var sessionErrRegex = common.GenErrRegex(sessionErrStrings)

func shouldStopSession(err error) bool {
	return sessionErrRegex.MatchString(err.Error())
}

func verify(verifier *verification.SegmentVerifier, cxn *rtmpConnection,
	sess *BroadcastSession, source *stream.HLSSegment,
	res *net.TranscodeData, URIs []string, segData [][]byte) error {

	// Cache segment contents in params.Renditions
	// If we need to retry transcoding because verification fails,
	// the the segments' OS location will be overwritten.
	// Cache the segments so we can restore them in OS if necessary.
	params := &verification.Params{
		ManifestID:   sess.Params.ManifestID,
		Source:       source,
		Profiles:     sess.Params.Profiles,
		Orchestrator: sess.OrchestratorInfo,
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
func updateSession(sess *BroadcastSession, res *ReceivedTranscodeResult) *BroadcastSession {
	// Instead of mutating the existing session we copy it and return an updated copy
	newSess := &BroadcastSession{}
	*newSess = *sess
	newSess.LatencyScore = res.LatencyScore

	if res.Info == nil {
		// Return newSess early if we do not need to update OrchestratorInfo
		return newSess
	}

	oInfo := res.Info
	newSess.OrchestratorInfo = oInfo

	if len(oInfo.Storage) > 0 {
		newSess.OrchestratorOS = drivers.NewSession(oInfo.Storage[0])
	}

	if newSess.Sender != nil && oInfo.TicketParams != nil {
		// Note: We do not validate the ticket params included in the OrchestratorInfo
		// message here. Instead, we store the ticket params with the current BroadcastSession
		// and the next time this BroadcastSession is used, the ticket params will be validated
		// during ticket creation in genPayment(). If ticket params validation during ticket
		// creation fails, then this BroadcastSession will be removed
		newSess.PMSessionID = newSess.Sender.StartSession(*pmTicketParams(oInfo.TicketParams))

		// Session ID changed so we need to make sure the balance tracks the new session ID
		if sess.OrchestratorInfo.AuthToken.SessionId != oInfo.AuthToken.SessionId {
			newSess.Balance = core.NewBalance(ethcommon.BytesToAddress(newSess.OrchestratorInfo.TicketParams.Recipient), core.ManifestID(newSess.OrchestratorInfo.AuthToken.SessionId), sess.Balances)
		}
	}

	return newSess
}

func refreshSession(sess *BroadcastSession) (*BroadcastSession, error) {
	uri, err := url.Parse(sess.OrchestratorInfo.Transcoder)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	oInfo, err := getOrchestratorInfoRPC(ctx, sess.Broadcaster, uri)
	if err != nil {
		return nil, err
	}

	// Create dummy result
	res := &ReceivedTranscodeResult{
		LatencyScore: sess.LatencyScore,
		Info:         oInfo,
	}

	return updateSession(sess, res), nil
}

func shouldRefreshSession(sess *BroadcastSession) (bool, error) {
	if sess.OrchestratorInfo.AuthToken == nil {
		return false, errors.New("missing auth token")
	}

	// Refresh auth token if we are within the last 10% of the token's valid period
	authTokenExpireBuffer := 0.1
	refreshPoint := sess.OrchestratorInfo.AuthToken.Expiration - int64(authTokenValidPeriod.Seconds()*authTokenExpireBuffer)
	if time.Now().After(time.Unix(refreshPoint, 0)) {
		glog.V(common.VERBOSE).Infof("Auth token expired, refreshing for orch=%v", sess.OrchestratorInfo.Transcoder)

		return true, nil
	}

	if sess.Sender != nil {
		if err := sess.Sender.ValidateTicketParams(pmTicketParams(sess.OrchestratorInfo.TicketParams)); err != nil {
			if err != pm.ErrTicketParamsExpired {
				return false, err
			}

			glog.V(common.VERBOSE).Infof("Ticket params expired, refreshing for orch=%v", sess.OrchestratorInfo.Transcoder)

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

func MinWorkingSetSize(numOrchs int) int {
	return int(math.Ceil(float64(numOrchs) * minWorkingSetRatio))
}
