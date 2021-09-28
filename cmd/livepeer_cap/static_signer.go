package main

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
)

// A signer with an Ethereum address that always returns a configured static signature
// Implements common.Broadcaster
type staticSigner struct {
	addr ethcommon.Address
	sig  []byte
}

func newStaticSigner(addr ethcommon.Address, sig []byte) *staticSigner {
	return &staticSigner{
		addr: addr,
		sig:  sig,
	}
}

func (s *staticSigner) Address() ethcommon.Address {
	return s.addr
}

func (s *staticSigner) Sign(msg []byte) ([]byte, error) {
	return s.sig, nil
}
