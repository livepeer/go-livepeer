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
	cr, ok := new(big.Int).SetString(string(body), 10)
	if !ok {
		return nil, fmt.Errorf("could not parse current round from response")
	}

	return cr, nil
}

func (w *wizard) initializeRound() {
	httpPost(fmt.Sprintf("http://%v:%v/initializeRound", w.host, w.httpPort))
}
