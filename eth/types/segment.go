package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Segment struct {
	StreamID              *big.Int
	SegmentSequenceNumber *big.Int
	DataHash              common.Hash
}

func (s *Segment) Hash() common.Hash {
	return crypto.Keccak256Hash(common.LeftPadBytes(s.StreamID.Bytes(), 32), common.LeftPadBytes(s.SegmentSequenceNumber.Bytes(), 32), s.DataHash.Bytes())
}
