package main

import (
	"fmt"
)

func (w *wizard) currentRound() string {
	return httpGet(fmt.Sprintf("http://%v:%v/currentRound", w.host, w.httpPort))
}

func (w *wizard) initializeRound() {
	httpPost(fmt.Sprintf("http://%v:%v/initializeRound", w.host, w.httpPort))
}
