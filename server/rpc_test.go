package server

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/http"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/core"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

func StubJob() *lpTypes.Job {
	return &lpTypes.Job{
		JobId:              big.NewInt(0),
		StreamId:           "abc",
		BroadcasterAddress: ethcommon.Address{},
		TranscoderAddress:  ethcommon.Address{},
	}
}

type stubOrchestrator struct {
	priv *ecdsa.PrivateKey
}

func (r *stubOrchestrator) Transcoder() string {
	return "abc"
}
func (r *stubOrchestrator) GetJob(jid int64) (*lpTypes.Job, error) {
	return StubJob(), nil
}
func (r *stubOrchestrator) Sign(msg []byte) ([]byte, error) {
	hash := ethcrypto.Keccak256(msg)
	ethMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, hash)
	return ethcrypto.Sign(ethcrypto.Keccak256([]byte(ethMsg)), r.priv)
}
func (r *stubOrchestrator) Address() ethcommon.Address {
	return ethcrypto.PubkeyToAddress(r.priv.PublicKey)
}
func (r *stubOrchestrator) TranscodeSeg(job *lpTypes.Job, seg *core.SignedSegment) error {
	return nil
}
func (r *stubOrchestrator) StreamIDs(job *lpTypes.Job) ([]core.StreamID, error) {
	return []core.StreamID{}, nil
}

func StubOrchestrator() *stubOrchestrator {
	pk, err := ethcrypto.GenerateKey()
	if err != nil {
		return &stubOrchestrator{}
	}
	return &stubOrchestrator{priv: pk}
}

func (r *stubOrchestrator) Job() *lpTypes.Job {
	return nil
}
func (r *stubOrchestrator) GetHTTPClient() *http.Client {
	return nil
}
func (r *stubOrchestrator) SetHTTPClient(ti *http.Client) {
}
func (r *stubOrchestrator) GetTranscoderInfo() *TranscoderInfo {
	return nil
}
func (r *stubOrchestrator) SetTranscoderInfo(ti *TranscoderInfo) {
}
func StubBroadcaster2() *stubOrchestrator {
	return StubOrchestrator() // lazy; leverage subtyping for interface commonalities
}

func TestRPCTranscoderReq(t *testing.T) {

	o := StubOrchestrator()
	b := StubBroadcaster2()

	j := StubJob()
	j.JobId = big.NewInt(1234)
	j.BroadcasterAddress = ethcrypto.PubkeyToAddress(b.priv.PublicKey)

	req, err := genTranscoderReq(b, j.JobId.Int64())
	if err != nil {
		t.Error("Unable to create transcoder req ", req)
	}
	if !verifyTranscoderReq(o, req, j) { // normal case
		t.Error("Unable to verify transcoder request")
	}

	// mismatched jobid
	req, _ = genTranscoderReq(b, 999)
	if verifyTranscoderReq(o, req, j) {
		t.Error("Did not expect verification to pass; should mismatch sig")
	}

	req, _ = genTranscoderReq(b, j.JobId.Int64()) // reset job
	if req.JobId != j.JobId.Int64() {             // sanity check
		t.Error("Sanity check failed")
	}

	// wrong broadcaster
	j.BroadcasterAddress = ethcrypto.PubkeyToAddress(StubBroadcaster2().priv.PublicKey)
	if verifyTranscoderReq(o, req, j) {
		t.Error("Did not expect verification to pass; should mismatch key")
	}
}

func TestRPCCreds(t *testing.T) {

	j := StubJob()
	r := StubOrchestrator()

	creds, err := genCreds(r, j)
	if err != nil {
		t.Error("Unable to generate creds from req ", err)
	}
	if _, ok := verifyCreds(r, creds); !ok {
		t.Error("Creds did not validate")
	}

	// corrupt the creds
	idx := len(creds) / 2
	kreds := creds[:idx] + string(^creds[idx]) + creds[idx+1:]
	if _, ok := verifyCreds(r, kreds); ok {
		t.Error("Creds unexpectedly validated")
	}

	// wrong orchestrator
	if _, ok := verifyCreds(StubOrchestrator(), creds); ok {
		t.Error("Orchestrator unexpectedly validated")
	}
}

func TestRPCSeg(t *testing.T) {
	b := StubBroadcaster2()
	baddr := ethcrypto.PubkeyToAddress(b.priv.PublicKey)

	j := StubJob()
	j.JobId = big.NewInt(1234)
	j.BroadcasterAddress = baddr

	segData := &SegData{Seq: 4, Hash: ethcommon.RightPadBytes([]byte("browns"), 32)}
	creds, err := genSegCreds(b, j.StreamId, segData)
	if err != nil {
		t.Error("Unable to generate seg creds ", err)
		return
	}
	if _, ok := verifySegCreds(j, creds); !ok {
		t.Error("Unable to verify seg creds")
		return
	}

	// test invalid jobid
	oldSid := j.StreamId
	j.StreamId = j.StreamId + j.StreamId
	if _, ok := verifySegCreds(j, creds); ok {
		t.Error("Unexpectedly verified seg creds: invalid jobid")
		return
	}
	j.StreamId = oldSid

	// test invalid bcast addr
	oldAddr := j.BroadcasterAddress
	key, _ := ethcrypto.GenerateKey()
	j.BroadcasterAddress = ethcrypto.PubkeyToAddress(key.PublicKey)
	if _, ok := verifySegCreds(j, creds); ok {
		t.Error("Unexpectedly verified seg creds: invalid bcast addr")
		return
	}
	j.BroadcasterAddress = oldAddr

	// sanity check
	if _, ok := verifySegCreds(j, creds); !ok {
		t.Error("Sanity check failed")
		return
	}

	// test corrupt creds
	idx := len(creds) / 2
	kreds := creds[:idx] + string(^creds[idx]) + creds[idx+1:]
	if _, ok := verifySegCreds(j, kreds); ok {
		t.Error("Unexpectedly verified bad creds")
		return
	}
}
