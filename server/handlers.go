package server

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
)

const MainnetChainId = 1
const RinkebyChainId = 4

func respondOk(w http.ResponseWriter, msg []byte) {
	w.WriteHeader(http.StatusOK)
	if msg != nil {
		w.Write(msg)
	}
}

func respondWith500(w http.ResponseWriter, errMsg string) {
	respondWithError(w, errMsg, http.StatusInternalServerError)
}

func respondWith400(w http.ResponseWriter, errMsg string) {
	respondWithError(w, errMsg, http.StatusBadRequest)
}

func respondWithError(w http.ResponseWriter, errMsg string, code int) {
	glog.Errorf("HTTP Response Error %v: %v", code, errMsg)
	http.Error(w, errMsg, code)
}

func mustHaveFormParams(h http.Handler, params ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			respondWith500(w, fmt.Sprintf("parse form error: %v", err))
			return
		}

		for _, param := range params {
			if r.FormValue(param) == "" {
				respondWith400(w, fmt.Sprintf("missing form param: %s", param))
				return
			}
		}

		h.ServeHTTP(w, r)
	})
}

func mustHaveClient(client eth.LivepeerEthClient, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}
		h.ServeHTTP(w, r)
	})
}

// BlockGetter is an interface which describes an object capable
// of getting blocks
type BlockGetter interface {
	// LastSeenBlock returns the last seen block number
	LastSeenBlock() (*big.Int, error)
}

func currentBlockHandler(getter BlockGetter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if getter == nil {
			respondWith500(w, "missing block getter")
			return
		}

		blk, err := getter.LastSeenBlock()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query last seen block: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(blk.Bytes())
	})
}

func currentRoundHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentRound, err := client.CurrentRound()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query current round: %v", err))
			return
		}

		respondOk(w, currentRound.Bytes())
	}),
	)
}

func fundDepositAndReserveHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		depositAmount, err := common.ParseBigInt(r.FormValue("depositAmount"))
		if err != nil {
			respondWith400(w, fmt.Sprintf("invalid depositAmount: %v", err))
			return
		}

		reserveAmount, err := common.ParseBigInt(r.FormValue("reserveAmount"))
		if err != nil {
			respondWith400(w, fmt.Sprintf("invalid reserveAmount: %v", err))
			return
		}

		tx, err := client.FundDepositAndReserve(depositAmount, reserveAmount)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute fundDepositAndReserve: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute fundDepositAndReserve: %v", err))
			return
		}

		respondOk(w, []byte("fundDepositAndReserve success"))
	}),
	)
}

func fundDepositHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amount, err := common.ParseBigInt(r.FormValue("amount"))
		if err != nil {
			respondWith400(w, fmt.Sprintf("invalid amount: %v", err))
			return
		}

		tx, err := client.FundDeposit(amount)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute fundDeposit: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute fundDeposit: %v", err))
			return
		}

		respondOk(w, []byte("fundDeposit success"))
	}),
	)
}

func unlockHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.Unlock()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute unlock: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute unlock: %v", err))
			return
		}

		respondOk(w, []byte("unlock success"))
	}),
	)
}

func cancelUnlockHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.CancelUnlock()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute cancelUnlock: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute cancelUnlock: %v", err))
			return
		}

		respondOk(w, []byte("cancelUnlock success"))
	}),
	)
}

func withdrawHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.Withdraw()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute withdraw: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute withdraw: %v", err))
			return
		}

		respondOk(w, []byte("withdraw success"))
	}),
	)
}

func senderInfoHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := client.GetSenderInfo(client.Account().Address)
		if err != nil {
			if err.Error() == "ErrNoResult" {
				info = &pm.SenderInfo{
					Deposit:       big.NewInt(0),
					WithdrawRound: big.NewInt(0),
					Reserve: &pm.ReserveInfo{
						FundsRemaining:        big.NewInt(0),
						ClaimedInCurrentRound: big.NewInt(0),
					},
				}
			} else {
				respondWith500(w, fmt.Sprintf("could not query sender info: %v", err))
				return
			}
		}

		data, err := json.Marshal(info)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not parse sender info: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}),
	)
}

func ticketBrokerParamsHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unlockPeriod, err := client.UnlockPeriod()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query TicketBroker unlockPeriod: %v", err))
			return
		}

		params := struct {
			UnlockPeriod *big.Int
		}{
			unlockPeriod,
		}

		data, err := json.Marshal(params)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query TicketBroker params: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}),
	)
}

func signMessageHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use EIP-191 (https://github.com/ethereum/EIPs/blob/master/EIPS/eip-191.md) signature versioning
		// The SigFormat terminology is taken from:
		// https://github.com/ethereum/go-ethereum/blob/dddf73abbddb297e61cee6a7e6aebfee87125e49/signer/core/apitypes/types.go#L171
		sigFormat := r.Header.Get("SigFormat")
		if sigFormat == "" {
			sigFormat = "text/plain"
		}

		message := r.FormValue("message")

		var signed []byte
		var err error

		// Only support text/plain and data/typed
		// By default use text/plain
		switch sigFormat {
		case "data/typed":
			var d apitypes.TypedData
			err = json.Unmarshal([]byte(message), &d)
			if err != nil {
				respondWith400(w, fmt.Sprintf("could not unmarshal typed data - err=%q", err))
				return
			}

			signed, err = client.SignTypedData(d)
			if err != nil {
				respondWith500(w, fmt.Sprintf("could not sign typed data - err=%q", err))
				return
			}
		default:
			// text/plain
			signed, err = client.Sign([]byte(message))
			if err != nil {
				respondWith500(w, fmt.Sprintf("could not sign message - err=%q", err))
				return
			}
		}

		respondOk(w, signed)
	}),
	)
}

func voteHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		poll := r.FormValue("poll")
		if poll == "" {
			respondWith500(w, "missing poll contract address")
			return
		}
		if !ethcommon.IsHexAddress(poll) {
			respondWith500(w, "invalid poll contract address")
			return
		}

		choiceStr := r.FormValue("choiceID")
		if choiceStr == "" {
			respondWith500(w, "missing choiceID")
			return
		}

		choiceID, ok := new(big.Int).SetString(choiceStr, 10)
		if !ok {
			respondWith500(w, "choiceID is not a valid integer value")
			return
		}
		if !types.VoteChoice(int(choiceID.Int64())).IsValid() {
			respondWith500(w, "invalid choiceID")
			return
		}

		// submit tx
		tx, err := client.Vote(
			ethcommon.HexToAddress(poll),
			choiceID,
		)
		if err != nil {
			respondWith500(w, fmt.Sprintf("unable to submit vote transaction err=%q", err))
			return
		}

		if err := client.CheckTx(tx); err != nil {
			respondWith500(w, fmt.Sprintf("unable to mine vote transaction err=%q", err))
			return
		}

		respondOk(w, tx.Hash().Bytes())
	}),
	)
}

func withdrawFeesHandler(client eth.LivepeerEthClient, getChainId func() (int64, error)) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// for L1 contracts backwards-compatibility
		var tx *ethtypes.Transaction
		isL1Network, err := isL1Network(getChainId)
		if err != nil {
			respondWith500(w, err.Error())
			return
		}
		if isL1Network {
			// L1 contracts
			tx, err = client.L1WithdrawFees()
			if err != nil {
				respondWith500(w, fmt.Sprintf("could not execute WithdrawFees: %v", err))
				return
			}
		} else {
			// L2 contracts
			amountStr := r.FormValue("amount")
			if amountStr == "" {
				respondWith400(w, "missing form param: amount")
				return
			}
			amount, err := common.ParseBigInt(amountStr)
			if err != nil {
				respondWith400(w, fmt.Sprintf("invalid amount: %v", err))
				return
			}

			tx, err = client.WithdrawFees(client.Account().Address, amount)
			if err != nil {
				respondWith500(w, fmt.Sprintf("could not execute WithdrawFees: %v", err))
				return
			}
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute WithdrawFees: %v", err))
			return
		}

		respondOk(w, nil)
	}),
	)
}

func minGasPriceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(client.Backend().GasPriceMonitor().MinGasPrice().String()))
	}),
	)
}

func setMinGasPriceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		minGasPrice, err := common.ParseBigInt(r.FormValue("minGasPrice"))
		if err != nil {
			respondWith400(w, fmt.Sprintf("invalid minGasPrice: %v", err))
			return
		}
		client.Backend().GasPriceMonitor().SetMinGasPrice(minGasPrice)

		respondOk(w, []byte("setMinGasPrice success"))
	}),
	)
}

func isL1Network(getChainId func() (int64, error)) (bool, error) {
	chainId, err := getChainId()
	if err != nil {
		return false, err
	}
	isL1Network := chainId == MainnetChainId || chainId == RinkebyChainId
	return isL1Network, err
}
