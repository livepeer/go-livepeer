package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/golang/protobuf/proto"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
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

func (r *stubOrchestrator) VerifySig(addr ethcommon.Address, msg string, sig []byte) bool {
	return pm.VerifySig(addr, ethcrypto.Keccak256([]byte(msg)), sig)
}

func (r *stubOrchestrator) Address() ethcommon.Address {
	return ethcrypto.PubkeyToAddress(r.priv.PublicKey)
}
func (r *stubOrchestrator) TranscodeSeg(md *core.SegTranscodingMetadata, seg *stream.HLSSegment) (*core.TranscodeResult, error) {
	return nil, nil
}
func (r *stubOrchestrator) StreamIDs(jobId string) ([]core.StreamID, error) {
	return []core.StreamID{}, nil
}

func (r *stubOrchestrator) ProcessPayment(payment net.Payment, manifestID core.ManifestID) error {
	return nil
}

func (r *stubOrchestrator) TicketParams(sender ethcommon.Address) *net.TicketParams {
	return nil
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

	addr := ethcommon.BytesToAddress(req.Address)
	if verifyOrchestratorReq(o, addr, req.Sig) != nil { // normal case
		t.Error("Unable to verify orchestrator request")
	}

	// wrong broadcaster
	addr = ethcrypto.PubkeyToAddress(StubBroadcaster2().priv.PublicKey)
	if verifyOrchestratorReq(o, addr, req.Sig) == nil {
		t.Error("Did not expect verification to pass; should mismatch broadcaster")
	}

	// invalid address
	addr = ethcommon.BytesToAddress([]byte("#non-hex address!"))
	if verifyOrchestratorReq(o, addr, req.Sig) == nil {
		t.Error("Did not expect verification to pass; should mismatch broadcaster")
	}

	// error signing
	b.signErr = fmt.Errorf("Signing error")
	_, err = genOrchestratorReq(b)
	if err == nil {
		t.Error("Did not expect to generate a orchestrator request with invalid address")
	}
}

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

	verified := o.VerifySig(o.Address(), string(pingSent), pong.Value)

	if !verified {
		t.Error("Unable to verify response from ping request")
	}
}

func TestProcessPayment_GivenInvalidBase64_ReturnsError(t *testing.T) {
	orch := &mockOrchestrator{}
	manifestID := core.ManifestID("some manifest")
	header := "not base64"

	err := processPayment(orch, header, manifestID)

	assert.Contains(t, err.Error(), "base64")
}

func TestProcessPayment_GivenInvalidProtoData_ReturnsError(t *testing.T) {
	orch := &mockOrchestrator{}
	manifestID := core.ManifestID("some manifest")
	data := pm.RandBytes(123)
	header := base64.StdEncoding.EncodeToString(data)

	err := processPayment(orch, header, manifestID)

	assert.Contains(t, err.Error(), "protobuf")
}

func TestProcessPayment_GivenValidHeader_InvokesOrchFunction(t *testing.T) {
	orch := &mockOrchestrator{}
	manifestID := core.ManifestID("some manifest")
	protoTicket := defaultTicket(t)
	protoPayment := defaultPayment(t, protoTicket)
	data, err := proto.Marshal(protoPayment)
	require.Nil(t, err)
	header := base64.StdEncoding.EncodeToString(data)
	// Had to use this long-winded arg matcher because simple equality and DeepEqual didn't work
	argMatcher := mock.MatchedBy(func(actualPayment net.Payment) bool {
		if !bytes.Equal(actualPayment.Ticket.Recipient, protoTicket.Recipient) {
			return false
		}
		if !bytes.Equal(actualPayment.Ticket.Sender, protoTicket.Sender) {
			return false
		}
		if !bytes.Equal(actualPayment.Ticket.FaceValue, protoTicket.FaceValue) {
			return false
		}
		if !bytes.Equal(actualPayment.Ticket.WinProb, protoTicket.WinProb) {
			return false
		}
		if !bytes.Equal(actualPayment.Ticket.RecipientRandHash, protoTicket.RecipientRandHash) {
			return false
		}
		if actualPayment.Ticket.SenderNonce != protoTicket.SenderNonce {
			return false
		}
		if !bytes.Equal(actualPayment.Sig, protoPayment.Sig) {
			return false
		}
		if !bytes.Equal(actualPayment.Seed, protoPayment.Seed) {
			return false
		}
		return true
	})

	orch.On("ProcessPayment", argMatcher, manifestID).Return(nil).Once()

	err = processPayment(orch, header, manifestID)

	assert.Nil(t, err)
	orch.AssertExpectations(t)
}

func TestProcessPayment_GivenErrorFromOrch_ForwardsTheError(t *testing.T) {
	orch := &mockOrchestrator{}
	manifestID := core.ManifestID("some manifest")
	protoTicket := defaultTicket(t)
	protoPayment := defaultPayment(t, protoTicket)
	data, err := proto.Marshal(protoPayment)
	require.Nil(t, err)
	header := base64.StdEncoding.EncodeToString(data)
	orch.On("ProcessPayment", mock.Anything, mock.Anything).Return(errors.New("error from mock")).Once()

	err = processPayment(orch, header, manifestID)

	assert.Equal(t, "error from mock", err.Error())
}

func TestGetOrchestrator_GivenValidSig_ReturnsTranscoderURI(t *testing.T) {
	orch := &mockOrchestrator{}
	drivers.NodeStorage = drivers.NewMemoryDriver("")
	uri := "http://someuri.com"
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
	orch.On("ServiceURI").Return(url.Parse(uri))
	orch.On("TicketParams", mock.Anything).Return(nil)

	oInfo, err := getOrchestrator(orch, &net.OrchestratorRequest{})

	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(uri, oInfo.Transcoder)
}

func TestGetOrchestrator_GivenInvalidSig_ReturnsError(t *testing.T) {
	orch := &mockOrchestrator{}
	drivers.NodeStorage = drivers.NewMemoryDriver("")
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(false)

	oInfo, err := getOrchestrator(orch, &net.OrchestratorRequest{})

	assert := assert.New(t)
	assert.Contains(err.Error(), "sig")
	assert.Nil(oInfo)
}

func TestGetOrchestrator_GivenValidSig_ReturnsOrchTicketParams(t *testing.T) {
	orch := &mockOrchestrator{}
	drivers.NodeStorage = drivers.NewMemoryDriver("")
	uri := "http://someuri.com"
	expectedParams := defaultTicketParams()
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
	orch.On("ServiceURI").Return(url.Parse(uri))
	orch.On("TicketParams", mock.Anything).Return(expectedParams)

	oInfo, err := getOrchestrator(orch, &net.OrchestratorRequest{})

	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(expectedParams, oInfo.TicketParams)
}

type mockOrchestrator struct {
	mock.Mock
}

func (o *mockOrchestrator) ServiceURI() *url.URL {
	args := o.Called()
	return args.Get(0).(*url.URL)
}
func (o *mockOrchestrator) Address() ethcommon.Address {
	o.Called()
	return ethcommon.Address{}
}
func (o *mockOrchestrator) TranscoderSecret() string {
	o.Called()
	return ""
}
func (o *mockOrchestrator) Sign(msg []byte) ([]byte, error) {
	o.Called(msg)
	return nil, nil
}
func (o *mockOrchestrator) VerifySig(addr ethcommon.Address, msg string, sig []byte) bool {
	args := o.Called(addr, msg, sig)
	return args.Bool(0)
}
func (o *mockOrchestrator) CurrentBlock() *big.Int {
	o.Called()
	return nil
}
func (o *mockOrchestrator) TranscodeSeg(md *core.SegTranscodingMetadata, seg *stream.HLSSegment) (*core.TranscodeResult, error) {
	o.Called(md, seg)
	return nil, nil
}
func (o *mockOrchestrator) ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer) {
	o.Called(stream)
}
func (o *mockOrchestrator) TranscoderResults(job int64, res *core.RemoteTranscoderResult) {
	o.Called(job, res)
}
func (o *mockOrchestrator) ProcessPayment(payment net.Payment, manifestID core.ManifestID) error {
	args := o.Called(payment, manifestID)
	return args.Error(0)
}

func (o *mockOrchestrator) TicketParams(sender ethcommon.Address) *net.TicketParams {
	args := o.Called(sender)
	if args.Get(0) != nil {
		return args.Get(0).(*net.TicketParams)
	}
	return nil
}

func defaultTicketParams() *net.TicketParams {
	return &net.TicketParams{
		Recipient:         pm.RandBytes(123),
		FaceValue:         pm.RandBytes(123),
		WinProb:           pm.RandBytes(123),
		RecipientRandHash: pm.RandBytes(123),
		Seed:              pm.RandBytes(123),
	}
}

func defaultPayment(t *testing.T, ticket *net.Ticket) *net.Payment {
	return &net.Payment{
		Ticket: ticket,
		Sig:    pm.RandBytes(123),
		Seed:   pm.RandBytes(123),
	}
}

func defaultTicket(t *testing.T) *net.Ticket {
	return &net.Ticket{
		Recipient:         pm.RandBytes(123),
		Sender:            pm.RandBytes(123),
		FaceValue:         pm.RandBytes(123),
		WinProb:           pm.RandBytes(123),
		SenderNonce:       456,
		RecipientRandHash: pm.RandBytes(123),
	}
}
