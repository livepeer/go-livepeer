package server

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
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

const pixelEstimateMultiplier = 1.02

var errSegEncoding = errors.New("ErrorSegEncoding")
var errSegSig = errors.New("ErrSegSig")

var tlsConfig = &tls.Config{InsecureSkipVerify: true}
var httpClient = &http.Client{
	Transport: &http2.Transport{TLSClientConfig: tlsConfig},
	Timeout:   common.HTTPTimeout,
}

func (h *lphttp) ServeSegment(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator

	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil {
		glog.Error("Could not parse payment")
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	sender := getPaymentSender(payment)

	// check the segment sig from the broadcaster
	seg := r.Header.Get(segmentHeader)

	segData, err := verifySegCreds(orch, seg, sender)
	if err != nil {
		glog.Error("Could not verify segment creds")
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// oInfo will be non-nil if we need to send an updated net.OrchestratorInfo to the broadcaster
	var oInfo *net.OrchestratorInfo

	if paymentError := orch.ProcessPayment(payment, segData.ManifestID); paymentError != nil {

		acceptableErr, ok := paymentError.(core.AcceptableError)
		if !ok || !acceptableErr.Acceptable() {
			glog.Errorf("Unacceptable error occured processing payment: %v", paymentError)
			http.Error(w, paymentError.Error(), http.StatusBadRequest)
			return
		}
		oInfo, err = orchestratorInfo(orch, sender, orch.ServiceURI().String())
		if err != nil {
			glog.Errorf("Error updating orchestrator info: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		glog.Errorf("Acceptable error occured when processing payment: %v", paymentError)
	}

	if !orch.SufficientBalance(sender, segData.ManifestID) {
		glog.Errorf("Insufficient credit balance for stream with manifestID %v\n", segData.ManifestID)
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
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
		if took > common.HTTPTimeout {
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
	var pixels int64
	for i := 0; err == nil && i < len(res.TranscodeData.Segments); i++ {
		name := fmt.Sprintf("%s/%d.ts", segData.Profiles[i].Name, segData.Seq) // ANGIE - NEED TO EDIT OUT JOB PROFILES
		uri, err := res.OS.SaveData(name, res.TranscodeData.Segments[i].Data)
		if err != nil {
			glog.Error("Could not upload segment ", segData.Seq)
			break
		}
		pixels += res.TranscodeData.Segments[i].Pixels
		d := &net.TranscodedSegmentData{
			Url:    uri,
			Pixels: res.TranscodeData.Segments[i].Pixels,
		}
		segments = append(segments, d)
	}

	// Debit the fee for the total pixel count
	orch.DebitFees(sender, segData.ManifestID, payment.GetExpectedPrice(), pixels)

	// construct the response
	var result net.TranscodeResult
	if err != nil {
		glog.Errorf("Could not transcode manifestID=%s seqNo=%d err=%v", segData.ManifestID, segData.Seq, err)
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

func makeFfmpegVideoProfiles(protoProfiles []*net.VideoProfile) []ffmpeg.VideoProfile {
	profiles := make([]ffmpeg.VideoProfile, 0, len(protoProfiles))
	for _, profile := range protoProfiles {
		name := profile.Name
		if name == "" {
			name = "net_" + common.DefaultProfileName(int(profile.Width), int(profile.Height), int(profile.Bitrate))
		}
		prof := ffmpeg.VideoProfile{
			Name:       name,
			Bitrate:    fmt.Sprint(profile.Bitrate),
			Framerate:  uint(profile.Fps),
			Resolution: fmt.Sprintf("%dx%d", profile.Width, profile.Height),
		}
		profiles = append(profiles, prof)
	}
	return profiles
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

	profiles := []ffmpeg.VideoProfile{}
	if len(segData.FullProfiles) > 0 {
		profiles = makeFfmpegVideoProfiles(segData.FullProfiles)
	} else if len(segData.Profiles) > 0 {
		profiles, err = common.BytesToVideoProfile(segData.Profiles)
		if err != nil {
			glog.Error("Unable to deserialize profiles ", err)
			return nil, err
		}
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

	priceInfo, err := ratPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
	if err != nil {
		return nil, err
	}

	fee, err := estimateFee(seg, sess.Profiles, priceInfo)
	if err != nil {
		return nil, err
	}

	// Create a BalanceUpdate to be completed when this function returns
	balUpdate, err := newBalanceUpdate(sess, fee)
	if err != nil {
		return nil, err
	}

	// The balance update should be completed when this function returns
	// The logic of balance update completion depends on the status of the update
	// at the time of completion
	defer completeBalanceUpdate(sess, balUpdate)

	payment, err := genPayment(sess, balUpdate.NumTickets)
	if err != nil {
		glog.Errorf("Could not create payment: %v", err)

		if monitor.Enabled && sess.OrchestratorInfo.TicketParams != nil {
			recipient := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient).String()
			monitor.PaymentCreateError(recipient, string(sess.ManifestID))
		}

		return nil, err
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

	// If the segment was submitted then we assume that any payment included was
	// submitted as well so we consider the update's credit as spent
	balUpdate.Status = CreditSpent
	if monitor.Enabled && sess.OrchestratorInfo.TicketParams != nil {
		recipient := ethcommon.BytesToAddress(sess.OrchestratorInfo.TicketParams.Recipient).String()
		mid := string(sess.ManifestID)

		monitor.TicketValueSent(recipient, mid, balUpdate.NewCredit)
		monitor.TicketsSent(recipient, mid, balUpdate.NumTickets)
	}

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
		defer updateOrchestratorInfo(sess, tr.Info)
	}

	// check for errors and exit early if there's anything unusual
	var tdata *net.TranscodeData
	switch res := tr.Result.(type) {
	case *net.TranscodeResult_Error:
		err = fmt.Errorf(res.Error)
		glog.Errorf("Transcode failed for segment nonce=%d manifestID=%s seqNo=%d:%v", nonce, sess.ManifestID, seg.SeqNo, err)
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

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	balUpdate.Status = ReceivedChange
	if priceInfo != nil {
		// The update's debit is the transcoding fee which is computed as the total number of pixels processed
		// for all results returned multiplied by the orchestrator's price
		var pixelCount int64
		for _, res := range tdata.Segments {
			pixelCount += res.Pixels
		}

		balUpdate.Debit.Mul(new(big.Rat).SetInt64(pixelCount), priceInfo)
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

	if sess.Sender != nil && oInfo.TicketParams != nil {
		// Note: We do not validate the ticket params included in the OrchestratorInfo
		// message here. Instead, we store the ticket params with the current BroadcastSession
		// and the next time this BroadcastSession is used, the ticket params will be validated
		// during ticket creation in genPayment(). If ticket params validation during ticket
		// creation fails, then this BroadcastSession will be removed
		sess.PMSessionID = sess.Sender.StartSession(*pmTicketParams(oInfo.TicketParams))
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

	fullProfiles, err := common.FFmpegProfiletoNetProfile(sess.Profiles)
	if err != nil {
		return "", err
	}

	// Generate serialized segment info
	segData := &net.SegData{
		ManifestId:   []byte(md.ManifestID),
		Seq:          md.Seq,
		Hash:         hash,
		FullProfiles: fullProfiles,
		Sig:          sig,
		Storage:      storage,
	}
	data, err := proto.Marshal(segData)
	if err != nil {
		glog.Error("Unable to marshal ", err)
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func estimateFee(seg *stream.HLSSegment, profiles []ffmpeg.VideoProfile, priceInfo *big.Rat) (*big.Rat, error) {
	if priceInfo == nil {
		return nil, nil
	}

	// TODO: Estimate the number of input pixels
	// Estimate the number of output pixels
	var outPixels int64
	for _, p := range profiles {
		w, h, err := ffmpeg.VideoProfileResolution(p)
		if err != nil {
			return nil, err
		}

		// Take the ceiling of the duration to always overestimate
		outPixels += int64(w*h) * int64(p.Framerate) * int64(math.Ceil(seg.Duration))
	}

	// feeEstimate = pixels * pixelEstimateMultiplier * priceInfo
	fee := new(big.Rat).SetInt64(outPixels)
	// Multiply pixels by pixelEstimateMultiplier to ensure that we never underpay
	fee.Mul(fee, new(big.Rat).SetFloat64(pixelEstimateMultiplier))
	fee.Mul(fee, priceInfo)

	return fee, nil
}

func newBalanceUpdate(sess *BroadcastSession, minCredit *big.Rat) (*BalanceUpdate, error) {
	update := &BalanceUpdate{
		ExistingCredit: big.NewRat(0, 1),
		NewCredit:      big.NewRat(0, 1),
		Debit:          big.NewRat(0, 1),
		Status:         Staged,
	}

	if sess.Sender == nil || sess.Balance == nil || minCredit == nil {
		return update, nil
	}

	ev, err := sess.Sender.EV(sess.PMSessionID)
	if err != nil {
		return nil, err
	}

	// The orchestrator requires the broadcaster's balance to be at least the EV of a single ticket
	// Use the ticket EV when creating the balance update if the passed in minCredit is less than the ticket EV
	safeMinCredit := minCredit
	if ev.Cmp(safeMinCredit) > 0 {
		safeMinCredit = ev
	}

	update.NumTickets, update.NewCredit, update.ExistingCredit = sess.Balance.StageUpdate(safeMinCredit, ev)

	return update, nil
}

func completeBalanceUpdate(sess *BroadcastSession, update *BalanceUpdate) {
	if sess.Balance == nil {
		return
	}

	// If the update's credit has not been spent then add the existing credit
	// back to the balance
	if update.Status == Staged {
		sess.Balance.Credit(update.ExistingCredit)
		return
	}

	// If the update did not include a processed debit then no change was received
	// so we exit without updating the balance because the credit was spent already
	if update.Status != ReceivedChange {
		return
	}

	credit := new(big.Rat).Add(update.ExistingCredit, update.NewCredit)
	// The change could be negative if the debit > credit
	change := credit.Sub(credit, update.Debit)

	// If the change is negative then this is equivalent to debiting abs(change)
	sess.Balance.Credit(change)
}

func genPayment(sess *BroadcastSession, numTickets int) (string, error) {
	if sess.Sender == nil {
		return "", nil
	}

	// Compare Orchestrator Price against BroadcastConfig.MaxPrice
	if err := validatePrice(sess); err != nil {
		return "", err
	}

	protoPayment := &net.Payment{
		Sender:        sess.Broadcaster.Address().Bytes(),
		ExpectedPrice: sess.OrchestratorInfo.PriceInfo,
	}

	if numTickets > 0 {
		batch, err := sess.Sender.CreateTicketBatch(sess.PMSessionID, numTickets)
		if err != nil {
			return "", err
		}

		protoPayment.TicketParams = &net.TicketParams{
			Recipient:         batch.Recipient.Bytes(),
			FaceValue:         batch.FaceValue.Bytes(),
			WinProb:           batch.WinProb.Bytes(),
			RecipientRandHash: batch.RecipientRandHash.Bytes(),
			Seed:              batch.Seed.Bytes(),
		}

		protoPayment.ExpirationParams = &net.TicketExpirationParams{
			CreationRound:          batch.CreationRound,
			CreationRoundBlockHash: batch.CreationRoundBlockHash.Bytes(),
		}

		senderParams := make([]*net.TicketSenderParams, len(batch.SenderParams))
		for i := 0; i < len(senderParams); i++ {
			senderParams[i] = &net.TicketSenderParams{
				SenderNonce: batch.SenderParams[i].SenderNonce,
				Sig:         batch.SenderParams[i].Sig,
			}
		}

		protoPayment.TicketSenderParams = senderParams
	}

	data, err := proto.Marshal(protoPayment)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func validatePrice(sess *BroadcastSession) error {
	oPrice, err := ratPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
	if err != nil {
		return err
	}
	if oPrice == nil {
		return errors.New("missing orchestrator price")
	}

	maxPrice := BroadcastCfg.MaxPrice()
	if maxPrice != nil && oPrice.Cmp(maxPrice) == 1 {
		return fmt.Errorf("Orchestrator price higher than the set maximum price of %v wei per %v pixels", maxPrice.Num().Int64(), maxPrice.Denom().Int64())
	}
	return nil
}

func ratPriceInfo(priceInfo *net.PriceInfo) (*big.Rat, error) {
	if priceInfo == nil {
		return nil, nil
	}

	pixelsPerUnit := priceInfo.PixelsPerUnit
	if pixelsPerUnit == 0 {
		return nil, errors.New("invalid priceInfo.pixelsPerUnit")
	}

	return big.NewRat(priceInfo.PricePerUnit, pixelsPerUnit), nil
}
