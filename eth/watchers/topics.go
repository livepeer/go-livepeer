package watchers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var eventSignatures = []string{
	"Unbond(address,address,uint256,uint256,uint256)",
	"Rebond(address,address,uint256,uint256)",
	"WithdrawStake(address,uint256,uint256,uint256)",
	"NewRound(uint256,bytes32)",
}

// FilterTopics returns a list of topics to be used when filtering logs
func FilterTopics() []common.Hash {
	topics := make([]common.Hash, len(eventSignatures))
	for i, sig := range eventSignatures {
		topics[i] = crypto.Keccak256Hash([]byte(sig))
	}

	return topics
}
