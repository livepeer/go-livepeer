package main

import (
	"fmt"
)

func (w *wizard) requestTokens() {
	httpGet(fmt.Sprintf("http://%v:%v/requestTokens", w.host, w.httpPort))
}
