package pm

import (
	"math"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

func hexToHash(in string) ethcommon.Hash {
	return ethcommon.BytesToHash(ethcommon.FromHex(in))
}
func TestHash(t *testing.T) {
	exp := hexToHash("24d4264d5ea56ba4362c8f535308005b2df1f9f77c34b4a61f6b5c3bef151b53")
	ticket := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       math.MaxUint64,
		RecipientRandHash: ethcommon.Hash{},
	}
	h := ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}

	exp = hexToHash("ce0918ba94518293e9712effbe5fca4f1f431089833a5b8c257cb1e024595f68")
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       1,
		RecipientRandHash: ethcommon.Hash{},
	}
	h = ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}

	exp = hexToHash("87ef13f2d37e4d5352a01d3c77b8179d80e0887f1953bfabfd0bfd7b0f689ddd")
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(1),
		WinProb:           big.NewInt(500),
		SenderNonce:       0,
		RecipientRandHash: ethcommon.Hash{},
	}
	h = ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}

	exp = hexToHash("aa4b35071043587992ac8b9dd1b2cf1d8311130e6458cf0b2342484e21af5f5b")
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(500),
		WinProb:           big.NewInt(1),
		SenderNonce:       0,
		RecipientRandHash: ethcommon.Hash{},
	}
	h = ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}

	exp = hexToHash("81e353ae39046e2b77b9f996abbaeed527fda559b61ef90f299c4499dd508ccc")
	ticket = &Ticket{
		Recipient:         ethcommon.HexToAddress("73AEd7b5dEb30222fa896f399d46cC99c7BEe57F"),
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: ethcommon.Hash{},
	}
	h = ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}

	var recipientRandHash [32]byte
	copy(recipientRandHash[:], ethcommon.FromHex("41b1a0649752af1b28b3dc29a1556eee781e4a4c3a1f7f53f90fa834de098c4d")[:32])

	exp = hexToHash("be5eaf9a49a39540b9bb02f1a21904f61324a29819e2c032824bfaa10c100b17")
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}
	h = ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}
}
