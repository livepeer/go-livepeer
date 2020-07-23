package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/verification"

	"github.com/livepeer/lpms/stream"
)

var refreshTimeout = 2500 * time.Millisecond
var maxDuration = (5 * time.Minute)
var maxDurationSec = maxDuration.Seconds()

var Policy *verification.Policy
var BroadcastCfg = &BroadcastConfig{}
var MaxAttempts = 3

var getOrchestratorInfoRPC = GetOrchestratorInfo
var downloadSeg = drivers.GetSegmentData

type BroadcastConfig struct {
	maxPrice *big.Rat
	mu       sync.RWMutex
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
}

type BroadcastSessionsManager struct {
	// Accessing or changing any of the below requires ownership of this mutex
	sessLock *sync.Mutex

	mid      core.ManifestID
	sel      BroadcastSessionsSelector
	sessMap  map[string]*BroadcastSession
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
		if numSess < int(math.Ceil(float64(m.numOrchs)/2.0)) {
			go m.refreshSessions()
		}
		return numSess > 0
	}
	for checkSessions(bsm) {
		sess := bsm.sel.Select()
		// Handle the case where there is an error during selection and Select() returns nil when session list length > 0
		if sess == nil {
			return nil
		}

		if _, ok := bsm.sessMap[sess.OrchestratorInfo.Transcoder]; ok {
			return sess
		}
		/*
		   Don't select sessions no longer in the map.

		   Retry if the first selected session has been removed from the map.
		   This may occur if the session is removed while still in the list.
		   To avoid a runtime search of the session list under lock, simply
		   fixup the session list at selection time by retrying the selection.
		*/
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

func (bsm *BroadcastSessionsManager) cleanup() {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.finished = true
	bsm.sel.Clear()
	bsm.sessMap = make(map[string]*BroadcastSession) // prevent segfaults
}

func (bsm *BroadcastSessionsManager) suspendOrch(sess *BroadcastSession) {
	bsm.sus.suspend(sess.OrchestratorInfo.GetTranscoder(), bsm.poolSize/bsm.numOrchs)
}

func NewSessionManager(node *core.LivepeerNode, params *core.StreamParameters, sel BroadcastSessionsSelector) *BroadcastSessionsManager {
	var poolSize float64
	if node.OrchestratorPool != nil {
		poolSize = float64(node.OrchestratorPool.Size())
	}
	maxInflight := common.HTTPTimeout.Seconds() / SegLen.Seconds()
	numOrchs := int(math.Min(poolSize, maxInflight*2))
	sus := newSuspender()
	bsm := &BroadcastSessionsManager{
		mid:     params.ManifestID,
		sel:     sel,
		sessMap: make(map[string]*BroadcastSession),
		createSessions: func() ([]*BroadcastSession, error) {
			return selectOrchestrator(node, params, numOrchs, sus)
		},
		sessLock: &sync.Mutex{},
		numOrchs: numOrchs,
		poolSize: int(poolSize),
		sus:      sus,
	}
	bsm.refreshSessions()
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

		if n.Sender != nil {
			ticketParams = pmTicketParams(tinfo.TicketParams)
			sessionID = n.Sender.StartSession(*ticketParams)
		}

		if n.Balances != nil {
			balance = core.NewBalance(ticketParams.Recipient, params.ManifestID, n.Balances)
		}

		var orchOS drivers.OSSession
		if len(tinfo.Storage) > 0 {
			orchOS = drivers.NewSession(tinfo.Storage[0])
		}

		bcastOS := params.OS
		if bcastOS.IsExternal() {
			// Give each O its own OS session to prevent front running uploads
			pfx := fmt.Sprintf("%v/%v", params.ManifestID, core.RandomManifestID())
			bcastOS = drivers.NodeStorage.NewSession(pfx)
		}

		session := &BroadcastSession{
			Broadcaster:      core.NewBroadcaster(n),
			Params:           params,
			OrchestratorInfo: tinfo,
			OrchestratorOS:   orchOS,
			BroadcasterOS:    bcastOS,
			Sender:           n.Sender,
			PMSessionID:      sessionID,
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

	glog.V(common.DEBUG).Infof("Processing segment nonce=%d manifestID=%s seqNo=%d dur=%v", nonce, mid, seg.SeqNo, seg.Duration)
	if monitor.Enabled {
		monitor.SegmentEmerged(nonce, seg.SeqNo, len(BroadcastJobVideoProfiles))
	}

	seg.Name = "" // hijack seg.Name to convey the uploaded URI
	ext, err := common.ProfileFormatExtension(vProfile.Format)
	if err != nil {
		glog.Errorf("Unknown format extension manifestID=%s seqNo=%d err=%s", mid, seg.SeqNo, err)
		return nil, err
	}
	name := fmt.Sprintf("%s/%d%s", vProfile.Name, seg.SeqNo, ext)
	uri, err := cpl.GetOSSession().SaveData(name, seg.Data)
	if err != nil {
		glog.Errorf("Error saving segment nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err.Error(), true)
		}
		return nil, err
	}
	if cpl.GetOSSession().IsExternal() {
		seg.Name = uri // hijack seg.Name to convey the uploaded URI
	}
	err = cpl.InsertHLSSegment(vProfile, seg.SeqNo, uri, seg.Duration)
	if monitor.Enabled {
		monitor.SourceSegmentAppeared(nonce, seg.SeqNo, string(mid), vProfile.Name)
	}
	if err != nil {
		glog.Errorf("Error inserting segment nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err.Error(), true)
		}
	}

	var sv *verification.SegmentVerifier
	if Policy != nil {
		sv = verification.NewSegmentVerifier(Policy)
	}

	for i := 0; i < MaxAttempts; i++ {
		// if fails, retry; rudimentary
		var urls []string
		if urls, err = transcodeSegment(cxn, seg, name, sv); err == nil {
			return urls, nil
		}

		if shouldStopStream(err) {
			glog.Warningf("Stopping current stream due to: %v", err)
			rtmpStrm.Close()
			return nil, err
		}

		// recoverable error, retry
	}
	if err != nil {
		err = fmt.Errorf("Hit max transcode attempts: %w", err)
	}
	return nil, err
}

func transcodeSegment(cxn *rtmpConnection, seg *stream.HLSSegment, name string,
	verifier *verification.SegmentVerifier) ([]string, error) {

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
		return nil, nil
	}

	glog.Infof("Trying to transcode segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
	if monitor.Enabled {
		monitor.TranscodeTry(nonce, seg.SeqNo)
	}

	// storage the orchestrator prefers
	if ios := sess.OrchestratorOS; ios != nil {
		// XXX handle case when orch expects direct upload
		uri, err := ios.SaveData(name, seg.Data)
		if err != nil {
			glog.Errorf("Error saving segment to OS nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
			if monitor.Enabled {
				monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorOS, err.Error(), false)
			}
			cxn.sessManager.suspendOrch(sess)
			cxn.sessManager.removeSession(sess)
			return nil, err
		}
		seg.Name = uri // hijack seg.Name to convey the uploaded URI
	}

	// send segment to the orchestrator
	if sess.Sender != nil {
		if err := sess.Sender.ValidateTicketParams(pmTicketParams(sess.OrchestratorInfo.TicketParams)); err != nil {
			if err != pm.ErrTicketParamsExpired {
				glog.Error("Invalid ticket params err=", err)
				cxn.sessManager.suspendOrch(sess)
				cxn.sessManager.removeSession(sess)
				return nil, err
			}

			glog.V(common.VERBOSE).Infof("Ticket params expired, refreshing for orch=%v", sess.OrchestratorInfo.Transcoder)
			newSess, err := refreshSession(sess)
			if err != nil {
				cxn.sessManager.suspendOrch(sess)
				cxn.sessManager.removeSession(sess)
				return nil, fmt.Errorf("unable to refresh ticket params for orch=%v err=%v", sess.OrchestratorInfo.Transcoder, err)
			}
			sess = newSess
		}
	}
	res, err := SubmitSegment(sess, seg, nonce)
	if err != nil || res == nil {
		cxn.sessManager.suspendOrch(sess)
		cxn.sessManager.removeSession(sess)
		if res == nil && err == nil {
			err = errors.New("empty response")
		}
		return nil, err
	}

	cxn.sessManager.completeSession(updateSession(sess, res))

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

	var dlErr error
	segData := make([][]byte, len(res.Segments))
	n := len(res.Segments)
	segURLs := make([]string, len(res.Segments))
	segLock := &sync.Mutex{}
	cond := sync.NewCond(segLock)

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

		var data []byte
		// Download segment data in the following cases:
		// - A verification policy is set. The segment data is needed for signature verification and/or pixel count verification
		// - The segment data needs to be uploaded to the broadcaster's own OS
		if verifier != nil || (bos != nil && !bos.IsOwn(url)) {
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
		}

		if bos != nil && !bos.IsOwn(url) {
			ext, err := common.ProfileFormatExtension(profile.Format)
			if err != nil {
				errFunc(monitor.SegmentTranscodeErrorSaveData, url, err)
				return
			}
			name := fmt.Sprintf("%s/%d%s", profile.Name, seg.SeqNo, ext)
			newURL, err := bos.SaveData(name, data)
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
			monitor.TranscodedSegmentAppeared(nonce, seg.SeqNo, profile.Name)
		}
	}

	for i, v := range res.Segments {
		go dlFunc(v.Url, v.Pixels, i)
	}

	cond.L.Lock()
	for n != 0 {
		cond.Wait()
	}
	cond.L.Unlock()
	if dlErr != nil {
		return nil, dlErr
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
				monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorPlaylist, nonce, seg.SeqNo, err, false)
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
				_, err := sess.BroadcasterOS.SaveData(uri, data)
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
