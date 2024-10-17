package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	gonet "net"
	"net/http"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

const paymentHeader = "Livepeer-Payment"
const segmentHeader = "Livepeer-Segment"

const pixelEstimateMultiplier = 1.02

// Maximum price change allowed in orchestrator pricing before the session is swapped.
var priceIncreaseThreshold = big.NewRat(2, 1)

var errSegEncoding = errors.New("ErrorSegEncoding")
var errSegSig = errors.New("ErrSegSig")
var errFormat = errors.New("unrecognized profile output format")
var errProfile = errors.New("unrecognized encoder profile")
var errEncoder = errors.New("unrecognized video codec")
var errDuration = errors.New("invalid duration")
var errCapCompat = errors.New("incompatible capabilities")

var tlsConfig = &tls.Config{InsecureSkipVerify: true}
var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: tlsConfig,
		DialTLSContext: func(ctx context.Context, network, addr string) (gonet.Conn, error) {
			cctx, cancel := context.WithTimeout(ctx, common.HTTPDialTimeout)
			defer cancel()

			tlsDialer := &tls.Dialer{Config: tlsConfig}
			return tlsDialer.DialContext(cctx, network, addr)
		},
		// Required for the transport to try to upgrade to HTTP/2 if TLSClientConfig is non-nil or
		// if custom dialers (i.e. via DialTLSContext) are used. This allows us to by default
		// transparently support HTTP/2 while maintaining the flexibility to use HTTP/1 by running
		// with GODEBUG=http2client=0
		ForceAttemptHTTP2: true,
	},
	// Don't set a timeout here; pass a context to the request
}

func (h *lphttp) ServeSegment(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator

	remoteAddr := getRemoteAddr(r)
	ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil {
		clog.Errorf(ctx, "Could not parse payment")
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	sender := getPaymentSender(payment)
	ctx = clog.AddVal(ctx, "sender", sender.Hex())

	// check the segment sig from the broadcaster
	seg := r.Header.Get(segmentHeader)

	segData, ctx, err := verifySegCreds(ctx, orch, seg, sender)
	if err != nil {
		clog.Errorf(ctx, "Could not verify segment creds err=%q", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	ctx = clog.AddSeqNo(ctx, uint64(segData.Seq))

	clog.V(common.VERBOSE).Infof(ctx, "Received segment dur=%v", segData.Duration)

	if monitor.Enabled {
		monitor.SegmentEmerged(ctx, 0, uint64(segData.Seq), len(segData.Profiles), segData.Duration.Seconds())
	}

	if err := orch.ProcessPayment(ctx, payment, core.ManifestID(segData.AuthToken.SessionId)); err != nil {
		clog.Errorf(ctx, "error processing payment: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Balance check is only necessary if the price is non-zero
	// We do not need to worry about differentiating between the case where the price is 0 as the default when no price is attached vs.
	// the case where the price is actually set to 0 because ProcessPayment() should guarantee a price attached
	if payment.GetExpectedPrice().GetPricePerUnit() > 0 && !orch.SufficientBalance(sender, core.ManifestID(segData.AuthToken.SessionId)) {
		clog.Errorf(ctx, "Insufficient credit balance for stream")
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	oInfo, err := orchestratorInfo(orch, sender, orch.ServiceURI().String(), core.ManifestID(segData.AuthToken.SessionId))
	if err != nil {
		clog.Errorf(ctx, "Error updating orchestrator info - err=%q", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Use existing auth token because new auth tokens should only be sent out in GetOrchestrator() RPC calls
	oInfo.AuthToken = segData.AuthToken

	// download the segment and check the hash
	dlStart := time.Now()
	data, err := common.ReadAtMost(r.Body, common.MaxSegSize)
	if err != nil {
		clog.Errorf(ctx, "Could not read request body - err=%q", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	dlDur := time.Since(dlStart)
	clog.V(common.VERBOSE).Infof(ctx, "Downloaded segment dur=%v", dlDur)

	if monitor.Enabled {
		monitor.SegmentDownloaded(ctx, 0, uint64(segData.Seq), dlDur)
	}

	uri := ""
	if r.Header.Get("Content-Type") == "application/vnd+livepeer.uri" {
		uri = string(data)
		clog.V(common.DEBUG).Infof(ctx, "Start getting segment from url=%s", uri)
		start := time.Now()
		data, err = core.GetSegmentData(ctx, uri)
		took := time.Since(start)
		clog.V(common.DEBUG).Infof(ctx, "Getting segment from url=%s took=%s bytes=%d", uri, took, len(data))
		if err != nil {
			clog.Errorf(ctx, "Error getting input segment from input OS - segment=%v err=%q", uri, err)
			http.Error(w, "BadRequest", http.StatusBadRequest)
			return
		}
		if took > common.HTTPTimeout {
			// download from object storage took more time when broadcaster will be waiting for result
			// so there is no point to start transcoding process
			clog.Errorf(ctx, " Getting segment from url=%s took too long, aborting", uri)
			http.Error(w, "BadRequest", http.StatusBadRequest)
			return
		}
	}

	hash := crypto.Keccak256(data)
	if !bytes.Equal(hash, segData.Hash.Bytes()) {
		clog.Errorf(ctx, "Mismatched hash for body; rejecting")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Send down 200OK early as an indication that the upload completed
	// Any further errors come through the response body
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	hlsStream := stream.HLSSegment{
		SeqNo: uint64(segData.Seq),
		Data:  data,
		Name:  uri,
	}

	res, err := orch.TranscodeSeg(ctx, segData, &hlsStream)

	// Upload to OS and construct segment result set
	var segments []*net.TranscodedSegmentData
	var pixels int64
	for i := 0; err == nil && i < len(res.TranscodeData.Segments); i++ {
		var ext string
		ext, err = common.ProfileFormatExtension(segData.Profiles[i].Format)
		if err != nil {
			clog.Errorf(ctx, "Unknown format extension err=%s", err)
			break
		}
		name := fmt.Sprintf("%s/%d%s", segData.Profiles[i].Name, segData.Seq, ext)
		// The use of := here is probably a bug?!?
		segData := bytes.NewReader(res.TranscodeData.Segments[i].Data)
		uri, err := res.OS.SaveData(ctx, name, segData, nil, 0)
		if err != nil {
			clog.Errorf(ctx, "Could not upload segment err=%q", err)
			break
		}
		pixels += res.TranscodeData.Segments[i].Pixels
		d := &net.TranscodedSegmentData{
			Url:    uri,
			Pixels: res.TranscodeData.Segments[i].Pixels,
		}
		// Save perceptual hash if generated
		if res.TranscodeData.Segments[i].PHash != nil {
			pHashFile := name + ".phash"
			pHashData := bytes.NewReader(res.TranscodeData.Segments[i].PHash)
			pHashUri, err := res.OS.SaveData(ctx, pHashFile, pHashData, nil, 0)
			if err != nil {
				clog.Errorf(ctx, "Could not upload segment perceptual hash err=%q", err)
				break
			}
			d.PerceptualHashUrl = pHashUri
		}
		segments = append(segments, d)
	}

	// Debit the fee for the total pixel count
	orch.DebitFees(sender, core.ManifestID(segData.AuthToken.SessionId), payment.GetExpectedPrice(), pixels)
	if monitor.Enabled {
		monitor.MilPixelsProcessed(ctx, float64(pixels)/1000000.0)
	}

	// construct the response
	var result net.TranscodeResult
	if err != nil {
		clog.Errorf(ctx, "Could not transcode err=%q", err)
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
		Info:   oInfo,
	}
	buf, err := proto.Marshal(tr)
	if err != nil {
		clog.Errorf(ctx, "Unable to marshal transcode result err=%q", err)
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

func makeFfmpegVideoProfiles(protoProfiles []*net.VideoProfile) ([]ffmpeg.VideoProfile, error) {
	profiles := make([]ffmpeg.VideoProfile, 0, len(protoProfiles))
	for _, profile := range protoProfiles {
		name := profile.Name
		if name == "" {
			name = "net_" + ffmpeg.DefaultProfileName(int(profile.Width), int(profile.Height), int(profile.Bitrate))
		}
		format := ffmpeg.FormatMPEGTS
		switch profile.Format {
		case net.VideoProfile_MPEGTS:
		case net.VideoProfile_MP4:
			format = ffmpeg.FormatMP4
		default:
			return nil, errFormat
		}
		encoderProf := ffmpeg.ProfileNone
		switch profile.Profile {
		case net.VideoProfile_ENCODER_DEFAULT:
		case net.VideoProfile_H264_BASELINE:
			encoderProf = ffmpeg.ProfileH264Baseline
		case net.VideoProfile_H264_MAIN:
			encoderProf = ffmpeg.ProfileH264Main
		case net.VideoProfile_H264_HIGH:
			encoderProf = ffmpeg.ProfileH264High
		case net.VideoProfile_H264_CONSTRAINED_HIGH:
			encoderProf = ffmpeg.ProfileH264ConstrainedHigh
		default:
			return nil, errProfile
		}
		encoder := ffmpeg.H264
		switch profile.Encoder {
		case net.VideoProfile_H264:
			encoder = ffmpeg.H264
		case net.VideoProfile_H265:
			encoder = ffmpeg.H265
		case net.VideoProfile_VP8:
			encoder = ffmpeg.VP8
		case net.VideoProfile_VP9:
			encoder = ffmpeg.VP9
		default:
			return nil, errEncoder
		}
		var gop time.Duration
		if profile.Gop < 0 {
			gop = time.Duration(profile.Gop)
		} else {
			gop = time.Duration(profile.Gop) * time.Millisecond
		}
		prof := ffmpeg.VideoProfile{
			Name:         name,
			Bitrate:      fmt.Sprint(profile.Bitrate),
			Framerate:    uint(profile.Fps),
			FramerateDen: uint(profile.FpsDen),
			Resolution:   fmt.Sprintf("%dx%d", profile.Width, profile.Height),
			Format:       format,
			Profile:      encoderProf,
			GOP:          gop,
			Encoder:      encoder,
			Quality:      uint(profile.Quality),
		}
		profiles = append(profiles, prof)
	}
	return profiles, nil
}

func verifySegCreds(ctx context.Context, orch Orchestrator, segCreds string, broadcaster ethcommon.Address) (*core.SegTranscodingMetadata, context.Context, error) {
	buf, err := base64.StdEncoding.DecodeString(segCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, ctx, errSegEncoding
	}
	var segData net.SegData
	err = proto.Unmarshal(buf, &segData)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, ctx, err
	}

	md, err := coreSegMetadata(&segData)
	if err != nil {
		return nil, ctx, err
	}
	ctx = clog.AddManifestID(ctx, string(md.ManifestID))

	if !orch.VerifySig(broadcaster, string(md.Flatten()), segData.Sig) {
		clog.Errorf(ctx, "Sig check failed")
		return nil, ctx, errSegSig
	}

	if !md.Caps.CompatibleWith(orch.Capabilities()) {
		clog.Errorf(ctx, "Capability check failed")
		return nil, ctx, errCapCompat
	}

	// Check that auth token is valid and not expired
	if segData.AuthToken == nil {
		return nil, ctx, errors.New("missing auth token")
	}

	verifyToken := orch.AuthToken(segData.AuthToken.SessionId, segData.AuthToken.Expiration)
	if !bytes.Equal(verifyToken.Token, segData.AuthToken.Token) {
		return nil, ctx, errors.New("invalid auth token")
	}
	ctx = clog.AddOrchSessionID(ctx, segData.AuthToken.SessionId)

	expiration := time.Unix(segData.AuthToken.Expiration, 0)
	if time.Now().After(expiration) {
		return nil, ctx, errors.New("expired auth token")
	}

	if err := orch.CheckCapacity(core.ManifestID(segData.AuthToken.SessionId)); err != nil {
		clog.Errorf(ctx, "Cannot process manifest err=%q", err)
		return nil, ctx, err
	}

	return md, ctx, nil
}

func SubmitSegment(ctx context.Context, sess *BroadcastSession, seg *stream.HLSSegment, segPar *core.SegmentParameters,
	nonce uint64, calcPerceptualHash, verified bool) (*ReceivedTranscodeResult, error) {

	uploaded := seg.Name != "" // hijack seg.Name to convey the uploaded URI
	if sess.OrchestratorInfo != nil {
		if sess.OrchestratorInfo.AuthToken != nil {
			ctx = clog.AddOrchSessionID(ctx, sess.OrchestratorInfo.AuthToken.SessionId)
		}
		ctx = clog.AddVal(ctx, "orchestrator", sess.OrchestratorInfo.Transcoder)
	}

	segCreds, err := genSegCreds(sess, seg, segPar, calcPerceptualHash)
	if err != nil {
		if monitor.Enabled {
			monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorGenCreds, err, false, sess.OrchestratorInfo.Transcoder)
		}
		return nil, err
	}
	data := seg.Data
	if uploaded {
		data = []byte(seg.Name)
	}

	priceInfo, err := common.RatPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
	if err != nil {
		return nil, err
	}

	params := sess.Params
	fee, err := estimateFee(seg, params.Profiles, priceInfo)
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

	payment, err := genPayment(ctx, sess, balUpdate.NumTickets)
	if err != nil {
		clog.Errorf(ctx, "Could not create payment bytes=%v err=%q", len(data), err)

		if monitor.Enabled {
			monitor.PaymentCreateError(ctx)
		}

		return nil, err
	}

	// timeout for the whole HTTP call: segment upload, transcoding, reading response
	httpTimeout := common.HTTPTimeout
	// set a minimum timeout to accommodate transport / processing overhead
	paddedDur := common.SegHttpPushTimeoutMultiplier * seg.Duration
	if paddedDur > httpTimeout.Seconds() {
		httpTimeout = time.Duration(paddedDur * float64(time.Second))
	}
	// timeout for the segment upload, until HTTP returns OK 200
	uploadTimeout := time.Duration(common.SegUploadTimeoutMultiplier * seg.Duration * float64(time.Second))
	if uploadTimeout < common.MinSegmentUploadTimeout {
		uploadTimeout = common.MinSegmentUploadTimeout
	}
	if params.TimeoutMultiplier > 1 {
		uploadTimeout = time.Duration(params.TimeoutMultiplier) * uploadTimeout
		httpTimeout = time.Duration(params.TimeoutMultiplier) * httpTimeout
	}

	ctx, cancel := context.WithTimeout(clog.Clone(context.Background(), ctx), httpTimeout)
	defer cancel()

	ti := sess.OrchestratorInfo

	req, err := http.NewRequestWithContext(ctx, "POST", ti.Transcoder+"/segment", bytes.NewBuffer(data))
	if err != nil {
		clog.Errorf(ctx, "Could not generate transcode request to orch=%s", ti.Transcoder)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorGenCreds, err, false, sess.OrchestratorInfo.Transcoder)
		}
		return nil, err
	}

	req.Header.Set(segmentHeader, segCreds)
	req.Header.Set(paymentHeader, payment)
	if uploaded {
		req.Header.Set("Content-Type", "application/vnd+livepeer.uri")
	} else {
		// Technically incorrect for MP4 uploads but doesn't really matter
		// TODO should we set this to some generic "Livepeer video" type?
		req.Header.Set("Content-Type", "video/MP2T")
	}

	clog.Infof(ctx, "Submitting segment bytes=%v orch=%s timeout=%s uploadTimeout=%s segDur=%v",
		len(data), ti.Transcoder, httpTimeout, uploadTimeout, seg.Duration)
	start := time.Now()
	resp, err := sendReqWithTimeout(req, uploadTimeout)
	uploadDur := time.Since(start)
	if err != nil {
		clog.Errorf(ctx, "Unable to submit segment orch=%v orch=%s uploadDur=%s err=%q", ti.Transcoder, ti.Transcoder, uploadDur, err)
		if monitor.Enabled {
			monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorUnknown, err, false, sess.OrchestratorInfo.Transcoder)
		}
		return nil, fmt.Errorf("header timeout: %w", err)
	}
	defer resp.Body.Close()

	// If the segment was submitted then we assume that any payment included was
	// submitted as well so we consider the update's credit as spent
	balUpdate.Status = CreditSpent
	if monitor.Enabled {
		monitor.TicketValueSent(ctx, balUpdate.NewCredit)
		monitor.TicketsSent(ctx, balUpdate.NumTickets)
	}

	if resp.StatusCode != 200 {
		data, _ := ioutil.ReadAll(resp.Body)
		errorString := strings.TrimSpace(string(data))
		clog.Errorf(ctx, "Error submitting segment statusCode=%d orch=%s err=%q", resp.StatusCode, ti.Transcoder, string(data))
		if monitor.Enabled {
			if resp.StatusCode == 403 && strings.Contains(errorString, "OrchestratorCapped") {
				monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadErrorOrchestratorCapped, errors.New(errorString), false, sess.OrchestratorInfo.Transcoder)
			} else {
				monitor.SegmentUploadFailed(ctx, nonce, seg.SeqNo, monitor.SegmentUploadError(resp.Status),
					fmt.Errorf("Code: %d Error: %s", resp.StatusCode, errorString), false, sess.OrchestratorInfo.Transcoder)
			}
		}
		return nil, fmt.Errorf(errorString)
	}
	clog.Infof(ctx, "Uploaded segment orch=%s dur=%s", ti.Transcoder, uploadDur)
	if monitor.Enabled {
		monitor.SegmentUploaded(ctx, nonce, seg.SeqNo, uploadDur, ti.Transcoder)
	}

	data, err = ioutil.ReadAll(resp.Body)
	tookAllDur := time.Since(start)
	if err != nil {
		clog.Errorf(ctx, "Unable to read response body for segment orch=%s err=%q", ti.Transcoder, err)
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorReadBody, nonce, seg.SeqNo, err, false)
		}
		return nil, fmt.Errorf("body timeout: %w", err)
	}
	transcodeDur := tookAllDur - uploadDur

	var tr net.TranscodeResult
	err = proto.Unmarshal(data, &tr)
	if err != nil {
		clog.Errorf(ctx, "Unable to parse response for segment orch=%s err=%q", ti.Transcoder, err)
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorParseResponse, nonce, seg.SeqNo, err, false)
		}
		return nil, err
	}

	// check for errors and exit early if there's anything unusual
	var tdata *net.TranscodeData
	switch res := tr.Result.(type) {
	case *net.TranscodeResult_Error:
		err = fmt.Errorf(res.Error)
		clog.Errorf(ctx, "Transcode failed for segment orch=%s err=%q", ti.Transcoder, err)
		if err.Error() == "MediaStats Failure" {
			clog.Infof(ctx, "Ensure the keyframe interval is 4 seconds or less")
		}
		if monitor.Enabled {
			switch res.Error {
			case "OrchestratorBusy":
				monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorOrchestratorBusy, nonce, seg.SeqNo, err, false)
			case "OrchestratorCapped":
				monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorOrchestratorCapped, nonce, seg.SeqNo, err, false)
			default:
				monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorTranscode, nonce, seg.SeqNo, err, false)
			}
		}
		return nil, err
	case *net.TranscodeResult_Data:
		// fall through here for the normal case
		tdata = res.Data
	default:
		clog.Errorf(ctx, "Unexpected or unset transcode response field for orch=%s", ti.Transcoder)
		err = fmt.Errorf("UnknownResponse")
		if monitor.Enabled {
			monitor.SegmentTranscodeFailed(ctx, monitor.SegmentTranscodeErrorUnknownResponse, nonce, seg.SeqNo, err, false)
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

		if monitor.Enabled {
			monitor.MilPixelsProcessed(ctx, float64(pixelCount)/1000000.0)
		}
	}

	// transcode succeeded; continue processing response
	if monitor.Enabled {
		monitor.SegmentTranscoded(ctx, nonce, seg.SeqNo, time.Duration(seg.Duration*float64(time.Second)), transcodeDur,
			common.ProfilesNames(params.Profiles), sess.IsTrusted(), verified)
	}

	clog.Infof(ctx, "Successfully transcoded segment segName=%s seqNo=%d orch=%s dur=%s",
		seg.Name, seg.SeqNo, ti.Transcoder, transcodeDur)

	// Use 1.5s for segments that are shorter than 1.5s
	// Otherwise, the latency score is too high which results in a high number session swaps
	segDuration := math.Max(1.5, seg.Duration)
	return &ReceivedTranscodeResult{
		TranscodeData: tdata,
		Info:          tr.Info,
		LatencyScore:  tookAllDur.Seconds() / segDuration,
	}, nil
}

func genSegCreds(sess *BroadcastSession, seg *stream.HLSSegment, segPar *core.SegmentParameters, calcPerceptualHash bool) (string, error) {

	// Send credentials for our own storage
	var storage *net.OSInfo
	if bos := sess.BroadcasterOS; bos != nil && bos.IsExternal() {
		storage = core.ToNetOSInfo(bos.GetInfo())
	}

	// Generate signature for relevant parts of segment
	params := sess.Params
	hash := crypto.Keccak256(seg.Data)
	md := &core.SegTranscodingMetadata{
		ManifestID:         params.ManifestID,
		Seq:                int64(seg.SeqNo),
		Hash:               ethcommon.BytesToHash(hash),
		Profiles:           params.Profiles,
		OS:                 storage,
		Duration:           time.Duration(seg.Duration * float64(time.Second)),
		Caps:               params.Capabilities,
		AuthToken:          sess.OrchestratorInfo.GetAuthToken(),
		CalcPerceptualHash: calcPerceptualHash,
		SegmentParameters:  segPar,
	}
	sig, err := sess.Broadcaster.Sign(md.Flatten())
	if err != nil {
		return "", err
	}

	// convert to segData
	segData, err := core.NetSegData(md)
	if err != nil {
		return "", err
	}
	segData.Sig = sig

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
		framerate := p.Framerate
		if framerate == 0 {
			// FPS is being passed through (no fps adjustment)
			// TODO incorporate the actual number of frames from the input
			framerate = 60 // conservative estimate of input fps
		}
		framerateDen := p.FramerateDen
		if framerateDen == 0 {
			// Denominator not set, treat as 1
			framerateDen = 1
		}
		// Take ceilings, as it is better to overestimate
		fps := math.Ceil((float64(framerate) / float64(framerateDen)))
		outPixels += int64(w*h) * int64(fps) * int64(math.Ceil(seg.Duration))
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

func genPayment(ctx context.Context, sess *BroadcastSession, numTickets int) (string, error) {
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
			ExpirationBlock:   batch.ExpirationBlock.Bytes(),
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

		ratPrice, _ := common.RatPriceInfo(protoPayment.ExpectedPrice)
		clog.Infof(ctx, "Created new payment - manifestID=%v sessionID=%v recipient=%v faceValue=%v winProb=%v price=%v numTickets=%v",
			sess.Params.ManifestID,
			sess.OrchestratorInfo.AuthToken.SessionId,
			batch.Recipient.Hex(),
			eth.FormatUnits(batch.FaceValue, "ETH"),
			batch.WinProbRat().FloatString(10),
			ratPrice.FloatString(3)+" wei/pixel",
			numTickets,
		)
	}

	data, err := proto.Marshal(protoPayment)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func validatePrice(sess *BroadcastSession) error {
	oPrice, err := common.RatPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
	if err != nil {
		return err
	}
	if oPrice == nil {
		return errors.New("missing orchestrator price")
	}

	initPrice, err := common.RatPriceInfo(sess.InitialPrice)
	if err != nil {
		glog.Warningf("Error parsing session initial price (%d / %d): %v",
			sess.InitialPrice.PricePerUnit, sess.InitialPrice.PixelsPerUnit, err)
	}
	if initPrice != nil {
		// Prices are dynamic if configured with a custom currency, so we need to allow some change during the session.
		// TODO: Make sure prices stay the same during a session so we can make this logic more strict, disallowing any price changes.
		maxIncreasedPrice := new(big.Rat).Mul(initPrice, priceIncreaseThreshold)
		if oPrice.Cmp(maxIncreasedPrice) > 0 {
			return fmt.Errorf("Orchestrator price has more than doubled, Orchestrator price: %v, Orchestrator initial price: %v", oPrice.RatString(), initPrice.RatString())
		}
	}

	return nil
}

func sendReqWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	timeouter := time.AfterFunc(timeout, cancel)

	req = req.WithContext(ctx)
	resp, err := httpClient.Do(req)
	if timeouter.Stop() {
		return resp, err
	}
	// timeout has already fired and cancelled the request
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return nil, context.DeadlineExceeded
}
