package byoc

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
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/core"

	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockJobOrchestrator struct {
	node                 *core.LivepeerNode
	priv                 *ecdsa.PrivateKey
	block                *big.Int
	signErr              error
	sessCapErr           error
	priceInfo            *net.PriceInfo
	serviceURI           string
	res                  *core.TranscodeResult
	offchain             bool
	caps                 *core.Capabilities
	authToken            *net.AuthToken
	externalCapabilities map[string]*core.ExternalCapability
	extraNodes           int

	registerExternalCapability      func(string) (*core.ExternalCapability, error)
	unregisterExternalCapability    func(string) error
	verifySignature                 func(ethcommon.Address, string, []byte) bool
	checkExternalCapabilityCapacity func(string) int64
	reserveCapacity                 func(string) error
	getUrlForCapability             func(string) string
	balance                         func(ethcommon.Address, core.ManifestID) *big.Rat
	processPayment                  func(context.Context, net.Payment, core.ManifestID) error
	debitFees                       func(ethcommon.Address, core.ManifestID, *net.PriceInfo, int64)
	freeCapacity                    func(string) error
	jobPriceInfo                    func(ethcommon.Address, string) (*net.PriceInfo, error)
	ticketParams                    func(ethcommon.Address, *net.PriceInfo) (*net.TicketParams, error)

	mock.Mock
}

func (r *mockJobOrchestrator) ServiceURI() *url.URL {
	if r.serviceURI == "" {
		r.serviceURI = "http://localhost:1234"
	}
	url, _ := url.Parse(r.serviceURI)
	return url
}

// Nodes, ExtraNodes and Sign methods needed because the mockJobOrchestrator is reused as a stubGateway
func (r *mockJobOrchestrator) Nodes() []string {
	return nil
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
func (r *mockJobOrchestrator) ExtraNodes() int {
	return r.extraNodes
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
func (r *mockJobOrchestrator) ProcessPayment(ctx context.Context, payment net.Payment, manifestID core.ManifestID) error {
	if r.processPayment != nil {
		return r.processPayment(ctx, payment, manifestID)
	}
	return nil
}
func (r *mockJobOrchestrator) TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error) {
	return r.ticketParams(sender, priceInfo)
}
func (r *mockJobOrchestrator) DebitFees(addr ethcommon.Address, manifestID core.ManifestID, price *net.PriceInfo, pixels int64) {
	if r.debitFees != nil {
		r.debitFees(addr, manifestID, price, pixels)
	}
}
func (r *mockJobOrchestrator) Balance(addr ethcommon.Address, manifestID core.ManifestID) *big.Rat {
	if r.balance != nil {
		return r.balance(addr, manifestID)
	} else {
		return big.NewRat(0, 1)
	}
}
func (r *mockJobOrchestrator) TranscoderSecret() string {
	return "secret"
}
func (r *mockJobOrchestrator) RegisterExternalCapability(extCapabilitySettings string) (*core.ExternalCapability, error) {
	return r.registerExternalCapability(extCapabilitySettings)
}
func (r *mockJobOrchestrator) RemoveExternalCapability(extCapability string) error {
	// RemoveExternalCapability deletes the key from the external capabilities map
	// if the key does not exist the delete is a no-op so no error is possible
	delete(r.externalCapabilities, extCapability)
	if r.unregisterExternalCapability != nil {
		return r.unregisterExternalCapability(extCapability)
	}

	return nil
}
func (r *mockJobOrchestrator) CheckExternalCapabilityCapacity(extCap string) int64 {
	if r.checkExternalCapabilityCapacity == nil {
		return 1
	} else {
		return r.checkExternalCapabilityCapacity(extCap)
	}
}
func (r *mockJobOrchestrator) ReserveExternalCapabilityCapacity(extCap string) error {
	if r.reserveCapacity == nil {
		return nil
	} else {
		return r.reserveCapacity(extCap)
	}
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

	return mockOrch
}

func mockJobLivepeerNode() *core.LivepeerNode {
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.NodeType = core.OrchestratorNode
	node.OrchSecret = "verbigsecret"
	node.LiveMu = &sync.RWMutex{}
	return node
}

func TestRegisterCapability_MethodNotAllowed(t *testing.T) {
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
	}
	req := httptest.NewRequest("GET", "/capability/register", nil)
	w := httptest.NewRecorder()

	handler := bso.RegisterCapability()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestRegisterCapability_InvalidAuthorization(t *testing.T) {
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: newMockJobOrchestrator(),
	}
	bso.orch.TranscoderSecret()

	req := httptest.NewRequest("POST", "/capability/register", nil)
	req.Header.Set("Authorization", "invalid-secret")
	w := httptest.NewRecorder()

	handler := bso.RegisterCapability()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterCapability_Success(t *testing.T) {
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.registerExternalCapability = func(settings string) (*core.ExternalCapability, error) {
		return &core.ExternalCapability{
			Name:         "test-cap",
			Url:          "http://localhost:8080",
			PricePerUnit: 1,
			PriceScaling: 1,
		}, nil
	}

	bso := &BYOCOrchestratorServer{
		orch: mockJobOrch,
	}

	req := httptest.NewRequest("POST", "/capability/register", bytes.NewBufferString("test settings"))
	req.Header.Set("Authorization", mockJobOrch.TranscoderSecret())
	w := httptest.NewRecorder()

	handler := bso.RegisterCapability()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRegisterCapability_Error(t *testing.T) {
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.registerExternalCapability = func(settings string) (*core.ExternalCapability, error) {
		return nil, errors.New("registration failed")
	}

	bso := &BYOCOrchestratorServer{
		orch: mockJobOrch,
	}

	req := httptest.NewRequest("POST", "/capability/register", bytes.NewBufferString("test settings"))
	req.Header.Set("Authorization", mockJobOrch.TranscoderSecret())
	w := httptest.NewRecorder()

	handler := bso.RegisterCapability()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUnregisterCapability(t *testing.T) {
	// Setup
	mockOrch := newMockJobOrchestrator()
	secret := mockOrch.TranscoderSecret()
	// Register a test capability we'll unregister
	capName := "test-capability"
	mockOrch.externalCapabilities = make(map[string]*core.ExternalCapability)
	mockOrch.externalCapabilities[capName] = &core.ExternalCapability{Name: capName}

	// Create handler with our mock orchestrator
	bso := &BYOCOrchestratorServer{
		orch: mockOrch,
	}

	t.Run("SuccessfulUnregister", func(t *testing.T) {

		// Create test request
		req := httptest.NewRequest(http.MethodPost, "/capability/unregister",
			bytes.NewBufferString(capName))
		req.Header.Set("Authorization", secret)

		// Execute request
		recorder := httptest.NewRecorder()
		handler := bso.UnregisterCapability()
		handler.ServeHTTP(recorder, req)

		// Verify results
		resp := recorder.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_, exists := mockOrch.externalCapabilities[capName]
		assert.False(t, exists, "Capability should be removed")
	})

	t.Run("WrongMethod", func(t *testing.T) {
		// Try with GET instead of POST
		req := httptest.NewRequest(http.MethodGet, "/capability/unregister",
			bytes.NewBufferString(capName))
		req.Header.Set("Authorization", secret)

		recorder := httptest.NewRecorder()
		handler := bso.UnregisterCapability()
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusMethodNotAllowed, recorder.Result().StatusCode)
	})

	t.Run("InvalidAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/capability/unregister",
			bytes.NewBufferString(capName))
		req.Header.Set("Authorization", "wrong-secret")

		recorder := httptest.NewRecorder()
		handler := bso.UnregisterCapability()
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Result().StatusCode)
	})

	t.Run("MissingAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/capability/unregister",
			bytes.NewBufferString(capName))

		recorder := httptest.NewRecorder()
		handler := bso.UnregisterCapability()
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Result().StatusCode)
	})

	t.Run("ErrorFromOrchestrator", func(t *testing.T) {
		// Set up orchestrator to return an error
		mockOrch.unregisterExternalCapability = func(capability string) error {
			return errors.New("no capability")
		}

		req := httptest.NewRequest(http.MethodPost, "/capability/unregister",
			bytes.NewBufferString("non-existent-capability"))
		req.Header.Set("Authorization", secret)

		recorder := httptest.NewRecorder()
		handler := bso.UnregisterCapability()
		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusBadRequest, recorder.Result().StatusCode)

		mockOrch.unregisterExternalCapability = nil
	})

	t.Run("EmptyCapabilityName", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/capability/unregister",
			bytes.NewBufferString(""))
		req.Header.Set("Authorization", secret)

		recorder := httptest.NewRecorder()
		handler := bso.UnregisterCapability()
		handler.ServeHTTP(recorder, req)

		// Should still work, but will attempt to remove an empty string capability
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})
}

func TestGetJobToken_MethodNotAllowed(t *testing.T) {
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: &mockJobOrchestrator{},
	}

	req := httptest.NewRequest("POST", "/process/token", nil)
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestGetJobToken_NotOrchestrator(t *testing.T) {
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: &mockJobOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/process/token", nil)
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_MissingEthAddressHeader(t *testing.T) {
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: &mockJobOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/process/token", nil)
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_InvalidEthAddressHeader(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return false
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_MissingCapabilityHeader(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	js := &JobSender{
		Addr: "0x0000000000000000000000000000000000000000",
		Sig:  "0x000000000000000000000000000000000000000000000000000000000000000000",
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_NoCapacity(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}
	mockCheckExternalCapabilityCapacity := func(extCap string) int64 {
		return 0
	}

	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
		}, nil
	}

	mockTicketParams := func(addr ethcommon.Address, price *net.PriceInfo) (*net.TicketParams, error) {
		return &net.TicketParams{
			Recipient:         ethcommon.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
			FaceValue:         big.NewInt(1000).Bytes(),
			WinProb:           big.NewInt(1).Bytes(),
			RecipientRandHash: []byte("hash"),
			Seed:              big.NewInt(1234).Bytes(),
			ExpirationBlock:   big.NewInt(100000).Bytes(),
		}, nil
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.checkExternalCapabilityCapacity = mockCheckExternalCapabilityCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	mockJobOrch.ticketParams = mockTicketParams

	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	gateway := newMockJobOrchestrator()
	sig, _ := gateway.Sign([]byte(hexutil.Encode(gateway.Address().Bytes())))
	js := &JobSender{
		Addr: hexutil.Encode(gateway.Address().Bytes()),
		Sig:  hexutil.Encode(sig),
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	var jobToken JobToken
	json.Unmarshal(body, &jobToken)
	assert.Equal(t, int64(0), jobToken.AvailableCapacity)
}

func TestGetJobToken_JobPriceInfoError(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return nil, errors.New("price error")
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	gateway := newMockJobOrchestrator()
	sig, _ := gateway.Sign([]byte(hexutil.Encode(gateway.Address().Bytes())))
	js := &JobSender{
		Addr: hexutil.Encode(gateway.Address().Bytes()),
		Sig:  hexutil.Encode(sig),
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_InsufficientReserve(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return nil, errors.New("insufficient sender reserve")
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo

	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	gateway := newMockJobOrchestrator()
	sig, _ := gateway.Sign([]byte(hexutil.Encode(gateway.Address().Bytes())))
	js := &JobSender{
		Addr: hexutil.Encode(gateway.Address().Bytes()),
		Sig:  hexutil.Encode(sig),
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestGetJobToken_TicketParamsError(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
		}, nil
	}

	mockTicketParams := func(addr ethcommon.Address, price *net.PriceInfo) (*net.TicketParams, error) {
		return nil, errors.New("ticket params error")
	}
	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	mockJobOrch.ticketParams = mockTicketParams

	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	gateway := newMockJobOrchestrator()
	sig, _ := gateway.Sign([]byte(hexutil.Encode(gateway.Address().Bytes())))
	js := &JobSender{
		Addr: hexutil.Encode(gateway.Address().Bytes()),
		Sig:  hexutil.Encode(sig),
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetJobToken_Success(t *testing.T) {
	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}

	mockReserveCapacity := func(cap string) error {
		return nil
	}

	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
		}, nil
	}

	mockTicketParams := func(addr ethcommon.Address, price *net.PriceInfo) (*net.TicketParams, error) {
		return &net.TicketParams{
			Recipient:         ethcommon.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
			FaceValue:         big.NewInt(1000).Bytes(),
			WinProb:           big.NewInt(1).Bytes(),
			RecipientRandHash: []byte("hash"),
			Seed:              big.NewInt(1234).Bytes(),
			ExpirationBlock:   big.NewInt(100000).Bytes(),
		}, nil
	}

	mockBalance := func(addr ethcommon.Address, manifestID core.ManifestID) *big.Rat {
		return big.NewRat(1000, 1)
	}

	mockJobOrch := newMockJobOrchestrator()
	mockJobOrch.verifySignature = mockVerifySig
	mockJobOrch.reserveCapacity = mockReserveCapacity
	mockJobOrch.jobPriceInfo = mockJobPriceInfo
	mockJobOrch.ticketParams = mockTicketParams
	mockJobOrch.balance = mockBalance

	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: mockJobOrch,
	}

	// Create a valid JobSender structure
	gateway := newMockJobOrchestrator()
	sig, _ := gateway.Sign([]byte(hexutil.Encode(gateway.Address().Bytes())))
	js := &JobSender{
		Addr: hexutil.Encode(gateway.Address().Bytes()),
		Sig:  hexutil.Encode(sig),
	}
	jsBytes, _ := json.Marshal(js)
	jsBase64 := base64.StdEncoding.EncodeToString(jsBytes)

	req := httptest.NewRequest("GET", "/process/token", nil)
	req.Header.Set(jobEthAddressHdr, jsBase64)
	req.Header.Set(jobCapabilityHdr, "test-cap")
	w := httptest.NewRecorder()

	handler := bso.GetJobToken()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var token JobToken
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &token)

	assert.NotNil(t, token.TicketParams)
	assert.Equal(t, int64(1000), token.Balance)
}

func TestProcessJob_MethodNotAllowed(t *testing.T) {
	bso := &BYOCOrchestratorServer{
		node: mockJobLivepeerNode(),
		orch: &mockJobOrchestrator{},
	}

	req := httptest.NewRequest("GET", "/process/request/gg", nil)
	w := httptest.NewRecorder()

	handler := bso.ProcessJob()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestProcessPayment(t *testing.T) {

	ctx := context.Background()
	sender := ethcommon.HexToAddress("0x1111111111111111111111111111111111111111")

	cases := []struct {
		name        string
		capability  string
		expectDelta bool
	}{
		{"empty header", "testcap", false},
		{"empty capability", "", false},
		{"random capability", "randomcap", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate a mutable balance for the test
			testBalance := big.NewRat(100, 1)
			balanceCalled := 0
			paymentCalled := 0
			orch := newMockJobOrchestrator()
			bso := &BYOCOrchestratorServer{node: orch.node, orch: orch, sharedBalMtx: &sync.Mutex{}}

			orch.node.Balances = core.NewAddressBalances(1 * time.Second)
			defer orch.node.Balances.StopCleanup()
			orch.balance = func(addr ethcommon.Address, manifestID core.ManifestID) *big.Rat {
				balanceCalled++
				return new(big.Rat).Set(testBalance)
			}
			orch.processPayment = func(ctx context.Context, payment net.Payment, manifestID core.ManifestID) error {
				paymentCalled++
				// Simulate payment by increasing balance
				testBalance = testBalance.Add(testBalance, big.NewRat(50, 1))
				return nil
			}

			testPmtHdr, err := createTestPayment(tc.capability)
			if err != nil {
				t.Fatalf("Failed to create test payment: %v", err)
			}

			before := orch.Balance(sender, core.ManifestID(tc.capability)).FloatString(0)
			bal, err := bso.processPayment(ctx, sender, tc.capability, testPmtHdr)
			after := orch.Balance(sender, core.ManifestID(tc.capability)).FloatString(0)
			t.Logf("Balance before: %s, after: %s", before, after)
			assert.NoError(t, err)
			assert.NotNil(t, bal)
			if testPmtHdr != "" {
				assert.NotEqual(t, before, after, "Balance should change if payment header is not empty")
				assert.Equal(t, 1, paymentCalled, "ProcessPayment should be called once for non-empty header")
			} else {
				assert.Equal(t, before, after, "Balance should not change if payment header is empty")
				assert.Equal(t, 0, paymentCalled, "ProcessPayment should not be called for empty header")
			}
		})
	}
}

// marshalToString is a helper to marshal a struct to a JSON string
func marshalToString(t *testing.T, v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalToString failed: %v", err)
	}
	return string(b)
}

func orchTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/process/token" {
		http.NotFound(w, r)
		return
	}

	token := createMockJobToken("http://" + r.Host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)

}

func createMockJobToken(hostUrl string) *JobToken {
	maxWinProb := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	winProb10 := new(big.Int).Div(maxWinProb, big.NewInt(10))
	return &JobToken{
		ServiceAddr: hostUrl,
		SenderAddress: &JobSender{
			Addr: "0x1234567890abcdef1234567890abcdef123456",
			Sig:  "0x456",
		},
		TicketParams: &net.TicketParams{
			Recipient: ethcommon.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
			FaceValue: big.NewInt(1000).Bytes(),
			WinProb:   winProb10.Bytes(),
		},
		Price: &net.PriceInfo{
			PricePerUnit:  100,
			PixelsPerUnit: 1,
		},
		AvailableCapacity: 1,
	}
}
