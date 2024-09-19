package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Status
type mockChainIdGetter struct {
	mock.Mock
}

func (m *mockChainIdGetter) ChainID() (*big.Int, error) {
	return mockBigInt(m.Called())
}

func TestChainIdHandler_Error(t *testing.T) {
	assert := assert.New(t)

	db := &mockChainIdGetter{}
	db.On("ChainID").Return(nil, errors.New("ChainID error"))

	handler := ethChainIdHandler(db)
	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("Error getting eth network ID err=\"ChainID error\"", body)
}

func TestChainIdHandler_Success(t *testing.T) {
	assert := assert.New(t)

	db := &mockChainIdGetter{}
	db.On("ChainID").Return(big.NewInt(4), nil)

	handler := ethChainIdHandler(db)
	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("4", body)
}

type mockBlockGetter struct {
	mock.Mock
}

func (m *mockBlockGetter) LastSeenBlock() (*big.Int, error) {
	return mockBigInt(m.Called())
}

func TestCurrentBlockHandler_Error(t *testing.T) {
	assert := assert.New(t)

	db := &mockBlockGetter{}
	db.On("LastSeenBlock").Return(nil, errors.New("LastSeenBlock error"))

	handler := currentBlockHandler(db)
	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not query last seen block: LastSeenBlock error", body)
}

func TestCurrentBlockHandler_Success(t *testing.T) {
	assert := assert.New(t)

	db := &mockBlockGetter{}
	db.On("LastSeenBlock").Return(big.NewInt(50), nil)

	handler := currentBlockHandler(db)
	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal(big.NewInt(50), new(big.Int).SetBytes([]byte(body)))
}

func TestOrchestratorInfoHandler_TranscoderError(t *testing.T) {
	assert := assert.New(t)

	s := &LivepeerServer{}
	client := &eth.MockClient{}
	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(nil, errors.New("ErrNoResult"))

	handler := s.orchestratorInfoHandler(client)
	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not get transcoder", body)
}

func TestOrchestratorInfoHandler_Success(t *testing.T) {
	assert := assert.New(t)

	n, _ := core.NewLivepeerNode(nil, "", nil)
	s := &LivepeerServer{LivepeerNode: n}

	price := big.NewRat(1, 2)
	s.LivepeerNode.SetBasePrice("default", core.NewFixedPrice(price))

	trans := &types.Transcoder{
		ServiceURI: "127.0.0.1:8935",
	}
	client := &eth.MockClient{}
	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil)

	handler := s.orchestratorInfoHandler(client)
	status, body := get(handler)

	var info orchInfo
	err := json.Unmarshal([]byte(body), &info)
	assert.NoError(err)

	assert.Equal(http.StatusOK, status)
	assert.Equal(price, info.PriceInfo)
	assert.Equal(trans, info.Transcoder)
}

func TestSetMaxFaceValueHandler(t *testing.T) {
	assert := assert.New(t)
	s := stubOrchestratorWithRecipient(t)

	handler := s.setMaxFaceValueHandler()
	status, _ := postForm(handler, url.Values{
		"maxfacevalue": {"10000000000000000"},
	})
	assert.Equal(http.StatusOK, status)
}

func TestSetMaxFaceValueHandler_WrongVariableSet(t *testing.T) {
	assert := assert.New(t)
	s := stubOrchestratorWithRecipient(t)

	handler := s.setMaxFaceValueHandler()
	status, body := postForm(handler, url.Values{
		"facevalue": {"10000000000000000"},
	})
	assert.Equal(http.StatusBadRequest, status)
	assert.Equal("need to set 'maxfacevalue'", body)
}

func TestSetMaxFaceValueHandler_WrongValueSet(t *testing.T) {
	assert := assert.New(t)
	s := stubOrchestratorWithRecipient(t)

	handler := s.setMaxFaceValueHandler()
	status, body := postForm(handler, url.Values{
		"maxfacevalue": {"test"},
	})
	assert.Equal(http.StatusBadRequest, status)
	assert.Equal("maxfacevalue not set to number", body)
}

// Broadcast / Transcoding config
func TestSetBroadcastConfigHandler_MissingPricePerUnitError(t *testing.T) {
	assert := assert.New(t)

	handler := setBroadcastConfigHandler()
	status, body := postForm(handler, url.Values{
		"maxPricePerUnit": {"1"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Equal("missing form params (maxPricePerUnit AND pixelsPerUnit) or transcodingOptions", body)
}

func TestSetBroadcastConfigHandler_ConvertPricePerUnitError(t *testing.T) {
	assert := assert.New(t)

	handler := setBroadcastConfigHandler()
	status, body := postForm(handler, url.Values{
		"maxPricePerUnit": {"invalidParameter"},
		"pixelsPerUnit":   {"2"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "Error parsing pricePerUnit value")
}

func TestSetBroadcastConfigHandler_ConvertPixelsPerUnitError(t *testing.T) {
	assert := assert.New(t)

	handler := setBroadcastConfigHandler()
	status, body := postForm(handler, url.Values{
		"maxPricePerUnit": {"1"},
		"pixelsPerUnit":   {"invalidParameter"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "Error parsing pixelsPerUnit value")
}

func TestSetBroadcastConfigHandler_NegativePixelPerUnitError(t *testing.T) {
	assert := assert.New(t)

	handler := setBroadcastConfigHandler()
	status, body := postForm(handler, url.Values{
		"maxPricePerUnit": {"1"},
		"pixelsPerUnit":   {"-2"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "pixels per unit must be greater than 0")
}

func TestSetBroadcastConfigHandler_TranscodingOptionsError(t *testing.T) {
	assert := assert.New(t)

	handler := setBroadcastConfigHandler()
	status, body := postForm(handler, url.Values{
		"transcodingOptions": {"invalidTranscodingOptions"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "invalid transcoding options")
}

func TestSetBroadcastConfigHandler_Success(t *testing.T) {
	assert := assert.New(t)

	handler := setBroadcastConfigHandler()
	status, _ := postForm(handler, url.Values{
		"maxPricePerUnit":    {"1"},
		"pixelsPerUnit":      {"2"},
		"transcodingOptions": {"P720p25fps16x9,P360p25fps16x9"},
	})

	assert.Equal(http.StatusOK, status)
	assert.Equal(big.NewRat(1, 2), BroadcastCfg.MaxPrice())
	profiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfileLookup["P720p25fps16x9"],
		ffmpeg.VideoProfileLookup["P360p25fps16x9"],
	}
	assert.Equal(profiles, BroadcastJobVideoProfiles)
}

func TestGetBroadcastConfigHandler(t *testing.T) {
	assert := assert.New(t)

	BroadcastCfg.maxPrice = core.NewFixedPrice(big.NewRat(1, 2))
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{
		ffmpeg.VideoProfileLookup["P240p25fps16x9"],
	}

	handler := getBroadcastConfigHandler()

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	expected := `{"MaxPrice":"1/2","TranscodingOptions":"P240p25fps16x9"}`
	assert.JSONEq(expected, body)
}

func TestGetAvailableTranscodingOptionsHandler(t *testing.T) {
	assert := assert.New(t)

	handler := getAvailableTranscodingOptionsHandler()

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.NotEmpty(body)
}

// Rounds
func TestCurrentRoundHandler_Error(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := currentRoundHandler(client)
	client.On("CurrentRound").Return(nil, errors.New("CurrentRound error")).Once()

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not query current round: CurrentRound error", body)
}

func TestCurrentRoundHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := currentRoundHandler(client)
	currentRound := big.NewInt(7)
	client.On("CurrentRound").Return(currentRound, nil)

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	cr, _ := new(big.Int).SetString(body, 10)

	assert.Equal(currentRound, cr)
}

func TestInitializeRoundHandler_InitializeRoundError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := initializeRoundHandler(client)
	client.On("InitializeRound").Return(nil, errors.New("InitializeRound error")).Once()

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not initialize round: InitializeRound error", body)
}

func TestInitializeRoundHandler_CheckTxError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := initializeRoundHandler(client)
	client.On("InitializeRound").Return(nil, nil).Once()
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error")).Once()

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not initialize round: CheckTx error", body)
}

func TestInitializeRoundHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := initializeRoundHandler(client)
	client.On("InitializeRound").Return(nil, nil).Once()
	client.On("CheckTx", mock.Anything).Return(nil).Once()

	status, _ := get(handler)

	assert.Equal(http.StatusOK, status)
}

func TestRoundInitialized_Error(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := roundInitializedHandler(client)
	client.On("CurrentRoundInitialized").Return(false, errors.New("CurrentRoundInitialized error")).Once()

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not get initialized round: CurrentRoundInitialized error", body)
}

func TestRoundInitialized_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := roundInitializedHandler(client)
	client.On("CurrentRoundInitialized").Return(true, nil).Once()

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("true", body)
}

// Orchestrator registration/activation
func TestActivateOrchestratorHandler_GetTranscoderError(t *testing.T) {
	assert := assert.New(t)

	server := stubServer()
	client := &eth.MockClient{}
	handler := server.activateOrchestratorHandler(client)

	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(nil, errors.New("GetTranscoder error")).Once()

	status, body := post(handler)
	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("GetTranscoder error", body)
}

func TestActivateOrchestratorHandler_TranscoderAlreadyRegisteredError(t *testing.T) {
	assert := assert.New(t)

	server := stubServer()
	client := &eth.MockClient{}
	handler := server.activateOrchestratorHandler(client)

	addr := ethcommon.Address{}
	trans := &types.Transcoder{Status: "Registered"}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil).Once()

	status, body := post(handler)

	assert.Equal(http.StatusBadRequest, status)
	assert.Equal("orchestrator already registered", body)
}

func TestActivateOrchestratorHandler_CurrentRoundLockedError(t *testing.T) {
	assert := assert.New(t)

	server := stubServer()
	client := &eth.MockClient{}
	handler := server.activateOrchestratorHandler(client)

	addr := ethcommon.Address{}
	trans := &types.Transcoder{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil).Once()
	client.On("CurrentRoundLocked").Return(false, errors.New("CurrentRoundLocked error")).Once()

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("CurrentRoundLocked error", body)
}

func TestActivateOrchestratorHandler_CurrentRoundIsLockedError(t *testing.T) {
	assert := assert.New(t)

	server := stubServer()
	client := &eth.MockClient{}
	handler := server.activateOrchestratorHandler(client)

	addr := ethcommon.Address{}
	trans := &types.Transcoder{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil).Once()
	client.On("CurrentRoundLocked").Return(true, nil).Once()

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("current round is locked", body)
}

func TestActivateOrchestratorHandler_Success(t *testing.T) {
	assert := assert.New(t)

	server := stubServer()
	client := &eth.MockClient{}
	handler := server.activateOrchestratorHandler(client)

	addr := ethcommon.Address{}
	trans := &types.Transcoder{}
	serviceURI := "http://127.0.0.1:8935"
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil).Once()
	client.On("CurrentRoundLocked").Return(false, nil).Once()
	client.On("CheckTx", mock.Anything).Return(nil).Once()
	client.On("GetServiceURI", addr).Return(serviceURI, nil).Once()

	status, _ := postForm(handler, url.Values{
		"blockRewardCut": {"0.5"},
		"feeShare":       {"0.5"},
		"pricePerUnit":   {"1"},
		"pixelsPerUnit":  {"1"},
		"serviceURI":     {serviceURI},
	})

	assert.Equal(http.StatusOK, status)
}

func TestSetOrchestratorConfigHandler_NoChangeSuccess(t *testing.T) {
	assert := assert.New(t)

	server := stubServer()
	client := &eth.MockClient{}
	handler := server.setOrchestratorConfigHandler(client)

	addr := ethcommon.Address{}
	trans := &types.Transcoder{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil).Once()

	status, _ := post(handler)

	assert.Equal(http.StatusOK, status)
}

func TestSetOrchestratorPriceInfo(t *testing.T) {
	s := stubServer()

	// pricePerUnit is not an integer
	err := s.setOrchestratorPriceInfo("default", "nil", "1", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing pricePerUnit value")

	// pixelsPerUnit is not an integer
	err = s.setOrchestratorPriceInfo("default", "1", "nil", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing pixelsPerUnit value")

	// price feed watcher is not initialized and one attempts a custom currency
	err = s.setOrchestratorPriceInfo("default", "1e12", "0.7", "USD")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PriceFeedWatcher is not initialized")

	err = s.setOrchestratorPriceInfo("default", "1", "1", "")
	assert.Nil(t, err)
	assert.Zero(t, s.LivepeerNode.GetBasePrice("default").Cmp(big.NewRat(1, 1)))

	err = s.setOrchestratorPriceInfo("default", "-5", "1", "")
	assert.EqualError(t, err, fmt.Sprintf("price unit must be greater than or equal to 0, provided %d", -5))

	// pixels per unit <= 0
	err = s.setOrchestratorPriceInfo("default", "1", "0", "")
	assert.EqualError(t, err, fmt.Sprintf("pixels per unit must be greater than 0, provided %d", 0))
	err = s.setOrchestratorPriceInfo("default", "1", "-5", "")
	assert.EqualError(t, err, fmt.Sprintf("pixels per unit must be greater than 0, provided %d", -5))

}
func TestSetPriceForBroadcasterHandler(t *testing.T) {
	assert := assert.New(t)
	s := stubServer()
	s.LivepeerNode.NodeType = core.OrchestratorNode

	handler := s.setPriceForBroadcaster()

	//set price per pixel for separate B eth address
	b1 := ethcommon.Address{}
	b1p := big.NewRat(1, 1)
	b2 := ethcommon.Address{1}
	b2p := big.NewRat(2, 1)

	statusd, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"10"},
		"pixelsPerUnit":      {"1"},
		"broadcasterEthAddr": {"default"},
	})
	assert.Equal(http.StatusOK, statusd)
	assert.Equal(big.NewRat(10, 1), s.LivepeerNode.GetBasePrice("default"))

	status1, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"1"},
		"pixelsPerUnit":      {"1"},
		"broadcasterEthAddr": {b1.String()},
	})
	assert.Equal(http.StatusOK, status1)
	assert.Equal(b1p, s.LivepeerNode.GetBasePrice(b1.String()))

	status2, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"2"},
		"pixelsPerUnit":      {"1"},
		"broadcasterEthAddr": {b2.String()},
	})

	assert.Equal(http.StatusOK, status2)
	assert.Equal(b2p, s.LivepeerNode.GetBasePrice(b2.String()))
	assert.NotEqual(b1p, s.LivepeerNode.GetBasePrice("default"))
	assert.NotEqual(b2p, s.LivepeerNode.GetBasePrice("default"))
}

func TestSetPriceForBroadcasterHandler_NotOrchestrator(t *testing.T) {
	assert := assert.New(t)
	s := stubServer()
	s.LivepeerNode.NodeType = core.TranscoderNode

	handler := s.setPriceForBroadcaster()

	status, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"10"},
		"pixelsPerUnit":      {"1"},
		"broadcasterEthAddr": {"default"},
	})
	assert.Equal(http.StatusBadRequest, status)
}

func TestSetPriceForBroadcasterHandler_WrongInput(t *testing.T) {
	assert := assert.New(t)
	s := stubServer()
	s.LivepeerNode.NodeType = core.TranscoderNode

	handler := s.setPriceForBroadcaster()

	status1, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"a"},
		"pixelsPerUnit":      {"1"},
		"broadcasterEthAddr": {"default"},
	})
	assert.Equal(http.StatusBadRequest, status1)

	status2, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"1"},
		"pixelsPerUnit":      {"a"},
		"broadcasterEthAddr": {"default"},
	})
	assert.Equal(http.StatusBadRequest, status2)

	status3, _ := postForm(handler, url.Values{
		"pricePerUnit":       {"1"},
		"pixelsPerUnit":      {"1"},
		"broadcasterEthAddr": {"--------"},
	})
	assert.Equal(http.StatusBadRequest, status3)
}

// Bond, withdraw, reward
func TestBondHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := bondHandler(client)

	addr := ethcommon.Address{}
	trans := &types.Transcoder{}
	serviceURI := "http://127.0.0.1:8935"
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetTranscoder", addr).Return(trans, nil).Once()
	client.On("CurrentRoundLocked").Return(false, nil).Once()
	client.On("CheckTx", mock.Anything).Return(nil).Once()
	client.On("GetServiceURI", addr).Return(serviceURI, nil).Once()

	status, _ := postForm(handler, url.Values{
		"amount":         {"1"},
		"blockRewardCut": {"0.5"},
		"feeShare":       {"0.5"},
		"pricePerUnit":   {"1"},
		"pixelsPerUnit":  {"1"},
		"serviceURI":     {serviceURI},
	})

	assert.Equal(http.StatusOK, status)
}

func TestRebondHandler_RebondFromUnbonded(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := rebondHandler(client)

	client.On("CheckTx", mock.Anything).Return(nil).Once()

	status, _ := postForm(handler, url.Values{
		"unbondingLockId": {"1"},
		"toAddr":          {"0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"},
	})

	assert.Equal(http.StatusOK, status)
}

func TestRebondHandler_Rebond(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := rebondHandler(client)

	client.On("CheckTx", mock.Anything).Return(nil).Once()

	status, _ := postForm(handler, url.Values{
		"unbondingLockId": {"1"},
	})

	assert.Equal(http.StatusOK, status)
}

func TestUnbondHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := unbondHandler(client)

	client.On("CheckTx", mock.Anything).Return(nil).Once()

	status, _ := postForm(handler, url.Values{
		"amount": {"1"},
	})

	assert.Equal(http.StatusOK, status)
}

func TestWithdrawStakeHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := withdrawStakeHandler(client)

	client.On("CheckTx", mock.Anything).Return(nil).Once()

	status, _ := postForm(handler, url.Values{
		"unbondingLockId": {"1"},
	})

	assert.Equal(http.StatusOK, status)
}

func TestL1WithdrawFeesHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	db := &mockChainIdGetter{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(MainnetChainId), nil)
	client.On("L1WithdrawFees").Return(nil, errors.New("WithdrawFees error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute WithdrawFees: WithdrawFees error", body)
}

func TestL1WithdrawFeesHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	db := &mockChainIdGetter{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(MainnetChainId), nil)
	client.On("L1WithdrawFees").Return(nil, nil)
	client.On("CheckTx").Return(errors.New("CheckTx error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute WithdrawFees: CheckTx error", body)
}

func TestL1WithdrawFeesHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	db := &mockChainIdGetter{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(MainnetChainId), nil)
	client.On("L1WithdrawFees").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, _ := post(handler)

	assert.Equal(http.StatusOK, status)
}

func TestWithdrawFeesHandler_InvalidAmount(t *testing.T) {
	assert := assert.New(t)

	db := &mockChainIdGetter{}
	client := &eth.MockClient{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(123), nil)

	status, body := postForm(handler, url.Values{
		"amount": {"foo"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "invalid amount")
}

func TestWithdrawFeesHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	db := &mockChainIdGetter{}
	client := &eth.MockClient{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(123), nil)
	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("WithdrawFees", addr, big.NewInt(50)).Return(nil, errors.New("WithdrawFees error"))

	status, body := postForm(handler, url.Values{
		"amount": {"50"},
	})

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute WithdrawFees: WithdrawFees error", body)
}

func TestWithdrawFeesHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	db := &mockChainIdGetter{}
	client := &eth.MockClient{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(123), nil)
	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("WithdrawFees", addr, big.NewInt(50)).Return(nil, nil)
	client.On("CheckTx").Return(errors.New("CheckTx error"))

	status, body := postForm(handler, url.Values{
		"amount": {"50"},
	})

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute WithdrawFees: CheckTx error", body)
}

func TestWithdrawFeesHandler_Success(t *testing.T) {
	assert := assert.New(t)

	db := &mockChainIdGetter{}
	client := &eth.MockClient{}
	handler := withdrawFeesHandler(client, db)

	db.On("ChainID").Return(big.NewInt(123), nil)
	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("WithdrawFees", addr, big.NewInt(50)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, _ := postForm(handler, url.Values{
		"amount": {"50"},
	})

	assert.Equal(http.StatusOK, status)
}

func TestClaimEarningsHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := claimEarningsHandler(client)

	client.On("CurrentRoundInitialized").Return(true, nil).Once()
	client.On("CurrentRound").Return(big.NewInt(7), nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, _ := post(handler)

	assert.Equal(http.StatusOK, status)
}

func TestDelegatorInfoHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := delegatorInfoHandler(client)
	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	delegator := &types.Delegator{
		StartRound: big.NewInt(4),
	}
	client.On("GetDelegator", addr).Return(delegator, nil)

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.Contains(body, `"StartRound":4`)
}

func TestRewardHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := rewardHandler(client)
	client.On("Reward").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, _ := post(handler)

	assert.Equal(http.StatusOK, status)
}

// Eth
func TestTransferTokensHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := transferTokensHandler(client)

	client.On("CheckTx", mock.Anything).Return(nil)

	status, _ := postForm(handler, url.Values{
		"to":     {"0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"},
		"amount": {"10"},
	})

	assert.Equal(http.StatusOK, status)
}

func TestSignMessageHandler(t *testing.T) {
	assert := assert.New(t)

	// Test missing client
	handler := signMessageHandler(nil)
	status, body := postForm(handler, nil)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("missing ETH client", body)

	// Test signing error
	err := errors.New("signing error")
	client := &eth.StubClient{Err: err}
	handler = signMessageHandler(client)
	status, body = post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal(fmt.Sprintf("could not sign message - err=%q", err), body)

	// Test signing success
	client.Err = nil
	msg := "foo"
	form := url.Values{
		"message": {msg},
	}
	handler = signMessageHandler(client)
	status, body = postForm(handler, form)
	assert.Equal(http.StatusOK, status)
	assert.Equal(msg, body)

	// Test signing typed data JSON unmarshal error
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"SigFormat":    "data/typed",
	}

	status, body = postFormWithHeaders(handler, form, headers)
	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "could not unmarshal typed data")

	// Test signing typed data success
	jsonTypedData := `
    {
      "types": {
        "EIP712Domain": [
          {
            "name": "name",
            "type": "string"
          },
          {
            "name": "version",
            "type": "string"
          },
          {
            "name": "chainId",
            "type": "uint256"
          },
          {
            "name": "verifyingContract",
            "type": "address"
          }
        ],
        "Person": [
          {
            "name": "name",
            "type": "string"
          },
          {
            "name": "wallet",
            "type": "address"
          }
        ],
        "Mail": [
          {
            "name": "from",
            "type": "Person"
          },
          {
            "name": "to",
            "type": "Person"
          },
          {
            "name": "contents",
            "type": "string"
          }
        ]
      },
      "primaryType": "Mail",
      "domain": {
        "name": "Ether Mail",
        "version": "1",
        "chainId": "1",
        "verifyingContract": "0xCCCcccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC"
      },
      "message": {
        "from": {
          "name": "Cow",
          "wallet": "0xcD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826"
        },
        "to": {
          "name": "Bob",
          "wallet": "0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbB"
        },
        "contents": "Hello, Bob!"
      }
    }
	`

	form = url.Values{
		"message": {jsonTypedData},
	}

	status, body = postFormWithHeaders(handler, form, headers)
	assert.Equal(http.StatusOK, status)
	assert.Equal("foo", body)
}

func TestVoteHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.StubClient{}

	// Test missing client
	handler := voteHandler(nil)
	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("missing ETH client", body)

	// Test invalid poll address
	form := url.Values{
		"poll":     {"foo"},
		"choiceID": {"-1"},
	}
	handler = voteHandler(client)
	status, body = postForm(handler, form)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("invalid poll contract address", body)

	// Test choiceID invalid integer
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"foo"},
	}
	handler = voteHandler(client)
	status, body = postForm(handler, form)
	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("choiceID is not a valid integer value", body)

	// Test invalid choiceID
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"-1"},
	}
	handler = voteHandler(client)
	status, body = postForm(handler, form)
	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("invalid choiceID", body)

	// Test Vote() error
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"1"},
	}
	err := errors.New("voting error")
	client.Err = err
	handler = voteHandler(client)
	status, body = postForm(handler, form)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal(fmt.Sprintf("unable to submit vote transaction err=%q", err), body)
	client.Err = nil

	// Test CheckTx() error
	err = errors.New("unable to mine tx")
	client.CheckTxErr = err
	handler = voteHandler(client)
	status, body = postForm(handler, form)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal(fmt.Sprintf("unable to mine vote transaction err=%q", err), body)
	client.CheckTxErr = nil

	// Test Vote() success
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"0"},
	}
	handler = voteHandler(client)
	status, body = postForm(handler, form)
	assert.Equal(http.StatusOK, status)
}

// Tickets
func TestFundDepositAndReserveHandler_InvalidDepositAmount(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	status, body := postForm(handler, url.Values{
		"depositAmount": {"foo"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "invalid depositAmount")
}

func TestFundDepositAndReserveHandler_InvalidReserveAmount(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	status, body := postForm(handler, url.Values{
		"depositAmount": {"100"},
		"reserveAmount": {"foo"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "invalid reserveAmount")
}

func TestFundDepositAndReserveHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	client.On("FundDepositAndReserve", big.NewInt(50), big.NewInt(50)).Return(nil, errors.New("FundDepositAndReserve error"))

	status, body := postForm(handler, url.Values{
		"depositAmount": {"50"},
		"reserveAmount": {"50"},
	})

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute fundDepositAndReserve: FundDepositAndReserve error", body)
}

func TestFundDepositAndReserveHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	client.On("FundDepositAndReserve", big.NewInt(50), big.NewInt(50)).Return(nil, nil)
	client.On("CheckTx").Return(errors.New("CheckTx error"))

	status, body := postForm(handler, url.Values{
		"depositAmount": {"50"},
		"reserveAmount": {"50"},
	})

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute fundDepositAndReserve: CheckTx error", body)
}

func TestFundDepositAndReserveHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	client.On("FundDepositAndReserve", big.NewInt(50), big.NewInt(50)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, body := postForm(handler, url.Values{
		"depositAmount": {"50"},
		"reserveAmount": {"50"},
	})

	assert.Equal(http.StatusOK, status)
	assert.Equal("fundDepositAndReserve success", body)
}

func TestFundDepositHandler_InvalidAmount(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	status, body := postForm(handler, url.Values{
		"amount": {"foo"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Contains(body, "invalid amount")
}

func TestFundDepositHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	client.On("FundDeposit", big.NewInt(100)).Return(nil, errors.New("FundDeposit error"))

	status, body := postForm(handler, url.Values{
		"amount": {"100"},
	})

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute fundDeposit: FundDeposit error", body)
}

func TestFundDepositHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	client.On("FundDeposit", big.NewInt(100)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	status, body := postForm(handler, url.Values{
		"amount": {"100"},
	})

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute fundDeposit: CheckTx error", body)
}

func TestFundDepositHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	client.On("FundDeposit", big.NewInt(100)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, body := postForm(handler, url.Values{
		"amount": {"100"},
	})

	assert.Equal(http.StatusOK, status)
	assert.Equal("fundDeposit success", body)
}

func TestUnlockHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, errors.New("Unlock error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute unlock: Unlock error", body)
}

func TestUnlockHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute unlock: CheckTx error", body)
}

func TestUnlockHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, body := post(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("unlock success", body)
}

func TestCancelUnlockHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, errors.New("CancelUnlock error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute cancelUnlock: CancelUnlock error", body)
}

func TestCancelUnlockHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute cancelUnlock: CheckTx error", body)
}

func TestCancelUnlockHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, body := post(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("cancelUnlock success", body)
}

func TestWithdrawHandler_TransactionSubmissionError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, errors.New("Withdraw error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute withdraw: Withdraw error", body)
}
func TestWithdrawHandler_TransactionWaitError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	status, body := post(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not execute withdraw: CheckTx error", body)
}

func TestWithdrawHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	status, body := post(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("withdraw success", body)
}

func TestSenderInfoHandler_GetSenderInfoErrNoResult(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := senderInfoHandler(client)
	addr := ethcommon.Address{}

	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetSenderInfo", addr).Return(nil, errors.New("ErrNoResult"))

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.JSONEq(`{"Deposit":0,"WithdrawRound":0,"Reserve":{"FundsRemaining":0,"ClaimedInCurrentRound":0}}`, body)
}

func TestSenderInfoHandler_GetSenderInfoOtherError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := senderInfoHandler(client)

	addr := ethcommon.Address{}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetSenderInfo", addr).Return(nil, errors.New("foo"))

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not query sender info: foo", body)
}

func TestSenderInfoHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := senderInfoHandler(client)
	addr := ethcommon.Address{}

	mockInfo := &pm.SenderInfo{
		Deposit:       big.NewInt(0),
		WithdrawRound: big.NewInt(102),
		Reserve: &pm.ReserveInfo{
			FundsRemaining:        big.NewInt(104),
			ClaimedInCurrentRound: big.NewInt(0),
		}}
	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetSenderInfo", addr).Return(mockInfo, nil)

	status, body := get(handler)

	var info pm.SenderInfo
	err := json.Unmarshal([]byte(body), &info)
	assert.NoError(err)

	assert.Equal(http.StatusOK, status)
	assert.Equal(mockInfo.Deposit, info.Deposit)
	assert.Equal(mockInfo.WithdrawRound, info.WithdrawRound)
	assert.Equal(mockInfo.Reserve, info.Reserve)
}

func TestTicketBrokerParamsHandler_UnlockPeriodError(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := ticketBrokerParamsHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(50), nil)
	client.On("UnlockPeriod").Return(nil, errors.New("UnlockPeriod error"))

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("could not query TicketBroker unlockPeriod: UnlockPeriod error", body)
}

func TestTicketBrokerParamsHandler_Success(t *testing.T) {
	assert := assert.New(t)

	client := &eth.MockClient{}
	handler := ticketBrokerParamsHandler(client)
	minPenaltyEscrow := big.NewInt(50)
	unlockPeriod := big.NewInt(51)

	client.On("MinPenaltyEscrow").Return(minPenaltyEscrow, nil)
	client.On("UnlockPeriod").Return(unlockPeriod, nil)

	status, body := get(handler)

	var params struct {
		MinPenaltyEscrow *big.Int
		UnlockPeriod     *big.Int
	}
	err := json.Unmarshal([]byte(body), &params)
	assert.NoError(err)
	assert.Equal(http.StatusOK, status)
	assert.Equal(unlockPeriod, params.UnlockPeriod)
}

// Helpers
func TestMustHaveFormParams_NoParamsRequired(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveFormParams(dummyHandler())

	status, body := post(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("success", body)
}

func TestMustHaveFormParams_SingleParamRequiredNotProvided(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveFormParams(dummyHandler(), "a")

	status, body := post(handler)

	assert.Equal(http.StatusBadRequest, status)
	assert.Equal("missing form param: a", body)
}

func TestMustHaveFormParams_SingleParamRequiredAndProvided(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveFormParams(dummyHandler(), "a")

	status, body := postForm(handler, url.Values{
		"a": {"foo"},
	})

	assert.Equal(http.StatusOK, status)
	assert.Equal("success", body)
}

func TestMustHaveFormParams_MultipleParamsRequiredOneNotProvided(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveFormParams(dummyHandler(), "a", "b")

	status, body := postForm(handler, url.Values{
		"a": {"foo"},
	})

	assert.Equal(http.StatusBadRequest, status)
	assert.Equal("missing form param: b", body)
}

func TestMustHaveFormParams_MultipleParamsRequiredAllProvided(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveFormParams(dummyHandler(), "a", "b")

	status, body := postForm(handler, url.Values{
		"a": {"foo"},
		"b": {"foo"},
	})

	assert.Equal(http.StatusOK, status)
	assert.Equal("success", body)
}

func TestMustHaveClient_MissingClient(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveClient(nil, dummyHandler())

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("missing ETH client", body)
}

func TestMustHaveClient_Success(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveClient(&eth.MockClient{}, dummyHandler())

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("success", body)
}

func TestMustHaveDb_MissingDb(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveDb(nil, dummyHandler())

	status, body := get(handler)

	assert.Equal(http.StatusInternalServerError, status)
	assert.Equal("missing database", body)
}

func TestMustHaveDb_Success(t *testing.T) {
	assert := assert.New(t)

	handler := mustHaveDb(struct{}{}, dummyHandler())

	status, body := get(handler)

	assert.Equal(http.StatusOK, status)
	assert.Equal("success", body)
}
func TestSetServiceURI(t *testing.T) {
	s := stubServer()
	client := &eth.MockClient{}
	serviceURI := "https://8.8.8.8:8935"

	t.Run("Valid Service URI", func(t *testing.T) {
		client.On("SetServiceURI", serviceURI).Return(&ethtypes.Transaction{}, nil)
		client.On("CheckTx", mock.Anything).Return(nil)

		err := s.setServiceURI(client, serviceURI)

		assert.NoError(t, err)
	})

	t.Run("Invalid Service URI", func(t *testing.T) {
		invalidServiceURI := "https://0.0.0.0:8935"

		err := s.setServiceURI(client, invalidServiceURI)

		assert.Error(t, err)
	})

}
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
}

func stubServer() *LivepeerServer {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	return &LivepeerServer{
		LivepeerNode: n,
	}
}

func stubOrchestratorWithRecipient(t *testing.T) *LivepeerServer {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	n.NodeType = core.OrchestratorNode
	n.Recipient = &pm.MockRecipient{}
	return &LivepeerServer{
		LivepeerNode: n,
	}
}

func post(handler http.Handler) (int, string) {
	return postForm(handler, url.Values{})
}

func postForm(handler http.Handler, form url.Values) (int, string) {
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}
	return postFormWithHeaders(handler, form, headers)
}

func postFormWithHeaders(handler http.Handler, form url.Values, headers map[string]string) (int, string) {
	resp := httpPostResp(handler, strings.NewReader(form.Encode()), headers)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, trim(body)
}

func httpPostResp(handler http.Handler, body io.Reader, headers map[string]string) *http.Response {
	return httpResp(handler, "POST", body, headers)
}

func get(handler http.Handler) (int, string) {
	resp := httpGetResp(handler)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, trim(body)
}

func httpGetResp(handler http.Handler) *http.Response {
	return httpResp(handler, "GET", nil, nil)
}

func httpResp(handler http.Handler, method string, body io.Reader, headers map[string]string) *http.Response {
	req := httptest.NewRequest(method, "http://example.com", body)

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w.Result()
}

func mockBigInt(args mock.Arguments) (*big.Int, error) {
	var blk *big.Int
	if args.Get(0) != nil {
		blk = args.Get(0).(*big.Int)
	}

	return blk, args.Error(1)
}

func trim(str []byte) string {
	return strings.TrimSpace(string(str))
}
