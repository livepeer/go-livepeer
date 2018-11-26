package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

const HTTPTimeout = 8 * time.Second
const GRPCTimeout = 8 * time.Second
const GRPCConnectTimeout = 3 * time.Second

const AuthType_LPE = "Livepeer-Eth-1"

const JobOutOfRangeError = "Job out of range"

type Orchestrator interface {
	ServiceURI() *url.URL
	Address() ethcommon.Address
	TranscoderSecret() string
	Sign([]byte) ([]byte, error)
	CurrentBlock() *big.Int
	GetJob(int64) (*lpTypes.Job, error)
	TranscodeSeg(*lpTypes.Job, *core.SignedSegment) (*core.TranscodeResult, error)
	StreamIDs(*lpTypes.Job) ([]core.StreamID, error)

	ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer)
	TranscoderResults(job int64, res *core.RemoteTranscoderResult)
}

type Broadcaster interface {
	Sign([]byte) ([]byte, error)
	Job() *lpTypes.Job
	SetHTTPClient(*http.Client)
	GetHTTPClient() *http.Client
	SetTranscoderInfo(*net.TranscoderInfo)
	GetTranscoderInfo() *net.TranscoderInfo
	SetOrchestratorOS(drivers.OSSession)
	GetOrchestratorOS() drivers.OSSession
	SetBroadcasterOS(drivers.OSSession)
	GetBroadcasterOS() drivers.OSSession
}

func genTranscoderReq(b Broadcaster, jid int64) (*net.TranscoderRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", jid)))
	if err != nil {
		return nil, err
	}
	return &net.TranscoderRequest{JobId: jid, Sig: sig}, nil
}

func CheckTranscoderAvailability(orch Orchestrator) bool {
	ts := time.Now()
	ts_signature, err := orch.Sign([]byte(fmt.Sprintf("%v", ts)))
	if err != nil {
		return false
	}

	ping := crypto.Keccak256(ts_signature)

	orch_client, conn, err := startOrchestratorClient(orch.ServiceURI().String())
	if err != nil {
		return false
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	pong, err := orch_client.Ping(ctx, &net.PingPong{Value: ping})
	if err != nil {
		glog.Error("Was not able to submit Ping: ", err)
		return false
	}

	return verifyMsgSig(orch.Address(), string(ping), pong.Value)
}

func startOrchestratorClient(url_string string) (net.OrchestratorClient, *grpc.ClientConn, error) {
	uri, err := url.Parse(url_string)

	if err != nil {
		glog.Error("Could not parse orchestrator URI: ", err)
		return nil, nil, err
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	glog.Infof("Connecting RPC to %v", uri)
	conn, err := grpc.Dial(uri.Host,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithTimeout(GRPCConnectTimeout))
	if err != nil {
		glog.Error("Did not connect: ", err)
		return nil, nil, errors.New("Did not connect: " + err.Error())
	}
	c := net.NewOrchestratorClient(conn)

	return c, conn, nil
}

func jobClaimable(orch Orchestrator, job *lpTypes.Job) bool {
	if len(job.Profiles) <= 0 {
		// This is just to be extra cautious:
		// We don't need to do any work, so nothing to claim
		return false
	}

	blk := orch.CurrentBlock()
	if blk == nil {
		// The benefit of doubt.
		// May be offchain or have internal errors in fetching a block.
		return true
	}

	canClaim := job.FirstClaimSubmitted || blk.Cmp(new(big.Int).Add(job.CreationBlock, big.NewInt(256))) <= 0

	return canClaim && !(blk.Cmp(job.CreationBlock) == -1 || blk.Cmp(job.EndBlock) == 1)
}

func verifyMsgSig(addr ethcommon.Address, msg string, sig []byte) bool {
	return eth.VerifySig(addr, crypto.Keccak256([]byte(msg)), sig)
}

func verifyTranscoderReq(orch Orchestrator, req *net.TranscoderRequest, job *lpTypes.Job) error {
	if orch.Address() != job.TranscoderAddress {
		glog.Error("Transcoder was not assigned")
		return fmt.Errorf("Transcoder was not assigned")
	}
	if !jobClaimable(orch, job) {
		glog.Error(JobOutOfRangeError)
		return fmt.Errorf(JobOutOfRangeError)
	}
	if !verifyMsgSig(job.BroadcasterAddress, fmt.Sprintf("%v", job.JobId), req.Sig) {
		glog.Error("transcoder req sig check failed")
		return fmt.Errorf("transcoder req sig check failed")
	}
	return nil
}

func genToken(orch Orchestrator, job *lpTypes.Job) (string, error) {
	sig, err := orch.Sign([]byte(fmt.Sprintf("%v", job.JobId)))
	if err != nil {
		return "", err
	}
	data, err := proto.Marshal(&net.AuthToken{JobId: job.JobId.Int64(), Sig: sig})
	if err != nil {
		glog.Error("Unable to marshal ", err)
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func verifyToken(orch Orchestrator, creds string) (*lpTypes.Job, error) {
	buf, err := base64.StdEncoding.DecodeString(creds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, err
	}
	var token net.AuthToken
	err = proto.Unmarshal(buf, &token)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}
	if !verifyMsgSig(orch.Address(), fmt.Sprintf("%v", token.JobId), token.Sig) {
		glog.Error("Sig check failed")
		return nil, fmt.Errorf("Token sig check failed")
	}
	job, err := orch.GetJob(token.JobId)
	if err != nil || job == nil {
		glog.Error("Could not get job ", err)
		return nil, fmt.Errorf("Missing job (%s)", err.Error())
	}
	if !jobClaimable(orch, job) {
		glog.Errorf("Job %v too early or expired", job.JobId)
		return nil, fmt.Errorf(JobOutOfRangeError)
	}
	return job, nil
}

func genSegCreds(bcast Broadcaster, streamId string, segData *net.SegData) (string, error) {
	seg := &lpTypes.Segment{
		StreamID:              streamId,
		SegmentSequenceNumber: big.NewInt(segData.Seq),
		DataHash:              ethcommon.BytesToHash(segData.Hash),
	}
	sig, err := bcast.Sign(seg.Flatten())
	if err != nil {
		return "", nil
	}
	segData.Sig = sig
	data, err := proto.Marshal(segData)
	if err != nil {
		glog.Error("Unable to marshal ", err)
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func verifySegCreds(job *lpTypes.Job, segCreds string) (*net.SegData, error) {
	buf, err := base64.StdEncoding.DecodeString(segCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, err
	}
	var segData net.SegData
	err = proto.Unmarshal(buf, &segData)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}
	seg := &lpTypes.Segment{
		StreamID:              job.StreamId,
		SegmentSequenceNumber: big.NewInt(segData.Seq),
		DataHash:              ethcommon.BytesToHash(segData.Hash),
	}
	if !verifyMsgSig(job.BroadcasterAddress, string(seg.Flatten()), segData.Sig) {
		glog.Error("Sig check failed")
		return nil, fmt.Errorf("Segment sig check failed")
	}
	return &segData, nil
}

func ping(context context.Context, req *net.PingPong, orch Orchestrator) (*net.PingPong, error) {
	glog.Info("Received Ping request")
	value, err := orch.Sign(req.Value)
	if err != nil {
		glog.Error("Unable to sign Ping request")
		return nil, err
	}
	return &net.PingPong{Value: value}, nil
}

func getTranscoder(context context.Context, orch Orchestrator, req *net.TranscoderRequest) (*net.TranscoderInfo, error) {
	glog.Info("Got transcoder request for job ", req.JobId)
	job, err := orch.GetJob(req.JobId)
	if err != nil {
		glog.Error("Unable to get job ", err)
		return nil, fmt.Errorf("Unable to get job (%s)", err.Error())
	}
	if err := verifyTranscoderReq(orch, req, job); err != nil {
		return nil, fmt.Errorf("Invalid transcoder request (%v)", err)
	}
	creds, err := genToken(orch, job)
	if err != nil {
		return nil, err
	}
	sids, err := orch.StreamIDs(job)
	if err != nil {
		return nil, err
	}
	stringStreamIds := make(map[string]string)
	for i, s := range sids {
		stringStreamIds[s.String()] = job.Profiles[i].Name
	}
	tr := net.TranscoderInfo{
		Transcoder:  orch.ServiceURI().String(), // currently,  orchestrator == transcoder
		AuthType:    AuthType_LPE,
		Credentials: creds,
		StreamIds:   stringStreamIds,
	}
	mid, err := core.StreamID(job.StreamId).ManifestIDFromStreamID()
	if err != nil {
		return nil, err
	}
	os := drivers.NodeStorage.NewSession(string(mid))
	if os != nil && os.IsExternal() {
		tr.Storage = []*net.OSInfo{os.GetInfo()}
	}
	return &tr, nil
}

type lphttp struct {
	orchestrator Orchestrator
	orchRpc      *grpc.Server
	transRpc     *http.ServeMux
}

func (h *lphttp) ServeSegment(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator
	// check the credentials from the orchestrator
	authType := r.Header.Get("Authorization")
	creds := r.Header.Get("Credentials")
	if AuthType_LPE != authType {
		glog.Error("Invalid auth type ", authType)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	job, err := verifyToken(orch, creds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// check the segment sig from the broadcaster
	seg := r.Header.Get("Livepeer-Segment")
	segData, err := verifySegCreds(job, seg)
	if err != nil {
		glog.Error("Could not verify segment creds")
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// download the segment and check the hash
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Error("Could not read request body", err)
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
		}
	}

	hash := crypto.Keccak256(data)
	if !bytes.Equal(hash, segData.Hash) {
		glog.Error("Mismatched hash for body; rejecting")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Send down 200OK early as an indication that the upload completed
	// Any further errors come through the response body
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()

	var prefOS *net.OSInfo
	if len(segData.Storage) > 0 {
		prefOS = segData.Storage[0]
	}
	ss := core.SignedSegment{
		Seg: stream.HLSSegment{
			SeqNo: uint64(segData.Seq),
			Data:  data,
			Name:  uri,
		},
		Sig: segData.Sig,
		OS:  prefOS,
	}

	res, err := orch.TranscodeSeg(job, &ss)

	// sanity check
	if err == nil && len(res.Data) != len(job.Profiles) {
		err = fmt.Errorf("Mismatched result lengths")
	}

	// Upload to OS and construct segment result set
	var segments []*net.TranscodedSegmentData
	for i := 0; err == nil && i < len(res.Data); i++ {
		name := fmt.Sprintf("%s/%d.ts", job.Profiles[i].Name, segData.Seq)
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
	}
	buf, err := proto.Marshal(tr)
	if err != nil {
		glog.Error("Unable to marshal transcode result ", err)
		return
	}
	w.Write(buf)
}

// grpc methods
func (h *lphttp) GetTranscoder(context context.Context, req *net.TranscoderRequest) (*net.TranscoderInfo, error) {
	return getTranscoder(context, h.orchestrator, req)
}

func (h *lphttp) Ping(context context.Context, req *net.PingPong) (*net.PingPong, error) {
	return ping(context, req, h.orchestrator)
}

func (h *lphttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if r.ProtoMajor == 2 && strings.HasPrefix(ct, "application/grpc") {
		h.orchRpc.ServeHTTP(w, r)
	} else {
		h.transRpc.ServeHTTP(w, r)
	}
}

// XXX do something about the implicit start of the http mux? this smells
func StartTranscodeServer(orch Orchestrator, bind string, mux *http.ServeMux, workDir string) {
	s := grpc.NewServer()
	lp := lphttp{
		orchestrator: orch,
		orchRpc:      s,
		transRpc:     mux,
	}
	net.RegisterOrchestratorServer(s, &lp)
	net.RegisterTranscoderServer(s, &lp)
	lp.transRpc.HandleFunc("/segment", lp.ServeSegment)
	lp.transRpc.HandleFunc("/transcodeResults", lp.TranscodeResults)

	cert, key, err := getCert(orch.ServiceURI(), workDir)
	if err != nil {
		return // XXX return error
	}

	glog.Info("Listening for RPC on ", bind)
	srv := http.Server{
		Addr:    bind,
		Handler: &lp,
		// XXX doesn't handle streaming RPC well; split remote transcoder RPC?
		//ReadTimeout:  HTTPTimeout,
		//WriteTimeout: HTTPTimeout,
	}
	srv.ListenAndServeTLS(cert, key)
}

func StartBroadcastClient(bcast Broadcaster, orchestratorServer string) error {
	c, conn, err := startOrchestratorClient(orchestratorServer)
	if err != nil {
		return err
	}
	defer conn.Close()
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	httpc := &http.Client{
		Transport: &http2.Transport{TLSClientConfig: tlsConfig},
		Timeout:   HTTPTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	bcast.SetHTTPClient(httpc)
	req, err := genTranscoderReq(bcast, bcast.Job().JobId.Int64())
	r, err := c.GetTranscoder(ctx, req)
	if err != nil {
		glog.Errorf("Could not get transcoder for job %d: %s", bcast.Job().JobId.Int64(), err.Error())
		return errors.New("Could not get transcoder: " + err.Error())
	}
	bcast.SetTranscoderInfo(r)

	return nil
}

func SubmitSegment(bcast Broadcaster, seg *stream.HLSSegment, nonce uint64) (*net.TranscodeData, error) {
	if monitor.Enabled {
		monitor.SegmentUploadStart(nonce, seg.SeqNo)
	}
	hc := bcast.GetHTTPClient()
	segData := &net.SegData{
		Seq:  int64(seg.SeqNo),
		Hash: crypto.Keccak256(seg.Data),
	}
	uploaded := seg.Name != "" // hijack seg.Name to convey the uploaded URI

	// send credentials for our own storage
	if bos := bcast.GetBroadcasterOS(); bos != nil && bos.IsExternal() {
		segData.Storage = []*net.OSInfo{bos.GetInfo()}
	}

	segCreds, err := genSegCreds(bcast, bcast.Job().StreamId, segData)
	if err != nil {
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return nil, err
	}
	data := seg.Data
	if uploaded {
		data = []byte(seg.Name)
	}

	ti := bcast.GetTranscoderInfo()
	req, err := http.NewRequest("POST", ti.Transcoder+"/segment", bytes.NewBuffer(data))
	if err != nil {
		glog.Error("Could not generate trascode request to ", ti.Transcoder)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return nil, err
	}

	req.Header.Set("Authorization", ti.AuthType)
	req.Header.Set("Credentials", ti.Credentials)
	req.Header.Set("Livepeer-Segment", segCreds)
	if uploaded {
		req.Header.Set("Content-Type", "application/vnd+livepeer.uri")
	} else {
		req.Header.Set("Content-Type", "video/MP2T")
	}

	glog.Infof("Submitting segment %v : %v bytes", seg.SeqNo, len(data))
	start := time.Now()
	resp, err := hc.Do(req)
	uploadDur := time.Since(start)
	if err != nil {
		glog.Error("Unable to submit segment ", seg.SeqNo, err)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := ioutil.ReadAll(resp.Body)
		errorString := strings.TrimSpace(string(data))
		glog.Errorf("Error submitting segment %d: code %d error %v", seg.SeqNo, resp.StatusCode, string(data))
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo,
				fmt.Sprintf("Code: %d Error: %s", resp.StatusCode, errorString))
		}
		return nil, fmt.Errorf(errorString)
	}
	glog.Infof("Uploaded segment %v", seg.SeqNo)
	if monitor.Enabled {
		monitor.LogSegmentUploaded(nonce, seg.SeqNo, uploadDur)
	}

	data, err = ioutil.ReadAll(resp.Body)
	tookAllDur := time.Since(start)

	if err != nil {
		glog.Error(fmt.Sprintf("Unable to read response body for segment %v : %v", seg.SeqNo, err))
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed("ReadBody", nonce, seg.SeqNo, err)
		}
		return nil, err
	}
	transcodeDur := tookAllDur - uploadDur

	var tr net.TranscodeResult
	err = proto.Unmarshal(data, &tr)
	if err != nil {
		glog.Error(fmt.Sprintf("Unable to parse response for segment %v : %v", seg.SeqNo, err))
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed("ParseResponse", nonce, seg.SeqNo, err)
		}
		return nil, err
	}

	// check for errors and exit early if there's anything unusual
	var tdata *net.TranscodeData
	switch res := tr.Result.(type) {
	case *net.TranscodeResult_Error:
		err = fmt.Errorf(res.Error)
		glog.Errorf("Transcode failed for segment %v: %v", seg.SeqNo, err)
		if err.Error() == "MediaStats Failure" {
			glog.Info("Ensure the keyframe interval is 4 seconds or less")
		}
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed("Transcode", nonce, seg.SeqNo, err)
		}
		return nil, err
	case *net.TranscodeResult_Data:
		// fall through here for the normal case
		tdata = res.Data
	default:
		glog.Error("Unexpected or unset transcode response field for ", seg.SeqNo)
		err = fmt.Errorf("UnknownResponse")
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed("UnknownResponse", nonce, seg.SeqNo, err)
		}
		return nil, err
	}

	// transcode succeeded; continue processing response
	if monitor.Enabled {
		monitor.LogSegmentTranscoded(nonce, seg.SeqNo, transcodeDur, tookAllDur)
	}

	glog.Info("Successfully transcoded segment ", seg.SeqNo)

	return tdata, nil
}
