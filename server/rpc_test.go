package server

import (
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
	priv       *ecdsa.PrivateKey
	block      *big.Int
	signErr    error
	sessCapErr error
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

func (r *stubOrchestrator) CheckCapacity(mid core.ManifestID) error {
	return r.sessCapErr
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
	addr = ethcommon.BytesToAddress(req.Address)

	// at capacity
	o.sessCapErr = fmt.Errorf("At capacity")
	if err := verifyOrchestratorReq(o, addr, req.Sig); err != o.sessCapErr {
		t.Errorf("Expected %v; got %v", o.sessCapErr, err)
	}
	o.sessCapErr = nil

	// error signing
	b.signErr = fmt.Errorf("Signing error")
	_, err = genOrchestratorReq(b)
	if err == nil {
		t.Error("Did not expect to generate a orchestrator request with invalid address")
	}
}

func TestRPCSeg(t *testing.T) {
	mid := core.RandomManifestID()
	b := StubBroadcaster2()
	o := StubOrchestrator()
	s := &BroadcastSession{
		Broadcaster: b,
		ManifestID:  mid,
	}

	baddr := ethcrypto.PubkeyToAddress(b.priv.PublicKey)

	segData := &stream.HLSSegment{}

	creds, err := genSegCreds(s, segData)
	if err != nil {
		t.Error("Unable to generate seg creds ", err)
		return
	}
	if _, err := verifySegCreds(o, creds, baddr); err != nil {
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
	oldAddr := baddr
	key, _ := ethcrypto.GenerateKey()
	baddr = ethcrypto.PubkeyToAddress(key.PublicKey)
	if _, err := verifySegCreds(o, creds, baddr); err != ErrSegSig {
		t.Error("Unexpectedly verified seg creds: invalid bcast addr", err)
	}
	baddr = oldAddr

	// sanity check
	if _, err := verifySegCreds(o, creds, baddr); err != nil {
		t.Error("Sanity check failed", err)
	}

	// test corrupt creds
	idx := len(creds) / 2
	kreds := creds[:idx] + string(^creds[idx]) + creds[idx+1:]
	if _, err := verifySegCreds(o, kreds, baddr); err != ErrSegEncoding {
		t.Error("Unexpectedly verified bad creds", err)
	}

	corruptSegData := func(segData *net.SegData, expectedErr error) {
		data, _ := proto.Marshal(segData)
		creds = base64.StdEncoding.EncodeToString(data)
		if _, err := verifySegCreds(o, creds, baddr); err != expectedErr {
			t.Errorf("Expected to fail with '%v' but got '%v'", expectedErr, err)
		}
	}

	// corrupt profiles
	corruptSegData(&net.SegData{Profiles: []byte("abc")}, common.ErrProfile)

	// corrupt sig
	sd := &net.SegData{ManifestId: []byte(s.ManifestID)}
	corruptSegData(sd, ErrSegSig) // missing sig
	sd.Sig = []byte("abc")
	corruptSegData(sd, ErrSegSig) // invalid sig

	// at capacity
	sd = &net.SegData{ManifestId: []byte(s.ManifestID)}
	sd.Sig, _ = b.Sign((&core.SegTranscodingMetadata{ManifestID: s.ManifestID}).Flatten())
	o.sessCapErr = fmt.Errorf("At capacity")
	corruptSegData(sd, o.sessCapErr)
	o.sessCapErr = nil
}

func TestGenPayment(t *testing.T) {
	mid := core.RandomManifestID()
	b := StubBroadcaster2()
	s := &BroadcastSession{
		Broadcaster: b,
		ManifestID:  mid,
	}

	assert := assert.New(t)
	require := require.New(t)

	// Test missing sender
	payment, err := genPayment(s)
	assert.Equal("", payment)
	assert.Nil(err)

	sender := &pm.MockSender{}
	s.Sender = sender

	// Test CreateTicket error
	sender.On("CreateTicket", mock.Anything).Return(nil, nil, nil, errors.New("CreateTicket error")).Once()

	_, err = genPayment(s)
	assert.Equal("CreateTicket error", err.Error())

	decodePayment := func(payment string) net.Payment {
		buf, err := base64.StdEncoding.DecodeString(payment)
		assert.Nil(err)

		var protoPayment net.Payment
		err = proto.Unmarshal(buf, &protoPayment)
		assert.Nil(err)

		return protoPayment
	}

	// Test payment creation
	ticket := &pm.Ticket{
		Recipient:         pm.RandAddress(),
		Sender:            pm.RandAddress(),
		FaceValue:         big.NewInt(1234),
		WinProb:           big.NewInt(5678),
		SenderNonce:       777,
		RecipientRandHash: pm.RandHash(),
	}
	seed := big.NewInt(7777)
	sig := pm.RandBytes(42)

	sender.On("CreateTicket", mock.Anything).Return(ticket, seed, sig, nil).Once()

	payment, err = genPayment(s)
	require.Nil(err)

	protoPayment := decodePayment(payment)
	protoTicket := protoPayment.Ticket
	assert.Equal(ticket.Recipient, ethcommon.BytesToAddress(protoTicket.Recipient))
	assert.Equal(ticket.Sender, ethcommon.BytesToAddress(protoTicket.Sender))
	assert.Equal(ticket.FaceValue, new(big.Int).SetBytes(protoTicket.FaceValue))
	assert.Equal(ticket.WinProb, new(big.Int).SetBytes(protoTicket.WinProb))
	assert.Equal(ticket.SenderNonce, protoTicket.SenderNonce)
	assert.Equal(ticket.RecipientRandHash, ethcommon.BytesToHash(protoTicket.RecipientRandHash))
	assert.Equal(sig, protoPayment.Sig)
	assert.Equal(seed, new(big.Int).SetBytes(protoPayment.Seed))
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

func TestGetPayment_GivenInvalidBase64_ReturnsError(t *testing.T) {
	header := "not base64"

	_, err := getPayment(header)

	assert.Contains(t, err.Error(), "base64")
}

func TestGetPayment_GivenEmptyHeader_ReturnsEmptyPayment(t *testing.T) {
	payment, err := getPayment("")

	assert := assert.New(t)
	assert.Nil(err)
	assert.Nil(payment.Ticket)
	assert.Nil(payment.Sig)
	assert.Nil(payment.Seed)
}

func TestGetPayment_GivenInvalidProtoData_ReturnsError(t *testing.T) {
	data := pm.RandBytes(123)
	header := base64.StdEncoding.EncodeToString(data)

	_, err := getPayment(header)

	assert.Contains(t, err.Error(), "protobuf")
}

func TestGetPayment_GivenValidHeader_ReturnsPayment(t *testing.T) {
	protoTicket := defaultTicket(t)
	protoPayment := defaultPayment(t, protoTicket)
	data, err := proto.Marshal(protoPayment)
	require.Nil(t, err)
	header := base64.StdEncoding.EncodeToString(data)

	payment, err := getPayment(header)

	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(protoTicket.Recipient, payment.Ticket.Recipient)
	assert.Equal(protoTicket.Sender, payment.Ticket.Sender)
	assert.Equal(protoTicket.FaceValue, payment.Ticket.FaceValue)
	assert.Equal(protoTicket.WinProb, payment.Ticket.WinProb)
	assert.Equal(protoTicket.SenderNonce, payment.Ticket.SenderNonce)
	assert.Equal(protoTicket.RecipientRandHash, payment.Ticket.RecipientRandHash)
	assert.Equal(protoPayment.Sig, payment.Sig)
	assert.Equal(protoPayment.Seed, payment.Seed)
}

func TestGetPaymentSender_GivenPaymentTicketIsNil(t *testing.T) {
	protoTicket := defaultTicket(t)
	protoPayment := defaultPayment(t, protoTicket)
	protoPayment.Ticket = nil

	assert.Equal(t, ethcommon.Address{}, getPaymentSender(*protoPayment))
}

func TestGetPaymentSender_GivenPaymentTicketSenderIsNil(t *testing.T) {
	protoTicket := defaultTicket(t)
	protoTicket.Sender = nil
	protoPayment := defaultPayment(t, protoTicket)

	assert.Equal(t, ethcommon.Address{}, getPaymentSender(*protoPayment))
}

func TestGetPaymentSender_GivenValidPaymentTicket(t *testing.T) {
	protoTicket := defaultTicket(t)
	protoPayment := defaultPayment(t, protoTicket)

	assert.Equal(t, ethcommon.BytesToAddress(protoTicket.Sender), getPaymentSender(*protoPayment))
}

func TestGetOrchestrator_GivenValidSig_ReturnsTranscoderURI(t *testing.T) {
	orch := &mockOrchestrator{}
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
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
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(false)

	oInfo, err := getOrchestrator(orch, &net.OrchestratorRequest{})

	assert := assert.New(t)
	assert.Contains(err.Error(), "sig")
	assert.Nil(oInfo)
}

func TestGetOrchestrator_GivenValidSig_ReturnsOrchTicketParams(t *testing.T) {
	orch := &mockOrchestrator{}
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
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

type mockOSSession struct {
	mock.Mock
}

func (s *mockOSSession) SaveData(name string, data []byte) (string, error) {
	args := s.Called()
	return args.String(0), args.Error(1)
}

func (s *mockOSSession) EndSession() {
	s.Called()
}

func (s *mockOSSession) GetInfo() *net.OSInfo {
	args := s.Called()
	if args.Get(0) != nil {
		return args.Get(0).(*net.OSInfo)
	}
	return nil
}

func (s *mockOSSession) IsExternal() bool {
	args := s.Called()
	return args.Bool(0)
}

type mockOrchestrator struct {
	mock.Mock
}

func (o *mockOrchestrator) ServiceURI() *url.URL {
	args := o.Called()
	if args.Get(0) != nil {
		return args.Get(0).(*url.URL)
	}
	return nil
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
	args := o.Called(md, seg)

	var res *core.TranscodeResult
	if args.Get(0) != nil {
		res = args.Get(0).(*core.TranscodeResult)
	}

	return res, args.Error(1)
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

func (o *mockOrchestrator) CheckCapacity(mid core.ManifestID) error {
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
