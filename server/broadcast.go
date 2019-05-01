package server

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/pm"

	"github.com/livepeer/lpms/stream"
)

type BroadcastSessionsManager struct {
	// Accessing or changing any of the below requires ownership of this mutex
	sessLock *sync.Mutex

	sessList []*BroadcastSession
	sessMap  map[string]*BroadcastSession
	numOrchs int // how many orchs to request at once

	refreshing bool // only allow one refresh in-flight
	finished   bool // set at stream end

	createSessions func() ([]*BroadcastSession, error)
}

func (bsm *BroadcastSessionsManager) selectSession() *BroadcastSession {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()

	checkSessions := func(m *BroadcastSessionsManager) bool {
		numSess := len(m.sessList)
		if numSess < int(math.Ceil(float64(m.numOrchs)/2.0)) {
			go m.refreshSessions()
		}
		return numSess > 0
	}
	for checkSessions(bsm) {
		last := len(bsm.sessList) - 1
		sess, sessions := bsm.sessList[last], bsm.sessList[:last]
		bsm.sessList = sessions
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

	if _, ok := bsm.sessMap[sess.OrchestratorInfo.Transcoder]; ok {
		bsm.sessList = append(bsm.sessList, sess)
	}
}

func (bsm *BroadcastSessionsManager) refreshSessions() {

	glog.V(common.DEBUG).Info("Starting session refresh")
	defer glog.V(common.DEBUG).Info("Ending session refresh")
	bsm.sessLock.Lock()
	if bsm.finished || bsm.refreshing {
		bsm.sessLock.Unlock()
		return
	}
	bsm.refreshing = true
	bsm.sessLock.Unlock()

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
	bsm.sessList = append(uniqueSessions, bsm.sessList...)
}

func (bsm *BroadcastSessionsManager) cleanup() {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.finished = true
	bsm.sessList = nil
	bsm.sessMap = make(map[string]*BroadcastSession) // prevent segfaults
}

func NewSessionManager(node *core.LivepeerNode, pl core.PlaylistManager) *BroadcastSessionsManager {
	var poolSize float64
	if node.OrchestratorPool != nil {
		poolSize = float64(node.OrchestratorPool.Size())
	}
	maxInflight := HTTPTimeout.Seconds() / SegLen.Seconds()
	numOrchs := int(math.Min(poolSize, maxInflight*2))
	bsm := &BroadcastSessionsManager{
		sessMap:        make(map[string]*BroadcastSession),
		createSessions: func() ([]*BroadcastSession, error) { return selectOrchestrator(node, pl, numOrchs) },
		sessLock:       &sync.Mutex{},
		numOrchs:       numOrchs,
	}
	bsm.refreshSessions()
	return bsm
}

func selectOrchestrator(n *core.LivepeerNode, cpl core.PlaylistManager, count int) ([]*BroadcastSession, error) {
	if n.OrchestratorPool == nil {
		glog.Info("No orchestrators specified; not transcoding")
		return nil, ErrDiscovery
	}

	rpcBcast := core.NewBroadcaster(n)

	tinfos, err := n.OrchestratorPool.GetOrchestrators(count)
	if len(tinfos) <= 0 {
		glog.Info("No orchestrators found; not transcoding. Error: ", err)
		return nil, ErrNoOrchs
	}
	if err != nil {
		return nil, err
	}

	var sessions []*BroadcastSession

	for _, tinfo := range tinfos {
		var sessionID string

		if n.Sender != nil {
			protoParams := tinfo.TicketParams
			params := pm.TicketParams{
				Recipient:         ethcommon.BytesToAddress(protoParams.Recipient),
				FaceValue:         new(big.Int).SetBytes(protoParams.FaceValue),
				WinProb:           new(big.Int).SetBytes(protoParams.WinProb),
				RecipientRandHash: ethcommon.BytesToHash(protoParams.RecipientRandHash),
				Seed:              new(big.Int).SetBytes(protoParams.Seed),
			}

			sessionID = n.Sender.StartSession(params)
		}

		var orchOS drivers.OSSession
		if len(tinfo.Storage) > 0 {
			orchOS = drivers.NewSession(tinfo.Storage[0])
		}

		bcastOS := cpl.GetOSSession()
		if bcastOS.IsExternal() {
			// Give each O its own OS session to prevent front running uploads
			pfx := fmt.Sprintf("%v/%v", cpl.ManifestID(), core.RandomManifestID())
			bcastOS = drivers.NodeStorage.NewSession(pfx)
		}

		session := &BroadcastSession{
			Broadcaster:      rpcBcast,
			ManifestID:       cpl.ManifestID(),
			Profiles:         BroadcastJobVideoProfiles,
			OrchestratorInfo: tinfo,
			OrchestratorOS:   orchOS,
			BroadcasterOS:    bcastOS,
			Sender:           n.Sender,
			PMSessionID:      sessionID,
		}

		sessions = append(sessions, session)
	}
	return sessions, nil
}

func processSegment(cxn *rtmpConnection, seg *stream.HLSSegment) {

	nonce := cxn.nonce
	cpl := cxn.pl
	mid := cxn.mid
	vProfile := cxn.profile

	glog.V(common.DEBUG).Infof("Processing segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
	if monitor.Enabled {
		monitor.SegmentEmerged(nonce, seg.SeqNo, len(BroadcastJobVideoProfiles))
	}

	seg.Name = "" // hijack seg.Name to convey the uploaded URI
	name := fmt.Sprintf("%s/%d.ts", vProfile.Name, seg.SeqNo)
	uri, err := cpl.GetOSSession().SaveData(name, seg.Data)
	if err != nil {
		glog.Errorf("Error saving segment nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err.Error(), true)
		}
		return
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

	// Process the rest of the segment asynchronously - transcode
	go func() {
		for true {
			// if fails, retry; rudimentary
			if err := transcodeSegment(cxn, seg, name); err == nil {
				return
			}
		}
	}()
}

func transcodeSegment(cxn *rtmpConnection, seg *stream.HLSSegment, name string) error {

	nonce := cxn.nonce
	rtmpStrm := cxn.stream
	cpl := cxn.pl
	sess := cxn.sessManager.selectSession()
	// Return early under a few circumstances:
	// View-only (non-transcoded) streams or no sessions available
	if sess == nil {
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorNoOrchestrators, nonce, seg.SeqNo, ErrNoOrchs, true)
		}
		glog.Infof("No sessions available for segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
		return nil
	}
	{
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
				cxn.sessManager.removeSession(sess)
				return err
			}
			seg.Name = uri // hijack seg.Name to convey the uploaded URI
		}

		// send segment to the orchestrator
		glog.V(common.DEBUG).Infof("Submitting segment nonce=%d seqNo=%d orch=%s", nonce, seg.SeqNo, sess.OrchestratorInfo.Transcoder)

		res, err := SubmitSegment(sess, seg, nonce)
		if err != nil || res == nil {
			cxn.sessManager.removeSession(sess)
			if res == nil && err == nil {
				return errors.New("Empty response")
			}
			if shouldStopStream(err) {
				glog.Warningf("Stopping current stream due to: %v", err)
				rtmpStrm.Close()
				return err
			}
			if shouldStopSession(err) {
			}
			return err
		}

		cxn.sessManager.completeSession(sess)

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

		var dlErr, saveErr error
		segHashes := make([][]byte, len(res.Segments))
		n := len(res.Segments)
		segHashLock := &sync.Mutex{}
		cond := sync.NewCond(segHashLock)

		dlFunc := func(url string, i int) {
			defer func() {
				cond.L.Lock()
				n--
				if n == 0 {
					cond.Signal()
				}
				cond.L.Unlock()
			}()

			if bos := sess.BroadcasterOS; bos != nil && !drivers.IsOwnExternal(url) {
				data, err := drivers.GetSegmentData(url)
				if err != nil {
					errFunc(monitor.SegmentTranscodeErrorDownload, url, err)
					segHashLock.Lock()
					dlErr = err
					segHashLock.Unlock()
					cxn.sessManager.removeSession(sess)
					return
				}
				name := fmt.Sprintf("%s/%d.ts", sess.Profiles[i].Name, seg.SeqNo)
				newUrl, err := bos.SaveData(name, data)
				if err != nil {
					segHashLock.Lock()
					saveErr = err
					segHashLock.Unlock()
					switch err.Error() {
					case "Session ended":
						errFunc(monitor.SegmentTranscodeErrorSessionEnded, url, err)
					default:
						errFunc(monitor.SegmentTranscodeErrorSaveData, url, err)
					}
					return
				}
				url = newUrl

				hash := crypto.Keccak256(data)
				segHashLock.Lock()
				segHashes[i] = hash
				segHashLock.Unlock()
			}

			if monitor.Enabled {
				monitor.TranscodedSegmentAppeared(nonce, seg.SeqNo, sess.Profiles[i].Name)
			}
			err = cpl.InsertHLSSegment(&sess.Profiles[i], seg.SeqNo, url, seg.Duration)
			if err != nil {
				errFunc(monitor.SegmentTranscodeErrorPlaylist, url, err)
				return
			}
		}

		for i, v := range res.Segments {
			go dlFunc(v.Url, i)
		}

		cond.L.Lock()
		for n != 0 {
			cond.Wait()
		}
		cond.L.Unlock()
		if dlErr != nil {
			return dlErr
		}
		ticketParams := sess.OrchestratorInfo.GetTicketParams()
		if ticketParams != nil && // may be nil in offchain mode
			saveErr == nil && // save error leads to early exit before sighash computation
			!pm.VerifySig(ethcommon.BytesToAddress(ticketParams.Recipient), crypto.Keccak256(segHashes...), res.Sig) {
			glog.Errorf("Sig check failed for segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
			cxn.sessManager.removeSession(sess)
			return ErrPMCheckFailed
		}
		if monitor.Enabled {
			monitor.SegmentFullyTranscoded(nonce, seg.SeqNo, common.ProfilesNames(sess.Profiles), errCode)
		}

		glog.V(common.DEBUG).Infof("Successfully validated segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
		return nil
	}
}

var sessionErrStrings = []string{"dial tcp", "unexpected EOF", core.ErrOrchBusy.Error(), core.ErrOrchCap.Error()}

func generateSessionErrors() *regexp.Regexp {
	// Given a list [err1, err2, err3] generates a regexp `(err1)|(err2)|(err3)`
	groups := []string{}
	for _, v := range sessionErrStrings {
		groups = append(groups, fmt.Sprintf("(%v)", v))
	}
	return regexp.MustCompile(strings.Join(groups, "|"))
}

var sessionErrRegex = generateSessionErrors()

func shouldStopSession(err error) bool {
	return sessionErrRegex.MatchString(err.Error())
}
