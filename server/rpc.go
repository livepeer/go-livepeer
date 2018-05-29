package server

import (
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

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

const HTTPTimeout = 4 * time.Second

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
	GetJob(int64) (*lpTypes.Job, error)
}

// Orchestator interface methods
func (orch *orchestrator) Transcoder() string {
	return orch.transcoder
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
		ta, err := orch.node.Eth.AssignedTranscoder(job.JobId)
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

// grpc methods
func (o *orchestrator) GetTranscoder(context context.Context, req *TranscoderRequest) (*TranscoderInfo, error) {
	return GetTranscoder(context, o, req)
}

type broadcaster struct {
	node  *core.LivepeerNode
	httpc *http.Client
	job   *lpTypes.Job
}

type Broadcaster interface {
	Sign([]byte) ([]byte, error)
	Job() *lpTypes.Job
	SetHTTPClient(*http.Client)
	GetHTTPClient() *http.Client
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

func genTranscoderReq(b Broadcaster, jid int64) (*TranscoderRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", jid)))
	if err != nil {
		return nil, err
	}
	return &TranscoderRequest{JobId: jid, Sig: sig}, nil
}

func verifyMsgSig(addr ethcommon.Address, msg string, sig []byte) bool {
	return eth.VerifySig(addr, crypto.Keccak256([]byte(msg)), sig)
}

func verifyTranscoderReq(orch Orchestrator, req *TranscoderRequest, job *lpTypes.Job) bool {
	if !verifyMsgSig(job.BroadcasterAddress, fmt.Sprintf("%v", job.JobId), req.Sig) {
		glog.Error("Transcoder req sig check failed")
		return false
	}
	return true
}

func genToken(orch Orchestrator, job *lpTypes.Job) (string, error) {
	// TODO add issuance and expiry
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

func verifyCreds(orch Orchestrator, creds string) (*AuthToken, bool) {
	buf, err := base64.StdEncoding.DecodeString(creds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, false
	}
	var token AuthToken
	err = proto.Unmarshal(buf, &token)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, false
	}
	if !verifyMsgSig(orch.Address(), fmt.Sprintf("%v", token.JobId), token.Sig) {
		glog.Error("Sig check failed")
		return nil, false
	}
	return &token, true
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
	creds, err := genCreds(orch, job)
	if err != nil {
		return nil, err
	}
	tr := TranscoderInfo{
		Transcoder:  orch.Transcoder(),
		AuthType:    AuthType_LPE,
		Credentials: creds,
	}
	return &tr, nil
}

func (orch *orchestrator) ServeSegment(w http.ResponseWriter, r *http.Request) {
	authType := r.Header.Get("Authorization")
	creds := r.Header.Get("Credentials")
	if AuthType_LPE != authType {
		glog.Error("Invalid auth type ", authType)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, ok := verifyCreds(orch, creds)
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Write([]byte("The segment has been successfully transcoded."))
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
	srv := http.Server {
		Addr: bind,
		Handler: &lp,
		ReadTimeout: HTTPTimeout,
		WriteTimeout: HTTPTimeout,
	}
	srv.ListenAndServeTLS(cert, key)
}

func StartBroadcastClient(orchestratorServer string, node *core.LivepeerNode, job *lpTypes.Job) (*broadcaster, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	httpc := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		Timeout: HTTPTimeout,
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	b := broadcaster{node: node, httpc: httpc, job: job}
	req, err := genTranscoderReq(&b, job.JobId.Int64())
	r, err := c.GetTranscoder(ctx, req)
	if err != nil {
		glog.Error("Could not get transcoder: ", err)
		return nil, err
	}

	return &b, nil
}
