package server

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
)

type mockJobOrchestrator struct {
	stubOrchestrator
	priv                       *ecdsa.PrivateKey
	block                      *big.Int
	node                       *core.LivepeerNode
	registerExternalCapability func(string) (*core.ExternalCapability, error)
	verifySignature            func(common.Address, string, []byte) bool
	reserveCapacity            func(string) error
	getUrlForCapability        func(string) string
	jobPriceInfo               func(common.Address, string) (*net.PriceInfo, error)
	ticketParams               func(common.Address, *net.PriceInfo) (*net.TicketParams, error)
	balance                    func(common.Address, core.ManifestID) *big.Rat
	debitFees                  func(common.Address, core.ManifestID, *net.PriceInfo, int64)
	freeCapacity               func(string)
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
	req.Header.Set("Authorization", "secret")
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
	req.Header.Set("Authorization", "secret")
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
