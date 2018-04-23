package http

import (
	"gx/ipfs/QmfAkMSt9Fwzk48QDJecPcwCUjnf2uG7MLnmCGTp4C6ouL/go-ipfs-cmds"
	"net/http"
)

type flushfwder struct {
	cmds.ResponseEmitter
	http.Flusher
}

func NewFlushForwarder(r cmds.ResponseEmitter, f http.Flusher) ResponseEmitter {
	return flushfwder{ResponseEmitter: r, Flusher: f}
}
