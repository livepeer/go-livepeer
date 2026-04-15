package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
)

type turnkeyWalletRow struct {
	WalletID  string `json:"walletId"`
	AccountID string `json:"accountId"`
	Address   string `json:"address"`
}

// TurnkeyListWallets returns Ethereum accounts in the Turnkey org (remote signer).
func (ls *LivepeerServer) TurnkeyListWallets(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	if ls.TurnkeyAdmin == nil || ls.LivepeerNode == nil || ls.LivepeerNode.TurnkeyOrgID == "" {
		respondJsonError(ctx, w, errTurnkeyNotConfigured, http.StatusServiceUnavailable)
		return
	}
	accts, err := eth.ListTurnkeyEthereumAccounts(ls.TurnkeyAdmin, ls.LivepeerNode.TurnkeyOrgID)
	if err != nil {
		clog.Errorf(ctx, "turnkey list wallets: %v", err)
		respondJsonError(ctx, w, err, http.StatusBadGateway)
		return
	}
	rows := make([]turnkeyWalletRow, 0, len(accts))
	for _, a := range accts {
		rows = append(rows, turnkeyWalletRow{
			WalletID:  a.WalletID,
			AccountID: a.AccountID,
			Address:   a.Address.Hex(),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

type turnkeyCreateWalletBody struct {
	WalletName string `json:"walletName"`
}

// TurnkeyCreateWalletHTTP creates a new Turnkey HD wallet with a default Ethereum account.
func (ls *LivepeerServer) TurnkeyCreateWalletHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	if ls.TurnkeyAdmin == nil || ls.LivepeerNode == nil || ls.LivepeerNode.TurnkeyOrgID == "" {
		respondJsonError(ctx, w, errTurnkeyNotConfigured, http.StatusServiceUnavailable)
		return
	}
	var body turnkeyCreateWalletBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.WalletName)
	if name == "" {
		respondJsonError(ctx, w, errTurnkeyWalletNameRequired, http.StatusBadRequest)
		return
	}
	wid, addr, err := eth.TurnkeyCreateWallet(ls.TurnkeyAdmin, ls.LivepeerNode.TurnkeyOrgID, name)
	if err != nil {
		clog.Errorf(ctx, "turnkey create wallet: %v", err)
		respondJsonError(ctx, w, err, http.StatusBadGateway)
		return
	}
	ls.refreshTurnkeyAddressBook(ctx)
	glog.Info("Turnkey wallet created ", "walletId", wid, "address", addr.Hex())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"walletId": wid,
		"address":  addr.Hex(),
	})
}

type turnkeyCreateAccountBody struct {
	WalletID string `json:"walletId"`
}

// TurnkeyCreateAccountHTTP derives another Ethereum address on an existing wallet.
func (ls *LivepeerServer) TurnkeyCreateAccountHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	if ls.TurnkeyAdmin == nil || ls.LivepeerNode == nil || ls.LivepeerNode.TurnkeyOrgID == "" {
		respondJsonError(ctx, w, errTurnkeyNotConfigured, http.StatusServiceUnavailable)
		return
	}
	var body turnkeyCreateAccountBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	wid := strings.TrimSpace(body.WalletID)
	if wid == "" {
		respondJsonError(ctx, w, errTurnkeyWalletIDRequired, http.StatusBadRequest)
		return
	}
	addr, err := eth.TurnkeyCreateWalletAccount(ls.TurnkeyAdmin, ls.LivepeerNode.TurnkeyOrgID, wid)
	if err != nil {
		clog.Errorf(ctx, "turnkey create account: %v", err)
		respondJsonError(ctx, w, err, http.StatusBadGateway)
		return
	}
	ls.refreshTurnkeyAddressBook(ctx)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"address": addr.Hex()})
}

type turnkeySelectAddressBody struct {
	Address string `json:"address"`
}

// TurnkeySelectAddressHTTP sets the default Turnkey signing address for this remote signer.
func (ls *LivepeerServer) TurnkeySelectAddressHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	if ls.LivepeerNode == nil || ls.LivepeerNode.TurnkeyAccount == nil {
		respondJsonError(ctx, w, errTurnkeyNotConfigured, http.StatusServiceUnavailable)
		return
	}
	var body turnkeySelectAddressBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJsonError(ctx, w, err, http.StatusBadRequest)
		return
	}
	addr := ethcommon.HexToAddress(strings.TrimSpace(body.Address))
	if addr == (ethcommon.Address{}) {
		respondJsonError(ctx, w, errTurnkeyAddressRequired, http.StatusBadRequest)
		return
	}
	if !ls.LivepeerNode.TurnkeySigningAddressAllowed(addr) {
		respondJsonError(ctx, w, errTurnkeyAddressNotAllowed, http.StatusBadRequest)
		return
	}
	ls.LivepeerNode.TurnkeyAccount.SetSigningAddress(addr)
	glog.Info("Turnkey default signing address set to ", addr.Hex())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"address": addr.Hex()})
}

func (ls *LivepeerServer) refreshTurnkeyAddressBook(ctx context.Context) {
	if ls.TurnkeyAdmin == nil || ls.LivepeerNode == nil || ls.LivepeerNode.TurnkeyOrgID == "" {
		return
	}
	accts, err := eth.ListTurnkeyEthereumAccounts(ls.TurnkeyAdmin, ls.LivepeerNode.TurnkeyOrgID)
	if err != nil {
		clog.Errorf(ctx, "refresh turnkey address book: %v", err)
		return
	}
	list := make([]ethcommon.Address, 0, len(accts))
	for _, a := range accts {
		list = append(list, a.Address)
	}
	ls.LivepeerNode.ReplaceTurnkeyAddressBook(list)
}

var (
	errTurnkeyNotConfigured      = errors.New("turnkey not configured on this node")
	errTurnkeyWalletNameRequired = errors.New("walletName is required")
	errTurnkeyWalletIDRequired   = errors.New("walletId is required")
	errTurnkeyAddressRequired    = errors.New("address is required")
	errTurnkeyAddressNotAllowed  = errors.New("address is not in the Turnkey address book")
)
