package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/log"
)

func (w *wizard) turnkeyBase() string {
	return fmt.Sprintf("http://%s:%s", w.host, w.httpPort)
}

func (w *wizard) turnkeyListWallets() {
	resp, err := http.Get(w.turnkeyBase() + "/turnkey/wallets")
	if err != nil {
		log.Error("Turnkey list wallets failed", "err", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Error("Turnkey list wallets", "status", resp.StatusCode, "body", string(body))
		return
	}
	fmt.Println(string(body))
}

func (w *wizard) turnkeyCreateWallet() {
	fmt.Println("Wallet name?")
	name := w.readString()
	payload, _ := json.Marshal(map[string]string{"walletName": name})
	resp, err := http.Post(w.turnkeyBase()+"/turnkey/create-wallet", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Error("Turnkey create wallet failed", "err", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Error("Turnkey create wallet", "status", resp.StatusCode, "body", string(body))
		return
	}
	fmt.Println(string(body))
}

func (w *wizard) turnkeyCreateAccount() {
	fmt.Println("Wallet ID?")
	wid := w.readString()
	payload, _ := json.Marshal(map[string]string{"walletId": wid})
	resp, err := http.Post(w.turnkeyBase()+"/turnkey/create-account", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Error("Turnkey create account failed", "err", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Error("Turnkey create account", "status", resp.StatusCode, "body", string(body))
		return
	}
	fmt.Println(string(body))
}

func (w *wizard) turnkeySelectAddress() {
	fmt.Println("Ethereum address (0x…)?")
	addr := w.readString()
	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}
	payload, _ := json.Marshal(map[string]string{"address": addr})
	resp, err := http.Post(w.turnkeyBase()+"/turnkey/select-address", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Error("Turnkey select address failed", "err", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Error("Turnkey select address", "status", resp.StatusCode, "body", string(body))
		return
	}
	fmt.Println(string(body))
}
