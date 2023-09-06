package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
)

func (w *wizard) currentRound() (*big.Int, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/currentRound", w.host, w.httpPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http response status %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(body), nil
}

func (w *wizard) initializeRound() {
	httpPost(fmt.Sprintf("http://%v:%v/initializeRound", w.host, w.httpPort))
}
