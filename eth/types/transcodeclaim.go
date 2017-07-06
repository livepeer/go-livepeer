package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type TranscodeClaim struct {
	StreamID              string
	SegmentSequenceNumber *big.Int
	DataHash              common.Hash
	TranscodedDataHash    common.Hash
	BroadcasterSig        []byte
}

func (tc *TranscodeClaim) Hash() common.Hash {
	return crypto.Keccak256Hash([]byte(tc.StreamID), common.LeftPadBytes(tc.SegmentSequenceNumber.Bytes(), 32), tc.DataHash.Bytes(), tc.TranscodedDataHash.Bytes(), tc.BroadcasterSig)
}
