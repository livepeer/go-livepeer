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

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/stream"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

const HTTPTimeout = 8 * time.Second
const GRPCTimeout = 8 * time.Second

const AuthType_LPE = "Livepeer-Eth-1"

type Orchestrator interface {
	ServiceURI() *url.URL
	Address() ethcommon.Address
	Sign([]byte) ([]byte, error)
	CurrentBlock() *big.Int
	GetJob(int64) (*lpTypes.Job, error)
	TranscodeSeg(*lpTypes.Job, *core.SignedSegment) error
	StreamIDs(*lpTypes.Job) ([]core.StreamID, error)
}

type Broadcaster interface {
	Sign([]byte) ([]byte, error)
	Job() *lpTypes.Job
	SetHTTPClient(*http.Client)
	GetHTTPClient() *http.Client
	SetTranscoderInfo(interface{})
	GetTranscoderInfo() interface{}
}

func genTranscoderReq(b Broadcaster, jid int64) (*TranscoderRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", jid)))
	if err != nil {
		return nil, err
	}
	return &TranscoderRequest{JobId: jid, Sig: sig}, nil
}

func blockInRange(orch Orchestrator, job *lpTypes.Job) bool {
	blk := orch.CurrentBlock()
	if blk == nil {
		// The benefit of doubt.
		// May be offchain or have internal errors in fetching a block.
		return true
	}
	return !(blk.Cmp(job.CreationBlock) == -1 || blk.Cmp(job.EndBlock) == 1)
}

func verifyMsgSig(addr ethcommon.Address, msg string, sig []byte) bool {
	return eth.VerifySig(addr, crypto.Keccak256([]byte(msg)), sig)
}

func verifyTranscoderReq(orch Orchestrator, req *TranscoderRequest, job *lpTypes.Job) error {
	if orch.Address() != job.TranscoderAddress {
		glog.Error("Transcoder was not assigned")
		return fmt.Errorf("Transcoder was not assigned")
	}
	if !blockInRange(orch, job) {
		glog.Error("Job out of range")
		return fmt.Errorf("Job out of range")
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
	data, err := proto.Marshal(&AuthToken{JobId: job.JobId.Int64(), Sig: sig})
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
	var token AuthToken
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
	if !blockInRange(orch, job) {
		glog.Errorf("Job %v too early or expired", job.JobId)
		return nil, fmt.Errorf("Job out of range")
	}
	return job, nil
}

func genSegCreds(bcast Broadcaster, streamId string, segData *SegData) (string, error) {
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

func verifySegCreds(job *lpTypes.Job, segCreds string) (*SegData, error) {
	buf, err := base64.StdEncoding.DecodeString(segCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, err
	}
	var segData SegData
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

func GetTranscoder(context context.Context, orch Orchestrator, req *TranscoderRequest) (*TranscoderInfo, error) {
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

	tr := TranscoderInfo{
		Transcoder:  orch.ServiceURI().String(), // currently,  orchestrator == transcoder
		AuthType:    AuthType_LPE,
		Credentials: creds,
		StreamIds:   stringStreamIds,
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
		glog.Error("Could not read request body")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
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

	ss := core.SignedSegment{
		Seg: stream.HLSSegment{
			SeqNo: uint64(segData.Seq),
			Data:  data,
		},
		Sig: segData.Sig,
	}
	if err := orch.TranscodeSeg(job, &ss); err != nil {
		glog.Error("Could not transcode ", err)
		w.Write([]byte(fmt.Sprintf("Error transcoding segment %v : %v", segData.Seq, err)))
		return
	}
	w.Write([]byte(fmt.Sprintf("Successfully transcoded segment %v", segData.Seq)))
}

// grpc methods
func (h *lphttp) GetTranscoder(context context.Context, req *TranscoderRequest) (*TranscoderInfo, error) {
	return GetTranscoder(context, h.orchestrator, req)
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
	RegisterOrchestratorServer(s, &lp)
	lp.transRpc.HandleFunc("/segment", lp.ServeSegment)

	cert, key, err := getCert(orch.ServiceURI(), workDir)
	if err != nil {
		return // XXX return error
	}

	glog.Info("Listening for RPC on ", bind)
	srv := http.Server{
		Addr:         bind,
		Handler:      &lp,
		ReadTimeout:  HTTPTimeout,
		WriteTimeout: HTTPTimeout,
	}
	srv.ListenAndServeTLS(cert, key)
}

func StartBroadcastClient(bcast Broadcaster, orchestratorServer string) error {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	httpc := &http.Client{
		Transport: &http2.Transport{TLSClientConfig: tlsConfig},
		Timeout:   HTTPTimeout,
	}
	uri, err := url.Parse(orchestratorServer)
	if err != nil {
		glog.Error("Could not parse orchestrator URI: ", err)
		return err
	}
	glog.Infof("Connecting RPC to %v", orchestratorServer)
	conn, err := grpc.Dial(uri.Host,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		glog.Error("Did not connect: ", err)
		return errors.New("Did not connect: " + err.Error())
	}
	defer conn.Close()
	c := NewOrchestratorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	bcast.SetHTTPClient(httpc)
	req, err := genTranscoderReq(bcast, bcast.Job().JobId.Int64())
	r, err := c.GetTranscoder(ctx, req)
	if err != nil {
		glog.Error("Could not get transcoder: ", err)
		return errors.New("Could not get transcoder: " + err.Error())
	}
	bcast.SetTranscoderInfo(r)

	return nil
}

func SubmitSegment(bcast Broadcaster, seg *stream.HLSSegment, nonce uint64) {
	if monitor.Enabled {
		monitor.SegmentUploadStart(nonce, seg.SeqNo)
	}
	hc := bcast.GetHTTPClient()
	segData := &SegData{
		Seq:  int64(seg.SeqNo),
		Hash: crypto.Keccak256(seg.Data),
	}
	segCreds, err := genSegCreds(bcast, bcast.Job().StreamId, segData)
	if err != nil {
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return
	}
	ti := (bcast.GetTranscoderInfo()).(*TranscoderInfo)
	req, err := http.NewRequest("POST", ti.Transcoder+"/segment", bytes.NewBuffer(seg.Data))
	if err != nil {
		glog.Error("Could not generate trascode request to ", ti.Transcoder)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return
	}

	req.Header.Set("Authorization", ti.AuthType)
	req.Header.Set("Credentials", ti.Credentials)
	req.Header.Set("Livepeer-Segment", segCreds)
	req.Header.Set("Content-Type", "video/MP2T")

	glog.Infof("Submitting segment %v : %v bytes", seg.SeqNo, len(seg.Data))
	start := time.Now()
	resp, err := hc.Do(req)
	uploadDur := time.Since(start)
	if err != nil {
		glog.Error("Unable to submit segment ", seg.SeqNo, err)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return
	}
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		data, _ := ioutil.ReadAll(resp.Body)
		glog.Errorf("Error submitting segment %d: code %d error %s", seg.SeqNo, resp.StatusCode, string(data))
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, fmt.Sprintf("Code: %d Error: %s", resp.StatusCode,
				strings.TrimSpace(string(data))))
		}
		return
	}
	glog.Infof("Uploaded segment %v", seg.SeqNo)
	if monitor.Enabled {
		monitor.LogSegmentUploaded(nonce, seg.SeqNo, uploadDur)
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	tookAllDur := time.Since(start)
	if err != nil {
		glog.Error(fmt.Sprintf("Unable to read response body for segment %v : %v", seg.SeqNo, err))
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed(nonce, seg.SeqNo, err.Error())
		}
		return
	}
	transcodeDur := tookAllDur - uploadDur

	dataStr := string(data)
	glog.Infof("Response for segment %v: %s", seg.SeqNo, dataStr)
	if strings.Contains(dataStr, "Error transcoding segment") {
		if monitor.Enabled {
			monitor.LogSegmentTranscodeFailed(nonce, seg.SeqNo, dataStr[strings.Index(dataStr, ":")+2:])
		}
	} else {
		if monitor.Enabled {
			monitor.LogSegmentTranscoded(nonce, seg.SeqNo, transcodeDur, tookAllDur)
		}
	}
}
