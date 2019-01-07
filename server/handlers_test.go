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

type mockBlockGetter struct {
	mock.Mock
}

func (m *mockBlockGetter) LastSeenBlock() (*big.Int, error) {
	args := m.Called()

	var blk *big.Int
	if args.Get(0) != nil {
		blk = args.Get(0).(*big.Int)
	}

	return blk, args.Error(1)
}

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

func TestCurrentBlockHandler_MissingBlockGetter(t *testing.T) {
	handler := currentBlockHandler(nil)

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing block getter", strings.TrimSpace(string(body)))
}

func TestCurrentBlockHandler_LastSeenBlockError(t *testing.T) {
	getter := &mockBlockGetter{}
	handler := currentBlockHandler(getter)

	getter.On("LastSeenBlock").Return(nil, errors.New("LastSeenBlock error"))

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query last seen block: LastSeenBlock error", strings.TrimSpace(string(body)))
}

func TestCurrentBlockHandler_Success(t *testing.T) {
	getter := &mockBlockGetter{}
	handler := currentBlockHandler(getter)

	getter.On("LastSeenBlock").Return(big.NewInt(50), nil)

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(big.NewInt(50), new(big.Int).SetBytes(body))
}

func TestFundAndApproveSignersHandler_MissingClient(t *testing.T) {
	handler := fundAndApproveSignersHandler(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_InvalidDepositAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	form := url.Values{
		"depositAmount": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "invalid depositAmount")
}

func TestFundAndApproveSignersHandler_InvalidPenaltyEscrowAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	form := url.Values{
		"depositAmount":       {"100"},
		"penaltyEscrowAmount": {"foo"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "invalid penaltyEscrowAmount")
}

func TestFundAndApproveSignersHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("FundAndApproveSigners", big.NewInt(50), big.NewInt(50), []ethcommon.Address{}).Return(nil, errors.New("FundAndApproveSigners error"))

	form := url.Values{
		"depositAmount":       {"50"},
		"penaltyEscrowAmount": {"50"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundAndApproveSigners: FundAndApproveSigners error", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("FundAndApproveSigners", big.NewInt(50), big.NewInt(50), []ethcommon.Address{}).Return(nil, nil)
	client.On("CheckTx").Return(errors.New("CheckTx error"))

	form := url.Values{
		"depositAmount":       {"50"},
		"penaltyEscrowAmount": {"50"},
	}
	resp := httpResp(handler, "POST", strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundAndApproveSigners: CheckTx error", strings.TrimSpace(string(body)))
}

func TestFundAndApproveSignersHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundAndApproveSignersHandler(client)

	client.On("FundAndApproveSigners", big.NewInt(50), big.NewInt(50), []ethcommon.Address{}).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	form := url.Values{
		"depositAmount":       {"50"},
		"penaltyEscrowAmount": {"50"},
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
	assert.Contains(strings.TrimSpace(string(body)), "invalid amount")
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
	assert.Equal("could not execute fundDeposit: FundDeposit error", strings.TrimSpace(string(body)))
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
	assert.Equal("could not execute fundDeposit: CheckTx error", strings.TrimSpace(string(body)))
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

func TestUnlockHandler_MissingClient(t *testing.T) {
	handler := unlockHandler(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestUnlockHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, errors.New("Unlock error"))

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute unlock: Unlock error", strings.TrimSpace(string(body)))
}

func TestUnlockHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute unlock: CheckTx error", strings.TrimSpace(string(body)))
}

func TestUnlockHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("unlock success", strings.TrimSpace(string(body)))
}

func TestCancelUnlockHandler_MissingClient(t *testing.T) {
	handler := cancelUnlockHandler(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestCancelUnlockHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, errors.New("CancelUnlock error"))

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute cancelUnlock: CancelUnlock error", strings.TrimSpace(string(body)))
}

func TestCancelUnlockHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute cancelUnlock: CheckTx error", strings.TrimSpace(string(body)))
}

func TestCancelUnlockHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("cancelUnlock success", strings.TrimSpace(string(body)))
}

func TestWithdrawHandler_MissingClient(t *testing.T) {
	handler := withdrawHandler(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestWithdrawHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, errors.New("Withdraw error"))

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute withdraw: Withdraw error", strings.TrimSpace(string(body)))
}
func TestWithdrawHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(errors.New("CheckTx error"))

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute withdraw: CheckTx error", strings.TrimSpace(string(body)))
}

func TestWithdrawHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	resp := httpResp(handler, "POST", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("withdraw success", strings.TrimSpace(string(body)))
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
	client.On("Senders", addr).Return(nil, nil, nil, errors.New("Senders error"))

	resp := httpResp(handler, "GET", nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query sender info: Senders error", strings.TrimSpace(string(body)))
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
	assert.Equal("could not query TicketBroker minPenaltyEscrow: MinPenaltyEscrow error", strings.TrimSpace(string(body)))
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
	assert.Equal("could not query TicketBroker unlockPeriod: UnlockPeriod error", strings.TrimSpace(string(body)))
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
