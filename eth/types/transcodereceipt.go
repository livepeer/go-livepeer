package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type TranscodeReceipt struct {
	StreamID                 string
	SegmentSequenceNumber    *big.Int
	DataHash                 []byte
	ConcatTranscodedDataHash []byte
	BroadcasterSig           []byte
}

func (tc *TranscodeReceipt) Hash() common.Hash {
	return crypto.Keccak256Hash([]byte(tc.StreamID), common.LeftPadBytes(tc.SegmentSequenceNumber.Bytes(), 32), []byte(tc.DataHash), []byte(tc.ConcatTranscodedDataHash), tc.BroadcasterSig)
}
