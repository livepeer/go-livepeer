package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"golang.org/x/net/http2"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/lpms/stream"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

const HTTPTimeout = 8 * time.Second
const GRPCTimeout = 4 * time.Second

const AuthType_LPE = "Livepeer-Eth-1"

type orchestrator struct {
	transcoder string
	address    ethcommon.Address
	node       *core.LivepeerNode
}

type Orchestrator interface {
	Transcoder() string
	Address() ethcommon.Address
	Sign([]byte) ([]byte, error)
	CurrentBlock() *big.Int
	GetJob(int64) (*lpTypes.Job, error)
	TranscodeSeg(*lpTypes.Job, *core.SignedSegment) error
	StreamIDs(*lpTypes.Job) ([]core.StreamID, error)
}

// Orchestator interface methods
func (orch *orchestrator) Transcoder() string {
	return orch.transcoder
}

func (orch *orchestrator) CurrentBlock() *big.Int {
	if orch.node == nil || orch.node.Eth == nil {
		return nil
	}
	block, _ := orch.node.Eth.LatestBlockNum()
	return block
}

func (orch *orchestrator) GetJob(jid int64) (*lpTypes.Job, error) {
	if orch.node == nil || orch.node.Eth == nil {
		return nil, fmt.Errorf("Cannot get job; missing eth client")
	}
	job, err := orch.node.Eth.GetJob(big.NewInt(jid))
	if err != nil {
		return nil, err
	}
	if (job.TranscoderAddress == ethcommon.Address{}) {
		ta, err := orch.node.Eth.AssignedTranscoder(job)
		if err != nil {
			glog.Error("Could not get assigned transcoder for job %v", jid)
			// continue here without a valid transcoder address
		} else {
			job.TranscoderAddress = ta
		}
	}
	return job, nil
}

func (orch *orchestrator) Sign(msg []byte) ([]byte, error) {
	if orch.node == nil || orch.node.Eth == nil {
		return []byte{}, fmt.Errorf("Cannot sign; missing eth client")
	}
	return orch.node.Eth.Sign(crypto.Keccak256(msg))
}

func (orch *orchestrator) Address() ethcommon.Address {
	return orch.address
}

func (orch *orchestrator) StreamIDs(job *lpTypes.Job) ([]core.StreamID, error) {
	streamIds := make([]core.StreamID, len(job.Profiles))
	sid := core.StreamID(job.StreamId)
	vid := sid.GetVideoID()
	for i, p := range job.Profiles {
		strmId, err := core.MakeStreamID(orch.node.Identity, vid, p.Name)
		if err != nil {
			glog.Error("Error making stream ID: ", err)
			return []core.StreamID{}, err
		}
		streamIds[i] = strmId
	}
	return streamIds, nil
}

func (orch *orchestrator) TranscodeSeg(job *lpTypes.Job, ss *core.SignedSegment) error {
	return orch.node.TranscodeSegment(job, ss)
}

// grpc methods
func (o *orchestrator) GetTranscoder(context context.Context, req *TranscoderRequest) (*TranscoderInfo, error) {
	return GetTranscoder(context, o, req)
}

type broadcaster struct {
	node  *core.LivepeerNode
	httpc *http.Client
	job   *lpTypes.Job
	tinfo *TranscoderInfo
}

type Broadcaster interface {
	Sign([]byte) ([]byte, error)
	Job() *lpTypes.Job
	SetHTTPClient(*http.Client)
	GetHTTPClient() *http.Client
	SetTranscoderInfo(*TranscoderInfo)
	GetTranscoderInfo() *TranscoderInfo
}

func (bcast *broadcaster) Sign(msg []byte) ([]byte, error) {
	if bcast.node == nil || bcast.node.Eth == nil {
		return []byte{}, fmt.Errorf("Cannot sign; missing eth client")
	}
	return bcast.node.Eth.Sign(crypto.Keccak256(msg))
}
func (bcast *broadcaster) Job() *lpTypes.Job {
	return bcast.job
}
func (bcast *broadcaster) GetHTTPClient() *http.Client {
	return bcast.httpc
}
func (bcast *broadcaster) SetHTTPClient(hc *http.Client) {
	bcast.httpc = hc
}
func (bcast *broadcaster) GetTranscoderInfo() *TranscoderInfo {
	return bcast.tinfo
}
func (bcast *broadcaster) SetTranscoderInfo(t *TranscoderInfo) {
	bcast.tinfo = t
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

func verifyTranscoderReq(orch Orchestrator, req *TranscoderRequest, job *lpTypes.Job) bool {
	if orch.Address() != job.TranscoderAddress {
		glog.Error("Transcoder was not assigned")
		return false
	}
	if !blockInRange(orch, job) {
		glog.Error("Job out of range")
		return false
	}
	if !verifyMsgSig(job.BroadcasterAddress, fmt.Sprintf("%v", job.JobId), req.Sig) {
		glog.Error("Transcoder req sig check failed")
		return false
	}
	return true
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
		return nil, fmt.Errorf("Missing job")
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
		return nil, err
	}
	if !verifyTranscoderReq(orch, req, job) {
		return nil, fmt.Errorf("Invalid transcoder request")
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
		Transcoder:  orch.Transcoder(),
		AuthType:    AuthType_LPE,
		Credentials: creds,
		StreamIds:   stringStreamIds,
	}
	return &tr, nil
}

func (orch *orchestrator) ServeSegment(w http.ResponseWriter, r *http.Request) {
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

type lphttp struct {
	orchestrator *grpc.Server
	transcoder   *http.ServeMux
}

func (h *lphttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if r.ProtoMajor == 2 && strings.HasPrefix(ct, "application/grpc") {
		h.orchestrator.ServeHTTP(w, r)
	} else {
		h.transcoder.ServeHTTP(w, r)
	}
}

func StartTranscodeServer(bind string, publicURI *url.URL, node *core.LivepeerNode) {
	s := grpc.NewServer()
	addr := node.Eth.Account().Address
	orch := orchestrator{transcoder: publicURI.String(), node: node, address: addr}
	RegisterOrchestratorServer(s, &orch)
	lp := lphttp{
		orchestrator: s,
		transcoder:   http.NewServeMux(),
	}
	lp.transcoder.HandleFunc("/segment", orch.ServeSegment)

	cert, key, err := getCert(publicURI, node.WorkDir)
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

func StartBroadcastClient(orchestratorServer string, node *core.LivepeerNode, job *lpTypes.Job) (*broadcaster, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	httpc := &http.Client{
		Transport: &http2.Transport{TLSClientConfig: tlsConfig},
		Timeout:   HTTPTimeout,
	}
	uri, err := url.Parse(orchestratorServer)
	if err != nil {
		glog.Error("Could not parse orchestrator URI: ", err)
		return nil, err
	}
	glog.Infof("Connecting RPC to %v", orchestratorServer)
	conn, err := grpc.Dial(uri.Host,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		glog.Error("Did not connect: ", err)
		return nil, err
	}
	defer conn.Close()
	c := NewOrchestratorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	b := broadcaster{node: node, httpc: httpc, job: job}
	req, err := genTranscoderReq(&b, job.JobId.Int64())
	r, err := c.GetTranscoder(ctx, req)
	if err != nil {
		glog.Error("Could not get transcoder: ", err)
		return nil, err
	}
	b.tinfo = r

	return &b, nil
}

func SubmitSegment(bcast Broadcaster, seg *stream.HLSSegment) {
	hc := bcast.GetHTTPClient()
	segData := &SegData{
		Seq:  int64(seg.SeqNo),
		Hash: crypto.Keccak256(seg.Data),
	}
	segCreds, err := genSegCreds(bcast, bcast.Job().StreamId, segData)
	if err != nil {
		return
	}
	ti := bcast.GetTranscoderInfo()
	req, err := http.NewRequest("POST", ti.Transcoder+"/segment", bytes.NewBuffer(seg.Data))
	if err != nil {
		glog.Error("Could not generate trascode request to ", ti.Transcoder)
		return
	}

	req.Header.Set("Authorization", ti.AuthType)
	req.Header.Set("Credentials", ti.Credentials)
	req.Header.Set("Livepeer-Segment", segCreds)
	req.Header.Set("Content-Type", "video/MP2T")

	glog.Infof("Submitting segment %v : %v bytes", seg.SeqNo, len(seg.Data))
	resp, err := hc.Do(req)
	if err != nil {
		glog.Error("Unable to submit segment ", seg.SeqNo, err)
		return
	}
	glog.Infof("Uploaded segment %v", seg.SeqNo)

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error(fmt.Sprintf("Unable to read response body for segment %v : %v", seg.SeqNo, err))
		return
	}
	glog.Infof("Response for segment %v: %s", seg.SeqNo, string(data))
}
