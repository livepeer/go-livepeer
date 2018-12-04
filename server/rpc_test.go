package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/protobuf/proto"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

type stubOrchestrator struct {
	priv    *ecdsa.PrivateKey
	block   *big.Int
	signErr error
}

func (r *stubOrchestrator) ServiceURI() *url.URL {
	url, _ := url.Parse("http://localhost:1234")
	return url
}

func (r *stubOrchestrator) CurrentBlock() *big.Int {
	return r.block
}

func (r *stubOrchestrator) Sign(msg []byte) ([]byte, error) {
	if r.signErr != nil {
		return nil, r.signErr
	}
	hash := ethcrypto.Keccak256(msg)
	ethMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, hash)
	return ethcrypto.Sign(ethcrypto.Keccak256([]byte(ethMsg)), r.priv)
}
func (r *stubOrchestrator) Address() ethcommon.Address {
	return ethcrypto.PubkeyToAddress(r.priv.PublicKey)
}
func (r *stubOrchestrator) TranscodeSeg(md *core.SegmentMetadata, seg *stream.HLSSegment) (*core.TranscodeResult, error) {
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
	return &stubOrchestrator{priv: pk, block: big.NewInt(5)}
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

	o := StubOrchestrator()
	b := StubBroadcaster2()

	req, err := genOrchestratorReq(b)
	if err != nil {
		t.Error("Unable to create orchestrator req ", req)
	}
	if verifyOrchestratorReq(o, req) != nil { // normal case
		t.Error("Unable to verify orchestrator request")
	}

	// wrong broadcaster
	req.Address = ethcrypto.PubkeyToAddress(StubBroadcaster2().priv.PublicKey).Bytes()
	if verifyOrchestratorReq(o, req) == nil {
		t.Error("Did not expect verification to pass; should mismatch broadcaster")
	}

	// invalid address
	req.Address = []byte("#non-hex address!")
	if verifyOrchestratorReq(o, req) == nil {
		t.Error("Did not expect verification to pass; should mismatch broadcaster")
	}

	// error signing
	b.signErr = fmt.Errorf("Signing error")
	_, err = genOrchestratorReq(b)
	if err == nil {
		t.Error("Did not expect to generate a orchestrator request with invalid address")
	}
}

// func TestGenToken(t *testing.T) {
// 	r := StubOrchestrator()

// 	creds, err := genToken(r, )
// }

func TestRPCSeg(t *testing.T) {
	mid, _ := core.MakeManifestID(core.RandomVideoID())
	b := StubBroadcaster2()
	o := StubOrchestrator()
	s := &BroadcastSession{
		Broadcaster: b,
		ManifestID:  mid,
	}

	baddr := ethcrypto.PubkeyToAddress(b.priv.PublicKey)

	broadcasterAddress = baddr

	segData := &stream.HLSSegment{}

	creds, err := genSegCreds(s, segData)
	if err != nil {
		t.Error("Unable to generate seg creds ", err)
		return
	}
	if _, err := verifySegCreds(o, creds); err != nil {
		t.Error("Unable to verify seg creds", err)
		return
	}

	// error signing
	b.signErr = fmt.Errorf("SignErr")
	if _, err := genSegCreds(s, segData); err != b.signErr {
		t.Error("Generating seg creds ", err)
	}
	b.signErr = nil

	// test invalid bcast addr
	oldAddr := broadcasterAddress
	key, _ := ethcrypto.GenerateKey()
	broadcasterAddress = ethcrypto.PubkeyToAddress(key.PublicKey)
	if _, err := verifySegCreds(o, creds); err != ErrSegSig {
		t.Error("Unexpectedly verified seg creds: invalid bcast addr", err)
	}
	broadcasterAddress = oldAddr

	// sanity check
	if _, err := verifySegCreds(o, creds); err != nil {
		t.Error("Sanity check failed", err)
	}

	// test corrupt creds
	idx := len(creds) / 2
	kreds := creds[:idx] + string(^creds[idx]) + creds[idx+1:]
	if _, err := verifySegCreds(o, kreds); err != ErrSegEncoding {
		t.Error("Unexpectedly verified bad creds", err)
	}

	corruptSegData := func(segData *net.SegData, expectedErr error) {
		data, _ := proto.Marshal(segData)
		creds = base64.StdEncoding.EncodeToString(data)
		if _, err := verifySegCreds(o, creds); err != expectedErr {
			t.Errorf("Expected to fail with '%v' but got '%v'", expectedErr, err)
		}
	}

	// corrupt manifest id
	corruptSegData(&net.SegData{}, core.ErrManifestID)
	corruptSegData(&net.SegData{ManifestId: []byte("abc")}, core.ErrManifestID)

	// corrupt profiles
	corruptSegData(&net.SegData{Profiles: []byte("abc")}, common.ErrProfile)

	// corrupt sig
	sd := &net.SegData{ManifestId: s.ManifestID.GetVideoID()}
	corruptSegData(sd, ErrSegSig) // missing sig
	sd.Sig = []byte("abc")
	corruptSegData(sd, ErrSegSig) // invalid sig
}

func TestPing(t *testing.T) {
	o := StubOrchestrator()

	tsSignature, _ := o.Sign([]byte(fmt.Sprintf("%v", time.Now())))
	pingSent := ethcrypto.Keccak256(tsSignature)
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
