package server

import (
	"encoding/json"
	"errors"
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
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
}

func TestMustHaveFormParams_NoParamsRequired(t *testing.T) {
	handler := mustHaveFormParams(dummyHandler())

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("success", strings.TrimSpace(string(body)))
}

func TestMustHaveFormParams_SingleParamRequiredNotProvided(t *testing.T) {
	handler := mustHaveFormParams(dummyHandler(), "a")

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("missing form param: a", strings.TrimSpace(string(body)))
}

func TestMustHaveFormParams_SingleParamRequiredAndProvided(t *testing.T) {
	handler := mustHaveFormParams(dummyHandler(), "a")

	form := url.Values{
		"a": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("success", strings.TrimSpace(string(body)))
}

func TestMustHaveFormParams_MultipleParamsRequiredOneNotProvided(t *testing.T) {
	handler := mustHaveFormParams(dummyHandler(), "a", "b")

	form := url.Values{
		"a": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("missing form param: b", strings.TrimSpace(string(body)))
}
func TestMustHaveFormParams_MultipleParamsRequiredAllProvided(t *testing.T) {
	handler := mustHaveFormParams(dummyHandler(), "a", "b")

	form := url.Values{
		"a": {"foo"},
		"b": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("success", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_MissingClient(t *testing.T) {
	handler := fundAndApproveSignersHandler(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_InvalidAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	form := url.Values{
		"amount": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("invalid amount", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_MinPenaltyEscrowError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("MinPenaltyEscrow").Return(nil, errors.New("MinPenaltyEscrow error"))

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundAndApproveSigners", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_InsufficientAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(101), nil)

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("amount is not sufficient for minimum penalty escrow", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(50), nil)
	client.On("FundAndApproveSigners", big.NewInt(50), big.NewInt(50), []ethcommon.Address{}).Return(nil, errors.New("FundAndApproveSigners error"))

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundAndApproveSigners", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(50), nil)
	client.On("FundAndApproveSigners", big.NewInt(50), big.NewInt(50), []ethcommon.Address{}).Return(nil, nil)
	client.On("CheckTx").Return(errors.New("CheckTx error"))

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundAndApproveSigners", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(50), nil)
	client.On("FundAndApproveSigners", big.NewInt(50), big.NewInt(50), []ethcommon.Address{}).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("fundAndApproveSigners success", strings.TrimSpace(string(body)))
}

func TestFundDepositHandler_MissingClient(t *testing.T) {
	handler := fundDepositHandler(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestFundDepositHandler_InvalidAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	form := url.Values{
		"amount": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Equal("invalid amount", strings.TrimSpace(string(body)))
}

func TestFundDepositHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	client.On("FundDeposit", big.NewInt(100)).Return(nil, errors.New("FundDeposit error"))

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundDeposit", strings.TrimSpace(string(body)))
}

func TestFundDepositHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	client.On("FundDeposit", big.NewInt(100)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundDeposit", strings.TrimSpace(string(body)))
}

func TestFundDepositHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositHandler(client)

	client.On("FundDeposit", big.NewInt(100)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	form := url.Values{
		"amount": {"100"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("fundDeposit success", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_MissingClient(t *testing.T) {
	handler := senderInfoHandler(nil)

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_SendersError(t *testing.T) {
	client := &eth.MockClient{}
	handler := senderInfoHandler(client)
	addr := ethcommon.Address{}

	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("Senders", addr).Return(nil, nil, nil, errors.New("Senders Error"))

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query sender info", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := senderInfoHandler(client)
	addr := ethcommon.Address{}

	deposit := big.NewInt(100)
	penaltyEscrow := big.NewInt(101)
	withdrawBlock := big.NewInt(102)

	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("Senders", addr).Return(deposit, penaltyEscrow, withdrawBlock, nil)

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	var sender struct {
		Deposit       *big.Int
		PenaltyEscrow *big.Int
		WithdrawBlock *big.Int
	}
	err := json.Unmarshal(body, &sender)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(deposit, sender.Deposit)
	assert.Equal(penaltyEscrow, sender.PenaltyEscrow)
	assert.Equal(withdrawBlock, sender.WithdrawBlock)
}

func TestTicketBrokerParamsHandler_MissingClient(t *testing.T) {
	handler := ticketBrokerParamsHandler(nil)

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestTicketBrokerParamsHandler_MinPenaltyEscrowError(t *testing.T) {
	client := &eth.MockClient{}
	handler := ticketBrokerParamsHandler(client)

	client.On("MinPenaltyEscrow").Return(nil, errors.New("MinPenaltyEscrow error"))

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query TicketBroker minPenaltyEscrow", strings.TrimSpace(string(body)))
}

func TestTicketBrokerParamsHandler_UnlockPeriodError(t *testing.T) {
	client := &eth.MockClient{}
	handler := ticketBrokerParamsHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(50), nil)
	client.On("UnlockPeriod").Return(nil, errors.New("UnlockPeriod error"))

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query TicketBroker unlockPeriod", strings.TrimSpace(string(body)))
}

func TestTicketBrokerParamsHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := ticketBrokerParamsHandler(client)
	minPenaltyEscrow := big.NewInt(50)
	unlockPeriod := big.NewInt(51)

	client.On("MinPenaltyEscrow").Return(minPenaltyEscrow, nil)
	client.On("UnlockPeriod").Return(unlockPeriod, nil)

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	var params struct {
		MinPenaltyEscrow *big.Int
		UnlockPeriod     *big.Int
	}
	err := json.Unmarshal(body, &params)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(minPenaltyEscrow, params.MinPenaltyEscrow)
	assert.Equal(unlockPeriod, params.UnlockPeriod)
}

func httpResp(handler http.Handler, method string, body io.Reader) *http.Response {
	req := httptest.NewRequest(method, "http://example.com", body)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w.Result()
}
