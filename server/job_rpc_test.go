package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
	"github.com/stretchr/testify/assert"
)

type mockJobOrchestrator struct {
	node       *core.LivepeerNode
	priv       *ecdsa.PrivateKey
	block      *big.Int
	signErr    error
	sessCapErr error
	priceInfo  *net.PriceInfo
	serviceURI string
	res        *core.TranscodeResult
	offchain   bool
	caps       *core.Capabilities
	authToken  *net.AuthToken

	registerExternalCapability func(string) (*core.ExternalCapability, error)
	verifySignature            func(common.Address, string, []byte) bool
	reserveCapacity            func(string) error
	getUrlForCapability        func(string) string
	balance                    func(common.Address, core.ManifestID) *big.Rat
	debitFees                  func(common.Address, core.ManifestID, *net.PriceInfo, int64)
	freeCapacity               func(string) error
	jobPriceInfo               func(common.Address, string) (*net.PriceInfo, error)
	ticketParams               func(ethcommon.Address, *net.PriceInfo) (*net.TicketParams, error)
}

func (r *mockJobOrchestrator) ServiceURI() *url.URL {
	if r.serviceURI == "" {
		r.serviceURI = "http://localhost:1234"
	}
	url, _ := url.Parse(r.serviceURI)
	return url
}

func (r *mockJobOrchestrator) Sign(msg []byte) ([]byte, error) {
	if r.offchain {
		return nil, nil
	}
	if r.signErr != nil {
		return nil, r.signErr
	}

	ethMsg := accounts.TextHash(ethcrypto.Keccak256(msg))
	sig, err := ethcrypto.Sign(ethMsg, r.priv)
	if err != nil {
		return nil, err
	}

	// sig is in the [R || S || V] format where V is 0 or 1
	// Convert the V param to 27 or 28
	v := sig[64]
	if v == byte(0) || v == byte(1) {
		v += 27
	}

	return append(sig[:64], v), nil
}

func (r *mockJobOrchestrator) VerifySig(addr ethcommon.Address, msg string, sig []byte) bool {
	return r.verifySignature(addr, msg, sig)
}

func (r *mockJobOrchestrator) Address() ethcommon.Address {
	if r.offchain {
		return ethcommon.Address{}
	}
	return ethcrypto.PubkeyToAddress(r.priv.PublicKey)
}
func (r *mockJobOrchestrator) TranscodeSeg(ctx context.Context, md *core.SegTranscodingMetadata, seg *stream.HLSSegment) (*core.TranscodeResult, error) {
	return r.res, nil
}
func (r *mockJobOrchestrator) StreamIDs(jobID string) ([]core.StreamID, error) {
	return []core.StreamID{}, nil
}

func (r *mockJobOrchestrator) ProcessPayment(ctx context.Context, payment net.Payment, manifestID core.ManifestID) error {
	return nil
}

func (r *mockJobOrchestrator) TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error) {
	return r.ticketParams(sender, priceInfo)
}

func (r *mockJobOrchestrator) PriceInfo(sender ethcommon.Address, manifestID core.ManifestID) (*net.PriceInfo, error) {
	return r.priceInfo, nil
}

func (r *mockJobOrchestrator) GetCapabilitiesPrices(sender ethcommon.Address) ([]*net.PriceInfo, error) {
	return []*net.PriceInfo{}, nil
}

func (r *mockJobOrchestrator) SufficientBalance(addr ethcommon.Address, manifestID core.ManifestID) bool {
	return true
}

func (r *mockJobOrchestrator) DebitFees(addr ethcommon.Address, manifestID core.ManifestID, price *net.PriceInfo, pixels int64) {
}

func (r *mockJobOrchestrator) Balance(addr ethcommon.Address, manifestID core.ManifestID) *big.Rat {
	return big.NewRat(0, 1)
}

func (r *mockJobOrchestrator) Capabilities() *net.Capabilities {
	if r.caps != nil {
		return r.caps.ToNetCapabilities()
	}
	return core.NewCapabilities(nil, nil).ToNetCapabilities()
}
func (r *mockJobOrchestrator) LegacyOnly() bool {
	return true
}

func (r *mockJobOrchestrator) AuthToken(sessionID string, expiration int64) *net.AuthToken {
	if r.authToken != nil {
		return r.authToken
	}
	return &net.AuthToken{Token: []byte("foo"), SessionId: sessionID, Expiration: expiration}
}

func (r *mockJobOrchestrator) CheckCapacity(mid core.ManifestID) error {
	return r.sessCapErr
}
func (r *mockJobOrchestrator) ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int, capabilities *net.Capabilities) {
}
func (r *mockJobOrchestrator) TranscoderResults(job int64, res *core.RemoteTranscoderResult) {
}
func (r *mockJobOrchestrator) TranscoderSecret() string {
	return "secret"
}
func (r *mockJobOrchestrator) PriceInfoForCaps(sender ethcommon.Address, manifestID core.ManifestID, caps *net.Capabilities) (*net.PriceInfo, error) {
	return &net.PriceInfo{PricePerUnit: 4, PixelsPerUnit: 1}, nil
}
func (r *mockJobOrchestrator) TextToImage(ctx context.Context, requestID string, req worker.GenTextToImageJSONRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) ImageToImage(ctx context.Context, requestID string, req worker.GenImageToImageMultipartRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) ImageToVideo(ctx context.Context, requestID string, req worker.GenImageToVideoMultipartRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) Upscale(ctx context.Context, requestID string, req worker.GenUpscaleMultipartRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) AudioToText(ctx context.Context, requestID string, req worker.GenAudioToTextMultipartRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) LLM(ctx context.Context, requestID string, req worker.GenLLMJSONRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) SegmentAnything2(ctx context.Context, requestID string, req worker.GenSegmentAnything2MultipartRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) ImageToText(ctx context.Context, requestID string, req worker.GenImageToTextMultipartRequestBody) (interface{}, error) {
	return nil, nil
}
func (r *mockJobOrchestrator) TextToSpeech(ctx context.Context, requestID string, req worker.GenTextToSpeechJSONRequestBody) (interface{}, error) {
	return nil, nil
}

func (r *mockJobOrchestrator) LiveVideoToVideo(ctx context.Context, requestID string, req worker.GenLiveVideoToVideoJSONRequestBody) (interface{}, error) {
	return nil, nil
}

func (r *mockJobOrchestrator) CheckAICapacity(pipeline, modelID string) bool {
	return true
}
func (r *mockJobOrchestrator) AIResults(job int64, res *core.RemoteAIWorkerResult) {
}
func (r *mockJobOrchestrator) CreateStorageForRequest(requestID string) error {
	return nil
}
func (r *mockJobOrchestrator) GetStorageForRequest(requestID string) (drivers.OSSession, bool) {
	return drivers.NewMockOSSession(), true
}
func (r *mockJobOrchestrator) WorkerHardware() []worker.HardwareInformation {
	return []worker.HardwareInformation{}
}
func (r *mockJobOrchestrator) ServeAIWorker(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities, hardware []*net.HardwareInformation) {
}
func (r *mockJobOrchestrator) RegisterExternalCapability(extCapabilitySettings string) (*core.ExternalCapability, error) {
	return r.registerExternalCapability(extCapabilitySettings)
}
func (r *mockJobOrchestrator) RemoveExternalCapability(extCapability string) error {
	return nil
}
func (r *mockJobOrchestrator) CheckExternalCapabilityCapacity(extCap string) bool {
	return true
}
func (r *mockJobOrchestrator) ReserveExternalCapabilityCapacity(extCap string) error {
	return nil
}
func (r *mockJobOrchestrator) FreeExternalCapabilityCapacity(extCap string) error {
	return r.freeCapacity(extCap)
}
func (r *mockJobOrchestrator) JobPriceInfo(sender ethcommon.Address, jobCapability string) (*net.PriceInfo, error) {
	return r.jobPriceInfo(sender, jobCapability)
}
func (r *mockJobOrchestrator) GetUrlForCapability(capability string) string {
	return r.getUrlForCapability(capability)
}

func newMockJobOrchestrator() *mockJobOrchestrator {
	pk, err := ethcrypto.GenerateKey()
	if err != nil {
		return &mockJobOrchestrator{}
	}
	mockOrch := &mockJobOrchestrator{
		priv:  pk,
		block: big.NewInt(5), // Set a default block number
	}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	mockOrch.node = node

	return &mockJobOrchestrator{priv: pk, block: big.NewInt(5)}
}

func mockJobLivepeerNode() *core.LivepeerNode {
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.NodeType = core.OrchestratorNode
	node.OrchSecret = "verbigsecret"
	return node
}

// Tests for RegisterCapability
func TestRegisterCapability_MethodNotAllowed(t *testing.T) {
	h := &lphttp{
		orchestrator: &mockOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/capability", nil)
	w := httptest.NewRecorder()

	h.RegisterCapability(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestRegisterCapability_InvalidAuthorization(t *testing.T) {

	h := &lphttp{
		orchestrator: newMockJobOrchestrator(),
	}
	h.orchestrator.TranscoderSecret()

	req := httptest.NewRequest("POST", "/capability", nil)
	req.Header.Set("Authorization", "invalid-secret")
	w := httptest.NewRecorder()

	h.RegisterCapability(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterCapability_Success(t *testing.T) {
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.registerExternalCapability = func(settings string) (*core.ExternalCapability, error) {
		return &core.ExternalCapability{
			Name: "test-cap",
			Url:  "http://localhost:8080",
		}, nil
	}

	h := &lphttp{
		orchestrator: mockJobOrch,
	}

	req := httptest.NewRequest("POST", "/capability", bytes.NewBufferString("test settings"))
	req.Header.Set("Authorization", mockJobOrch.TranscoderSecret())
	w := httptest.NewRecorder()

	h.RegisterCapability(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRegisterCapability_Error(t *testing.T) {
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.registerExternalCapability = func(settings string) (*core.ExternalCapability, error) {
		return nil, errors.New("registration failed")
	}

	h := &lphttp{
		orchestrator: mockJobOrch,
	}

	req := httptest.NewRequest("POST", "/capability", bytes.NewBufferString("test settings"))
	req.Header.Set("Authorization", mockJobOrch.TranscoderSecret())
	w := httptest.NewRecorder()

	h.RegisterCapability(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// Tests for GetJobToken
func TestGetJobToken_MethodNotAllowed(t *testing.T) {
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: &mockJobOrchestrator{},
	}

	req := httptest.NewRequest("POST", "/token", nil)
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestGetJobToken_NotOrchestrator(t *testing.T) {
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: &mockJobOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/token", nil)
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_MissingEthAddressHeader(t *testing.T) {
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: &mockOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/token", nil)
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_InvalidEthAddressHeader(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return false
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_MissingCapabilityHeader(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return true
	}
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_NoCapacity(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return errors.New("no capacity")
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity

	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestGetJobToken_JobPriceInfoError(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr common.Address, cap string) (*net.PriceInfo, error) {
		return nil, errors.New("price error")
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_InsufficientReserve(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr common.Address, cap string) (*net.PriceInfo, error) {
		return nil, errors.New("insufficient sender reserve")
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo

	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestGetJobToken_TicketParamsError(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr common.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
		}, nil
	}

	mockTicketParams := func(addr common.Address, price *net.PriceInfo) (*net.TicketParams, error) {
		return nil, errors.New("ticket params error")
	}
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	mockJobOrch.ticketParams = mockTicketParams

	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_Success(t *testing.T) {
	mockVerifySig := func(addr common.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr common.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
		}, nil
	}

	mockTicketParams := func(addr common.Address, price *net.PriceInfo) (*net.TicketParams, error) {
		return &net.TicketParams{
			Recipient:         common.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
			FaceValue:         big.NewInt(1000).Bytes(),
			WinProb:           big.NewInt(1).Bytes(),
			RecipientRandHash: []byte("hash"),
			Seed:              big.NewInt(1234).Bytes(),
			ExpirationBlock:   big.NewInt(100000).Bytes(),
		}, nil
	}

	mockBalance := func(addr common.Address, manifestID core.ManifestID) *big.Rat {
		return big.NewRat(1000, 1)
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	mockJobOrch.ticketParams = mockTicketParams
	mockJobOrch.balance = mockBalance

	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	h.GetJobToken(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var token JobToken
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &token)

	assert.NotNil(t, token.TicketParams)
	assert.Equal(t, int64(1), token.Balance)
}

// Tests for ProcessJob
func TestProcessJob_MethodNotAllowed(t *testing.T) {
	h := &lphttp{
		node:         mockJobLivepeerNode(),
		orchestrator: &mockOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/process", nil)
	w := httptest.NewRecorder()

	h.ProcessJob(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

// Tests for SubmitJob handler
func TestSubmitJob_MethodNotAllowed(t *testing.T) {
	ls := &LivepeerServer{
		LivepeerNode: mockJobLivepeerNode(),
	}

	handler := ls.SubmitJob()

	req := httptest.NewRequest("GET", "/submit", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
