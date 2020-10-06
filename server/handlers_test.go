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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/pm"
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

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("success", strings.TrimSpace(string(body)))
}

func TestMustHaveFormParams_SingleParamRequiredNotProvided(t *testing.T) {
	handler := mustHaveFormParams(dummyHandler(), "a")

	resp := httpPostFormResp(handler, nil)
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("success", strings.TrimSpace(string(body)))
}

func TestCurrentBlockHandler_MissingBlockGetter(t *testing.T) {
	handler := currentBlockHandler(nil)

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing block getter", strings.TrimSpace(string(body)))
}

func TestCurrentBlockHandler_LastSeenBlockError(t *testing.T) {
	getter := &mockBlockGetter{}
	handler := currentBlockHandler(getter)

	getter.On("LastSeenBlock").Return(nil, errors.New("LastSeenBlock error"))

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query last seen block: LastSeenBlock error", strings.TrimSpace(string(body)))
}

func TestCurrentBlockHandler_Success(t *testing.T) {
	getter := &mockBlockGetter{}
	handler := currentBlockHandler(getter)

	getter.On("LastSeenBlock").Return(big.NewInt(50), nil)

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(big.NewInt(50), new(big.Int).SetBytes(body))
}
func TestCurrentRoundHandler(t *testing.T) {
	assert := assert.New(t)

	// Test missing client
	handler := currentRoundHandler(nil)

	resp := httpGetResp(handler)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))

	// Test CurrentRound() error
	client := &eth.MockClient{}
	handler = currentRoundHandler(client)

	client.On("CurrentRound").Return(nil, errors.New("CurrentRound error")).Once()

	resp = httpGetResp(handler)
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query current round: CurrentRound error", strings.TrimSpace(string(body)))

	// Test success
	currentRound := big.NewInt(7)
	client.On("CurrentRound").Return(currentRound, nil)

	resp = httpGetResp(handler)
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(currentRound, new(big.Int).SetBytes(body))
}

func TestFundDepositAndReserveHandler_MissingClient(t *testing.T) {
	handler := fundDepositAndReserveHandler(nil)

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestFundDepositAndReserveHandler_InvalidDepositAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	form := url.Values{
		"depositAmount": {"foo"},
	}
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "invalid depositAmount")
}

func TestFundDepositAndReserveHandler_InvalidReserveAmount(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	form := url.Values{
		"depositAmount": {"100"},
		"reserveAmount": {"foo"},
	}
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "invalid reserveAmount")
}

func TestFundDepositAndReserveHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	client.On("FundDepositAndReserve", big.NewInt(50), big.NewInt(50)).Return(nil, errors.New("FundDepositAndReserve error"))

	form := url.Values{
		"depositAmount": {"50"},
		"reserveAmount": {"50"},
	}
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundDepositAndReserve: FundDepositAndReserve error", strings.TrimSpace(string(body)))
}

func TestFundDepositAndReserveHandler_TransactionWaitError(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	client.On("FundDepositAndReserve", big.NewInt(50), big.NewInt(50)).Return(nil, nil)
	client.On("CheckTx").Return(errors.New("CheckTx error"))

	form := url.Values{
		"depositAmount": {"50"},
		"reserveAmount": {"50"},
	}
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not execute fundDepositAndReserve: CheckTx error", strings.TrimSpace(string(body)))
}

func TestFundDepositAndReserveHandler_Success(t *testing.T) {
	client := &eth.MockClient{}
	handler := fundDepositAndReserveHandler(client)

	client.On("FundDepositAndReserve", big.NewInt(50), big.NewInt(50)).Return(nil, nil)
	client.On("CheckTx", mock.Anything).Return(nil)

	form := url.Values{
		"depositAmount": {"50"},
		"reserveAmount": {"50"},
	}
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("fundDepositAndReserve success", strings.TrimSpace(string(body)))
}

func TestFundDepositHandler_MissingClient(t *testing.T) {
	handler := fundDepositHandler(nil)

	resp := httpPostFormResp(handler, nil)
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
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
	resp := httpPostFormResp(handler, strings.NewReader(form.Encode()))
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("fundDeposit success", strings.TrimSpace(string(body)))
}

func TestUnlockHandler_MissingClient(t *testing.T) {
	handler := unlockHandler(nil)

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestUnlockHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := unlockHandler(client)

	client.On("Unlock").Return(nil, errors.New("Unlock error"))

	resp := httpPostFormResp(handler, nil)
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

	resp := httpPostFormResp(handler, nil)
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

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("unlock success", strings.TrimSpace(string(body)))
}

func TestCancelUnlockHandler_MissingClient(t *testing.T) {
	handler := cancelUnlockHandler(nil)

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestCancelUnlockHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := cancelUnlockHandler(client)

	client.On("CancelUnlock").Return(nil, errors.New("CancelUnlock error"))

	resp := httpPostFormResp(handler, nil)
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

	resp := httpPostFormResp(handler, nil)
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

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("cancelUnlock success", strings.TrimSpace(string(body)))
}

func TestWithdrawHandler_MissingClient(t *testing.T) {
	handler := withdrawHandler(nil)

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestWithdrawHandler_TransactionSubmissionError(t *testing.T) {
	client := &eth.MockClient{}
	handler := withdrawHandler(client)

	client.On("Withdraw").Return(nil, errors.New("Withdraw error"))

	resp := httpPostFormResp(handler, nil)
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

	resp := httpPostFormResp(handler, nil)
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

	resp := httpPostFormResp(handler, nil)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("withdraw success", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_MissingClient(t *testing.T) {
	handler := senderInfoHandler(nil)

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_GetSenderInfoErrNoResult(t *testing.T) {
	client := &eth.MockClient{}
	handler := senderInfoHandler(client)
	addr := ethcommon.Address{}

	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetSenderInfo", addr).Return(nil, errors.New("ErrNoResult"))

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("{\"Deposit\":0,\"WithdrawRound\":0,\"Reserve\":{\"FundsRemaining\":0,\"ClaimedInCurrentRound\":0}}", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_GetSenderInfoOtherError(t *testing.T) {
	client := &eth.MockClient{}
	handler := senderInfoHandler(client)
	addr := ethcommon.Address{}

	client.On("Account").Return(accounts.Account{Address: addr})
	client.On("GetSenderInfo", addr).Return(nil, errors.New("foo"))

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("could not query sender info: foo", strings.TrimSpace(string(body)))
}

func TestSenderInfoHandler_Success(t *testing.T) {
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

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	var info pm.SenderInfo
	err := json.Unmarshal(body, &info)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(mockInfo.Deposit, info.Deposit)
	assert.Equal(mockInfo.WithdrawRound, info.WithdrawRound)
	assert.Equal(mockInfo.Reserve, info.Reserve)
}

func TestTicketBrokerParamsHandler_MissingClient(t *testing.T) {
	handler := ticketBrokerParamsHandler(nil)

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	assert := assert.New(t)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))
}

func TestTicketBrokerParamsHandler_UnlockPeriodError(t *testing.T) {
	client := &eth.MockClient{}
	handler := ticketBrokerParamsHandler(client)

	client.On("MinPenaltyEscrow").Return(big.NewInt(50), nil)
	client.On("UnlockPeriod").Return(nil, errors.New("UnlockPeriod error"))

	resp := httpGetResp(handler)
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

	resp := httpGetResp(handler)
	body, _ := ioutil.ReadAll(resp.Body)

	var params struct {
		MinPenaltyEscrow *big.Int
		UnlockPeriod     *big.Int
	}
	err := json.Unmarshal(body, &params)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal(unlockPeriod, params.UnlockPeriod)
}

func TestSignMessageHandler(t *testing.T) {
	assert := assert.New(t)

	// Test missing client
	handler := signMessageHandler(nil)
	resp := httpPostFormResp(handler, nil)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))

	// Test signing error
	err := errors.New("signing error")
	client := &eth.StubClient{Err: err}
	handler = signMessageHandler(client)
	resp = httpPostFormResp(handler, nil)
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(fmt.Sprintf("could not sign message - err=%v", err), strings.TrimSpace(string(body)))

	// Test signing success
	client.Err = nil
	msg := "foo"
	form := url.Values{
		"message": {msg},
	}
	handler = signMessageHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal([]byte(msg), body)
}

func TestVoteHandler(t *testing.T) {
	assert := assert.New(t)

	client := &eth.StubClient{}

	// Test missing client
	handler := voteHandler(nil)
	resp := httpPostFormResp(handler, nil)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing ETH client", strings.TrimSpace(string(body)))

	// Test missing form params - poll
	form := url.Values{
		"choiceID": {"0"},
	}
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing poll contract address", strings.TrimSpace(string(body)))

	// Test invalid poll address
	form = url.Values{
		"poll":     {"foo"},
		"choiceID": {"-1"},
	}
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("invalid poll contract address", strings.TrimSpace(string(body)))

	// Test missing form params - choiceID
	form = url.Values{
		"poll": {"0xbf790e51fa21e1515cece96975b3505350b20083"},
	}
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("missing choiceID", strings.TrimSpace(string(body)))

	// Test choiceID invalid integer
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"foo"},
	}
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("choiceID is not a valid integer value", strings.TrimSpace(string(body)))

	// Test invalid choiceID
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"-1"},
	}
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("invalid choiceID", strings.TrimSpace(string(body)))

	// Test Vote() error
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"1"},
	}
	err := errors.New("voting error")
	client.Err = err
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(fmt.Sprintf("unable to submit vote transaction err=%v", err), strings.TrimSpace(string(body)))
	client.Err = nil

	// Test CheckTx() error
	err = errors.New("unable to mine tx")
	client.CheckTxErr = err
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(fmt.Sprintf("unable to mine vote transaction err=%v", err), strings.TrimSpace(string(body)))
	client.CheckTxErr = nil

	// Test Vote() success
	form = url.Values{
		"poll":     {"0xbf790e51fa21e1515cece96975b3505350b20083"},
		"choiceID": {"0"},
	}
	handler = voteHandler(client)
	resp = httpPostFormResp(handler, strings.NewReader(form.Encode()))
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal((&types.Transaction{}).Hash().Bytes(), body)
}

func httpPostFormResp(handler http.Handler, body io.Reader) *http.Response {
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	return httpPostResp(handler, body, headers)
}

func httpPostResp(handler http.Handler, body io.Reader, headers map[string]string) *http.Response {
	return httpResp(handler, "POST", body, headers)
}

func httpGetResp(handler http.Handler) *http.Response {
	return httpResp(handler, "GET", nil, nil)
}

func httpResp(handler http.Handler, method string, body io.Reader, headers map[string]string) *http.Response {
	req := httptest.NewRequest(method, "http://example.com", body)

	if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w.Result()
}
