package core

import (
	"errors"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var ErrNotFound = errors.New("ErrNotFound")

// Broadcaster RPC interface implementation

type broadcaster struct {
	node *LivepeerNode
}

func (bcast *broadcaster) Sign(msg []byte) ([]byte, error) {
	if bcast.node == nil || bcast.node.Eth == nil {
		return []byte{}, nil
	}
	return bcast.node.Eth.Sign(crypto.Keccak256(msg))
}
func (bcast *broadcaster) Address() ethcommon.Address {
	if bcast.node == nil || bcast.node.Eth == nil {
		return ethcommon.Address{}
	}
	return bcast.node.Eth.Account().Address
}
func NewBroadcaster(node *LivepeerNode) *broadcaster {
	return &broadcaster{
		node: node,
	}
}
