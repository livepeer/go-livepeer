package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestSegmentHash(t *testing.T) {
	var (
		streamID      = "1"
		segmentNumber = big.NewInt(0)
		d0            = "QmR9BnJQisvevpCoSVWWKyownN58nydb2zQt9Z2VtnTnKe"

		sHash = common.BytesToHash(common.FromHex("7fa493826bf6dc8fdd3e65ad6170193ec1a92cee5d78311953b3d0da928d7871"))
	)

	segment := &Segment{
		streamID,
		segmentNumber,
		d0,
	}

	if segment.Hash() != sHash {
		t.Fatalf("Invalid segment hash")
	}
}
