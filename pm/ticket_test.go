package pm

import (
	"math"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

func TestHash(t *testing.T) {
	exp := ethcommon.HexToHash("e1393fc7f6de093780674022f96cb8e3872235167d037c04d554e58c0e63d280")
	ticket := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       math.MaxUint32,
		RecipientRandHash: ethcommon.Hash{},
	}
	h := ticket.Hash()

	if h != exp {
		t.Errorf("Expected %v got %v", exp, h)
	}

	exp = ethcommon.HexToHash("ce0918ba94518293e9712effbe5fca4f1f431089833a5b8c257cb1e024595f68")
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

	exp = ethcommon.HexToHash("87ef13f2d37e4d5352a01d3c77b8179d80e0887f1953bfabfd0bfd7b0f689ddd")
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

	exp = ethcommon.HexToHash("aa4b35071043587992ac8b9dd1b2cf1d8311130e6458cf0b2342484e21af5f5b")
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

	exp = ethcommon.HexToHash("81e353ae39046e2b77b9f996abbaeed527fda559b61ef90f299c4499dd508ccc")
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

	exp = ethcommon.HexToHash("be5eaf9a49a39540b9bb02f1a21904f61324a29819e2c032824bfaa10c100b17")
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
