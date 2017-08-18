package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestTranscodeReceiptHash(t *testing.T) {
	var (
		streamID      = "1"
		segmentNumber = big.NewInt(0)
		d0            = "QmR9BnJQisvevpCoSVWWKyownN58nydb2zQt9Z2VtnTnKe"
		tD0           = "0x42538602949f370aa331d2c07a1ee7ff26caac9cc676288f94b82eb2188b8465"
		bSig0         = common.FromHex("dd9778112dee59e9b584f3b29f9a932b2f7ebf8d353a0ea6eb4f825ad0c9af5547e379a1ff5bd9d1a946d8b70272f900022d8ddb38ac8e09c741702682faed5200")

		trHash = common.BytesToHash(common.FromHex("facd37361c8a9a9b90378edbcb4db7204ede8d92660048987fc9239ba10b26e0"))
	)

	tr := &TranscodeReceipt{
		streamID,
		segmentNumber,
		d0,
		tD0,
		bSig0,
	}

	if tr.Hash() != trHash {
		t.Fatalf("Invalid transcode receipt hash")
	}
}
