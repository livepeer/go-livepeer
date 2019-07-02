package server

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/stream"
	"golang.org/x/net/http2"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

const paymentHeader = "Livepeer-Payment"
const segmentHeader = "Livepeer-Segment"

var errSegEncoding = errors.New("ErrorSegEncoding")
var errSegSig = errors.New("ErrSegSig")

var tlsConfig = &tls.Config{InsecureSkipVerify: true}
var httpClient = &http.Client{
	Transport: &http2.Transport{TLSClientConfig: tlsConfig},
	Timeout:   HTTPTimeout,
}

func (h *lphttp) ServeSegment(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator

	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil {
		glog.Error("Could not parse payment")
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	// check the segment sig from the broadcaster
	seg := r.Header.Get(segmentHeader)

	segData, err := verifySegCreds(orch, seg, getPaymentSender(payment))
	if err != nil {
		glog.Error("Could not verify segment creds")
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// oInfo will be non-nil if we need to send an updated net.OrchestratorInfo
	// to the broadcaster
	var oInfo *net.OrchestratorInfo

	if err := orch.ProcessPayment(payment, segData.ManifestID); err != nil {
		glog.Errorf("Error processing payment: %v", err)

		oInfo, err = orchestratorInfo(orch, getPaymentSender(payment), orch.ServiceURI().String())
		if err != nil {
			glog.Errorf("Error updating orchestrator info: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// download the segment and check the hash
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Error("Could not read request body: ", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	uri := ""
	if r.Header.Get("Content-Type") == "application/vnd+livepeer.uri" {
		uri = string(data)
		glog.V(common.DEBUG).Infof("Start getting segment from %s", uri)
		start := time.Now()
		data, err = drivers.GetSegmentData(uri)
		took := time.Since(start)
		glog.V(common.DEBUG).Infof("Getting segment from %s took %s", uri, took)
		if err != nil {
			glog.Errorf("Error getting input segment %v from input OS: %v", uri, err)
			http.Error(w, "BadRequest", http.StatusBadRequest)
			return
		}
		if took > HTTPTimeout {
			// download from object storage took more time when broadcaster will be waiting for result
			// so there is no point to start transcoding process
			glog.Errorf(" Getting segment from %s took too long, aborting", uri)
			http.Error(w, "BadRequest", http.StatusBadRequest)
			return
		}
	}

	hash := crypto.Keccak256(data)
	if !bytes.Equal(hash, segData.Hash.Bytes()) {
		glog.Error("Mismatched hash for body; rejecting")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Send down 200OK early as an indication that the upload completed
	// Any further errors come through the response body
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()

	hlsStream := stream.HLSSegment{
		SeqNo: uint64(segData.Seq),
		Data:  data,
		Name:  uri,
	}

	res, err := orch.TranscodeSeg(segData, &hlsStream) // ANGIE - NEED TO CHANGE ALL JOBIDS IN TRANSCODING LOOP INTO STRINGS

	// Upload to OS and construct segment result set
	var segments []*net.TranscodedSegmentData
	for i := 0; err == nil && i < len(res.Data); i++ {
		name := fmt.Sprintf("%s/%d.ts", segData.Profiles[i].Name, segData.Seq) // ANGIE - NEED TO EDIT OUT JOB PROFILES
		uri, err := res.OS.SaveData(name, res.Data[i])
		if err != nil {
			glog.Error("Could not upload segment ", segData.Seq)
			break
		}
		d := &net.TranscodedSegmentData{
			Url: uri,
		}
		segments = append(segments, d)
	}

	// construct the response
	var result net.TranscodeResult
	if err != nil {
		glog.Error("Could not transcode ", err)
		result = net.TranscodeResult{Result: &net.TranscodeResult_Error{Error: err.Error()}}
	} else {
		result = net.TranscodeResult{Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: segments,
				Sig:      res.Sig,
			}},
		}
	}

	tr := &net.TranscodeResult{
		Seq:    segData.Seq,
		Result: result.Result,
		Info:   oInfo, // oInfo will be non-nil if we need to send an update to the broadcaster
	}
	buf, err := proto.Marshal(tr)
	if err != nil {
		glog.Error("Unable to marshal transcode result ", err)
		return
	}
	w.Write(buf)
}

func getPayment(header string) (net.Payment, error) {
	buf, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		return net.Payment{}, errors.Wrap(err, "base64 decode error")
	}
	var payment net.Payment
	if err := proto.Unmarshal(buf, &payment); err != nil {
		return net.Payment{}, errors.Wrap(err, "protobuf unmarshal error")
	}

	return payment, nil
}

func getPaymentSender(payment net.Payment) ethcommon.Address {
	if payment.Sender == nil {
		return ethcommon.Address{}
	}
	return ethcommon.BytesToAddress(payment.Sender)
}

func verifySegCreds(orch Orchestrator, segCreds string, broadcaster ethcommon.Address) (*core.SegTranscodingMetadata, error) {
	buf, err := base64.StdEncoding.DecodeString(segCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, errSegEncoding
	}
	var segData net.SegData
	err = proto.Unmarshal(buf, &segData)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}
	profiles, err := common.BytesToVideoProfile(segData.Profiles)
	if err != nil {
		glog.Error("Unable to deserialize profiles ", err)
		return nil, err
	}
	mid := core.ManifestID(segData.ManifestId)

	var os *net.OSInfo
	if len(segData.Storage) > 0 {
		os = segData.Storage[0]
	}

	md := &core.SegTranscodingMetadata{
		ManifestID: mid,
		Seq:        segData.Seq,
		Hash:       ethcommon.BytesToHash(segData.Hash),
		Profiles:   profiles,
		OS:         os,
	}

	if !orch.VerifySig(broadcaster, string(md.Flatten()), segData.Sig) {
		glog.Error("Sig check failed")
		return nil, errSegSig
	}

	if err := orch.CheckCapacity(mid); err != nil {
		glog.Error("Cannot process manifest: ", err)
		return nil, err
	}

	return md, nil
}

func SubmitSegment(sess *BroadcastSession, seg *stream.HLSSegment, nonce uint64) (*net.TranscodeData, error) {
	uploaded := seg.Name != "" // hijack seg.Name to convey the uploaded URI

	segCreds, err := genSegCreds(sess, seg)
	if err != nil {
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorGenCreds, err.Error(), false)
		}
		return nil, err
	}
	data := seg.Data
	if uploaded {
		data = []byte(seg.Name)
	}

	payment, err := genPayment(sess)
	if err != nil {
		glog.Errorf("Could not create payment: %v", err)
	}

	ti := sess.OrchestratorInfo
	req, err := http.NewRequest("POST", ti.Transcoder+"/segment", bytes.NewBuffer(data))
	if err != nil {
		glog.Error("Could not generate transcode request to ", ti.Transcoder)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorGenCreds, err.Error(), false)
		}
		return nil, err
	}

	req.Header.Set(segmentHeader, segCreds)
	req.Header.Set(paymentHeader, payment)
	if uploaded {
		req.Header.Set("Content-Type", "application/vnd+livepeer.uri")
	} else {
		req.Header.Set("Content-Type", "video/MP2T")
	}

	glog.Infof("Submitting segment nonce=%d seqNo=%d : %v bytes", nonce, seg.SeqNo, len(data))
	start := time.Now()
	resp, err := httpClient.Do(req)
	uploadDur := time.Since(start)
	if err != nil {
		glog.Errorf("Unable to submit segment nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err.Error(), false)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := ioutil.ReadAll(resp.Body)
		errorString := strings.TrimSpace(string(data))
		glog.Errorf("Error submitting segment nonce=%d seqNo=%d code=%d error=%v", nonce, seg.SeqNo, resp.StatusCode, string(data))
		if monitor.Enabled {
			monitor.SegmentUploadFailed(nonce, seg.SeqNo, monitor.SegmentUploadError(resp.Status),
				fmt.Sprintf("Code: %d Error: %s", resp.StatusCode, errorString), false)
		}
		return nil, fmt.Errorf(errorString)
	}
	glog.Infof("Uploaded segment nonce=%d seqNo=%d", nonce, seg.SeqNo)
	if monitor.Enabled {
		monitor.SegmentUploaded(nonce, seg.SeqNo, uploadDur)
	}

	data, err = ioutil.ReadAll(resp.Body)
	tookAllDur := time.Since(start)

	if err != nil {
		glog.Errorf("Unable to read response body for segment nonce=%d seqNo=%d : %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorReadBody, nonce, seg.SeqNo, err, false)
		}
		return nil, err
	}
	transcodeDur := tookAllDur - uploadDur

	var tr net.TranscodeResult
	err = proto.Unmarshal(data, &tr)
	if err != nil {
		glog.Errorf("Unable to parse response for segment nonce=%d seqNo=%d : %v", nonce, seg.SeqNo, err)
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorParseResponse, nonce, seg.SeqNo, err, false)
		}
		return nil, err
	}

	// update OrchestratorInfo if necessary
	if tr.Info != nil {
		updateOrchestratorInfo(sess, tr.Info)
	}

	// check for errors and exit early if there's anything unusual
	var tdata *net.TranscodeData
	switch res := tr.Result.(type) {
	case *net.TranscodeResult_Error:
		err = fmt.Errorf(res.Error)
		glog.Errorf("Transcode failed for segment nonce=%d seqNo=%d: %v", nonce, seg.SeqNo, err)
		if err.Error() == "MediaStats Failure" {
			glog.Info("Ensure the keyframe interval is 4 seconds or less")
		}
		if monitor.Enabled {
			switch res.Error {
			case "OrchestratorBusy":
				monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorOrchestratorBusy, nonce, seg.SeqNo, err, false)
			case "OrchestratorCapped":
				monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorOrchestratorCapped, nonce, seg.SeqNo, err, false)
			default:
				monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorTranscode, nonce, seg.SeqNo, err, false)
			}
		}
		return nil, err
	case *net.TranscodeResult_Data:
		// fall through here for the normal case
		tdata = res.Data
	default:
		glog.Errorf("Unexpected or unset transcode response field for nonce=%d seqNo=%d", nonce, seg.SeqNo)
		err = fmt.Errorf("UnknownResponse")
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(monitor.SegmentTranscodeErrorUnknownResponse, nonce, seg.SeqNo, err, false)
		}
		return nil, err
	}

	// transcode succeeded; continue processing response
	if monitor.Enabled {
		monitor.SegmentTranscoded(nonce, seg.SeqNo, transcodeDur, common.ProfilesNames(sess.Profiles))
	}

	glog.Infof("Successfully transcoded segment nonce=%d manifestID=%s segName=%s seqNo=%d", nonce, string(sess.ManifestID), seg.Name, seg.SeqNo)

	return tdata, nil
}

func updateOrchestratorInfo(sess *BroadcastSession, oInfo *net.OrchestratorInfo) {
	sess.OrchestratorInfo = oInfo

	if len(oInfo.Storage) > 0 {
		sess.OrchestratorOS = drivers.NewSession(oInfo.Storage[0])
	}

	if oInfo.TicketParams == nil {
		return
	}

	if sess.Sender != nil {
		protoParams := oInfo.TicketParams
		params := pm.TicketParams{
			Recipient:         ethcommon.BytesToAddress(protoParams.Recipient),
			FaceValue:         new(big.Int).SetBytes(protoParams.FaceValue),
			WinProb:           new(big.Int).SetBytes(protoParams.WinProb),
			RecipientRandHash: ethcommon.BytesToHash(protoParams.RecipientRandHash),
			Seed:              new(big.Int).SetBytes(protoParams.Seed),
		}

		sess.PMSessionID = sess.Sender.StartSession(params)
	}
}

func genSegCreds(sess *BroadcastSession, seg *stream.HLSSegment) (string, error) {

	// Generate signature for relevant parts of segment
	hash := crypto.Keccak256(seg.Data)
	md := &core.SegTranscodingMetadata{
		ManifestID: sess.ManifestID,
		Seq:        int64(seg.SeqNo),
		Hash:       ethcommon.BytesToHash(hash),
		Profiles:   sess.Profiles,
	}
	sig, err := sess.Broadcaster.Sign(md.Flatten())
	if err != nil {
		return "", err
	}

	// Send credentials for our own storage
	var storage []*net.OSInfo
	if bos := sess.BroadcasterOS; bos != nil && bos.IsExternal() {
		storage = []*net.OSInfo{bos.GetInfo()}
	}

	// Generate serialized segment info
	segData := &net.SegData{
		ManifestId: []byte(md.ManifestID),
		Seq:        md.Seq,
		Hash:       hash,
		Profiles:   common.ProfilesToTranscodeOpts(sess.Profiles),
		Sig:        sig,
		Storage:    storage,
	}
	data, err := proto.Marshal(segData)
	if err != nil {
		glog.Error("Unable to marshal ", err)
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func genPayment(sess *BroadcastSession) (string, error) {
	if sess.Sender == nil {
		return "", nil
	}
	/** TODO
	 * Check necessary credit balance
	 * See how many tickets to make according to needed credit and ticket EV
	 * Invoke CreateTicket that many times and add to protoPayment
	 */
	ticket, seed, sig, err := sess.Sender.CreateTicket(sess.PMSessionID)
	if err != nil {
		return "", err
	}

	protoTicketParams := &net.TicketParams{
		Recipient:         ticket.Recipient.Bytes(),
		FaceValue:         ticket.FaceValue.Bytes(),
		WinProb:           ticket.WinProb.Bytes(),
		RecipientRandHash: ticket.RecipientRandHash.Bytes(),
		Seed:              seed.Bytes(),
	}

	protoTicketSenderParams := &net.TicketSenderParams{
		SenderNonce: ticket.SenderNonce,
		Sig:         sig,
	}

	protoExpirationParams := &net.TicketExpirationParams{
		CreationRound:          ticket.CreationRound,
		CreationRoundBlockHash: ticket.CreationRoundBlockHash.Bytes(),
	}

	protoPayment := &net.Payment{
		TicketParams:       protoTicketParams,
		Sender:             ticket.Sender.Bytes(),
		ExpirationParams:   protoExpirationParams,
		TicketSenderParams: []*net.TicketSenderParams{protoTicketSenderParams},
	}

	data, err := proto.Marshal(protoPayment)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
