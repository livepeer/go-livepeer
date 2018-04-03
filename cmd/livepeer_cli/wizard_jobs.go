package main

import (
	"fmt"
)

func (w *wizard) printLast5Jobs() {
	fmt.Print("Enter # of the latest jobs you would like to print ")
	c := w.readInt()

	fmt.Println(httpGet(fmt.Sprintf("http://%v:%v/latestJobs?count=%v", w.host, w.httpPort, c)))
	return
}
