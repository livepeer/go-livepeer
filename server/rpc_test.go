package server

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
)

type stubOrchestrator struct {
	priv  *ecdsa.PrivateKey
	block *big.Int
	jobId string
}

func (r *stubOrchestrator) JobId() string {
	return "iamajobidstring"
}

func StubJob() string {
	return "iamajobstring"
}
func (r *stubOrchestrator) SetBroadcasterOS(ios drivers.OSSession) {
}
func (r *stubOrchestrator) GetBroadcasterOS() drivers.OSSession {
	return nil
}
func (r *stubOrchestrator) SetOrchestratorOS(oos drivers.OSSession) {
}
func (r *stubOrchestrator) GetOrchestratorOS() drivers.OSSession {
	return nil
}
func (r *stubOrchestrator) GetIno() *url.URL {
	url, _ := url.Parse("http://localhost:1234")
	return url
}

func (r *stubOrchestrator) ServiceURI() *url.URL {
	url, _ := url.Parse("http://localhost:1234")
	return url
}

func (r *stubOrchestrator) CurrentBlock() *big.Int {
	return r.block
}

func (r *stubOrchestrator) Sign(msg []byte) ([]byte, error) {
	hash := ethcrypto.Keccak256(msg)
	ethMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, hash)
	return ethcrypto.Sign(ethcrypto.Keccak256([]byte(ethMsg)), r.priv)
}
func (r *stubOrchestrator) Address() ethcommon.Address {
	return ethcrypto.PubkeyToAddress(r.priv.PublicKey)
}
func (r *stubOrchestrator) TranscodeSeg(jobId int64, seg *core.SignedSegment) (*core.TranscodeResult, error) {
	return nil, nil
}
func (r *stubOrchestrator) StreamIDs(jobId string) ([]core.StreamID, error) {
	return []core.StreamID{}, nil
}

func StubOrchestrator() *stubOrchestrator {
	pk, err := ethcrypto.GenerateKey()
	if err != nil {
		return &stubOrchestrator{}
	}
	return &stubOrchestrator{priv: pk, block: big.NewInt(5), jobId: StubJob()}
}

func (r *stubOrchestrator) GetHTTPClient() *http.Client {
	return nil
}
func (r *stubOrchestrator) SetHTTPClient(ti *http.Client) {
}
func (r *stubOrchestrator) GetTranscoderInfo() *net.TranscoderInfo {
	return nil
}
func (r *stubOrchestrator) SetTranscoderInfo(ti *net.TranscoderInfo) {
}
func (r *stubOrchestrator) ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer) {
}
func (r *stubOrchestrator) TranscoderResults(job int64, res *core.RemoteTranscoderResult) {
}
func (r *stubOrchestrator) TranscoderSecret() string {
	return ""
}
func StubBroadcaster2() *stubOrchestrator {
	return StubOrchestrator() // lazy; leverage subtyping for interface commonalities
}

func TestRPCTranscoderReq(t *testing.T) {

	// o := StubOrchestrator()
	b := StubBroadcaster2()

	jobId := StubJob()
	// broadcasterAddress := ethcrypto.PubkeyToAddress(b.priv.PublicKey)
	// transcoderAddress := ethcrypto.PubkeyToAddress(o.priv.PublicKey)

	req, err := genTranscoderReq(b, jobId)
	if err != nil {
		t.Error("Unable to create transcoder req ", req)
	}
	// if verifyTranscoderReq(o, req, j) != nil { // normal case
	// 	t.Error("Unable to verify transcoder request")
	// }

	// // mismatched jobid
	// req, _ = genTranscoderReq(b, 999)
	// if verifyTranscoderReq(o, req, j) == nil {
	// 	t.Error("Did not expect verification to pass; should mismatch sig")
	// }

	// req, _ = genTranscoderReq(b, jobId) // reset job
	// if req.JobId != jobId {             // sanity check
	// 	t.Error("Sanity check failed")
	// }

	// // wrong transcoder
	// if verifyTranscoderReq(StubOrchestrator(), req, j) == nil {
	// 	t.Error("Did not expect verification to pass; should mismatch transcoder")
	// }

	// // wrong broadcaster
	// broadcasterAddress = ethcrypto.PubkeyToAddress(StubBroadcaster2().priv.PublicKey)
	// if verifyTranscoderReq(o, req, j) == nil {
	// 	t.Error("Did not expect verification to pass; should mismatch broadcaster")
	// }
	// broadcasterAddress = ethcrypto.PubkeyToAddress(b.priv.PublicKey)

	// job is too early
	// o.block = big.NewInt(-1)
	// if err := verifyTranscoderReq(o, req, j); err == nil || err.Error() != "Job out of range" {
	// 	t.Error("Early request unexpectedly validated", err)
	// }

	// // job is too late
	// o.block = big.NewInt(0).Add(j.EndBlock, big.NewInt(1))
	// if err := verifyTranscoderReq(o, req, j); err == nil || err.Error() != "Job out of range" {
	// 	t.Error("Late request unexpectedly validated", err)
	// }

	// // can't claim
	// o.block = big.NewInt(0).Add(j.CreationBlock, big.NewInt(257))
	// if err := verifyTranscoderReq(o, req, j); err == nil || err.Error() != "Job out of range" {
	// 	t.Error("Unclaimable request unexpectedly validated", err)
	// }

	// // can now claim with a prior claim
	// j.FirstClaimSubmitted = true
	// if err := verifyTranscoderReq(o, req, j); err != nil {
	// 	t.Error("Request not validated as expected validated", err)
	// }

	// // empty profiles
	// j.Profiles = []ffmpeg.VideoProfile{}
	// if err := verifyTranscoderReq(o, req, j); err == nil || err.Error() != "Job out of range" {
	// 	t.Error("Unclaimable request unexpectedly validated", err)
	// }
	// j.Profiles = StubJob().Profiles

}

func TestRPCCreds(t *testing.T) {

	r := StubOrchestrator()
	jobId := StubJob()

	creds, err := genToken(r, jobId)
	if err != nil {
		t.Error("Unable to generate creds from req ", err)
	}
	if _, err := verifyToken(r, creds); err != nil {
		t.Error("Creds did not validate: ", err)
	}

	// // corrupt the creds
	// idx := len(creds) / 2
	// kreds := creds[:idx] + string(^creds[idx]) + creds[idx+1:]
	// if _, err := verifyToken(r, kreds); err == nil || err.Error() != "illegal base64 data at input byte 46" {
	// 	t.Error("Creds unexpectedly validated", err)
	// }

	// wrong orchestrator
	if _, err := verifyToken(StubOrchestrator(), creds); err == nil || err.Error() != "Token sig check failed" {
		t.Error("Orchestrator unexpectedly validated", err)
	}

	// // empty profiles
	// r.job.Profiles = []ffmpeg.VideoProfile{}
	// if _, err := verifyToken(r, creds); err.Error() != "Job out of range" {
	// 	t.Error("Unclaimable job unexpectedly validated", err)
	// }

	// // reset to sanity check once again
	// r.job = StubJob()
	// r.block = big.NewInt(0)
	// if _, err := verifyToken(r, creds); err != nil {
	// 	t.Error("Block did not validate", err)
	// }

}

func TestRPCSeg(t *testing.T) {
	b := StubBroadcaster2()
	o := StubOrchestrator()

	baddr := ethcrypto.PubkeyToAddress(b.priv.PublicKey)

	StreamId := StubJob()
	jobId := StubJob()
	broadcasterAddress = baddr

	segData := &net.SegData{Seq: 4, Hash: ethcommon.RightPadBytes([]byte("browns"), 32)}

	creds, err := genSegCreds(b, StreamId, segData)
	if err != nil {
		t.Error("Unable to generate seg creds ", err)
		return
	}
	if _, err := verifySegCreds(o, jobId, creds); err != nil {
		t.Error("Unable to verify seg creds", err)
		return
	}

	// test invalid jobid
	// oldSid := StreamId
	// StreamId = StreamId + StreamId
	// if _, err := verifySegCreds(o, jobId, creds); err == nil || err.Error() != "Segment sig check failed" {
	// 	t.Error("Unexpectedly verified seg creds: invalid jobid", err)
	// 	return
	// }
	// StreamId = oldSid

	// test invalid bcast addr
	oldAddr := broadcasterAddress
	key, _ := ethcrypto.GenerateKey()
	broadcasterAddress = ethcrypto.PubkeyToAddress(key.PublicKey)
	if _, err := verifySegCreds(o, jobId, creds); err == nil || err.Error() != "Segment sig check failed" {
		t.Error("Unexpectedly verified seg creds: invalid bcast addr", err)
	}
	broadcasterAddress = oldAddr

	// sanity check
	if _, err := verifySegCreds(o, jobId, creds); err != nil {
		t.Error("Sanity check failed", err)
	}

	// test corrupt creds
	idx := len(creds) / 2
	kreds := creds[:idx] + string(^creds[idx]) + creds[idx+1:]
	if _, err := verifySegCreds(o, jobId, kreds); err == nil || err.Error() != "illegal base64 data at input byte 70" {
		t.Error("Unexpectedly verified bad creds", err)
	}
}

func TestPing(t *testing.T) {
	o := StubOrchestrator()

	tsSignature, _ := o.Sign([]byte(fmt.Sprintf("%v", time.Now())))
	pingSent := crypto.Keccak256(tsSignature)
	req := &net.PingPong{Value: pingSent}

	pong, err := ping(context.Background(), req, o)
	if err != nil {
		t.Error("Unable to send Ping request")
	}

	verified := verifyMsgSig(o.Address(), string(pingSent), pong.Value)

	if !verified {
		t.Error("Unable to verify response from ping request")
	}
}
