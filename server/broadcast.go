package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net/url"
	"os"
	"sync"

	"github.com/golang/glog"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/pm"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

var BroadcastCfg = &BroadcastConfig{}

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
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	cfg.maxPrice = price
}

type BroadcastSessionsManager struct {
	// Accessing or changing any of the below requires ownership of this mutex
	sessLock *sync.Mutex

	sel      BroadcastSessionsSelector
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
		numSess := m.sel.Size()
		if numSess < int(math.Ceil(float64(m.numOrchs)/2.0)) {
			go m.refreshSessions()
		}
		return numSess > 0
	}
	for checkSessions(bsm) {
		sess := bsm.sel.Select()
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

	bsm.sel.Add(uniqueSessions)
}

func (bsm *BroadcastSessionsManager) cleanup() {
	bsm.sessLock.Lock()
	defer bsm.sessLock.Unlock()
	bsm.finished = true
	bsm.sel.Clear()
	bsm.sessMap = make(map[string]*BroadcastSession) // prevent segfaults
}

func NewSessionManager(node *core.LivepeerNode, params *streamParameters, pl core.PlaylistManager, sel BroadcastSessionsSelector) *BroadcastSessionsManager {
	var poolSize float64
	if node.OrchestratorPool != nil {
		poolSize = float64(node.OrchestratorPool.Size())
	}
	maxInflight := common.HTTPTimeout.Seconds() / SegLen.Seconds()
	numOrchs := int(math.Min(poolSize, maxInflight*2))
	bsm := &BroadcastSessionsManager{
		sel:            sel,
		sessMap:        make(map[string]*BroadcastSession),
		createSessions: func() ([]*BroadcastSession, error) { return selectOrchestrator(node, params, pl, numOrchs) },
		sessLock:       &sync.Mutex{},
		numOrchs:       numOrchs,
	}
	bsm.refreshSessions()
	return bsm
}

func selectOrchestrator(n *core.LivepeerNode, params *streamParameters, cpl core.PlaylistManager, count int) ([]*BroadcastSession, error) {
	if n.OrchestratorPool == nil {
		glog.Info("No orchestrators specified; not transcoding")
		return nil, errDiscovery
	}

	tinfos, err := n.OrchestratorPool.GetOrchestrators(count)
	if len(tinfos) <= 0 {
		glog.Info("No orchestrators found; not transcoding. Error: ", err)
		return nil, errNoOrchs
	}
	if err != nil {
		return nil, err
	}

	var sessions []*BroadcastSession

	for _, tinfo := range tinfos {
		var sessionID string
		var balance Balance

		ticketParams := pmTicketParams(tinfo.TicketParams)

		if n.Sender != nil {
			sessionID = n.Sender.StartSession(*ticketParams)
		}

		if n.Balances != nil {
			balance = core.NewBalance(ticketParams.Recipient, params.mid, n.Balances)
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
			Broadcaster:      core.NewBroadcaster(n),
			ManifestID:       params.mid,
			Profiles:         params.profiles,
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

func processSegment(cxn *rtmpConnection, seg *stream.HLSSegment) error {

	nonce := cxn.nonce
	cpl := cxn.pl
	mid := cxn.mid
	vProfile := cxn.profile

	glog.V(common.DEBUG).Infof("Processing segment nonce=%d manifestID=%s seqNo=%d dur=%v", nonce, mid, seg.SeqNo, seg.Duration)
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
		return err
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

	for {
		// if fails, retry; rudimentary
		if err := transcodeSegment(cxn, seg, name); err == nil {
			return nil
		}
	}
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
			monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorNoOrchestrators, nonce, seg.SeqNo, errNoOrchs, true)
		}
		glog.Infof("No sessions available for segment nonce=%d manifestID=%s seqNo=%d", nonce, cxn.mid, seg.SeqNo)
		// We may want to introduce a "non-retryable" error type here
		// would help error propagation for live ingest.
		// similar to the orchestrator's RemoteTranscoderFatalError
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
		glog.V(common.DEBUG).Infof("Submitting segment nonce=%d manifestID=%s seqNo=%d orch=%s", nonce, cxn.mid, seg.SeqNo, sess.OrchestratorInfo.Transcoder)

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

		// Instead of mutating the existing session we copy it and use the copy in BroadcastSessionsManager
		newSess := &BroadcastSession{}
		*newSess = *sess
		newSess.LatencyScore = res.LatencyScore
		if res.Info != nil {
			updateOrchestratorInfo(newSess, res.Info)
		}

		cxn.sessManager.completeSession(newSess)

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

		dlFunc := func(url string, pixels int64, i int) {
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
				newURL, err := bos.SaveData(name, data)
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
				url = newURL

				hash := crypto.Keccak256(data)
				segHashLock.Lock()
				segHashes[i] = hash
				segHashLock.Unlock()
			}

			// If running in on-chain mode, run pixels verification asynchronously
			if sess.Sender != nil {
				go func() {
					if err := verifyPixels(url, sess.BroadcasterOS, pixels); err != nil {
						glog.Error(err)
						cxn.sessManager.removeSession(sess)
					}
				}()
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
			go dlFunc(v.Url, v.Pixels, i)
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
			// Might not have seg hashes if results are directly uploaded to the broadcaster's OS
			// TODO: Consider downloading the results to generate seg hashes if results are directly uploaded to the broadcaster's OS
			len(segHashes) != len(res.Segments) &&
			!pm.VerifySig(ethcommon.BytesToAddress(ticketParams.Recipient), crypto.Keccak256(segHashes...), res.Sig) {
			glog.Errorf("Sig check failed for segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
			cxn.sessManager.removeSession(sess)
			return errPMCheckFailed
		}
		if monitor.Enabled {
			monitor.SegmentFullyTranscoded(nonce, seg.SeqNo, common.ProfilesNames(sess.Profiles), errCode)
		}

		glog.V(common.DEBUG).Infof("Successfully validated segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
		return nil
	}
}

var sessionErrStrings = []string{"dial tcp", "unexpected EOF", core.ErrOrchBusy.Error(), core.ErrOrchCap.Error()}

var sessionErrRegex = common.GenErrRegex(sessionErrStrings)

func shouldStopSession(err error) bool {
	return sessionErrRegex.MatchString(err.Error())
}

func verifyPixels(fname string, bos drivers.OSSession, reportedPixels int64) error {
	uri, err := url.ParseRequestURI(fname)
	memOS, ok := bos.(*drivers.MemorySession)
	// If the filename is a relative URI and the broadcaster is using local memory storage
	// fetch the data and write it to a temp file
	if err == nil && !uri.IsAbs() && ok {
		tempfile, err := ioutil.TempFile("", common.RandName())
		if err != nil {
			return fmt.Errorf("error creating temp file for pixels verification: %v", err)
		}
		defer os.Remove(tempfile.Name())

		data := memOS.GetData(fname)
		if data == nil {
			return errors.New("error fetching data from local memory storage")
		}

		if _, err := tempfile.Write(memOS.GetData(fname)); err != nil {
			return fmt.Errorf("error writing temp file for pixels verification: %v", err)
		}

		fname = tempfile.Name()
	}

	p, err := pixels(fname)
	if err != nil {
		return err
	}

	if p != reportedPixels {
		return errors.New("mismatch between calculated and reported pixels")
	}

	return nil
}

func pixels(fname string) (int64, error) {
	in := &ffmpeg.TranscodeOptionsIn{Fname: fname}
	res, err := ffmpeg.Transcode3(in, nil)
	if err != nil {
		return 0, err
	}

	return res.Decoded.Pixels, nil
}
