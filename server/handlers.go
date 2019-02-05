package server

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
)

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

func fundAndApproveSignersHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

		depositAmount, err := common.ParseBigInt(r.FormValue("depositAmount"))
		if err != nil {
			respondWith400(w, fmt.Sprintf("invalid depositAmount: %v", err))
			return
		}

		penaltyEscrowAmount, err := common.ParseBigInt(r.FormValue("penaltyEscrowAmount"))
		if err != nil {
			respondWith400(w, fmt.Sprintf("invalid penaltyEscrowAmount: %v", err))
			return
		}

		tx, err := client.FundAndApproveSigners(depositAmount, penaltyEscrowAmount, []ethcommon.Address{})
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute fundAndApproveSigners: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not execute fundAndApproveSigners: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fundAndApproveSigners success"))
	})
}

func fundDepositHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

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

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fundDeposit success"))
	})
}

func unlockHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

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

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("unlock success"))
	})
}

func cancelUnlockHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

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

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cancelUnlock success"))
	})
}

func withdrawHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

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

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("withdraw success"))
	})
}

func senderInfoHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

		sender, err := client.Senders(client.Account().Address)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query sender info: %v", err))
			return
		}

		data, err := json.Marshal(sender)
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not parse sender info: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
}

func ticketBrokerParamsHandler(client eth.LivepeerEthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respondWith500(w, "missing ETH client")
			return
		}

		minPenaltyEscrow, err := client.MinPenaltyEscrow()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query TicketBroker minPenaltyEscrow: %v", err))
			return
		}

		unlockPeriod, err := client.UnlockPeriod()
		if err != nil {
			respondWith500(w, fmt.Sprintf("could not query TicketBroker unlockPeriod: %v", err))
			return
		}

		params := struct {
			MinPenaltyEscrow *big.Int
			UnlockPeriod     *big.Int
		}{
			minPenaltyEscrow,
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
	})
}
