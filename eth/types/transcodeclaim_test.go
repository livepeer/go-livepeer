package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestTranscodeClaimHash(t *testing.T) {
	var (
		streamID              = big.NewInt(1)
		segmentSequenceNumber = big.NewInt(0)
		d0                    = common.BytesToHash(common.FromHex("80084bf2fba02475726feb2cab2d8215eab14bc6bdd8bfb2c8151257032ecd8b"))
		tD0                   = common.BytesToHash(common.FromHex("42538602949f370aa331d2c07a1ee7ff26caac9cc676288f94b82eb2188b8465"))
		bSig0                 = common.FromHex("dd9778112dee59e9b584f3b29f9a932b2f7ebf8d353a0ea6eb4f825ad0c9af5547e379a1ff5bd9d1a946d8b70272f900022d8ddb38ac8e09c741702682faed5200")

		tcHash = common.BytesToHash(common.FromHex("c5dc10e85c55b0819c9000a5c8a8df58f2e1c1077b8b930fb211e6a195508df8"))
	)

	tc := &TranscodeClaim{
		streamID,
		segmentSequenceNumber,
		d0,
		tD0,
		bSig0,
	}

	if tc.Hash() != tcHash {
		t.Fatalf("Invalid transcode claim hash")
	}
}
