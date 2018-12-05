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
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

var broadcasterAddress = ethcommon.BytesToAddress([]byte("111 Transcoder Address 1")) // need to remove hard-coded broadcaster address

const HTTPTimeout = 8 * time.Second
const GRPCTimeout = 8 * time.Second
const GRPCConnectTimeout = 3 * time.Second

const JobOutOfRangeError = "Job out of range"

var ErrSegSig = errors.New("ErrSegSig")
var ErrSegEncoding = errors.New("ErrorSegEncoding")

var tlsConfig = &tls.Config{InsecureSkipVerify: true}
var httpClient = &http.Client{
	Transport: &http2.Transport{TLSClientConfig: tlsConfig},
	Timeout:   HTTPTimeout,
}

type Orchestrator interface {
	ServiceURI() *url.URL
	Address() ethcommon.Address
	TranscoderSecret() string
	Sign([]byte) ([]byte, error)
	CurrentBlock() *big.Int
	TranscodeSeg(int64, *core.SegmentMetadata, *stream.HLSSegment) (*core.TranscodeResult, error)
	StreamIDs(string) ([]core.StreamID, error) // ANGIE - THIS NEEDS TO BE EDITED. WE MIGHT NEED TO GET STREAMIDS ELSEWHERE
	ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer)
	TranscoderResults(job int64, res *core.RemoteTranscoderResult)
}

type Broadcaster interface {
	Address() ethcommon.Address
	Sign([]byte) ([]byte, error)
}

// Session-specific state for broadcasters
type BroadcastSession struct {
	Broadcaster      Broadcaster
	ManifestID       core.ManifestID
	Profiles         []ffmpeg.VideoProfile
	OrchestratorInfo *net.OrchestratorInfo
	OrchestratorOS   drivers.OSSession
	BroadcasterOS    drivers.OSSession
}

func genOrchestratorReq(b Broadcaster) (*net.OrchestratorRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", b.Address().Hex())))
	if err != nil {
		return nil, err
	}
	return &net.OrchestratorRequest{Address: b.Address().Bytes(), Sig: sig}, nil
}

func CheckOrchestratorAvailability(orch Orchestrator) bool {
	ts := time.Now()
	ts_signature, err := orch.Sign([]byte(fmt.Sprintf("%v", ts)))
	if err != nil {
		return false
	}

	ping := crypto.Keccak256(ts_signature)

	orch_client, conn, err := startOrchestratorClient(orch.ServiceURI())
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

func startOrchestratorClient(uri *url.URL) (net.OrchestratorClient, *grpc.ClientConn, error) {
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

func verifyOrchestratorReq(orch Orchestrator, req *net.OrchestratorRequest) error {
	addr := ethcommon.BytesToAddress(req.Address)
	if !verifyMsgSig(addr, addr.Hex(), req.Sig) {
		glog.Error("orchestrator req sig check failed")
		return fmt.Errorf("orchestrator req sig check failed")
	}
	return nil
}

func genSegCreds(sess *BroadcastSession, seg *stream.HLSSegment) (string, error) {

	// Generate signature for relevant parts of segment
	hash := crypto.Keccak256(seg.Data)
	md := &core.SegmentMetadata{
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
		ManifestId: md.ManifestID.GetVideoID(),
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

func verifySegCreds(orch Orchestrator, segCreds string) (*core.SegmentMetadata, error) {
	buf, err := base64.StdEncoding.DecodeString(segCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, ErrSegEncoding
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
	mid, err := core.MakeManifestID(segData.ManifestId)
	if err != nil {
		glog.Error("Unable to deserialize manifest ID ", err)
		return nil, err
	}

	var os *net.OSInfo
	if len(segData.Storage) > 0 {
		os = segData.Storage[0]
	}

	md := &core.SegmentMetadata{
		ManifestID: mid,
		Seq:        segData.Seq,
		Hash:       ethcommon.BytesToHash(segData.Hash),
		Profiles:   profiles,
		OS:         os,
	}

	if !verifyMsgSig(broadcasterAddress, string(md.Flatten()), segData.Sig) {
		glog.Error("Sig check failed")
		return nil, ErrSegSig
	}

	return md, nil
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

func getOrchestrator(context context.Context, orch Orchestrator, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	jobId := ""                                         // jobId prob not needed here anymore
	glog.Info("Got transcoder request for job ", jobId) // ANGIE - GET JOB/STREAMIDS FROM ELSEWHERE
	if err := verifyOrchestratorReq(orch, req); err != nil {
		return nil, fmt.Errorf("Invalid orchestrator request (%v)", err)
	}
	sids, err := orch.StreamIDs(jobId)
	if err != nil {
		return nil, err
	}
	stringStreamIds := make(map[string]string)

	for i, s := range sids {
		stringStreamIds[s.String()] = profiles[i].Name
	}

	tr := net.OrchestratorInfo{
		Transcoder: orch.ServiceURI().String(), // currently,  orchestrator == transcoder
		StreamIds:  stringStreamIds,
	}
	mid, err := core.StreamID(jobId).ManifestIDFromStreamID()
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

	// check the segment sig from the broadcaster
	seg := r.Header.Get("Livepeer-Segment")

	segData, err := verifySegCreds(orch, seg) // ANGIE : NEED BROADCASTER ADDRESS FROM PM
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

	fmt.Print("jobId needs to be changed into string", jobId)
	// res, err := orch.TranscodeSeg(int64(jobId), segData, &hlsStream) // ANGIE - NEED TO CHANGE ALL JOBIDS IN TRANSCODING LOOP INTO STRINGS
	res, err := orch.TranscodeSeg(int64(123), segData, &hlsStream) // ANGIE - NEED TO CHANGE ALL JOBIDS IN TRANSCODING LOOP INTO STRINGS

	// Upload to OS and construct segment result set
	var segments []*net.TranscodedSegmentData
	for i := 0; err == nil && i < len(res.Data); i++ {
		name := fmt.Sprintf("%s/%d.ts", profiles[i].Name, segData.Seq) // ANGIE - NEED TO EDIT OUT JOB PROFILES
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
func (h *lphttp) GetOrchestrator(context context.Context, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	return getOrchestrator(context, h.orchestrator, req)
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

func GetOrchestratorInfo(bcast Broadcaster, orchestratorServer *url.URL) (*net.OrchestratorInfo, error) {
	c, conn, err := startOrchestratorClient(orchestratorServer)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()

	req, err := genOrchestratorReq(bcast)
	r, err := c.GetOrchestrator(ctx, req)
	if err != nil {
		glog.Errorf("Could not get orchestrator %v: %v", orchestratorServer, err)
		return nil, errors.New("Could not get orchestrator: " + err.Error())
	}

	return r, nil
}

func SubmitSegment(sess *BroadcastSession, seg *stream.HLSSegment, nonce uint64) (*net.TranscodeData, error) {
	if monitor.Enabled {
		monitor.SegmentUploadStart(nonce, seg.SeqNo)
	}
	uploaded := seg.Name != "" // hijack seg.Name to convey the uploaded URI

	segCreds, err := genSegCreds(sess, seg)
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

	ti := sess.OrchestratorInfo
	req, err := http.NewRequest("POST", ti.Transcoder+"/segment", bytes.NewBuffer(data))
	if err != nil {
		glog.Error("Could not generate trascode request to ", ti.Transcoder)
		if monitor.Enabled {
			monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
		}
		return nil, err
	}

	req.Header.Set("Livepeer-Segment", segCreds)
	if uploaded {
		req.Header.Set("Content-Type", "application/vnd+livepeer.uri")
	} else {
		req.Header.Set("Content-Type", "video/MP2T")
	}

	glog.Infof("Submitting segment %v : %v bytes", seg.SeqNo, len(data))
	start := time.Now()
	resp, err := httpClient.Do(req)
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
