package server

import (
	"errors"
	"fmt"
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

func selectOrchestrator(n *core.LivepeerNode, cpl core.PlaylistManager) ([]*BroadcastSession, error) {
	if n.OrchestratorPool == nil {
		glog.Info("No orchestrators specified; not transcoding")
		return nil, ErrDiscovery
	}

	rpcBcast := core.NewBroadcaster(n)

	defaultNumOrchs := HTTPTimeout / SegLen
	tinfos, err := n.OrchestratorPool.GetOrchestrators(int(defaultNumOrchs))
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

func processSegment(cxn *rtmpConnection, seg *stream.HLSSegment, node *core.LivepeerNode) {

	nonce := cxn.nonce
	rtmpStrm := cxn.stream
	cpl := cxn.pl
	mid := cxn.mid
	vProfile := cxn.profile

	cxn.lock.RLock()
	sess := cxn.sessManager.selectSession(node, cxn.pl)
	cxn.lock.RUnlock()

	if monitor.Enabled {
		monitor.LogSegmentEmerged(nonce, seg.SeqNo, len(BroadcastJobVideoProfiles))
	}

	seg.Name = "" // hijack seg.Name to convey the uploaded URI
	name := fmt.Sprintf("%s/%d.ts", vProfile.Name, seg.SeqNo)
	uri, err := cpl.GetOSSession().SaveData(name, seg.Data)
	if err != nil {
		glog.Errorf("Error saving segment %d: %v", seg.SeqNo, err)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err.Error())
		}
		return
	}
	if cpl.GetOSSession().IsExternal() {
		seg.Name = uri // hijack seg.Name to convey the uploaded URI
	}
	err = cpl.InsertHLSSegment(vProfile, seg.SeqNo, uri, seg.Duration)
	if monitor.Enabled {
		monitor.LogSourceSegmentAppeared(nonce, seg.SeqNo, string(mid), vProfile.Name)
		glog.V(6).Infof("Appeared segment %d", seg.SeqNo)
	}
	if err != nil {
		glog.Errorf("Error inserting segment %d: %v", seg.SeqNo, err)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err.Error())
		}
	}

	// Return early under a few circumstances:
	// View-only (non-transcoded) streams or mid-failover
	if sess == nil {
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed(monitor.SegmentTranscodeErrorNoOrchestrators, nonce, seg.SeqNo, errors.New("No Orchestrators Error"))
		}
		return
	}

	// Process the rest of the segment asynchronously - transcode
	go func() {
		// storage the orchestrator prefers
		if ios := sess.OrchestratorOS; ios != nil {
			// XXX handle case when orch expects direct upload
			uri, err := ios.SaveData(name, seg.Data)
			if err != nil {
				glog.Error("Error saving segment to OS ", err)
				if monitor.Enabled {
					monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorOS, err.Error())
				}
				return
			}
			seg.Name = uri // hijack seg.Name to convey the uploaded URI
		}

		// send segment to the orchestrator
		glog.V(common.DEBUG).Infof("Submitting segment %d", seg.SeqNo)

		res, err := SubmitSegment(sess, seg, nonce)
		if err != nil {
			cxn.sessManager.removeSession(sess)
			if shouldStopStream(err) {
				glog.Warningf("Stopping current stream due to: %v", err)
				rtmpStrm.Close()
				return
			}
			if shouldStopSession(err) {
				cxn.needOrch <- struct{}{}
			}
			return
		} else {
			cxn.sessManager.completeSession(sess)
		}

		// download transcoded segments from the transcoder
		gotErr := false // only send one error msg per segment list
		errFunc := func(subType monitor.SegmentTranscodeError, url string, err error) {
			glog.Errorf("%v error with segment %v: %v (URL: %v)", subType, seg.SeqNo, err, url)
			if monitor.Enabled && !gotErr {
				monitor.LogSegmentTranscodeFailed(subType, nonce, seg.SeqNo, err)
				gotErr = true
			}
		}
		if res == nil {
			return
		}

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
					return
				}
				name := fmt.Sprintf("%s/%d.ts", sess.Profiles[i].Name, seg.SeqNo)
				newUrl, err := bos.SaveData(name, data)
				if err != nil {
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
				monitor.LogTranscodedSegmentAppeared(nonce, seg.SeqNo, sess.Profiles[i].Name)
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
		if monitor.Enabled {
			monitor.SegmentFullyTranscoded(nonce, seg.SeqNo, common.ProfilesNames(sess.Profiles), len(segHashes) == len(res.Segments))
		}

		ticketParams := sess.OrchestratorInfo.GetTicketParams()
		if ticketParams != nil && // may be nil in offchain mode
			!pm.VerifySig(ethcommon.BytesToAddress(ticketParams.Recipient), crypto.Keccak256(segHashes...), res.Sig) {
			glog.Error("Sig check failed for segment ", seg.SeqNo)
			return
		}

		glog.V(common.DEBUG).Info("Successfully validated segment ", seg.SeqNo)
	}()
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
